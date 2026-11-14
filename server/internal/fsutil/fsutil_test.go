package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestWriteFileAtomicCreatesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.json")
	if err := WriteFileAtomic(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "two" {
		t.Errorf("content = %q", got)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", st.Mode().Perm())
	}
	if n := names(t, dir); len(n) != 1 {
		t.Errorf("leftover files: %v", n)
	}
}

func TestWriteFileAtomicFailureKeepsOldFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.json")
	if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A directory in the way makes the final rename fail.
	target := filepath.Join(dir, "dir")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(target, []byte("new"), 0o600); err == nil {
		t.Fatal("expected an error")
	}
	got, _ := os.ReadFile(p)
	if string(got) != "old" {
		t.Errorf("old file changed: %q", got)
	}
	for _, n := range names(t, dir) {
		if n != "f.json" && n != "dir" {
			t.Errorf("temp file left behind: %s", n)
		}
	}
	// Missing directory fails without creating anything.
	if err := WriteFileAtomic(filepath.Join(dir, "nope", "f"), []byte("x"), 0o600); err == nil {
		t.Error("expected an error for a missing directory")
	}
}
