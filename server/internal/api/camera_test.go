package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/camera"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

const (
	cameraPath     = "/api/v1/settings/camera"
	dashCameraPath = "/api/v1/dashboard/camera"
	cameraURL      = "rtsp://admin:geheim-passwort@192.168.1.20:554/stream1"
)

// fakeGo2RTC records the calls the camera service makes to the camera service.
type fakeGo2RTC struct {
	mu    sync.Mutex
	calls []string // "METHOD ?query"
}

func (f *fakeGo2RTC) got() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// newTestCamera returns a running camera service wired to a fake go2rtc.
func newTestCamera(t *testing.T, store *settings.Store) (*camera.Service, *fakeGo2RTC) {
	t.Helper()
	f := &fakeGo2RTC{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.RawQuery)
		f.mu.Unlock()
	}))
	svc, err := camera.New(camera.Options{
		HTTPClient: srv.Client(),
		BaseURL:    srv.URL,
		Config: func() camera.Config {
			c := store.Get().Camera
			return camera.Config{Enabled: c.Enabled, StreamURL: c.StreamURL}
		},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backoff: func(int) time.Duration { return time.Millisecond },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
		srv.Close()
	})
	return svc, f
}

func goodCameraUpdate() CameraSettingsUpdate {
	return CameraSettingsUpdate{Enabled: true, Title: "Haustür", StreamUrl: ptr(cameraURL), Brightness: 110, Contrast: 100, Saturation: 90, Grayscale: true}
}

func waitCalls(t *testing.T, f *fakeGo2RTC, n int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for len(f.got()) < n {
		if time.Now().After(deadline) {
			t.Fatalf("camera service calls = %v, want %d", f.got(), n)
		}
		time.Sleep(2 * time.Millisecond)
	}
	return f.got()
}

func TestCameraSettingsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := goodCameraUpdate()
	wantProblem(t, e.do(http.MethodGet, cameraPath, nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPut, cameraPath, body, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPut, cameraPath, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPut, cameraPath, body, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")
	got := decode[CameraSettings](t, e.do(http.MethodGet, cameraPath, nil, opts{cookie: c.cookie}))
	if got.Enabled || got.HasStreamUrl {
		t.Errorf("refused PUT changed the settings: %+v", got)
	}
}

func TestCameraSettingsDefaultsAndSecretSemantics(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)

	want := CameraSettings{Title: "Kamera", Brightness: 100, Contrast: 100, Saturation: 100}
	if got := decode[CameraSettings](t, e.do(http.MethodGet, cameraPath, nil, o)); got != want {
		t.Errorf("defaults = %+v", got)
	}

	rec := e.do(http.MethodPut, cameraPath, goodCameraUpdate(), o)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	want = CameraSettings{Enabled: true, Title: "Haustür", HasStreamUrl: true, StreamUrlHint: "rtsp://192.168.1.20", Brightness: 110, Contrast: 100, Saturation: 90, Grayscale: true}
	if got := decode[CameraSettings](t, rec); got != want {
		t.Errorf("PUT answered %+v", got)
	}
	get := e.do(http.MethodGet, cameraPath, nil, o)
	// The dashboard endpoint is public, so it must be clean too.
	dash := e.do(http.MethodGet, dashCameraPath, nil, opts{})
	for name, body := range map[string]string{"PUT": rec.Body.String(), "GET": get.Body.String(), "dashboard": dash.Body.String()} {
		for _, secret := range []string{"geheim", "admin:", "/stream1", "554", `"streamUrl"`} {
			if strings.Contains(body, secret) {
				t.Errorf("%s body leaks %q: %s", name, secret, body)
			}
		}
	}

	// Absent or empty address keeps it.
	for name, u := range map[string]*string{"absent": nil, "empty": ptr("")} {
		up := goodCameraUpdate()
		up.StreamUrl, up.Brightness = u, 80
		if got := decode[CameraSettings](t, e.do(http.MethodPut, cameraPath, up, o)); !got.HasStreamUrl || got.Brightness != 80 {
			t.Errorf("%s address: %+v", name, got)
		}
	}
	// A new address together with clear is a field error and changes nothing.
	both := goodCameraUpdate()
	both.ClearStreamUrl = ptr(true)
	p := wantProblem(t, e.do(http.MethodPut, cameraPath, both, o), http.StatusBadRequest, "")
	if p.Errors == nil || (*p.Errors)["streamUrl"] == "" {
		t.Errorf("conflict errors = %v", p.Errors)
	}
	if !decode[CameraSettings](t, e.do(http.MethodGet, cameraPath, nil, o)).HasStreamUrl {
		t.Error("conflict cleared the address")
	}
	// Clear deletes it.
	clr := goodCameraUpdate()
	clr.StreamUrl, clr.ClearStreamUrl = nil, ptr(true)
	if got := decode[CameraSettings](t, e.do(http.MethodPut, cameraPath, clr, o)); got.HasStreamUrl || got.StreamUrlHint != "" {
		t.Errorf("clear: %+v", got)
	}
}

func TestCameraSettingsRejectInvalidValuesAndStoreNothing(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	bad := goodCameraUpdate()
	bad.StreamUrl = ptr("http://192.168.1.20/video.mjpg")
	bad.Brightness = 500
	bad.Title = strings.Repeat("x", 61)
	p := wantProblem(t, e.do(http.MethodPut, cameraPath, bad, o), http.StatusBadRequest, "")
	for _, field := range []string{"streamUrl", "brightness", "title"} {
		if p.Errors == nil || (*p.Errors)[field] == "" {
			t.Errorf("no error for %s: %v", field, p.Errors)
		}
	}
	if got := decode[CameraSettings](t, e.do(http.MethodGet, cameraPath, nil, o)); got.Enabled || got.HasStreamUrl {
		t.Errorf("rejected update changed the state: %+v", got)
	}
	// Only the initial "nothing configured" delete reached the camera service.
	time.Sleep(20 * time.Millisecond)
	for _, c := range e.go2rtc.got() {
		if !strings.HasPrefix(c, "DELETE") {
			t.Errorf("camera service got %q for a rejected update", c)
		}
	}
}

func TestCameraSettingsRegisterTheStreamWithTheCameraService(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	waitCalls(t, e.go2rtc, 1) // start-up: nothing configured yet

	if rec := e.do(http.MethodPut, cameraPath, goodCameraUpdate(), o); rec.Code != http.StatusOK {
		t.Fatal(rec.Body)
	}
	calls := waitCalls(t, e.go2rtc, 2)
	want := "PUT name=mirror-camera&src=rtsp%3A%2F%2Fadmin%3Ageheim-passwort%40192.168.1.20%3A554%2Fstream1"
	if calls[1] != want {
		t.Errorf("registration = %q, want %q", calls[1], want)
	}

	off := goodCameraUpdate()
	off.Enabled, off.StreamUrl = false, nil
	if rec := e.do(http.MethodPut, cameraPath, off, o); rec.Code != http.StatusOK {
		t.Fatal(rec.Body)
	}
	calls = waitCalls(t, e.go2rtc, 3)
	if calls[2] != "DELETE src=mirror-camera" {
		t.Errorf("after switching off = %q", calls[2])
	}
}

func TestDashboardCameraIsPublicAndReportsConfigured(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	// Public: no cookie.
	got := decode[DashboardCamera](t, e.do(http.MethodGet, dashCameraPath, nil, opts{}))
	if got != (DashboardCamera{Title: "Kamera", Brightness: 100, Contrast: 100, Saturation: 100}) {
		t.Errorf("defaults = %+v", got)
	}

	tests := []struct {
		name string
		mod  func(*CameraSettingsUpdate)
		want bool
	}{
		{"enabled with address", func(u *CameraSettingsUpdate) {}, true},
		{"disabled", func(u *CameraSettingsUpdate) { u.Enabled = false }, false},
	}
	for _, tt := range tests {
		up := goodCameraUpdate()
		tt.mod(&up)
		if rec := e.do(http.MethodPut, cameraPath, up, o); rec.Code != http.StatusOK {
			t.Fatal(rec.Body)
		}
		got := decode[DashboardCamera](t, e.do(http.MethodGet, dashCameraPath, nil, opts{}))
		want := DashboardCamera{Configured: tt.want, Title: "Haustür", Brightness: 110, Contrast: 100, Saturation: 90, Grayscale: true}
		if got != want {
			t.Errorf("%s: %+v, want %+v", tt.name, got, want)
		}
	}
	// Enabled but without an address is not configured.
	clr := goodCameraUpdate()
	clr.StreamUrl, clr.ClearStreamUrl = nil, ptr(true)
	e.do(http.MethodPut, cameraPath, clr, o)
	if decode[DashboardCamera](t, e.do(http.MethodGet, dashCameraPath, nil, opts{})).Configured {
		t.Error("enabled without an address counts as configured")
	}
}

func TestStreamRouteIsOnlyRegisteredWithACameraService(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/api/v1/camera/stream", nil))
	if pattern != "" {
		t.Errorf("stream route registered without a camera service: %q", pattern)
	}
}

// The stream route is GET only: any other method is a 405 from the mux, not a
// call into the proxy.
func TestStreamRouteIsGetOnly(t *testing.T) {
	svc, err := camera.New(camera.Options{
		BaseURL: "http://camera:1984",
		Config:  func() camera.Config { return camera.Config{Enabled: true, StreamURL: "rtsp://cam/x"} },
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Register(mux, Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Camera: svc})
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/camera/stream", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s stream = %d, want 405", method, rec.Code)
		}
	}
}
