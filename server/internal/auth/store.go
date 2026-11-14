package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const adminFile = "admin.json"

// Errors of the credential store that handlers translate into answers.
var (
	ErrAlreadyConfigured = errors.New("admin access is already set up")
	ErrNotConfigured     = errors.New("admin access is not set up")
	ErrWrongPassword     = errors.New("wrong password")
)

type adminRecord struct {
	PasswordHash string `json:"passwordHash"`
}

// Store holds the admin password hash, persisted in <dir>/admin.json.
type Store struct {
	mu   sync.Mutex
	path string
	hash string // empty while not configured
}

// Load opens the store in dir, creating the directory if needed. A missing
// file means "not configured"; an unreadable or corrupt one is an error,
// because treating it as "not configured" would let anyone re-run the setup.
func Load(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{path: filepath.Join(dir, adminFile)}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.path, err)
	}
	var rec adminRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	// Probe the hash so a damaged value fails at startup, not at first login.
	if _, err := VerifyPassword("", rec.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid password hash in %s: %w", s.path, err)
	}
	s.hash = rec.PasswordHash
	return s, nil
}

// IsConfigured reports whether an admin password is set.
func (s *Store) IsConfigured() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hash != ""
}

// Setup sets the first password. Exactly one of several concurrent calls
// succeeds; the others get ErrAlreadyConfigured.
func (s *Store) Setup(pw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// "Already set up" outranks a bad password: the caller must learn that
	// first, whatever it sent.
	if s.hash != "" {
		return ErrAlreadyConfigured
	}
	if err := ValidatePassword(pw); err != nil {
		return err
	}
	return s.store(pw)
}

// Verify checks pw against the stored hash.
func (s *Store) Verify(pw string) error {
	s.mu.Lock()
	hash := s.hash
	s.mu.Unlock()
	if hash == "" {
		return ErrNotConfigured
	}
	ok, err := VerifyPassword(pw, hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ErrWrongPassword
	}
	return nil
}

// Change replaces the password after checking the current one.
func (s *Store) Change(current, next string) error {
	if err := ValidatePassword(next); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hash == "" {
		return ErrNotConfigured
	}
	ok, err := VerifyPassword(current, s.hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ErrWrongPassword
	}
	return s.store(next)
}

// store hashes, persists and then adopts pw; memory changes only after the
// file is safely written. Callers hold s.mu.
func (s *Store) store(pw string) error {
	hash, err := HashPassword(pw)
	if err != nil {
		return err
	}
	data, err := json.Marshal(adminRecord{PasswordHash: hash})
	if err != nil {
		return fmt.Errorf("encode admin record: %w", err)
	}
	if err := writeFileAtomic(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", s.path, err)
	}
	s.hash = hash
	return nil
}

// writeFileAtomic writes to a temp file in the same directory, syncs it and
// renames it over path, so a crash leaves either the old or the new file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
