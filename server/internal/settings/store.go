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
	loaded, dropped, reason := parse(data)
	if reason != "" {
		aside := s.path + ".discarded-" + now().UTC().Format("20060102T150405Z")
		if err := os.Rename(s.path, aside); err != nil {
			return nil, fmt.Errorf("move aside %s: %w", s.path, err)
		}
		logger.Warn("settings file discarded, using defaults", "reason", reason, "movedTo", aside)
		return s, nil
	}
	if len(dropped) > 0 {
		logger.Warn("unavailable modules removed from layout", "keys", dropped)
	}
	s.cur = loaded
	return s, nil
}

// parse decodes data; a non-empty reason means the content is unusable.
// dropped lists layout keys removed because no such module exists any more.
//
// Unavailable module keys are dropped instead of failing validation: modules
// can disappear between versions (a deleted calendar section), and that must
// not throw away the whole file. Structural layout errors (duplicates, too
// many keys) still discard it.
func parse(data []byte) (st Settings, dropped []string, reason string) {
	// Decode on top of the defaults so a file written before a section
	// existed keeps working and the new sections get their defaults.
	st = Defaults()
	st.SchemaVersion = 0 // a file without a version must not inherit one
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&st); err != nil {
		return Settings{}, nil, "invalid JSON: " + err.Error()
	}
	if st.SchemaVersion != SchemaVersion {
		return Settings{}, nil, fmt.Sprintf("unsupported schemaVersion %d", st.SchemaVersion)
	}
	st = st.clone()
	st.Layout = st.Layout.Filtered(func(key string) bool {
		if st.ModuleAvailable(key) {
			return true
		}
		dropped = append(dropped, key)
		return false
	})
	if errs := st.Validate(); errs != nil {
		return Settings{}, nil, fmt.Sprintf("invalid content: %v", errs)
	}
	return st, dropped, ""
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur.clone()
}

// UpdateDisplay validates and stores new display settings. On validation
// errors (field names without the "display." prefix) nothing is written and
// the stored state is returned unchanged. The in-memory copy changes only
// after the file is safely written.
func (s *Store) UpdateDisplay(d DisplaySettings) (DisplaySettings, map[string]string, error) {
	cur, errs, err := s.update("display", func(next *Settings) map[string]string {
		next.Display = d
		return nil
	})
	return cur.Display, errs, err
}

// UpdateLayout validates and stores a new layout. Field errors are keyed by
// region name.
func (s *Store) UpdateLayout(l Layout) (Layout, map[string]string, error) {
	cur, errs, err := s.update("layout", func(next *Settings) map[string]string {
		next.Layout = l.normalized()
		return nil
	})
	return cur.Layout.normalized(), errs, err
}

// UpdateWeather merges u into the weather section (see WeatherUpdate for the
// secret rules), validates and stores it.
func (s *Store) UpdateWeather(u WeatherUpdate) (WeatherSettings, map[string]string, error) {
	cur, errs, err := s.update("weather", func(next *Settings) map[string]string {
		w, errs := u.apply(next.Weather)
		next.Weather = w
		return errs
	})
	return cur.Weather, errs, err
}

// UpdateWifi merges u into the Wi-Fi section (see WifiUpdate for the secret
// rules), validates and stores it.
func (s *Store) UpdateWifi(u WifiUpdate) (WifiSettings, map[string]string, error) {
	cur, errs, err := s.update("wifi", func(next *Settings) map[string]string {
		w, errs := u.apply(next.Wifi)
		next.Wifi = w
		return errs
	})
	return cur.Wifi, errs, err
}

// update applies mutate to a copy of the settings, validates it and writes
// it. It returns the state after the call (unchanged on field errors or I/O
// failure) and field errors with the section prefix removed.
func (s *Store) update(section string, mutate func(next *Settings) map[string]string) (Settings, map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cur.clone()
	if errs := mutate(&next); errs != nil {
		return s.cur, errs, nil
	}
	if errs := next.Validate(); errs != nil {
		fieldErrs := map[string]string{}
		for path, msg := range errs {
			// Only the edited section can be blamed for this request.
			if strings.HasPrefix(path, section+".") {
				fieldErrs[trimSection(path)] = msg
			}
		}
		if len(fieldErrs) > 0 {
			return s.cur, fieldErrs, nil
		}
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return s.cur, nil, fmt.Errorf("encode settings: %w", err)
	}
	if err := fsutil.WriteFileAtomic(s.path, append(data, '\n'), 0o600); err != nil {
		return s.cur, nil, fmt.Errorf("write %s: %w", s.path, err)
	}
	s.cur = next
	return next, nil, nil
}

func trimSection(path string) string {
	_, field, _ := strings.Cut(path, ".")
	return field
}
