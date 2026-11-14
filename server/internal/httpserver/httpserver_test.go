package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/api"
	"github.com/thazzr/mirrornextgen/server/internal/auth"
	"github.com/thazzr/mirrornextgen/server/internal/calendar"
	"github.com/thazzr/mirrornextgen/server/internal/icons"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
	"github.com/thazzr/mirrornextgen/server/internal/weather"
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
	ic, err := icons.Load(t.TempDir())
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
		Weather:  weather.New(weather.Options{Config: func() weather.Config { return weather.Config{} }}),
		Calendar: calendar.NewService(calendar.ServiceOptions{Feeds: calendar.NewFeeds(calendar.FeedsOptions{})}),
		Icons:    ic,
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

func TestLimitBodyCutsOversizedRequests(t *testing.T) {
	var readErr error
	h := limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.Copy(io.Discard, r.Body)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/icons", strings.NewReader(strings.Repeat("a", maxBodyBytes+1))))
	if readErr == nil {
		t.Error("a body above the limit was read completely")
	}
	readErr = nil
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/icons", strings.NewReader(strings.Repeat("a", maxBodyBytes))))
	if readErr != nil {
		t.Errorf("a body at the limit failed: %v", readErr)
	}
}

func TestMaxBodyBytesIs1MiB(t *testing.T) {
	if maxBodyBytes != 1024*1024 {
		t.Errorf("maxBodyBytes = %d, want 1048576", maxBodyBytes)
	}
}

func TestLimitBodyRejectsOversizedAPIBodiesButAllowsIconUploads(t *testing.T) {
	h := newHandler(t, fstest.MapFS{})
	do := func(req *http.Request, cookie, csrf string) *httptest.ResponseRecorder {
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: api.SessionCookieName, Value: cookie})
			req.Header.Set("X-CSRF-Token", csrf)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	rec := do(httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader(`{"password":"correct-horse-battery"}`)), "", "")
	var sess api.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &sess); err != nil || rec.Code != http.StatusOK || sess.CsrfToken == nil {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body)
	}
	var cookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == api.SessionCookieName {
			cookie = c.Value
		}
	}
	csrf := *sess.CsrfToken

	// Valid JSON behind leading whitespace: without the limit this would be
	// stored and answer 200, because the decoder stops after the first value.
	valid := `{"refreshMinutes":15,"calendars":[],"sections":[],"rules":[]}`
	put := func(pad int) *http.Request {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/calendar", strings.NewReader(strings.Repeat(" ", pad)+valid))
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	if rec := do(put(1024), cookie, csrf); rec.Code != http.StatusOK {
		t.Fatalf("small PUT: status = %d, body %s", rec.Code, rec.Body)
	}
	over := 1<<20 + 1 - len(valid) // a body of exactly 1 MiB + 1 bytes
	if rec := do(put(over), cookie, csrf); rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("1 MiB + 1 PUT: status = %d, want 413 or 400", rec.Code)
	}

	// A maximal icon (256 KB) still passes with its multipart overhead.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="big.png"`)
	w, _ := mw.CreatePart(hdr)
	_, _ = w.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, icons.MaxSize-8)...))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/icons", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if rec := do(req, cookie, csrf); rec.Code != http.StatusCreated {
		t.Fatalf("256 KB icon upload: status = %d, body %s", rec.Code, rec.Body)
	}
}
