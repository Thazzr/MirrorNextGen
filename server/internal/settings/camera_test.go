package settings

import (
	"strings"
	"testing"
)

func TestCameraDefaults(t *testing.T) {
	want := CameraSettings{Title: "Kamera", Brightness: 100, Contrast: 100, Saturation: 100}
	if got := Defaults().Camera; got != want {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}
	if errs := Defaults().Validate(); errs != nil {
		t.Errorf("defaults invalid: %v", errs)
	}
	if Defaults().Camera.Configured() {
		t.Error("the default camera must not count as configured")
	}
}

func TestValidateCamera(t *testing.T) {
	ok := func(mod func(*CameraSettings)) CameraSettings {
		c := defaultCamera()
		mod(&c)
		return c
	}
	tests := []struct {
		name  string
		c     CameraSettings
		field string
		msg   string
	}{
		{"defaults", defaultCamera(), "", ""},
		{"empty title", ok(func(c *CameraSettings) { c.Title = "" }), "", ""},
		{"title 60", ok(func(c *CameraSettings) { c.Title = strings.Repeat("ä", 60) }), "", ""},
		{"title 61", ok(func(c *CameraSettings) { c.Title = strings.Repeat("a", 61) }), "camera.title", msgCamTitle},
		{"rtsp", ok(func(c *CameraSettings) { c.StreamURL = "rtsp://u:p@192.168.1.20:554/stream1" }), "", ""},
		{"rtsps upper case", ok(func(c *CameraSettings) { c.StreamURL = "RTSPS://cam.local/live" }), "", ""},
		{"http", ok(func(c *CameraSettings) { c.StreamURL = "http://cam.local/live" }), "camera.streamUrl", msgCamURLShape},
		{"exec", ok(func(c *CameraSettings) { c.StreamURL = "exec:ffmpeg -i x" }), "camera.streamUrl", msgCamURLShape},
		{"ffmpeg", ok(func(c *CameraSettings) { c.StreamURL = "ffmpeg:rtsp://cam/live#video=h264" }), "camera.streamUrl", msgCamURLShape},
		{"no host", ok(func(c *CameraSettings) { c.StreamURL = "rtsp:///stream" }), "camera.streamUrl", msgCamURLShape},
		{"space", ok(func(c *CameraSettings) { c.StreamURL = "rtsp://cam/a b" }), "camera.streamUrl", msgCamURLShape},
		{"newline", ok(func(c *CameraSettings) { c.StreamURL = "rtsp://cam/a\nb" }), "camera.streamUrl", msgCamURLShape},
		{"url 2000", ok(func(c *CameraSettings) { c.StreamURL = "rtsp://cam/" + strings.Repeat("a", 1989) }), "", ""},
		{"url 2001", ok(func(c *CameraSettings) { c.StreamURL = "rtsp://cam/" + strings.Repeat("a", 1990) }), "camera.streamUrl", msgCamURLLength},
		{"brightness 0", ok(func(c *CameraSettings) { c.Brightness = 0 }), "", ""},
		{"brightness 200", ok(func(c *CameraSettings) { c.Brightness = 200 }), "", ""},
		{"brightness 201", ok(func(c *CameraSettings) { c.Brightness = 201 }), "camera.brightness", msgCamPercent},
		{"brightness -1", ok(func(c *CameraSettings) { c.Brightness = -1 }), "camera.brightness", msgCamPercent},
		{"contrast 201", ok(func(c *CameraSettings) { c.Contrast = 201 }), "camera.contrast", msgCamPercent},
		{"saturation -1", ok(func(c *CameraSettings) { c.Saturation = -1 }), "camera.saturation", msgCamPercent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := Defaults()
			st.Camera = tt.c
			checkField(t, st.Validate(), tt.field, tt.msg)
		})
	}
}

func TestCameraConfigured(t *testing.T) {
	tests := []struct {
		enabled bool
		url     string
		want    bool
	}{{true, "rtsp://cam/x", true}, {true, "", false}, {false, "rtsp://cam/x", false}, {false, "", false}}
	for _, tt := range tests {
		if got := (CameraSettings{Enabled: tt.enabled, StreamURL: tt.url}).Configured(); got != tt.want {
			t.Errorf("enabled=%v url=%q: %v", tt.enabled, tt.url, got)
		}
	}
}

func TestCameraURLHintShowsSchemeAndHostOnly(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"rtsp://admin:geheim@192.168.1.20:554/stream1?token=abc", "rtsp://192.168.1.20"},
		{"rtsps://cam.example.org/live/ch0", "rtsps://cam.example.org"},
		{"", ""},
		{"not a url", ""},
	}
	for _, tt := range tests {
		got := CameraURLHint(tt.raw)
		if got != tt.want {
			t.Errorf("hint(%q) = %q, want %q", tt.raw, got, tt.want)
		}
		if strings.Contains(got, "geheim") || strings.Contains(got, "token") {
			t.Errorf("hint leaks a secret: %q", got)
		}
	}
}

func TestCameraSecretSemanticsAndPersistence(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	base := CameraUpdate{Enabled: true, Title: " Haustür ", StreamURL: " rtsp://u:p@cam/s ", Brightness: 120, Contrast: 100, Saturation: 80, Grayscale: true}
	got, errs, err := s.UpdateCamera(base)
	if err != nil || errs != nil {
		t.Fatal(errs, err)
	}
	if got.Title != "Haustür" || got.StreamURL != "rtsp://u:p@cam/s" || !got.Enabled || !got.Grayscale || got.Brightness != 120 || got.Saturation != 80 {
		t.Fatalf("stored %+v", got)
	}

	// An empty address keeps the stored one.
	keep := base
	keep.StreamURL, keep.Brightness = "", 90
	if got, _, _ = s.UpdateCamera(keep); got.StreamURL != "rtsp://u:p@cam/s" || got.Brightness != 90 {
		t.Errorf("keep: %+v", got)
	}
	// It survives a reload.
	s2, _ := Load(dir, quiet)
	if s2.Get().Camera.StreamURL != "rtsp://u:p@cam/s" {
		t.Errorf("reloaded: %+v", s2.Get().Camera)
	}
	// A new address and clear together are a field error and change nothing.
	both := base
	both.StreamURL, both.ClearStreamURL = "rtsp://other/x", true
	if _, errs, _ = s.UpdateCamera(both); errs["streamUrl"] != msgSecretConflict {
		t.Errorf("conflict not reported: %v", errs)
	}
	if s.Get().Camera.StreamURL != "rtsp://u:p@cam/s" {
		t.Error("conflict changed the address")
	}
	// A non-RTSP address is refused and nothing is stored.
	bad := base
	bad.StreamURL = "exec:rm -rf /"
	if _, errs, _ = s.UpdateCamera(bad); errs["streamUrl"] != msgCamURLShape {
		t.Errorf("bad scheme: %v", errs)
	}
	if s.Get().Camera.StreamURL != "rtsp://u:p@cam/s" {
		t.Error("a refused address was stored")
	}
	// Clear deletes it.
	clr := base
	clr.StreamURL, clr.ClearStreamURL = "", true
	if got, _, _ = s.UpdateCamera(clr); got.StreamURL != "" || got.Configured() {
		t.Errorf("clear: %+v", got)
	}
}

func TestCameraFieldErrorsCarryNoSectionPrefix(t *testing.T) {
	s, _ := Load(t.TempDir(), quiet)
	_, errs, _ := s.UpdateCamera(CameraUpdate{Title: strings.Repeat("x", 61), Brightness: 300, Contrast: 100, Saturation: 100})
	if len(errs) != 2 || errs["title"] == "" || errs["brightness"] == "" {
		t.Errorf("errs = %v", errs)
	}
	if s.Get().Camera != Defaults().Camera {
		t.Errorf("rejected update changed the state: %+v", s.Get().Camera)
	}
}

func TestFileWithoutCameraSectionGetsDefaults(t *testing.T) {
	st, _, reason := parse([]byte(`{"schemaVersion":1}`))
	if reason != "" {
		t.Fatal(reason)
	}
	if st.Camera != defaultCamera() {
		t.Errorf("camera = %+v", st.Camera)
	}
}

func TestCameraSectionIsPartOfEqual(t *testing.T) {
	a, b := Defaults(), Defaults()
	b.Camera.Title = "Flur"
	if a.Equal(b) {
		t.Error("Equal ignores the camera section")
	}
}
