package api

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	bringPath     = "/api/v1/settings/bring"
	bringListPath = "/api/v1/settings/bring/lists"
	dashBringPath = "/api/v1/dashboard/bring"
)

func goodBringUpdate() BringSettingsUpdate {
	return BringSettingsUpdate{Email: fakeBringEmail, Password: ptr(fakeBringPassword), MaxItems: 18, RefreshMinutes: 5}
}

func TestBringSettingsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := goodBringUpdate()
	for _, path := range []string{bringPath, bringListPath} {
		wantProblem(t, e.do(http.MethodGet, path, nil, opts{}), http.StatusUnauthorized, "")
	}
	wantProblem(t, e.do(http.MethodPut, bringPath, body, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPut, bringPath, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPut, bringPath, body, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")
	// Nothing of the refused requests was stored.
	got := decode[BringSettings](t, e.do(http.MethodGet, bringPath, nil, opts{cookie: c.cookie}))
	if got.Email != "" || got.HasPassword {
		t.Errorf("refused PUT changed the settings: %+v", got)
	}
	if n := e.bring.calls.Load(); n != 0 {
		t.Errorf("%d Bring! calls without a session", n)
	}
}

func TestBringSettingsDefaultsAndSecretSemantics(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)

	if got := decode[BringSettings](t, e.do(http.MethodGet, bringPath, nil, o)); got != (BringSettings{MaxItems: 18, RefreshMinutes: 5}) {
		t.Errorf("defaults = %+v", got)
	}

	up := goodBringUpdate()
	up.ListUuid, up.MaxItems, up.RefreshMinutes = "list-x", 12, 10
	rec := e.do(http.MethodPut, bringPath, up, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	want := BringSettings{Email: fakeBringEmail, HasPassword: true, ListUuid: "list-x", MaxItems: 12, RefreshMinutes: 10}
	if got := decode[BringSettings](t, rec); got != want {
		t.Errorf("PUT answered %+v", got)
	}
	get := e.do(http.MethodGet, bringPath, nil, o)
	for name, body := range map[string]string{"PUT body": rec.Body.String(), "GET body": get.Body.String()} {
		if strings.Contains(body, fakeBringPassword) || strings.Contains(body, `"password"`) {
			t.Errorf("%s leaks the password: %s", name, body)
		}
	}

	// Absent or empty password keeps it.
	for name, pw := range map[string]*string{"absent": nil, "empty": ptr("")} {
		up := goodBringUpdate()
		up.Password, up.MaxItems = pw, 7
		if got := decode[BringSettings](t, e.do(http.MethodPut, bringPath, up, o)); !got.HasPassword || got.MaxItems != 7 {
			t.Errorf("%s password: %+v", name, got)
		}
	}
	// A new password together with clear is a field error and changes nothing.
	both := goodBringUpdate()
	both.ClearPassword = ptr(true)
	p := wantProblem(t, e.do(http.MethodPut, bringPath, both, o), http.StatusBadRequest, "")
	if p.Errors == nil || (*p.Errors)["password"] == "" {
		t.Errorf("conflict errors = %v", p.Errors)
	}
	if got := decode[BringSettings](t, e.do(http.MethodGet, bringPath, nil, o)); !got.HasPassword {
		t.Error("the rejected conflict cleared the password")
	}
	// Clear deletes it.
	clr := goodBringUpdate()
	clr.Password, clr.ClearPassword = nil, ptr(true)
	if got := decode[BringSettings](t, e.do(http.MethodPut, bringPath, clr, o)); got.HasPassword {
		t.Errorf("clear: %+v", got)
	}
}

func TestBringSettingsFieldErrors(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	good := goodBringUpdate()
	if rec := e.do(http.MethodPut, bringPath, good, o); rec.Code != http.StatusOK {
		t.Fatalf("setup PUT: %d", rec.Code)
	}
	tests := []struct {
		name   string
		mutate func(*BringSettingsUpdate)
		field  string
		msg    string
	}{
		{"max items 0", func(b *BringSettingsUpdate) { b.MaxItems = 0 }, "maxItems", "Erlaubt sind 1 bis 18 Einträge."},
		{"max items 19", func(b *BringSettingsUpdate) { b.MaxItems = 19 }, "maxItems", "Erlaubt sind 1 bis 18 Einträge."},
		{"refresh 0", func(b *BringSettingsUpdate) { b.RefreshMinutes = 0 }, "refreshMinutes", "Erlaubt sind 1 bis 1440 Minuten."},
		{"refresh 1441", func(b *BringSettingsUpdate) { b.RefreshMinutes = 1441 }, "refreshMinutes", "Erlaubt sind 1 bis 1440 Minuten."},
		{"email 201", func(b *BringSettingsUpdate) { b.Email = strings.Repeat("a", 201) }, "email", "Höchstens 200 Zeichen."},
		{"list 101", func(b *BringSettingsUpdate) { b.ListUuid = strings.Repeat("l", 101) }, "listUuid", "Höchstens 100 Zeichen."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := good
			body.Password = nil
			tt.mutate(&body)
			p := wantProblem(t, e.do(http.MethodPut, bringPath, body, o), http.StatusBadRequest, "")
			if p.Errors == nil || (*p.Errors)[tt.field] != tt.msg || len(*p.Errors) != 1 {
				t.Fatalf("errors = %v", p.Errors)
			}
			if got := decode[BringSettings](t, e.do(http.MethodGet, bringPath, nil, o)); got.MaxItems != 18 || got.Email != fakeBringEmail {
				t.Errorf("state changed to %+v", got)
			}
		})
	}
}

func TestBringListsConflictWhenNotConfigured(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	wantProblem(t, e.do(http.MethodGet, bringListPath, nil, o), http.StatusConflict, "Bitte speichere zuerst E-Mail und Passwort.")
	// An e-mail alone is not enough either.
	up := goodBringUpdate()
	up.Password = nil
	e.do(http.MethodPut, bringPath, up, o)
	wantProblem(t, e.do(http.MethodGet, bringListPath, nil, o), http.StatusConflict, "Bitte speichere zuerst E-Mail und Passwort.")
	if n := e.bring.calls.Load(); n != 0 {
		t.Errorf("%d Bring! calls while not configured", n)
	}
}

func TestBringListsWithStoredCredentials(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	e.do(http.MethodPut, bringPath, goodBringUpdate(), o)
	rec := e.do(http.MethodGet, bringListPath, nil, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	want := []BringList{{Uuid: fakeBringList, Name: "Zuhause"}, {Uuid: "list-uuid-2", Name: "Büro"}}
	if got := decode[[]BringList](t, rec); !reflect.DeepEqual(got, want) {
		t.Errorf("lists = %+v", got)
	}
}

func TestBringListsLoginFailureAndUpstreamFailure(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	bad := goodBringUpdate()
	bad.Password = ptr("wrong-SECRET-pass")
	e.do(http.MethodPut, bringPath, bad, o)
	rec := e.do(http.MethodGet, bringListPath, nil, o)
	wantProblem(t, rec, http.StatusBadRequest, "Anmeldung bei Bring! fehlgeschlagen.")
	if strings.Contains(rec.Body.String(), "wrong-SECRET-pass") {
		t.Error("the answer carries the password")
	}

	e.do(http.MethodPut, bringPath, goodBringUpdate(), o)
	e.bring.status.Store(http.StatusInternalServerError)
	wantProblem(t, e.do(http.MethodGet, bringListPath, nil, o), http.StatusBadGateway, "Die Bring!-Listen konnten nicht geladen werden.")
	if strings.Contains(e.logs.String(), "wrong-SECRET-pass") || strings.Contains(e.logs.String(), fakeBringPassword) {
		t.Errorf("password in the log: %s", e.logs.String())
	}
}

func TestDashboardBringNotConfigured(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(http.MethodGet, dashBringPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"configured":false`) || !strings.Contains(got, `"stale":false`) ||
		!strings.Contains(got, `"items":[]`) || strings.Contains(got, `"error"`) {
		t.Errorf("body = %s", got)
	}
	if n := e.bring.calls.Load(); n != 0 {
		t.Errorf("%d upstream calls while not configured", n)
	}
}

func TestDashboardBringIsPublicAndDeliversDeduplicatedItems(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	e.do(http.MethodPut, bringPath, goodBringUpdate(), o)

	// No cookie: the dashboard is public.
	rec := e.do(http.MethodGet, dashBringPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	got := decode[DashboardBring](t, rec)
	if !got.Configured || got.Stale || got.Error != nil {
		t.Fatalf("state = %+v", got)
	}
	want := []BringItem{
		{Name: "Milch", Specification: ptr("2 Liter"), IconUrl: "https://web.getbring.com/assets/images/items/milch.png"},
		{Name: "Äpfel", IconUrl: "https://web.getbring.com/assets/images/items/aepfel.png"},
	}
	// "milch"/"2 liter" is a duplicate; no specification means no field.
	if !reflect.DeepEqual(got.Items, want) {
		t.Errorf("items = %+v, want %+v", got.Items, want)
	}
	for _, secret := range []string{fakeBringPassword, fakeBringEmail} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("dashboard body leaks %q", secret)
		}
	}
}

func TestDashboardBringCachesAndSettingsChangeRefreshes(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	e.do(http.MethodPut, bringPath, goodBringUpdate(), o)
	e.do(http.MethodGet, dashBringPath, nil, opts{})
	e.do(http.MethodGet, dashBringPath, nil, opts{})
	if n := e.bring.logins.Load(); n != 1 {
		t.Fatalf("logins = %d, want 1 (cached session)", n)
	}
	calls := e.bring.calls.Load()
	e.do(http.MethodGet, dashBringPath, nil, opts{})
	if e.bring.calls.Load() != calls {
		t.Error("a second poll within the interval called Bring! again")
	}

	// Saving fewer items shows up on the very next poll, without waiting for the interval.
	up := goodBringUpdate()
	up.Password, up.MaxItems = nil, 1
	e.do(http.MethodPut, bringPath, up, o)
	if got := decode[DashboardBring](t, e.do(http.MethodGet, dashBringPath, nil, opts{})); len(got.Items) != 1 {
		t.Errorf("items after the change = %+v", got.Items)
	}
	// Past the interval the list is fetched again.
	e.bring.mu.Lock()
	e.bring.purchase = e.bring.purchase[:1]
	e.bring.mu.Unlock()
	up.MaxItems = 18
	e.do(http.MethodPut, bringPath, up, o)
	e.clock.add(6 * time.Minute)
	if got := decode[DashboardBring](t, e.do(http.MethodGet, dashBringPath, nil, opts{})); len(got.Items) != 1 {
		t.Errorf("items after the interval = %+v", got.Items)
	}
}

func TestDashboardBringErrorTravelsInTheBody(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	bad := goodBringUpdate()
	bad.Password = ptr("wrong-SECRET-pass")
	e.do(http.MethodPut, bringPath, bad, o)
	rec := e.do(http.MethodGet, dashBringPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 with the error in the body", rec.Code)
	}
	got := decode[DashboardBring](t, rec)
	if !got.Configured || got.Error == nil || *got.Error != "Anmeldung bei Bring! fehlgeschlagen." || len(got.Items) != 0 {
		t.Errorf("state = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "wrong-SECRET-pass") || strings.Contains(e.logs.String(), "wrong-SECRET-pass") {
		t.Error("password leaked")
	}
}
