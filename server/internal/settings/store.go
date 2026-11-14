package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/fsutil"
)

const fileName = "settings.json"

// Store keeps the settings in memory and persists them to <dir>/settings.json.
type Store struct {
	mu   sync.Mutex
	path string
	cur  Settings
	now  func() time.Time
}

// Load opens the store in dir. A missing file yields defaults without
// writing anything. A file that is not valid JSON, has another schema
// version or fails validation is moved aside (never deleted) and defaults
// are used, so bad content never stops the server. Only I/O failures are
// errors.
func Load(dir string, logger *slog.Logger) (*Store, error) {
	return load(dir, logger, time.Now)
}

func load(dir string, logger *slog.Logger, now func() time.Time) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{path: filepath.Join(dir, fileName), cur: Defaults(), now: now}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.path, err)
	}
	loaded, reason := parse(data)
	if reason != "" {
		aside := s.path + ".discarded-" + now().UTC().Format("20060102T150405Z")
		if err := os.Rename(s.path, aside); err != nil {
			return nil, fmt.Errorf("move aside %s: %w", s.path, err)
		}
		logger.Warn("settings file discarded, using defaults", "reason", reason, "movedTo", aside)
		return s, nil
	}
	s.cur = loaded
	return s, nil
}

// parse decodes data; a non-empty reason means the content is unusable.
func parse(data []byte) (Settings, string) {
	var st Settings
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&st); err != nil {
		return Settings{}, "invalid JSON: " + err.Error()
	}
	if st.SchemaVersion != SchemaVersion {
		return Settings{}, fmt.Sprintf("unsupported schemaVersion %d", st.SchemaVersion)
	}
	if errs := st.Validate(); errs != nil {
		return Settings{}, fmt.Sprintf("invalid content: %v", errs)
	}
	return st, ""
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// UpdateDisplay validates and stores new display settings. On validation
// errors (field names without the "display." prefix) nothing is written and
// the stored state is returned unchanged. The in-memory copy changes only
// after the file is safely written.
func (s *Store) UpdateDisplay(d DisplaySettings) (DisplaySettings, map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cur
	next.Display = d
	if errs := next.Validate(); errs != nil {
		fieldErrs := make(map[string]string, len(errs))
		for path, msg := range errs {
			fieldErrs[trimSection(path)] = msg
		}
		return s.cur.Display, fieldErrs, nil
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return s.cur.Display, nil, fmt.Errorf("encode settings: %w", err)
	}
	if err := fsutil.WriteFileAtomic(s.path, append(data, '\n'), 0o600); err != nil {
		return s.cur.Display, nil, fmt.Errorf("write %s: %w", s.path, err)
	}
	s.cur = next
	return next.Display, nil, nil
}

func trimSection(path string) string {
	_, field, _ := strings.Cut(path, ".")
	return field
}
