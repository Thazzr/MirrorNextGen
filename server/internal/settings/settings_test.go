package settings

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

var fixedNow = func() time.Time { return time.Date(2026, 11, 14, 12, 30, 45, 0, time.UTC) }

func TestValidate(t *testing.T) {
	long33 := strings.Repeat("a", 33)
	tests := []struct {
		name       string
		d          DisplaySettings
		wantField  string // "" = valid
		wantAbsent string
	}{
		{"rot 0", DisplaySettings{Rotation: 0}, "", ""},
		{"rot 90", DisplaySettings{Rotation: 90}, "", ""},
		{"rot 180", DisplaySettings{Rotation: 180}, "", ""},
		{"rot 270", DisplaySettings{Rotation: 270}, "", ""},
		{"rot 45", DisplaySettings{Rotation: 45}, "display.rotation", "display.output"},
		{"rot -90", DisplaySettings{Rotation: -90}, "display.rotation", ""},
		{"rot 360", DisplaySettings{Rotation: 360}, "display.rotation", ""},
		{"output empty", DisplaySettings{Output: ""}, "", ""},
		{"output HDMI", DisplaySettings{Output: "HDMI-A-1"}, "", ""},
		{"output 32", DisplaySettings{Output: strings.Repeat("a", 32)}, "", ""},
		{"output 33", DisplaySettings{Output: long33}, "display.output", "display.rotation"},
		{"output space", DisplaySettings{Output: "HDMI A 1"}, "display.output", ""},
		{"output injection", DisplaySettings{Output: "a;rm"}, "display.output", ""},
		{"output umlaut", DisplaySettings{Output: "äö"}, "display.output", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Settings{SchemaVersion: 1, Display: tt.d}.Validate()
			if tt.wantField == "" {
				if errs != nil {
					t.Fatalf("errs = %v", errs)
				}
				return
			}
			msg, ok := errs[tt.wantField]
			if !ok || msg == "" {
				t.Fatalf("errs = %v, want %s", errs, tt.wantField)
			}
			if tt.wantField == "display.rotation" && msg != "Erlaubt sind 0°, 90°, 180° und 270°." {
				t.Errorf("message %q", msg)
			}
			if tt.wantField == "display.output" && !strings.HasPrefix(msg, "Erlaubt sind 1 bis 32 Zeichen") {
				t.Errorf("message %q", msg)
			}
			if _, bad := errs[tt.wantAbsent]; tt.wantAbsent != "" && bad {
				t.Errorf("unexpected %s in %v", tt.wantAbsent, errs)
			}
		})
	}
}

func TestLoadMissingFileGivesDefaultsAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get(); got != Defaults() || got.Display.Rotation != 0 || got.SchemaVersion != 1 {
		t.Errorf("got %+v", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("files created: %v", entries)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	want := DisplaySettings{Rotation: 90, Output: "HDMI-A-1", TestPattern: true}
	got, errs, err := s.UpdateDisplay(want)
	if err != nil || errs != nil || got != want {
		t.Fatalf("got %+v %v %v", got, errs, err)
	}
	if s.Get().Display != want {
		t.Errorf("memory = %+v", s.Get().Display)
	}
	st, err := os.Stat(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", st.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("leftovers: %v", entries)
	}
	s2, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Get().Display != want || s2.Get().SchemaVersion != 1 {
		t.Errorf("reloaded = %+v", s2.Get())
	}
}

func TestBadFilesAreMovedAside(t *testing.T) {
	tests := []struct{ name, content string }{
		{"invalid json", "{garbage"},
		{"empty file", ""},
		{"schema 2", `{"schemaVersion":2,"display":{"rotation":90,"output":"","testPattern":false}}`},
		{"missing schema", `{"display":{"rotation":90,"output":"","testPattern":false}}`},
		{"invalid content", `{"schemaVersion":1,"display":{"rotation":45,"output":"","testPattern":false}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(p, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			s, err := load(dir, quiet, fixedNow)
			if err != nil {
				t.Fatal(err)
			}
			if s.Get() != Defaults() {
				t.Errorf("got %+v, want defaults", s.Get())
			}
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("original still there: %v", err)
			}
			aside, err := os.ReadFile(p + ".discarded-20261114T123045Z")
			if err != nil {
				t.Fatalf("discarded file: %v", err)
			}
			if string(aside) != tt.content {
				t.Errorf("bytes changed: %q", aside)
			}
		})
	}
}

func TestFailedValidationWritesNothing(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	good := DisplaySettings{Rotation: 180}
	if _, _, err := s.UpdateDisplay(good); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "settings.json"))

	got, errs, err := s.UpdateDisplay(DisplaySettings{Rotation: 45, Output: "a;b"})
	if err != nil {
		t.Fatal(err)
	}
	if errs["rotation"] == "" || errs["output"] == "" || len(errs) != 2 {
		t.Errorf("errs = %v (keys must have no section prefix)", errs)
	}
	if got != good || s.Get().Display != good {
		t.Errorf("state changed: %+v", got)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if string(before) != string(after) {
		t.Error("file changed")
	}
}

func TestFailedValidationOnFreshStoreCreatesNoFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	if _, errs, _ := s.UpdateDisplay(DisplaySettings{Rotation: 1}); errs == nil {
		t.Fatal("expected errors")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("files created: %v", entries)
	}
}

func TestConcurrentUpdates(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	rots := []int{0, 90, 180, 270}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, errs, err := s.UpdateDisplay(DisplaySettings{Rotation: rots[i%4], TestPattern: i%2 == 0}); err != nil || errs != nil {
				t.Errorf("update: %v %v", errs, err)
			}
			_ = s.Get()
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, reason := parse(data)
	if reason != "" {
		t.Fatalf("file not a complete valid version: %s", reason)
	}
	if loaded != s.Get() {
		t.Errorf("file %+v != memory %+v", loaded, s.Get())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("leftovers: %v", entries)
	}
}

func TestDiscardedNameUsesUTC(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte("{garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	cet := time.FixedZone("CET", 3600)
	now := func() time.Time { return time.Date(2026, 11, 14, 13, 30, 45, 0, cet) }
	if _, err := load(dir, quiet, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p + ".discarded-20261114T123045Z"); err != nil {
		t.Errorf("expected a UTC-named discarded file: %v", err)
	}
}

func TestFailedWriteKeepsOldState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	old := DisplaySettings{Rotation: 90}
	if _, _, err := s.UpdateDisplay(old); err != nil {
		t.Fatal(err)
	}
	// Removing the directory makes the write fail even for root.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, errs, err := s.UpdateDisplay(DisplaySettings{Rotation: 270, TestPattern: true}); err == nil || errs != nil {
		t.Fatalf("err = %v, errs = %v; want a write error", err, errs)
	}
	if got := s.Get().Display; got != old {
		t.Errorf("memory changed despite failed write: %+v", got)
	}
}
