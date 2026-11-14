package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/api"
	"github.com/thazzr/mirrornextgen/server/internal/auth"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

const indexHTML = "<!doctype html><title>SPA</title>"

func newHandler(t *testing.T, files fstest.MapFS) http.Handler {
	t.Helper()
	admin, err := auth.Load(t.TempDir())
	if err != nil {
		panic(err)
	}
	st, err := settings.Load(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	return New(api.Deps{
		Build:    api.BuildInfo{Version: "dev", Commit: "unknown"},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Admin:    admin,
		Sessions: auth.NewSessions(time.Now),
		Throttle: auth.NewThrottle(time.Now),
		Settings: st,
	}, files)
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestUnknownAPIPathIsJSON404NotIndex(t *testing.T) {
	h := newHandler(t, fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})

	for _, path := range []string{"/api/v1/nope", "/api/", "/api/v2/health"} {
		t.Run(path, func(t *testing.T) {
			rec := get(h, path)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type = %q", ct)
			}
			if strings.Contains(rec.Body.String(), "SPA") {
				t.Error("answered with index.html")
			}
			var p api.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Title != "Not Found" || p.Status != 404 {
				t.Errorf("problem = %+v", p)
			}
		})
	}
}

func TestHealthReachableThroughComposedHandler(t *testing.T) {
	h := newHandler(t, fstest.MapFS{})
	rec := get(h, "/api/v1/health")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestNoSniffOnEveryResponse(t *testing.T) {
	h := newHandler(t, fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})
	for _, path := range []string{"/api/v1/health", "/api/v1/nope", "/", "/admin/settings", "/missing.js"} {
		t.Run(path, func(t *testing.T) {
			if got := get(h, path).Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
		})
	}
}

func TestWebServedForOtherPaths(t *testing.T) {
	h := newHandler(t, fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})
	rec := get(h, "/admin/settings")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SPA") {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestProtectedEndpointNeedsSessionThroughComposedHandler(t *testing.T) {
	h := newHandler(t, fstest.MapFS{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAPIAnswersAreNoStoreButWebAssetsAreNot(t *testing.T) {
	h := newHandler(t, fstest.MapFS{"index.html": {Data: []byte(indexHTML)}, "assets/app.js": {Data: []byte("x")}, "assets/app-AbCd1234.js": {Data: []byte("y")}})
	for _, path := range []string{"/api/v1/health", "/api/v1/auth/session", "/api/v1/kiosk/state", "/api/v1/settings/display", "/api/v1/nope"} {
		rec := get(h, path)
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s (%d): Cache-Control = %q", path, rec.Code, got)
		}
	}
	// 200, 401 and the JSON 404 were all covered above.
	if get(h, "/api/v1/settings/display").Code != http.StatusUnauthorized || get(h, "/api/v1/nope").Code != http.StatusNotFound {
		t.Error("expected a 401 and a 404 among the checked paths")
	}
	if got := get(h, "/assets/app-AbCd1234.js").Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("hashed asset Cache-Control = %q", got)
	}
	for _, path := range []string{"/", "/admin/display"} {
		if got := get(h, path).Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", path, got)
		}
	}
}
