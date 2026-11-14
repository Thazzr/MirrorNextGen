package settings

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func keys(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "clock" // content is irrelevant for the size check below
	}
	return out
}

func TestDefaultLayout(t *testing.T) {
	want := Layout{
		TopLeft: []string{"clock"}, TopRight: []string{"weather"}, Middle: []string{},
		BottomLeft: []string{}, BottomRight: []string{"wifi"},
	}
	if got := Defaults().Layout; !reflect.DeepEqual(got, want) {
		t.Errorf("default layout = %+v", got)
	}
	if errs := Defaults().Validate(); errs != nil {
		t.Errorf("defaults invalid: %v", errs)
	}
}

func TestLoadWithoutLayoutGetsDefault(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"display":{"rotation":90,"output":"","testPattern":false}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := load(dir, quiet, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if !reflect.DeepEqual(got.Layout, DefaultLayout()) || got.Display.Rotation != 90 {
		t.Errorf("loaded = %+v", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("file was moved aside: %v", entries)
	}
}

func TestValidateLayout(t *testing.T) {
	tests := []struct {
		name   string
		layout Layout
		want   map[string]string // region -> message; nil = valid
	}{
		{"all empty", Layout{}, nil},
		{"valid", Layout{TopLeft: []string{"clock"}, Middle: []string{"weather", "wifi"}}, nil},
		{"unknown key", Layout{TopRight: []string{"clock", "nope"}},
			map[string]string{"topRight": "Unbekanntes Modul „nope“."}},
		{"duplicate within region", Layout{Middle: []string{"clock", "clock"}},
			map[string]string{"middle": "„clock“ steht mehrfach in der Anordnung."}},
		{"duplicate across regions reports second region", Layout{TopLeft: []string{"wifi"}, BottomRight: []string{"wifi"}},
			map[string]string{"bottomRight": "„wifi“ steht mehrfach in der Anordnung."}},
		{"too many", Layout{Middle: keys(13)},
			map[string]string{"middle": "Höchstens 12 Module je Region."}},
		{"unknown and ok regions", Layout{TopLeft: []string{"x"}, BottomLeft: []string{"y"}, BottomRight: []string{"clock"}},
			map[string]string{"topLeft": "Unbekanntes Modul „x“.", "bottomLeft": "Unbekanntes Modul „y“."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := Defaults()
			st.Layout = tt.layout
			errs := map[string]string{}
			for k, v := range st.Validate() {
				if strings.HasPrefix(k, "layout.") {
					errs[strings.TrimPrefix(k, "layout.")] = v
				}
			}
			if tt.want == nil {
				if len(errs) != 0 {
					t.Errorf("errs = %v", errs)
				}
				return
			}
			if !reflect.DeepEqual(errs, tt.want) {
				t.Errorf("errs = %v, want %v", errs, tt.want)
			}
		})
	}
}

func TestTwelveModulesAreAllowedThirteenAreNot(t *testing.T) {
	// Twelve distinct keys do not exist yet, so the limit is probed with a
	// region of duplicates: only the size message may be the first error.
	st := Defaults()
	st.Layout = Layout{Middle: keys(12)}
	if msg := st.Validate()["layout.middle"]; strings.Contains(msg, "Höchstens") {
		t.Errorf("12 keys rejected as too many: %q", msg)
	}
	st.Layout = Layout{Middle: keys(13)}
	if msg := st.Validate()["layout.middle"]; msg != msgMaxModules {
		t.Errorf("13 keys: %q", msg)
	}
}

func TestUpdateLayoutRoundTripAndErrors(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	want := Layout{TopLeft: []string{"weather", "clock"}, TopRight: []string{}, Middle: []string{"wifi"}, BottomLeft: []string{}, BottomRight: []string{}}
	got, errs, err := s.UpdateLayout(want)
	if err != nil || errs != nil {
		t.Fatalf("update: %v %v", errs, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("answered %+v", got)
	}
	s2, _ := Load(dir, quiet)
	if !reflect.DeepEqual(s2.Get().Layout, want) {
		t.Errorf("reloaded %+v", s2.Get().Layout)
	}

	// nil regions are stored as [], never as null.
	if _, _, err := s.UpdateLayout(Layout{TopLeft: []string{"clock"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if bytes.Contains(data, []byte("null")) {
		t.Errorf("null in file: %s", data)
	}

	before := s.Get().Layout
	bad, errs, err := s.UpdateLayout(Layout{TopLeft: []string{"nope"}})
	if err != nil || errs["topLeft"] != "Unbekanntes Modul „nope“." || len(errs) != 1 {
		t.Fatalf("errs = %v, err = %v", errs, err)
	}
	if !reflect.DeepEqual(bad, before) || !reflect.DeepEqual(s.Get().Layout, before) {
		t.Error("state changed on error")
	}
}

func TestGetReturnsIndependentLayoutCopy(t *testing.T) {
	s, _ := Load(t.TempDir(), quiet)
	s.Get().Layout.TopLeft[0] = "mutated"
	if got := s.Get().Layout.TopLeft[0]; got != "clock" {
		t.Errorf("stored layout changed through Get: %q", got)
	}
}

// Modules can disappear between versions, so a stored key that is merely
// unavailable is dropped with a warning; the rest of the file is kept.
func TestLoadDropsUnavailableKeysButKeepsFile(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"display":{"rotation":180,"output":"","testPattern":false},
"layout":{"topLeft":["clock","calendar:gone"],"topRight":[],"middle":["weather"],"bottomLeft":[],"bottomRight":["wifi"]}}`
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var logBuf bytes.Buffer
	s, err := load(dir, slog.New(slog.NewTextHandler(&logBuf, nil)), fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if !reflect.DeepEqual(got.Layout.TopLeft, []string{"clock"}) || got.Display.Rotation != 180 {
		t.Errorf("loaded = %+v", got)
	}
	if !strings.Contains(logBuf.String(), "calendar:gone") {
		t.Errorf("no warning logged: %s", logBuf.String())
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file moved aside: %v", err)
	}
}

// Structural layout errors still discard the file.
func TestLoadDiscardsDuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"layout":{"topLeft":["clock"],"topRight":["clock"],"middle":[],"bottomLeft":[],"bottomRight":[]}}`
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := load(dir, quiet, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Get().Equal(Defaults()) {
		t.Errorf("got %+v, want defaults", s.Get())
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("original still there")
	}
}
