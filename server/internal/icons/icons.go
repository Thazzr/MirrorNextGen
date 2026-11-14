// Package icons stores the icons the admin uploads for calendars, shifts and
// rules: <dir>/icons/<id>.svg|png plus an index.json with the metadata.
package icons

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/thazzr/mirrornextgen/server/internal/fsutil"
)

// Icon is the metadata of one stored icon.
type Icon struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

// idPattern is the shape of generated ids; it also guards file paths.
var idPattern = regexp.MustCompile(`^[a-z0-9]{16}$`)

// ValidID reports whether id has the shape of an icon id.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Store keeps the icons below dir/icons.
type Store struct {
	mu    sync.Mutex
	dir   string
	index []Icon
}

// Load opens the store in dataDir; a missing index is an empty store.
func Load(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "icons")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create icons dir: %w", err)
	}
	s := &Store{dir: dir, index: []Icon{}}
	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read icon index: %w", err)
	}
	if err := json.Unmarshal(data, &s.index); err != nil {
		return nil, fmt.Errorf("decode icon index: %w", err)
	}
	return s, nil
}

func extension(contentType string) string {
	if contentType == TypePNG {
		return ".png"
	}
	return ".svg"
}

func (s *Store) writeIndexLocked(next []Icon) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode icon index: %w", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(s.dir, "index.json"), append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write icon index: %w", err)
	}
	s.index = next
	return nil
}

// List returns the icons in upload order.
func (s *Store) List() []Icon {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Icon{}, s.index...)
}

// Exists reports whether an icon with this id is stored.
func (s *Store) Exists(id string) bool {
	_, ok := s.Get(id)
	return ok
}

// Get returns the metadata of one icon.
func (s *Store) Get(id string) (Icon, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ic := range s.index {
		if ic.ID == id {
			return ic, true
		}
	}
	return Icon{}, false
}

// File returns the bytes of an icon.
func (s *Store) File(id string) (Icon, []byte, bool, error) {
	ic, ok := s.Get(id)
	if !ok {
		return Icon{}, nil, false, nil
	}
	data, err := os.ReadFile(filepath.Join(s.dir, ic.ID+extension(ic.ContentType)))
	if err != nil {
		return Icon{}, nil, false, fmt.Errorf("read icon file: %w", err)
	}
	return ic, data, true, nil
}

// Add validates and stores an icon. name may be empty; defaultName (usually
// the file name) then supplies it. Upload errors are ErrTooLarge, ErrType,
// ErrUnsafe and ErrName.
func (s *Store) Add(name, defaultName string, data []byte) (Icon, error) {
	ct, err := Detect(data)
	if err != nil {
		return Icon{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		base := filepath.Base(strings.ReplaceAll(defaultName, "\\", "/"))
		name = strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	}
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		return Icon{}, ErrName
	}
	id, err := newID()
	if err != nil {
		return Icon{}, err
	}
	ic := Icon{ID: id, Name: name, ContentType: ct, Size: len(data)}

	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, id+extension(ct))
	if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return Icon{}, fmt.Errorf("write icon file: %w", err)
	}
	next := append(append([]Icon{}, s.index...), ic)
	if err := s.writeIndexLocked(next); err != nil {
		_ = os.Remove(path)
		return Icon{}, err
	}
	return ic, nil
}

// Delete removes an icon; it reports false when none has this id.
func (s *Store) Delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var next []Icon
	var found Icon
	for _, ic := range s.index {
		if ic.ID == id {
			found = ic
			continue
		}
		next = append(next, ic)
	}
	if found.ID == "" {
		return false, nil
	}
	if next == nil {
		next = []Icon{}
	}
	if err := s.writeIndexLocked(next); err != nil {
		return false, err
	}
	if err := os.Remove(filepath.Join(s.dir, found.ID+extension(found.ContentType))); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return true, fmt.Errorf("remove icon file: %w", err)
	}
	return true, nil
}

func newID() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random icon id: %w", err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}
