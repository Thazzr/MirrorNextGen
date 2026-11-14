package api

import (
	"net/http"
	"strings"
	"testing"
)

const (
	displayPath = "/api/v1/settings/display"
	kioskPath   = "/api/v1/kiosk/state"
)

func TestDisplaySettingsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)

	wantProblem(t, e.do(http.MethodGet, displayPath, nil, opts{}), http.StatusUnauthorized, "")
	body := DisplaySettings{Rotation: 90}
	wantProblem(t, e.do(http.MethodPut, displayPath, body, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPut, displayPath, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPut, displayPath, body, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")

	// Nothing of the refused requests was stored.
	got := decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{}))
	if got.Rotation != 0 {
		t.Errorf("refused PUT changed the state: %+v", got)
	}
}

func TestDisplaySettingsDefaultsAndUpdate(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}

	rec := e.do(http.MethodGet, displayPath, nil, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := decode[DisplaySettings](t, rec); got != (DisplaySettings{}) {
		t.Errorf("defaults = %+v", got)
	}

	want := DisplaySettings{Rotation: 90, Output: "HDMI-A-1", TestPattern: true}
	rec = e.do(http.MethodPut, displayPath, want, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status %d: %s", rec.Code, rec.Body)
	}
	if got := decode[DisplaySettings](t, rec); got != want {
		t.Errorf("PUT answered %+v", got)
	}
	if got := decode[DisplaySettings](t, e.do(http.MethodGet, displayPath, nil, o)); got != want {
		t.Errorf("GET after PUT = %+v", got)
	}
	// The kiosk state is public and reflects the stored settings.
	rec = e.do(http.MethodGet, kioskPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("kiosk status %d", rec.Code)
	}
	if got := decode[KioskState](t, rec); got.Rotation != 90 || got.Output != "HDMI-A-1" || !got.TestPattern {
		t.Errorf("kiosk state = %+v", got)
	}
}

func TestKioskStateIsPublicWithDefaults(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(http.MethodGet, kioskPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got := decode[KioskState](t, rec); got != (KioskState{}) {
		t.Errorf("defaults = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"rotation":0`) {
		t.Errorf("body %s must carry all fields", rec.Body)
	}
}

func TestUpdateDisplayRejectsBadValuesWithFieldErrors(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	good := DisplaySettings{Rotation: 180, Output: "DP-1", TestPattern: true}
	if rec := e.do(http.MethodPut, displayPath, good, o); rec.Code != http.StatusOK {
		t.Fatalf("setup PUT: %d", rec.Code)
	}

	tests := []struct {
		name      string
		body      DisplaySettings
		wantField string
		wantMsg   string
		other     string
	}{
		{"rotation 45", DisplaySettings{Rotation: 45}, "rotation", "Erlaubt sind 0°, 90°, 180° und 270°.", "output"},
		{"bad output", DisplaySettings{Rotation: 90, Output: "a;rm"}, "output", "Erlaubt sind 1 bis 32 Zeichen aus Buchstaben, Ziffern, Punkt, Bindestrich und Unterstrich.", "rotation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := wantProblem(t, e.do(http.MethodPut, displayPath, tt.body, o), http.StatusBadRequest, "")
			if p.Errors == nil || (*p.Errors)[tt.wantField] != tt.wantMsg {
				t.Fatalf("errors = %v", p.Errors)
			}
			if _, ok := (*p.Errors)[tt.other]; ok || len(*p.Errors) != 1 {
				t.Errorf("unexpected extra errors: %v", *p.Errors)
			}
			if got := decode[DisplaySettings](t, e.do(http.MethodGet, displayPath, nil, o)); got != good {
				t.Errorf("state changed to %+v", got)
			}
		})
	}
}
