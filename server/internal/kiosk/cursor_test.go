package kiosk

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteInvisibleCursorTheme(t *testing.T) {
	dir := t.TempDir()
	if err := WriteInvisibleCursorTheme(dir); err != nil {
		t.Fatal(err)
	}
	cursors := filepath.Join(dir, "default", "cursors")
	data, err := os.ReadFile(filepath.Join(cursors, "default"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "Xcur" {
		t.Fatalf("magic %q", data[:4])
	}
	u := func(off int) uint32 { return binary.LittleEndian.Uint32(data[off:]) }
	if len(data) != 28+36+4 {
		t.Fatalf("length %d", len(data))
	}
	header := []struct {
		name string
		off  int
		want uint32
	}{
		{"header size", 4, 16}, {"version", 8, 0x00010000}, {"ntoc", 12, 1},
		{"toc type", 16, 0xfffd0002}, {"toc subtype", 20, 24}, {"toc position", 24, 28},
		{"chunk header", 28, 36}, {"chunk type", 32, 0xfffd0002}, {"chunk subtype", 36, 24},
		{"chunk version", 40, 1}, {"width", 44, 1}, {"height", 48, 1},
		{"xhot", 52, 0}, {"yhot", 56, 0}, {"delay", 60, 0}, {"pixel", 64, 0},
	}
	for _, h := range header {
		if got := u(h.off); got != h.want {
			t.Errorf("%s: got %#x, want %#x", h.name, got, h.want)
		}
	}
	other, err := os.ReadFile(filepath.Join(cursors, "left_ptr"))
	if err != nil || !bytes.Equal(other, data) {
		t.Errorf("left_ptr: %v, same content: %v", err, bytes.Equal(other, data))
	}
	idx, err := os.ReadFile(filepath.Join(dir, "default", "index.theme"))
	if err != nil || !strings.Contains(string(idx), "[Icon Theme]") || !strings.Contains(string(idx), "Name=default") {
		t.Errorf("index.theme: %q, %v", idx, err)
	}
}

func TestWriteInvisibleCursorThemeFailsOnBadDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteInvisibleCursorTheme(filepath.Join(file, "sub")); err == nil {
		t.Fatal("want an error when dir is below a file")
	}
}

func TestChromiumPolicyFile(t *testing.T) {
	raw, err := os.ReadFile("../../../kiosk/chromium-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var policy map[string]any
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if v, ok := policy["TranslateEnabled"]; !ok || v != false {
		t.Errorf("TranslateEnabled = %v (present %v), want false", v, ok)
	}
}

func TestDockerfileInstallsPolicy(t *testing.T) {
	raw, err := os.ReadFile("../../../kiosk/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "/etc/chromium/policies/managed/") {
		t.Error("kiosk/Dockerfile does not copy the policy to /etc/chromium/policies/managed/")
	}
}
