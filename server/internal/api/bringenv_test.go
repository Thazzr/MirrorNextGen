package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/bring"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

const (
	fakeBringEmail    = "familie@example.org"
	fakeBringPassword = "bring-SECRET-pass-77"
	fakeBringList     = "list-uuid-1"
)

// fakeBring is a fake Bring! with one account, one list and two lists in the
// picker.
type fakeBring struct {
	server *httptest.Server
	logins atomic.Int32
	calls  atomic.Int32
	// status, when non-zero, is answered to every request after the login.
	status atomic.Int32
	mu     sync.Mutex
	// purchase is the content of the list.
	purchase []map[string]string
}

func newFakeBring(t *testing.T) *fakeBring {
	t.Helper()
	f := &fakeBring{purchase: []map[string]string{
		{"name": "Milch", "specification": "2 Liter"},
		{"name": "milch", "specification": "2 liter"},
		{"name": "Äpfel", "specification": ""},
	}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		if r.URL.Path == "/bringauth" {
			f.logins.Add(1)
			_ = r.ParseForm()
			if r.PostForm.Get("email") != fakeBringEmail || r.PostForm.Get("password") != fakeBringPassword {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			write(map[string]any{"uuid": "user-1", "bringListUUID": fakeBringList, "access_token": "tok", "expires_in": 3600})
			return
		}
		if st := f.status.Load(); st != 0 {
			w.WriteHeader(int(st))
			return
		}
		switch {
		case r.URL.Path == "/bringlists/"+fakeBringList:
			f.mu.Lock()
			defer f.mu.Unlock()
			write(map[string]any{"purchase": f.purchase})
		case r.URL.Path == "/bringlists/"+fakeBringList+"/details":
			write([]map[string]string{{"itemId": "Milch", "userIconItemId": "Milch"}})
		case r.URL.Path == "/bringusers/user-1/lists":
			write(map[string]any{"lists": []map[string]string{{"listUuid": fakeBringList, "name": "Zuhause"}, {"listUuid": "list-uuid-2", "name": "Büro"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

// newTestBring returns a Bring! service reading the store's settings and
// talking to a fake Bring!; log receives its log output.
func newTestBring(t *testing.T, store *settings.Store, now func() time.Time, log io.Writer) (*bring.Service, *fakeBring) {
	t.Helper()
	fb := newFakeBring(t)
	svc := bring.New(bring.Options{
		HTTPClient: fb.server.Client(),
		BaseURL:    fb.server.URL,
		Config: func() bring.Config {
			b := store.Get().Bring
			return bring.Config{Email: b.Email, Password: b.Password, ListUUID: b.ListUUID, MaxItems: b.MaxItems, RefreshMinutes: b.RefreshMinutes}
		},
		Now:    now,
		Logger: slog.New(slog.NewTextHandler(log, nil)),
	})
	return svc, fb
}
