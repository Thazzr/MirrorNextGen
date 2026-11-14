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

	"github.com/thazzr/mirrornextgen/server/internal/api"
)

const indexHTML = "<!doctype html><title>SPA</title>"

func newHandler(files fstest.MapFS) http.Handler {
	build := api.BuildInfo{Version: "dev", Commit: "unknown"}
	return New(build, files, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestUnknownAPIPathIsJSON404NotIndex(t *testing.T) {
	h := newHandler(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})

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
	h := newHandler(fstest.MapFS{})
	rec := get(h, "/api/v1/health")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestNoSniffOnEveryResponse(t *testing.T) {
	h := newHandler(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})
	for _, path := range []string{"/api/v1/health", "/api/v1/nope", "/", "/admin/settings", "/missing.js"} {
		t.Run(path, func(t *testing.T) {
			if got := get(h, path).Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
		})
	}
}

func TestWebServedForOtherPaths(t *testing.T) {
	h := newHandler(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})
	rec := get(h, "/admin/settings")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SPA") {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}
