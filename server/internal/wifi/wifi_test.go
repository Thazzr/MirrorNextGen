package wifi

import (
	"slices"
	"strings"
	"testing"
)

func TestPayload(t *testing.T) {
	tests := []struct {
		name, ssid, security, password, want string
	}{
		{"wpa", "Home", WPA, "secret123", `WIFI:T:WPA;S:Home;P:secret123;;`},
		{"escapes all specials", `a\b;c,d:e"f`, WPA, `p\;,:"x`, `WIFI:T:WPA;S:a\\b\;c\,d\:e\"f;P:p\\\;\,\:\"x;;`},
		{"open network has no password", "Cafe", NoPass, "ignored", `WIFI:T:nopass;S:Cafe;;`},
		{"open network escapes the ssid", "Cafe;1", NoPass, "", `WIFI:T:nopass;S:Cafe\;1;;`},
		{"unicode stays", "Küche", WPA, "geheim!!", `WIFI:T:WPA;S:Küche;P:geheim!!;;`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Payload(tt.ssid, tt.security, tt.password); got != tt.want {
				t.Errorf("Payload = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeMatrix(t *testing.T) {
	m, err := Encode(Payload("Home", WPA, "secret123"))
	if err != nil {
		t.Fatal(err)
	}
	// Version 1 is 21 modules; the payload needs a larger one, and every
	// version adds 4 modules per side.
	if m.Size < 25 || (m.Size-17)%4 != 0 {
		t.Errorf("size %d is not a QR version size", m.Size)
	}
	if len(m.Rows) != m.Size {
		t.Fatalf("%d rows for size %d", len(m.Rows), m.Size)
	}
	for i, r := range m.Rows {
		if len(r) != m.Size || strings.Trim(r, "01") != "" {
			t.Fatalf("row %d is not %d chars of 0/1: %q", i, m.Size, r)
		}
	}

	// The three finder patterns are 7x7: dark ring, light ring, 3x3 dark core.
	finder := []string{"1111111", "1000001", "1011101", "1011101", "1011101", "1000001", "1111111"}
	for _, o := range [][2]int{{0, 0}, {m.Size - 7, 0}, {0, m.Size - 7}} {
		for dy, want := range finder {
			if got := m.Rows[o[1]+dy][o[0] : o[0]+7]; got != want {
				t.Errorf("finder at %v row %d = %s, want %s", o, dy, got, want)
			}
		}
	}
	// The fourth corner has no finder pattern.
	if got := m.Rows[m.Size-7][m.Size-7 : m.Size]; got == finder[0] {
		t.Errorf("bottom-right corner looks like a finder pattern: %s", got)
	}
}

func TestEncodeIsDeterministicAndDependsOnInput(t *testing.T) {
	a, _ := Encode(Payload("Home", WPA, "secret123"))
	b, _ := Encode(Payload("Home", WPA, "secret123"))
	c, _ := Encode(Payload("Home", WPA, "secret124"))
	if !slices.Equal(a.Rows, b.Rows) {
		t.Error("same input gave different matrices")
	}
	if slices.Equal(a.Rows, c.Rows) {
		t.Error("different input gave the same matrix")
	}
}

func TestEncodeUsesErrorCorrectionQ(t *testing.T) {
	// At level Q a 30-byte payload needs version 3 (29 modules); level L
	// would fit version 2. The size therefore proves the level.
	text := strings.Repeat("a", 30)
	m, err := Encode(text)
	if err != nil {
		t.Fatal(err)
	}
	if m.Size != 29 {
		t.Errorf("size %d, want 29 (version 3 at level Q)", m.Size)
	}
}
