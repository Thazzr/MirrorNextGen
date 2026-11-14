package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const pw1 = "first-password-1"
const pw2 = "second-password-2"

func TestLoadEmptyDirIsNotConfigured(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data") // must be created
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsConfigured() {
		t.Error("empty dir must not be configured")
	}
	if err := s.Verify(pw1); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Verify = %v, want ErrNotConfigured", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("data dir not created: %v", err)
	}
}

func TestSetupPersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	if err := s.Setup(pw1); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "admin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", fi.Mode().Perm())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "admin.json"))
	if strings.Contains(string(data), pw1) || !strings.Contains(string(data), `"passwordHash":"$argon2id$`) {
		t.Errorf("unexpected file content %s", data)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}

	s2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.IsConfigured() {
		t.Fatal("reloaded store must be configured")
	}
	if err := s2.Verify(pw1); err != nil {
		t.Errorf("Verify right = %v", err)
	}
	if err := s2.Verify("not-the-password"); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("Verify wrong = %v", err)
	}
}

func TestSetupRules(t *testing.T) {
	s, _ := Load(t.TempDir())
	if err := s.Setup("short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short = %v", err)
	}
	if s.IsConfigured() {
		t.Fatal("a rejected password must not configure")
	}
	if err := s.Setup(pw1); err != nil {
		t.Fatal(err)
	}
	if err := s.Setup(pw2); !errors.Is(err, ErrAlreadyConfigured) {
		t.Errorf("second setup = %v", err)
	}
	// "already set up" outranks a bad password
	if err := s.Setup("x"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Errorf("second setup with short password = %v", err)
	}
	if err := s.Verify(pw1); err != nil {
		t.Errorf("second setup changed the password: %v", err)
	}
}

func TestLoadCorruptFileFails(t *testing.T) {
	tests := []struct{ name, content string }{
		{"garbage", "not json"},
		{"empty file", ""},
		{"no hash", `{}`},
		{"bad hash", `{"passwordHash":"plain"}`},
		{"empty hash", `{"passwordHash":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "admin.json"), []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(dir); err == nil {
				t.Fatal("Load must fail for a corrupt admin.json")
			}
		})
	}
}

func TestChange(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	if err := s.Change(pw1, pw2); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("change before setup = %v", err)
	}
	_ = s.Setup(pw1)
	if err := s.Change("wrong-password", pw2); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("wrong current = %v", err)
	}
	if err := s.Change(pw1, "short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("short new = %v", err)
	}
	if err := s.Verify(pw1); err != nil {
		t.Fatalf("failed changes altered the password: %v", err)
	}
	if err := s.Change(pw1, pw2); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(pw1); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("old password still valid: %v", err)
	}
	reloaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Verify(pw2); err != nil {
		t.Errorf("new password not persisted: %v", err)
	}
}

func TestSetupFailureKeepsStoreUnconfigured(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	// Make the data dir unwritable by replacing it with a file path.
	s.path = filepath.Join(dir, "missing-subdir", "admin.json")
	if err := s.Setup(pw1); err == nil {
		t.Fatal("expected a write error")
	}
	if s.IsConfigured() {
		t.Error("memory state must not change when the file could not be written")
	}
}

func TestConcurrentSetupExactlyOneWins(t *testing.T) {
	s, _ := Load(t.TempDir())
	var wins, already atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := s.Setup(pw1); {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, ErrAlreadyConfigured):
				already.Add(1)
			default:
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 || already.Load() != 7 {
		t.Fatalf("wins = %d, already = %d; want 1 and 7", wins.Load(), already.Load())
	}
}
