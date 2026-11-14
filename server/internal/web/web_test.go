package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func spaFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                  {Data: []byte("<!doctype html><title>SPA</title>")},
		"assets/index-AbCd1234.js":    {Data: []byte("console.log(1)")},
		"assets/weather/sun.svg":      {Data: []byte("<svg/>")},
		"assets/app.js":               {Data: []byte("x")},
		".gitignore":                  {Data: []byte("*")},
		"assets/.secret":              {Data: []byte("secret")},
		"assets/hashed-ab_-1234.woff": {Data: []byte("font")},
		"logo-AbCd1234.png":           {Data: []byte("png")},
		"other/x-AbCd1234.js":         {Data: []byte("js")},
	}
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

const (
	noCache   = "no-cache"
	immutable = "public, max-age=31536000, immutable"
)

func TestServing(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantBody     string
		wantCache    string
		wantNoSecret bool
	}{
		{"root is index", "/", 200, "SPA", noCache, false},
		{"direct index", "/index.html", 200, "SPA", noCache, false},
		{"client route falls back to index", "/admin/settings", 200, "SPA", noCache, false},
		{"hashed asset is immutable", "/assets/index-AbCd1234.js", 200, "console.log(1)", immutable, false},
		{"hash with dash and underscore", "/assets/hashed-ab_-1234.woff", 200, "font", immutable, false},
		{"unhashed asset is not cached", "/assets/weather/sun.svg", 200, "<svg/>", noCache, false},
		{"hash-like file at root is not immutable", "/logo-AbCd1234.png", 200, "png", noCache, false},
		{"hash-like file outside assets is not immutable", "/other/x-AbCd1234.js", 200, "js", noCache, false},
		{"short name is not a hash", "/assets/app.js", 200, "x", noCache, false},
		{"missing hashed asset is 404", "/assets/missing-AbCd1234.js", 404, "", noCache, false},
		{"missing file-like path is 404", "/favicon.ico", 404, "", noCache, false},
		{"dotfile at root is 404", "/.gitignore", 404, "", noCache, true},
		{"nested dotfile is 404", "/assets/.secret", 404, "", noCache, true},
		{"dot segments cannot escape", "/assets/../.gitignore", 404, "", noCache, true},
	}
	h := Handler(spaFS())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(h, http.MethodGet, tt.path)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantNoSecret && (strings.Contains(rec.Body.String(), "secret") || rec.Body.String() == "*") {
				t.Errorf("dotfile content leaked: %q", rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
		})
	}
}

func TestContentTypeOfServedFile(t *testing.T) {
	rec := do(Handler(spaFS()), http.MethodGet, "/assets/weather/sun.svg")
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHeadIsAllowed(t *testing.T) {
	rec := do(Handler(spaFS()), http.MethodHead, "/")
	if rec.Code != 200 || rec.Body.Len() != 0 {
		t.Errorf("status = %d, body length = %d", rec.Code, rec.Body.Len())
	}
}

func TestOtherMethodsAreRejected(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec := do(Handler(spaFS()), method, "/")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
				t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
			}
		})
	}
}

func TestDirectoriesAreNotListed(t *testing.T) {
	// "assets" has no dot, so it is a client route and answers index.html.
	rec := do(Handler(spaFS()), http.MethodGet, "/assets")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "sun.svg") {
		t.Errorf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestNotBuilt(t *testing.T) {
	const hint = `Die Oberfläche ist nicht gebaut. Führe im Ordner web "npm run build" aus.`
	// The embedded dist/ holds only .gitignore before the first build.
	empty := fstest.MapFS{".gitignore": {Data: []byte("*")}}
	for _, path := range []string{"/", "/admin", "/assets/x.js"} {
		t.Run(path, func(t *testing.T) {
			rec := do(Handler(empty), http.MethodGet, path)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
				t.Errorf("Content-Type = %q", ct)
			}
			if rec.Body.String() != hint {
				t.Errorf("body = %q", rec.Body.String())
			}
		})
	}
}

func TestDistEmbedsDirectory(t *testing.T) {
	// Must not panic and must resolve the dist root even without a build.
	if Dist() == nil {
		t.Fatal("Dist() returned nil")
	}
}
