package api

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

const (
	layoutPath          = "/api/v1/settings/layout"
	modulesPath         = "/api/v1/modules"
	dashboardLayoutPath = "/api/v1/dashboard/layout"
)

func defaultLayout() Layout {
	return Layout{
		TopLeft: []string{"clock"}, TopRight: []string{"weather"}, Middle: []string{},
		BottomLeft: []string{"bring"}, BottomRight: []string{"wifi"},
	}
}

func TestLayoutEndpointsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := Layout{TopLeft: []string{"wifi"}, TopRight: []string{}, Middle: []string{}, BottomLeft: []string{}, BottomRight: []string{}}

	wantProblem(t, e.do(http.MethodGet, layoutPath, nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodGet, modulesPath, nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPut, layoutPath, body, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPut, layoutPath, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPut, layoutPath, body, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")

	got := decode[Layout](t, e.do(http.MethodGet, dashboardLayoutPath, nil, opts{}))
	if !reflect.DeepEqual(got, defaultLayout()) {
		t.Errorf("refused PUT changed the layout: %+v", got)
	}
}

func TestLayoutDefaultAndRoundTrip(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}

	if got := decode[Layout](t, e.do(http.MethodGet, layoutPath, nil, o)); !reflect.DeepEqual(got, defaultLayout()) {
		t.Errorf("default = %+v", got)
	}
	want := Layout{TopLeft: []string{"wifi", "clock"}, TopRight: []string{}, Middle: []string{"weather"}, BottomLeft: []string{}, BottomRight: []string{}}
	rec := e.do(http.MethodPut, layoutPath, want, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status %d: %s", rec.Code, rec.Body)
	}
	if got := decode[Layout](t, rec); !reflect.DeepEqual(got, want) {
		t.Errorf("PUT answered %+v", got)
	}
	if got := decode[Layout](t, e.do(http.MethodGet, layoutPath, nil, o)); !reflect.DeepEqual(got, want) {
		t.Errorf("GET after PUT = %+v", got)
	}
	if got := decode[Layout](t, e.do(http.MethodGet, dashboardLayoutPath, nil, opts{})); !reflect.DeepEqual(got, want) {
		t.Errorf("dashboard layout = %+v", got)
	}
}

func TestUpdateLayoutRejectsBadLayoutsByRegion(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	empty := func(l Layout) Layout {
		for _, r := range []*[]string{&l.TopLeft, &l.TopRight, &l.Middle, &l.BottomLeft, &l.BottomRight} {
			if *r == nil {
				*r = []string{}
			}
		}
		return l
	}
	tests := []struct {
		name   string
		body   Layout
		region string
		msg    string
	}{
		{"unknown", empty(Layout{TopRight: []string{"nope"}}), "topRight", "Unbekanntes Modul „nope“."},
		{"duplicate", empty(Layout{TopLeft: []string{"clock"}, Middle: []string{"clock"}}), "middle", "„clock“ steht mehrfach in der Anordnung."},
		{"too many", empty(Layout{BottomLeft: make([]string, 13)}), "bottomLeft", "Höchstens 12 Module je Region."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := wantProblem(t, e.do(http.MethodPut, layoutPath, tt.body, o), http.StatusBadRequest, "")
			if p.Errors == nil || len(*p.Errors) != 1 {
				t.Fatalf("errors = %v", p.Errors)
			}
			msg, ok := (*p.Errors)[tt.region]
			if !ok || msg != tt.msg {
				t.Errorf("errors = %v", *p.Errors)
			}
			if got := decode[Layout](t, e.do(http.MethodGet, layoutPath, nil, o)); !reflect.DeepEqual(got, defaultLayout()) {
				t.Errorf("state changed to %+v", got)
			}
		})
	}
}

func TestListModules(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	got := decode[[]ModuleInfo](t, e.do(http.MethodGet, modulesPath, nil, opts{cookie: c.cookie}))
	want := []ModuleInfo{{Key: "clock", Name: "Uhr"}, {Key: "weather", Name: "Wetter"}, {Key: "wifi", Name: "WLAN-QR-Code"}, {Key: "bring", Name: "Bring!-Einkaufsliste"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("modules = %+v", got)
	}
}

// A settings file that names a module which no longer exists loads (the key
// is dropped) and the public dashboard layout never shows it.
func TestDashboardLayoutIsPublicAndWithoutUnavailableKeys(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"layout":{"topLeft":["clock","calendar:gone"],"topRight":[],"middle":["weather"],"bottomLeft":[],"bottomRight":["wifi"]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	e := newTestEnvWithSettingsDir(t, dir)
	rec := e.do(http.MethodGet, dashboardLayoutPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	want := Layout{TopLeft: []string{"clock"}, TopRight: []string{}, Middle: []string{"weather"}, BottomLeft: []string{}, BottomRight: []string{"wifi"}}
	if got := decode[Layout](t, rec); !reflect.DeepEqual(got, want) {
		t.Errorf("dashboard layout = %+v", got)
	}
}

func TestDashboardLayoutFiltersUnavailableKeys(t *testing.T) {
	st := settings.Defaults()
	st.Layout.Middle = []string{"calendar:gone", "weather"}
	got := dashboardLayout(st)
	if !reflect.DeepEqual(got.Middle, []string{"weather"}) || !reflect.DeepEqual(got.TopLeft, []string{"clock"}) {
		t.Errorf("layout = %+v", got)
	}
}
