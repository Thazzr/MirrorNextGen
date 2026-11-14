package bring

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var testCreds = Credentials{Email: "familie@example.org", Password: "pw-SECRET-1"}

// fakeAPI is a Bring! stand-in that records what it was asked.
type fakeAPI struct {
	srv *httptest.Server

	mu         sync.Mutex
	logins     int
	expiresIn  int64
	token      int      // number of the current token; bumped to invalidate
	rejectNext int      // answer this many authenticated calls with 401
	requests   []string // "METHOD path" of every request after the login
	authHeader []string
	headers    http.Header // headers of the last login
	form       map[string]string
	listStatus int
	detailsBad bool
	purchase   []map[string]string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{expiresIn: 3600, token: 1, purchase: []map[string]string{{"name": "Milch", "specification": "2 Liter"}}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if r.URL.Path == "/bringauth" {
		f.logins++
		f.headers = r.Header.Clone()
		_ = r.ParseForm()
		f.form = map[string]string{"email": r.PostForm.Get("email"), "password": r.PostForm.Get("password"), "method": r.Method, "type": r.Header.Get("Content-Type")}
		if r.PostForm.Get("email") != testCreds.Email || r.PostForm.Get("password") != testCreds.Password {
			w.WriteHeader(http.StatusUnauthorized)
			write(map[string]string{"message": "wrong password " + r.PostForm.Get("password")})
			return
		}
		write(map[string]any{"uuid": "user-1", "bringListUUID": "default-list", "access_token": "token-" + string(rune('0'+f.token)), "expires_in": f.expiresIn})
		return
	}
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	f.authHeader = append(f.authHeader, r.Header.Get("Authorization"))
	if r.Header.Get("X-BRING-API-KEY") == "" || r.Header.Get("Accept-Language") == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if f.rejectNext > 0 {
		f.rejectNext--
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/details"):
		if f.detailsBad {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		write([]map[string]string{{"itemId": "Milch", "userIconItemId": "Vollmilch"}, {"itemId": "milch", "userIconItemId": "ignored"}, {"itemId": "", "userIconItemId": "x"}})
	case strings.HasPrefix(r.URL.Path, "/bringlists/"):
		if f.listStatus != 0 {
			w.WriteHeader(f.listStatus)
			return
		}
		write(map[string]any{"purchase": f.purchase})
	case r.URL.Path == "/bringusers/user-1/lists":
		write(map[string]any{"lists": []map[string]string{{"listUuid": "l1", "name": "Zuhause"}, {"listUuid": "l2", "name": "Büro"}}})
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAPI) client(now func() time.Time) *Client {
	return NewClient(f.srv.Client(), f.srv.URL, now)
}

func TestClientLoginSendsFormAndApiKey(t *testing.T) {
	f := newFakeAPI(t)
	if _, err := f.client(nil).Content(context.Background(), testCreds, ""); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"email": testCreds.Email, "password": testCreds.Password, "method": "POST", "type": "application/x-www-form-urlencoded"}
	if !reflect.DeepEqual(f.form, want) {
		t.Errorf("login form = %v, want %v", f.form, want)
	}
	if got := f.headers.Get("X-BRING-API-KEY"); got != "cof4Nc6D8saplXjE3h3HXqHH8m7VU2i1Gs0g85Sp" {
		t.Errorf("X-BRING-API-KEY = %q", got)
	}
	if got := f.headers.Get("Accept-Language"); got != "de-DE,de;q=0.8,en;q=0.5" {
		t.Errorf("Accept-Language = %q", got)
	}
}

func TestClientContentUsesDefaultListAndDetails(t *testing.T) {
	f := newFakeAPI(t)
	got, err := f.client(nil).Content(context.Background(), testCreds, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []Item{{"Milch", "2 Liter"}}; !reflect.DeepEqual(got.Items, want) {
		t.Errorf("items = %v", got.Items)
	}
	// The first entry per catalogue id wins, blank ids are skipped, keys are lower-case.
	if want := map[string]string{"milch": "Vollmilch"}; !reflect.DeepEqual(got.UserIcons, want) {
		t.Errorf("user icons = %v", got.UserIcons)
	}
	wantReq := []string{"GET /bringlists/default-list", "GET /bringlists/default-list/details"}
	if !reflect.DeepEqual(f.requests, wantReq) {
		t.Errorf("requests = %v, want %v", f.requests, wantReq)
	}
	for _, h := range f.authHeader {
		if h != "Bearer token-1" {
			t.Errorf("Authorization = %q", h)
		}
	}
}

func TestClientContentUsesChosenList(t *testing.T) {
	f := newFakeAPI(t)
	if _, err := f.client(nil).Content(context.Background(), testCreds, "my list/1"); err != nil {
		t.Fatal(err)
	}
	// The server sees the decoded path; the slash inside the id must not split it.
	if want := []string{"GET /bringlists/my list/1", "GET /bringlists/my list/1/details"}; !reflect.DeepEqual(f.requests, want) {
		t.Errorf("requests = %v, want %v", f.requests, want)
	}
}

func TestClientDetailsFailureIsNotFatal(t *testing.T) {
	f := newFakeAPI(t)
	f.detailsBad = true
	got, err := f.client(nil).Content(context.Background(), testCreds, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || len(got.UserIcons) != 0 {
		t.Errorf("content = %+v", got)
	}
}

func TestClientListFailureIsAnError(t *testing.T) {
	f := newFakeAPI(t)
	f.listStatus = http.StatusInternalServerError
	_, err := f.client(nil).Content(context.Background(), testCreds, "")
	if err == nil || errors.Is(err, ErrLoginFailed) {
		t.Errorf("err = %v, want a plain failure", err)
	}
}

func TestClientLists(t *testing.T) {
	f := newFakeAPI(t)
	got, err := f.client(nil).Lists(context.Background(), testCreds)
	if err != nil {
		t.Fatal(err)
	}
	if want := []List{{"l1", "Zuhause"}, {"l2", "Büro"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("lists = %v", got)
	}
	if want := []string{"GET /bringusers/user-1/lists"}; !reflect.DeepEqual(f.requests, want) {
		t.Errorf("requests = %v", f.requests)
	}
}

func TestClientLoginFailure(t *testing.T) {
	f := newFakeAPI(t)
	wrong := Credentials{Email: testCreds.Email, Password: "nope-SECRET"}
	_, err := f.client(nil).Content(context.Background(), wrong, "")
	if !errors.Is(err, ErrLoginFailed) {
		t.Fatalf("err = %v, want ErrLoginFailed", err)
	}
	if strings.Contains(err.Error(), "nope-SECRET") {
		t.Errorf("error leaks the password: %v", err)
	}
	if len(f.requests) != 0 {
		t.Errorf("calls after a failed login: %v", f.requests)
	}
	if _, err := f.client(nil).Lists(context.Background(), wrong); !errors.Is(err, ErrLoginFailed) {
		t.Errorf("Lists err = %v", err)
	}
}

func TestClientServerErrorOnLoginIsNotALoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer srv.Close()
	_, err := NewClient(srv.Client(), srv.URL, nil).Lists(context.Background(), testCreds)
	if err == nil || errors.Is(err, ErrLoginFailed) {
		t.Errorf("err = %v, want a plain failure", err)
	}
}

func TestClientCachesSessionUntilShortlyBeforeExpiry(t *testing.T) {
	f := newFakeAPI(t)
	f.expiresIn = 3600
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	c := f.client(func() time.Time { return now })
	for range 3 {
		if _, err := c.Content(context.Background(), testCreds, ""); err != nil {
			t.Fatal(err)
		}
	}
	if f.logins != 1 {
		t.Fatalf("logins = %d, want 1 (session is cached)", f.logins)
	}
	// Valid for 3600-30 s, unused during the last minute: 58 min is still fine.
	now = now.Add(58 * time.Minute)
	if _, err := c.Content(context.Background(), testCreds, ""); err != nil {
		t.Fatal(err)
	}
	if f.logins != 1 {
		t.Errorf("logins = %d after 58 min, want 1", f.logins)
	}
	now = now.Add(2 * time.Minute)
	if _, err := c.Lists(context.Background(), testCreds); err != nil {
		t.Fatal(err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d after 60 min, want a new login", f.logins)
	}
}

func TestClientShortOrMissingExpiryDefaultsToTenMinutes(t *testing.T) {
	f := newFakeAPI(t)
	f.expiresIn = 0
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	c := f.client(func() time.Time { return now })
	_, _ = c.Lists(context.Background(), testCreds)
	now = now.Add(8 * time.Minute)
	_, _ = c.Lists(context.Background(), testCreds)
	if f.logins != 1 {
		t.Errorf("logins = %d after 8 min, want 1", f.logins)
	}
	now = now.Add(2 * time.Minute)
	_, _ = c.Lists(context.Background(), testCreds)
	if f.logins != 2 {
		t.Errorf("logins = %d after 10 min, want 2", f.logins)
	}
}

func TestClientReauthenticatesOnceAfter401(t *testing.T) {
	f := newFakeAPI(t)
	c := f.client(nil)
	if _, err := c.Content(context.Background(), testCreds, ""); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.rejectNext, f.token = 1, 2
	f.mu.Unlock()
	if _, err := c.Content(context.Background(), testCreds, ""); err != nil {
		t.Fatalf("the 401 must be retried with a new login: %v", err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want 2", f.logins)
	}
	if last := f.authHeader[len(f.authHeader)-1]; last != "Bearer token-2" {
		t.Errorf("retry used %q", last)
	}
}

func TestClientGivesUpAfterASecond401(t *testing.T) {
	f := newFakeAPI(t)
	f.rejectNext = 100
	_, err := f.client(nil).Content(context.Background(), testCreds, "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want exactly one re-login", f.logins)
	}
}

func TestClientNewCredentialsDropTheSession(t *testing.T) {
	f := newFakeAPI(t)
	c := f.client(nil)
	if _, err := c.Lists(context.Background(), testCreds); err != nil {
		t.Fatal(err)
	}
	other := Credentials{Email: testCreds.Email, Password: "changed"}
	if _, err := c.Lists(context.Background(), other); !errors.Is(err, ErrLoginFailed) {
		t.Errorf("err = %v, want login failure with the new password", err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want a login for the new credentials", f.logins)
	}
}

func TestClientConcurrentCallsShareOneLogin(t *testing.T) {
	f := newFakeAPI(t)
	c := f.client(nil)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Lists(context.Background(), testCreds); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if f.logins != 1 {
		t.Errorf("logins = %d, want 1", f.logins)
	}
}
