package crypto

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// identityFile is the name of the scrypt-encrypted file (in the repo root) that
// holds the X25519 secret key used to encrypt/decrypt notes.
const identityFile = "identity.age"

// EncryptScrypt encrypts plaintext with a passphrase (scrypt KDF). It is used
// only for the identity file and one-time migration of legacy notes.
func EncryptScrypt(plaintext []byte, passphrase string) ([]byte, error) {
	r, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, fmt.Errorf("create recipient: %w", err)
	}
	return encrypt(plaintext, r)
}

// DecryptScrypt decrypts a scrypt-encrypted payload with a passphrase. Used for
// the identity file and for reading legacy notes during migration.
func DecryptScrypt(ciphertext []byte, passphrase string) ([]byte, error) {
	id, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	return decrypt(ciphertext, id)
}

// Encrypt encrypts plaintext to an X25519 recipient. No scrypt KDF is run, so
// this is fast and is used for every note.
func Encrypt(plaintext []byte, recipient *age.X25519Recipient) ([]byte, error) {
	return encrypt(plaintext, recipient)
}

// Decrypt decrypts an X25519-encrypted note with the unlocked identity. Fast
// (no per-note scrypt KDF).
func Decrypt(ciphertext []byte, identity *age.X25519Identity) ([]byte, error) {
	return decrypt(ciphertext, identity)
}

func encrypt(plaintext []byte, recipient age.Recipient) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("create encryptor: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("finalize encrypt: %w", err)
	}
	return buf.Bytes(), nil
}

func decrypt(ciphertext []byte, identity age.Identity) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return io.ReadAll(r)
}

// IdentityPath returns the location of the scrypt-encrypted identity file.
func IdentityPath(repoPath string) string {
	return filepath.Join(repoPath, identityFile)
}

// LoadOrCreateIdentity unlocks the repo's X25519 identity using the passphrase,
// creating it on first use. The secret key is stored scrypt-encrypted at
// <repo>/identity.age, so the passphrase is only needed to derive the key once
// per startup (rather than once per note).
func LoadOrCreateIdentity(repoPath, passphrase string) (*age.X25519Identity, error) {
	path := IdentityPath(repoPath)
	data, err := os.ReadFile(path)
	if err == nil {
		plain, err := DecryptScrypt(data, passphrase)
		if err != nil {
			return nil, fmt.Errorf("unlock identity: %w", err)
		}
		id, err := age.ParseX25519Identity(string(bytes.TrimSpace(plain)))
		if err != nil {
			return nil, fmt.Errorf("parse identity: %w", err)
		}
		return id, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read identity: %w", err)
	}

	// First run: generate a new identity and persist it scrypt-encrypted.
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("generate identity: %w", err)
	}
	enc, err := EncryptScrypt([]byte(id.String()), passphrase)
	if err != nil {
		return nil, fmt.Errorf("encrypt identity: %w", err)
	}
	if err := os.WriteFile(path, enc, 0600); err != nil {
		return nil, fmt.Errorf("write identity: %w", err)
	}
	return id, nil
}
