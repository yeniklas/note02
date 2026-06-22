package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yeniklas/note02/internal/crypto"
	"github.com/yeniklas/note02/internal/model"
)

type Store struct {
	repoPath   string
	passphrase string
}

func New(repoPath, passphrase string) *Store {
	return &Store{repoPath: repoPath, passphrase: passphrase}
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
	plain, err := crypto.Decrypt(data, s.passphrase)
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
	enc, err := crypto.Encrypt(data, s.passphrase)
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

func (s *Store) Delete(id string) error {
	if err := os.Remove(s.notePath(id)); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}
