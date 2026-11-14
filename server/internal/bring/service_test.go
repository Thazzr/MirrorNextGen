package bring

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type svcEnv struct {
	svc  *Service
	api  *fakeAPI
	now  time.Time
	cfg  Config
	logs *bytes.Buffer
	mu   sync.Mutex
}

func (e *svcEnv) clock() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now
}

func (e *svcEnv) advance(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = e.now.Add(d)
}

func (e *svcEnv) config() Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg
}

func (e *svcEnv) setConfig(c Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = c
}

func newSvcEnv(t *testing.T) *svcEnv {
	t.Helper()
	e := &svcEnv{
		api:  newFakeAPI(t),
		now:  time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC),
		cfg:  Config{Email: testCreds.Email, Password: testCreds.Password, MaxItems: 18, RefreshMinutes: 5},
		logs: &bytes.Buffer{},
	}
	e.svc = New(Options{
		HTTPClient: e.api.srv.Client(),
		BaseURL:    e.api.srv.URL,
		Config:     e.config,
		Now:        e.clock,
		Logger:     slog.New(slog.NewTextHandler(e.logs, nil)),
	})
	return e
}

func (e *svcEnv) listCalls() int {
	e.api.mu.Lock()
	defer e.api.mu.Unlock()
	n := 0
	for _, r := range e.api.requests {
		if r == "GET /bringlists/default-list" {
			n++
		}
	}
	return n
}

func TestServiceNotConfigured(t *testing.T) {
	for name, cfg := range map[string]Config{
		"nothing":     {MaxItems: 18, RefreshMinutes: 5},
		"no password": {Email: "a@b.de", MaxItems: 18, RefreshMinutes: 5},
		"no email":    {Password: "pw", MaxItems: 18, RefreshMinutes: 5},
		"zero value":  {},
	} {
		t.Run(name, func(t *testing.T) {
			e := newSvcEnv(t)
			e.setConfig(cfg)
			res := e.svc.Get(context.Background())
			if res.Configured || res.Data != nil || res.Error != "" || res.Stale {
				t.Errorf("result = %+v", res)
			}
			if e.api.logins != 0 {
				t.Error("Bring! was called although nothing is configured")
			}
			if _, err := e.svc.Lists(context.Background()); !errors.Is(err, ErrNotConfigured) {
				t.Errorf("Lists err = %v", err)
			}
		})
	}
}

func TestServiceBuildsDashboardItems(t *testing.T) {
	e := newSvcEnv(t)
	e.api.purchase = []map[string]string{
		{"name": "Milch", "specification": "2 Liter"},
		{"name": "milch", "specification": "2 liter"},
		{"name": "Äpfel", "specification": ""},
		{"name": "🥛", "specification": ""},
		{"name": "Brot", "specification": ""},
	}
	e.cfg.MaxItems = 4
	res := e.svc.Get(context.Background())
	if !res.Configured || res.Error != "" || res.Stale || res.Data == nil {
		t.Fatalf("result = %+v", res)
	}
	want := []DashboardItem{
		{"Milch", "2 Liter", "https://web.getbring.com/assets/images/items/vollmilch.png"}, // user's picture choice
		{"Äpfel", "", "https://web.getbring.com/assets/images/items/aepfel.png"},
		{"🥛", "", ""},
		{"Brot", "", "https://web.getbring.com/assets/images/items/brot.png"},
	}
	if len(res.Data.Items) != len(want) {
		t.Fatalf("items = %+v", res.Data.Items)
	}
	for i := range want {
		if res.Data.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, res.Data.Items[i], want[i])
		}
	}
	if !res.Data.UpdatedAt.Equal(e.now) {
		t.Errorf("UpdatedAt = %v", res.Data.UpdatedAt)
	}
}

func TestServiceRefreshInterval(t *testing.T) {
	e := newSvcEnv(t)
	e.cfg.RefreshMinutes = 5
	e.svc.Get(context.Background())
	e.advance(4*time.Minute + 59*time.Second)
	e.svc.Get(context.Background())
	if n := e.listCalls(); n != 1 {
		t.Fatalf("list calls = %d before the interval ended, want 1", n)
	}
	e.advance(time.Second)
	e.svc.Get(context.Background())
	if n := e.listCalls(); n != 2 {
		t.Errorf("list calls = %d after the interval, want 2", n)
	}
}

func TestServiceInvalidateAndSettingsChangeRefresh(t *testing.T) {
	e := newSvcEnv(t)
	e.svc.Get(context.Background())
	e.svc.Invalidate()
	e.svc.Get(context.Background())
	if n := e.listCalls(); n != 2 {
		t.Errorf("after Invalidate: %d list calls, want 2", n)
	}
	// A changed setting (here the item limit) is a new key: no stale reuse.
	c := e.config()
	c.MaxItems = 3
	e.setConfig(c)
	e.svc.Get(context.Background())
	if n := e.listCalls(); n != 3 {
		t.Errorf("after a settings change: %d list calls, want 3", n)
	}
}

func TestServiceSingleFlight(t *testing.T) {
	var lists atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/bringauth":
			_, _ = w.Write([]byte(`{"uuid":"u","bringListUUID":"d","access_token":"t","expires_in":3600}`))
		case strings.HasSuffix(r.URL.Path, "/details"):
			_, _ = w.Write([]byte(`[]`))
		default:
			lists.Add(1)
			<-release
			_, _ = w.Write([]byte(`{"purchase":[{"name":"Brot"}]}`))
		}
	}))
	defer srv.Close()
	svc := New(Options{HTTPClient: srv.Client(), BaseURL: srv.URL, Now: func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) },
		Config: func() Config { return Config{Email: "a@b.de", Password: "pw", MaxItems: 18, RefreshMinutes: 5} }})
	var wg sync.WaitGroup
	results := make([]Result, 6)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = svc.Get(context.Background())
		}()
	}
	// Let the callers pile up behind the one refresh, then answer it.
	deadline := time.Now().Add(5 * time.Second)
	for lists.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := lists.Load(); n != 1 {
		t.Errorf("list calls = %d, want 1 shared refresh", n)
	}
	for i, r := range results {
		if r.Data == nil || len(r.Data.Items) != 1 {
			t.Errorf("caller %d got %+v", i, r)
		}
	}
}

func TestServiceLoginFailureMessageAndRetryPause(t *testing.T) {
	e := newSvcEnv(t)
	c := e.config()
	c.Password = "wrong-SECRET-pw"
	e.setConfig(c)
	res := e.svc.Get(context.Background())
	if !res.Configured || res.Data != nil || res.Error != "Anmeldung bei Bring! fehlgeschlagen." {
		t.Fatalf("result = %+v", res)
	}
	// A poll right after the failure does not hit Bring! again ...
	e.advance(30 * time.Second)
	e.svc.Get(context.Background())
	if e.api.logins != 1 {
		t.Errorf("logins = %d within the retry pause, want 1", e.api.logins)
	}
	// ... and after the pause it tries again.
	e.advance(31 * time.Second)
	e.svc.Get(context.Background())
	if e.api.logins != 2 {
		t.Errorf("logins = %d after the pause, want 2", e.api.logins)
	}
}

func TestServiceOtherFailureMessage(t *testing.T) {
	e := newSvcEnv(t)
	e.api.listStatus = http.StatusInternalServerError
	res := e.svc.Get(context.Background())
	if res.Error != "Die Bring!-Liste konnte nicht geladen werden." || res.Data != nil {
		t.Errorf("result = %+v", res)
	}
}

func TestServiceStaleRule(t *testing.T) {
	e := newSvcEnv(t)
	first := e.svc.Get(context.Background())
	if first.Data == nil {
		t.Fatalf("first = %+v", first)
	}
	e.api.mu.Lock()
	e.api.listStatus = http.StatusInternalServerError
	e.api.mu.Unlock()

	// Refresh fails 10 minutes later: the old list stays, marked stale.
	e.advance(10 * time.Minute)
	res := e.svc.Get(context.Background())
	if res.Data == nil || !res.Stale || res.Error != "" {
		t.Fatalf("after 10 min: %+v, want the old list as stale", res)
	}
	// Still failing just under 3 hours after the good data: stale.
	e.advance(2*time.Hour + 49*time.Minute) // 2 h 59 min since the fetch
	res = e.svc.Get(context.Background())
	if res.Data == nil || !res.Stale {
		t.Fatalf("after 2 h 59 min: %+v, want stale data", res)
	}
	// Three hours or more: the error replaces it.
	e.advance(time.Minute)
	res = e.svc.Get(context.Background())
	if res.Data != nil || res.Stale || res.Error != "Die Bring!-Liste konnte nicht geladen werden." {
		t.Errorf("after 3 h: %+v, want the error", res)
	}
	// Recovery clears everything.
	e.api.mu.Lock()
	e.api.listStatus = 0
	e.api.mu.Unlock()
	e.advance(2 * time.Minute)
	res = e.svc.Get(context.Background())
	if res.Data == nil || res.Stale || res.Error != "" {
		t.Errorf("after recovery: %+v", res)
	}
}

func TestServiceListsAndLoginFailure(t *testing.T) {
	e := newSvcEnv(t)
	lists, err := e.svc.Lists(context.Background())
	if err != nil || len(lists) != 2 || lists[0].Name != "Zuhause" {
		t.Fatalf("lists = %v, err %v", lists, err)
	}
	c := e.config()
	c.Password = "wrong-SECRET-pw"
	e.setConfig(c)
	if _, err := e.svc.Lists(context.Background()); !errors.Is(err, ErrLoginFailed) {
		t.Errorf("err = %v, want ErrLoginFailed", err)
	}
}

// The password goes into the login form only: neither the log nor an error
// text may carry it, even if Bring! echoes it back.
func TestServiceNeverLogsOrReturnsThePassword(t *testing.T) {
	const secret = "pw-SECRET-must-not-leak"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom " + r.PostForm.Get("password")))
	}))
	defer srv.Close()
	logs := &bytes.Buffer{}
	svc := New(Options{HTTPClient: srv.Client(), BaseURL: srv.URL, Logger: slog.New(slog.NewTextHandler(logs, nil)),
		Config: func() Config { return Config{Email: "a@b.de", Password: secret, MaxItems: 18, RefreshMinutes: 5} }})
	res := svc.Get(context.Background())
	_, listErr := svc.Lists(context.Background())
	if strings.Contains(logs.String(), secret) || strings.Contains(res.Error, secret) || (listErr != nil && strings.Contains(listErr.Error(), secret)) {
		t.Errorf("password leaked: logs %q, result %+v, lists error %v", logs, res, listErr)
	}
	if logs.Len() == 0 {
		t.Error("the failure was not logged at all")
	}
}

func TestRedact(t *testing.T) {
	err := errors.New("failed for pw123 twice pw123")
	if got := redact(err, "pw123"); strings.Contains(got, "pw123") {
		t.Errorf("redact = %q", got)
	}
	if got := redact(err, ""); got != err.Error() {
		t.Errorf("redact without a secret = %q", got)
	}
}
