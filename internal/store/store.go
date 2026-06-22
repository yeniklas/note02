package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/google/uuid"
	"github.com/yeniklas/note02/internal/crypto"
	"github.com/yeniklas/note02/internal/model"
)

type Store struct {
	repoPath  string
	identity  *age.X25519Identity
	recipient *age.X25519Recipient
}

func New(repoPath string, identity *age.X25519Identity) *Store {
	return &Store{
		repoPath:  repoPath,
		identity:  identity,
		recipient: identity.Recipient(),
	}
}

func (s *Store) RepoPath() string { return s.repoPath }

func (s *Store) notesDir() string {
	return filepath.Join(s.repoPath, "notes")
}

func (s *Store) notePath(id string) string {
	return filepath.Join(s.notesDir(), id+".age")
}

// ListIDs returns the IDs of all stored notes without decrypting them. It is
// cheap (a single directory scan) and is used to learn the note count up front,
// e.g. to drive a loading progress bar.
func (s *Store) ListIDs() ([]string, error) {
	entries, err := os.ReadDir(s.notesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read notes dir: %w", err)
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".age") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".age"))
	}
	return ids, nil
}

func (s *Store) List() ([]model.Note, error) {
	ids, err := s.ListIDs()
	if err != nil {
		return nil, err
	}

	var notes []model.Note
	for _, id := range ids {
		note, err := s.Read(id)
		if err != nil {
			return nil, fmt.Errorf("read note %s: %w", id, err)
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})
	return notes, nil
}

// Read decrypts and returns a single note by ID.
func (s *Store) Read(id string) (model.Note, error) {
	note, err := s.read(id)
	if err != nil {
		return model.Note{}, err
	}
	return *note, nil
}

func (s *Store) read(id string) (*model.Note, error) {
	data, err := os.ReadFile(s.notePath(id))
	if err != nil {
		return nil, err
	}
	plain, err := crypto.Decrypt(data, s.identity)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var note model.Note
	if err := json.Unmarshal(plain, &note); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &note, nil
}

func (s *Store) write(note *model.Note) error {
	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	enc, err := crypto.Encrypt(data, s.recipient)
	if err != nil {
		return err
	}
	return os.WriteFile(s.notePath(note.ID), enc, 0600)
}

func (s *Store) Create(note model.Note) (model.Note, error) {
	note.ID = uuid.New().String()
	now := time.Now().UTC()
	note.CreatedAt = now
	note.UpdatedAt = now

	if err := s.write(&note); err != nil {
		return model.Note{}, fmt.Errorf("write: %w", err)
	}
	return note, nil
}

// Import writes a note preserving its ID and timestamps (filling sensible
// defaults if unset). Used by one-off migration tooling.
func (s *Store) Import(note model.Note) (model.Note, error) {
	if note.ID == "" {
		note.ID = uuid.New().String()
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now().UTC()
	}
	if note.UpdatedAt.IsZero() {
		note.UpdatedAt = note.CreatedAt
	}
	if err := s.write(&note); err != nil {
		return model.Note{}, fmt.Errorf("write: %w", err)
	}
	return note, nil
}

func (s *Store) Update(note model.Note) error {
	note.UpdatedAt = time.Now().UTC()
	if err := s.write(&note); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// MigrateToIdentity re-encrypts any legacy scrypt-encrypted notes to the X25519
// identity so that subsequent reads don't run a per-note scrypt KDF. It is
// idempotent: notes already encrypted to the identity are detected cheaply (from
// the age header) and skipped, so an interrupted migration simply resumes on the
// next run. Returns the number of notes re-encrypted.
func MigrateToIdentity(repoPath, passphrase string, identity *age.X25519Identity) (int, error) {
	notesDir := filepath.Join(repoPath, "notes")
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read notes dir: %w", err)
	}

	recipient := identity.Recipient()
	migrated := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".age") {
			continue
		}
		path := filepath.Join(notesDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return migrated, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if !isScryptEncrypted(data) {
			continue
		}
		plain, err := crypto.DecryptScrypt(data, passphrase)
		if err != nil {
			return migrated, fmt.Errorf("decrypt %s: %w", e.Name(), err)
		}
		enc, err := crypto.Encrypt(plain, recipient)
		if err != nil {
			return migrated, fmt.Errorf("re-encrypt %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(path, enc, 0600); err != nil {
			return migrated, fmt.Errorf("write %s: %w", e.Name(), err)
		}
		migrated++
	}
	return migrated, nil
}

// isScryptEncrypted reports whether an age file uses a scrypt recipient stanza.
// Recipient stanzas appear in the plaintext header (before the "---" HMAC line),
// so this is a cheap header inspection with no decryption.
func isScryptEncrypted(data []byte) bool {
	header := data
	if i := strings.Index(string(data), "\n---"); i >= 0 {
		header = data[:i]
	}
	return strings.Contains(string(header), "-> scrypt")
}

func (s *Store) Delete(id string) error {
	if err := os.Remove(s.notePath(id)); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}
