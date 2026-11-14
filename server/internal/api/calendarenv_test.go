package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// syncBuffer is a log sink safe for the concurrent loggers of a test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeFeed serves the calendar package's .ics fixtures under /<name>.ics;
// /fail answers 500.
type fakeFeed struct {
	server *httptest.Server
	calls  atomic.Int32
}

func newFakeFeed(t *testing.T) *fakeFeed {
	t.Helper()
	f := &fakeFeed{}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		b, err := os.ReadFile(filepath.Join("..", "calendar", "testdata", strings.TrimPrefix(r.URL.Path, "/")))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// url is the address of a fixture; "secret-token" stands for the token real
// feed addresses carry.
func (f *fakeFeed) url(name string) string {
	return f.server.URL + "/" + name + "?token=secret-token"
}

// doMultipart posts a multipart form with an optional file and name.
func (e *testEnv) doMultipart(path string, file []byte, fileName string, fields map[string]string, o opts) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if file != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="`+fileName+`"`)
		h.Set("Content-Type", "text/plain") // a lie on purpose: the type comes from the content
		w, _ := mw.CreatePart(h)
		_, _ = w.Write(file)
	}
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if o.cookie != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: o.cookie})
	}
	if o.csrf != "" {
		req.Header.Set("X-CSRF-Token", o.csrf)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}
