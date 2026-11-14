package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/auth"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

const (
	testPassword  = "correct-horse-battery"
	testPassword2 = "another-long-password"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// recordingMux is a router that remembers the patterns the generated code
// registers, so tests can walk every operation without a hand-written list.
type recordingMux struct {
	*http.ServeMux
	patterns []string
}

func (m *recordingMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	m.patterns = append(m.patterns, pattern)
	m.ServeMux.HandleFunc(pattern, h)
}

type testEnv struct {
	handler  http.Handler
	clock    *testClock
	patterns []string
	// lastOp is the operation ID of the latest request that reached the
	// middleware chain (set by an extra, outermost middleware).
	lastOp *string
	ow     *fakeOW
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnvWithSettingsDir(t, t.TempDir())
}

// newTestEnvWithSettingsDir is newTestEnv with a settings directory the
// caller may have prepared (for example with a settings.json to load).
func newTestEnvWithSettingsDir(t *testing.T, settingsDir string) *testEnv {
	t.Helper()
	admin, err := auth.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settingsStore, err := settings.Load(settingsDir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	clock := &testClock{t: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	weatherSvc, ow := newTestWeather(t, settingsStore, clock.now)
	deps := Deps{
		Build:    BuildInfo{Version: "1.2.3", Commit: "abc123", BuildTime: "2026-01-02T03:04:05Z"},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Admin:    admin,
		Sessions: auth.NewSessions(clock.now),
		Throttle: auth.NewThrottle(clock.now),
		Settings: settingsStore,
		Weather:  weatherSvc,
	}
	mux := &recordingMux{ServeMux: http.NewServeMux()}
	lastOp := new(string)
	var mu sync.Mutex
	register(mux, deps, func(next StrictHandlerFunc, operationID string) StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			mu.Lock()
			*lastOp = operationID
			mu.Unlock()
			return next(ctx, w, r, req)
		}
	})
	return &testEnv{handler: mux, clock: clock, patterns: mux.patterns, lastOp: lastOp, ow: ow}
}

type opts struct {
	cookie string
	csrf   string
	ip     string
	tls    bool
	header http.Header
}

func (e *testEnv) do(method, path string, body any, o opts) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if o.cookie != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: o.cookie})
	}
	if o.csrf != "" {
		req.Header.Set("X-CSRF-Token", o.csrf)
	}
	if o.ip != "" {
		req.RemoteAddr = o.ip + ":4321"
	}
	if o.tls {
		req.TLS = &tls.ConnectionState{}
	}
	for k, v := range o.header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

type creds struct{ cookie, csrf string }

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func sessionCookieOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie in the response")
	return nil
}

func credsOf(t *testing.T, rec *httptest.ResponseRecorder) creds {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	s := decode[Session](t, rec)
	if s.CsrfToken == nil {
		t.Fatal("no csrfToken in the answer")
	}
	return creds{cookie: sessionCookieOf(t, rec).Value, csrf: *s.CsrfToken}
}

func (e *testEnv) setup(t *testing.T) creds {
	t.Helper()
	return credsOf(t, e.do(http.MethodPost, "/api/v1/auth/setup", PasswordRequest{Password: testPassword}, opts{}))
}

func (e *testEnv) login(t *testing.T, pw string, o opts) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(http.MethodPost, "/api/v1/auth/login", PasswordRequest{Password: pw}, o)
}

func wantProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, detail string) Problem {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	p := decode[Problem](t, rec)
	if p.Status != status || p.Title != http.StatusText(status) {
		t.Errorf("problem = %+v", p)
	}
	if detail != "" && (p.Detail == nil || *p.Detail != detail) {
		t.Errorf("detail = %v, want %q", p.Detail, detail)
	}
	return p
}

func TestSessionStates(t *testing.T) {
	e := newTestEnv(t)

	s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{}))
	if !s.SetupRequired || s.Authenticated || s.CsrfToken != nil {
		t.Errorf("before setup: %+v", s)
	}

	c := e.setup(t)

	s = decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: c.cookie}))
	if s.SetupRequired || !s.Authenticated || s.CsrfToken == nil || *s.CsrfToken != c.csrf {
		t.Errorf("after setup with cookie: %+v", s)
	}

	s = decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{}))
	if s.SetupRequired || s.Authenticated || s.CsrfToken != nil {
		t.Errorf("after setup without cookie must not leak a token: %+v", s)
	}

	s = decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: "unknown"}))
	if s.Authenticated || s.CsrfToken != nil {
		t.Errorf("unknown cookie: %+v", s)
	}
}

func TestSetupCookieAttributes(t *testing.T) {
	tests := []struct {
		name       string
		tls        bool
		wantSecure bool
	}{
		{"plain http", false, false},
		{"tls", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			rec := e.do(http.MethodPost, "/api/v1/auth/setup", PasswordRequest{Password: testPassword}, opts{tls: tt.tls})
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			c := sessionCookieOf(t, rec)
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
				t.Errorf("cookie = %+v", c)
			}
			if c.Secure != tt.wantSecure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.wantSecure)
			}
			if c.MaxAge != 0 || !c.Expires.IsZero() {
				t.Errorf("must be a session cookie (server decides expiry): %+v", c)
			}
			if len(c.Value) != 43 {
				t.Errorf("token %q", c.Value)
			}
		})
	}
}

func TestSetupErrors(t *testing.T) {
	e := newTestEnv(t)

	rec := e.do(http.MethodPost, "/api/v1/auth/setup", PasswordRequest{Password: "123456789"}, opts{})
	p := wantProblem(t, rec, http.StatusBadRequest, "")
	if p.Errors == nil || (*p.Errors)["password"] != "Das Passwort muss mindestens 10 Zeichen lang sein." {
		t.Errorf("errors = %v", p.Errors)
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Error("a rejected setup must not start a session")
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	if _, ok := raw["errors"].(map[string]any); !ok {
		t.Errorf("body %s lacks an errors object", rec.Body)
	}

	rec = e.do(http.MethodPost, "/api/v1/auth/setup", PasswordRequest{Password: strings.Repeat("x", 129)}, opts{})
	p = wantProblem(t, rec, http.StatusBadRequest, "")
	if p.Errors == nil || !strings.Contains((*p.Errors)["password"], "höchstens 128 Zeichen") {
		t.Errorf("errors = %v", p.Errors)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", strings.NewReader("{not json"))
	rec = httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	wantProblem(t, rec, http.StatusBadRequest, "")

	e.setup(t)
	rec = e.do(http.MethodPost, "/api/v1/auth/setup", PasswordRequest{Password: testPassword2}, opts{})
	wantProblem(t, rec, http.StatusConflict, "Der Zugang ist bereits eingerichtet.")
	if rec.Header().Get("Set-Cookie") != "" {
		t.Error("a second setup must not start a session")
	}
	// The second setup must not have replaced the password.
	if rec := e.login(t, testPassword, opts{}); rec.Code != http.StatusOK {
		t.Errorf("original password no longer works: %d", rec.Code)
	}
}

func TestLoginBeforeSetupIs409(t *testing.T) {
	e := newTestEnv(t)
	wantProblem(t, e.login(t, testPassword, opts{}), http.StatusConflict, "Der Zugang ist noch nicht eingerichtet.")
}

func TestLoginWrongAndRight(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	wantProblem(t, e.login(t, "wrong-password-x", opts{}), http.StatusUnauthorized, "Das Passwort stimmt nicht.")

	rec := e.login(t, testPassword, opts{})
	c := credsOf(t, rec)
	cookie := sessionCookieOf(t, rec)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure {
		t.Errorf("cookie = %+v", cookie)
	}
	s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: c.cookie}))
	if !s.Authenticated {
		t.Error("login did not create a usable session")
	}
}

func TestLoginThrottle(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	for range 5 {
		wantProblem(t, e.login(t, "wrong-password-x", opts{}), http.StatusUnauthorized, "")
	}
	// The correct password is refused as well while blocked.
	rec := e.login(t, testPassword, opts{})
	wantProblem(t, rec, http.StatusTooManyRequests, "Zu viele Fehlversuche. Versuch es später noch einmal.")
	if got := rec.Header().Get("Retry-After"); got != "900" {
		t.Errorf("Retry-After = %q, want 900", got)
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Error("a blocked login must not start a session")
	}

	e.clock.add(15*time.Minute - time.Second)
	rec = e.login(t, testPassword, opts{})
	wantProblem(t, rec, http.StatusTooManyRequests, "")
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}

	e.clock.add(time.Second)
	if rec := e.login(t, testPassword, opts{}); rec.Code != http.StatusOK {
		t.Fatalf("login after the window = %d", rec.Code)
	}
}

func TestLoginThrottleRetryAfterRoundsUp(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)
	e.login(t, "wrong-password-x", opts{})
	e.clock.add(500 * time.Millisecond)
	for range 4 {
		e.login(t, "wrong-password-x", opts{})
	}
	rec := e.login(t, testPassword, opts{})
	// Oldest failure is 0.5 s old: 899.5 s remain, must be rounded up.
	if got := rec.Header().Get("Retry-After"); got != "900" {
		t.Errorf("Retry-After = %q, want 900", got)
	}
}

func TestLoginSuccessResetsThrottle(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)
	for range 4 {
		e.login(t, "wrong-password-x", opts{})
	}
	if rec := e.login(t, testPassword, opts{}); rec.Code != http.StatusOK {
		t.Fatalf("login = %d", rec.Code)
	}
	for i := range 5 {
		if rec := e.login(t, "wrong-password-x", opts{}); rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d after reset answered %d, want 401", i+1, rec.Code)
		}
	}
	if rec := e.login(t, testPassword, opts{}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("sixth attempt = %d, want 429", rec.Code)
	}
}

func TestLoginThrottleIsPerPeerAddressNotForwardedHeader(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)
	for i := range 5 {
		xff := http.Header{"X-Forwarded-For": {"10.0.0." + strconv.Itoa(i)}}
		e.login(t, "wrong-password-x", opts{ip: "192.0.2.50", header: xff})
	}
	xff := http.Header{"X-Forwarded-For": {"10.9.9.9"}}
	if rec := e.login(t, testPassword, opts{ip: "192.0.2.50", header: xff}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a fresh X-Forwarded-For dodged the throttle: %d", rec.Code)
	}
	if rec := e.login(t, testPassword, opts{ip: "192.0.2.51"}); rec.Code != http.StatusOK {
		t.Errorf("another address was blocked too: %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)

	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: c.cookie}), http.StatusForbidden, "Ungültiges Sicherheits-Token.")
	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{csrf: c.csrf}), http.StatusUnauthorized, "Bitte melde dich an.")
	// None of the rejected attempts ended the session.
	if s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: c.cookie})); !s.Authenticated {
		t.Fatal("a rejected logout ended the session")
	}

	rec := e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: c.cookie, csrf: c.csrf})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	cookie := sessionCookieOf(t, rec)
	if cookie.MaxAge != -1 || cookie.Value != "" || !cookie.HttpOnly || cookie.Path != "/" {
		t.Errorf("expiry cookie = %+v", cookie)
	}
	if got := rec.Header().Get("Set-Cookie"); !strings.Contains(got, "Max-Age=0") {
		t.Errorf("Set-Cookie = %q, want Max-Age=0", got)
	}

	if s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: c.cookie})); s.Authenticated {
		t.Error("session still valid after logout")
	}
	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusUnauthorized, "")
}

func TestChangePassword(t *testing.T) {
	e := newTestEnv(t)
	cur := e.setup(t)
	other := credsOf(t, e.login(t, testPassword, opts{}))
	change := func(c creds, current, next string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/v1/auth/password", ChangePasswordRequest{CurrentPassword: current, NewPassword: next}, opts{cookie: c.cookie, csrf: c.csrf})
	}

	// A wrong current password answers 401 and is not a login attempt.
	for range 8 {
		wantProblem(t, change(cur, "wrong-password-x", testPassword2), http.StatusUnauthorized, "Das aktuelle Passwort stimmt nicht.")
	}
	if rec := e.login(t, testPassword, opts{}); rec.Code != http.StatusOK {
		t.Fatalf("login after wrong current passwords = %d; they fed the throttle", rec.Code)
	}

	p := wantProblem(t, change(cur, testPassword, "short"), http.StatusBadRequest, "")
	if p.Errors == nil || (*p.Errors)["newPassword"] != "Das Passwort muss mindestens 10 Zeichen lang sein." {
		t.Errorf("errors = %v", p.Errors)
	}

	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/password",
		ChangePasswordRequest{CurrentPassword: testPassword, NewPassword: testPassword2}, opts{cookie: cur.cookie}),
		http.StatusForbidden, "")

	if rec := change(cur, testPassword, testPassword2); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: cur.cookie})); !s.Authenticated {
		t.Error("the current session must survive a password change")
	}
	if s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: other.cookie})); s.Authenticated {
		t.Error("another session must end with a password change")
	}
	wantProblem(t, e.login(t, testPassword, opts{}), http.StatusUnauthorized, "")
	if rec := e.login(t, testPassword2, opts{}); rec.Code != http.StatusOK {
		t.Errorf("login with the new password = %d", rec.Code)
	}
}

func TestSessionSlidingExpiryOverHTTP(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	e.clock.add(auth.SessionTTL - time.Minute)
	if s := decode[Session](t, e.do(http.MethodGet, "/api/v1/auth/session", nil, opts{cookie: c.cookie})); !s.Authenticated {
		t.Fatal("expired too early")
	}
	e.clock.add(auth.SessionTTL - time.Minute) // alive only because the request above extended it
	if rec := e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: c.cookie, csrf: c.csrf}); rec.Code != http.StatusNoContent {
		t.Fatalf("logout after sliding = %d", rec.Code)
	}
	c = credsOf(t, e.login(t, testPassword, opts{}))
	e.clock.add(auth.SessionTTL)
	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusUnauthorized, "")
}

func TestUnknownCookieIs401(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)
	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/logout", nil, opts{cookie: "forged", csrf: "forged"}), http.StatusUnauthorized, "Bitte melde dich an.")
}

func TestHealthAndInfoStayPublic(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t) // configured, nobody signed in
	for _, path := range []string{"/api/v1/health", "/api/v1/info"} {
		if rec := e.do(http.MethodGet, path, nil, opts{}); rec.Code != http.StatusOK {
			t.Errorf("%s = %d without a session", path, rec.Code)
		}
	}
}

// TestEveryOperationIsProtectedUnlessPublic walks every route the generated
// router registers (captured through recordingMux, so a new endpoint appears
// here without anyone editing this test) and proves that each operation not
// in publicOperations answers 401 without a session cookie.
func TestEveryOperationIsProtectedUnlessPublic(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	if len(e.patterns) == 0 {
		t.Fatal("no routes captured")
	}
	seen := map[string]bool{}
	for _, pattern := range e.patterns {
		method, path, _ := strings.Cut(pattern, " ")
		path = strings.NewReplacer("{", "", "}", "").Replace(path)
		t.Run(pattern, func(t *testing.T) {
			var body any
			if method != http.MethodGet {
				body = map[string]string{} // decodes fine, so the middleware is reached
			}
			*e.lastOp = ""
			rec := e.do(method, path, body, opts{})
			op := *e.lastOp
			if op == "" {
				t.Fatalf("%s never reached the middleware chain (status %d)", pattern, rec.Code)
			}
			seen[op] = true
			if publicOperations[op] {
				// login may legitimately answer 401 (wrong password); what a public
				// operation must never answer is the middleware's own refusal.
				body := rec.Body.String()
				if strings.Contains(body, msgLoginRequired) || strings.Contains(body, msgBadCSRF) {
					t.Errorf("public operation %s was refused by the middleware: %s", op, body)
				}
				return
			}
			wantProblem(t, rec, http.StatusUnauthorized, "Bitte melde dich an.")
		})
	}

	// The captured routes must cover every operation of the generated interface.
	iface := reflect.TypeFor[ServerInterface]()
	for i := range iface.NumMethod() {
		op := iface.Method(i).Name
		if !seen[op] {
			t.Errorf("operation %s of the generated interface was not exercised", op)
		}
	}
	// A stale entry in the public list would silently stay public if the
	// operation came back under that name.
	for op := range publicOperations {
		if !seen[op] {
			t.Errorf("publicOperations names %q, which is not an operation", op)
		}
	}
	protected := 0
	for op := range seen {
		if !publicOperations[op] {
			protected++
		}
	}
	if protected < 2 {
		t.Errorf("only %d protected operations found; the walk is not testing anything", protected)
	}
	if want := []string{"ChangePassword", "GetDisplaySettings", "GetKioskState", "UpdateDisplaySettings", "GetHealth", "GetInfo", "GetSession", "Login", "Logout", "SetupAdmin"}; len(seen) < len(want) {
		var got []string
		for op := range seen {
			got = append(got, op)
		}
		slices.Sort(got)
		t.Errorf("operations seen %v, want at least %v", got, want)
	}
}
