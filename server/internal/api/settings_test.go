package api

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// nightly is a valid, enabled night schedule.
var nightly = DisplaySchedule{Enabled: true, OffTime: "23:00", OnTime: "06:00"}

const (
	displayPath = "/api/v1/settings/display"
	kioskPath   = "/api/v1/kiosk/state"
)

func TestDisplaySettingsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)

	wantProblem(t, e.do(http.MethodGet, displayPath, nil, opts{}), http.StatusUnauthorized, "")
	body := DisplaySettings{Rotation: 90, Schedule: nightly}
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
	if got := decode[DisplaySettings](t, rec); got != (DisplaySettings{Schedule: DisplaySchedule{OffTime: "23:00", OnTime: "06:00"}}) {
		t.Errorf("defaults = %+v", got)
	}

	want := DisplaySettings{Rotation: 90, Output: "HDMI-A-1", TestPattern: true, Schedule: nightly}
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
	if got := decode[KioskState](t, rec); got != (KioskState{PowerOn: true}) {
		t.Errorf("defaults = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"rotation":0`) || !strings.Contains(rec.Body.String(), `"powerOn":true`) {
		t.Errorf("body %s must carry all fields", rec.Body)
	}
}

func TestUpdateDisplayRejectsBadValuesWithFieldErrors(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	good := DisplaySettings{Rotation: 180, Output: "DP-1", TestPattern: true, Schedule: nightly}
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
		{"rotation 45", DisplaySettings{Rotation: 45, Schedule: nightly}, "rotation", "Erlaubt sind 0°, 90°, 180° und 270°.", "output"},
		{"bad output", DisplaySettings{Rotation: 90, Output: "a;rm", Schedule: nightly}, "output", "Erlaubt sind 1 bis 32 Zeichen aus Buchstaben, Ziffern, Punkt, Bindestrich und Unterstrich.", "rotation"},
		{"bad off time", DisplaySettings{Rotation: 90, Schedule: DisplaySchedule{Enabled: true, OffTime: "25:00", OnTime: "06:00"}}, "schedule.offTime", "Erlaubt ist eine Uhrzeit im Format HH:mm.", "schedule.onTime"},
		{"bad on time", DisplaySettings{Rotation: 90, Schedule: DisplaySchedule{OffTime: "23:00", OnTime: ""}}, "schedule.onTime", "Erlaubt ist eine Uhrzeit im Format HH:mm.", "schedule.offTime"},
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

const (
	statusPath  = "/api/v1/display/status"
	testOffPath = "/api/v1/display/test-off"
)

// putSchedule stores a schedule next to otherwise default display settings.
func putSchedule(t *testing.T, e *testEnv, o opts, sched DisplaySchedule) {
	t.Helper()
	if rec := e.do(http.MethodPut, displayPath, DisplaySettings{Schedule: sched}, o); rec.Code != http.StatusOK {
		t.Fatalf("PUT schedule: %d %s", rec.Code, rec.Body)
	}
}

func TestKioskStatePowerOnFollowsTheSchedule(t *testing.T) {
	e := newTestEnv(t) // 2026-11-14 12:00 UTC = 13:00 in Berlin
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	putSchedule(t, e, o, nightly)

	if got := decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{})); !got.PowerOn {
		t.Error("powerOn false at 13:00")
	}
	e.clock.add(10*time.Hour + 30*time.Minute) // 23:30 Berlin
	if got := decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{})); got.PowerOn {
		t.Error("powerOn true at 23:30")
	}
	e.clock.add(6*time.Hour + 30*time.Minute) // 06:00 Berlin, the on time itself
	if got := decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{})); !got.PowerOn {
		t.Error("powerOn false at exactly the on time")
	}
}

func TestKioskStatePowerOnIsTrueWithoutAnEnabledSchedule(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	putSchedule(t, e, o, DisplaySchedule{Enabled: false, OffTime: "00:00", OnTime: "23:59"})
	e.clock.add(11 * time.Hour) // 00:00 Berlin
	if got := decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{})); !got.PowerOn {
		t.Error("a disabled schedule switched the monitor off")
	}
}

func TestDisplayStatusEndpoint(t *testing.T) {
	e := newTestEnv(t)
	wantProblem(t, e.do(http.MethodGet, statusPath, nil, opts{}), http.StatusUnauthorized, "")
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}

	// No schedule: always on, no next switch.
	st := decode[DisplayStatus](t, e.do(http.MethodGet, statusPath, nil, o))
	if !st.PowerOn || st.Reason != "always" || st.NextSwitchAt != nil || st.NextSwitchOn != nil || st.TimeZone != "Europe/Berlin" {
		t.Errorf("status without schedule = %+v", st)
	}
	if want := e.clock.now(); !st.Now.Equal(want) {
		t.Errorf("now = %v, want %v", st.Now, want)
	}
	if _, off := st.Now.Zone(); off != 3600 {
		t.Errorf("now carries offset %d s, want Berlin's 3600", off)
	}

	putSchedule(t, e, o, nightly)
	st = decode[DisplayStatus](t, e.do(http.MethodGet, statusPath, nil, o))
	wantSwitch := time.Date(2026, 11, 14, 22, 0, 0, 0, time.UTC) // 23:00 Berlin
	if !st.PowerOn || st.Reason != "schedule" || st.NextSwitchAt == nil || !st.NextSwitchAt.Equal(wantSwitch) || st.NextSwitchOn == nil || *st.NextSwitchOn {
		t.Errorf("daytime status = %+v", st)
	}

	// Off from 12:00 to 18:00 Berlin; it is 13:00 now. (The clock is not moved:
	// the admin session would expire.)
	putSchedule(t, e, o, DisplaySchedule{Enabled: true, OffTime: "12:00", OnTime: "18:00"})
	st = decode[DisplayStatus](t, e.do(http.MethodGet, statusPath, nil, o))
	wantSwitch = time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC) // 18:00 Berlin
	if st.PowerOn || st.Reason != "schedule" || st.NextSwitchAt == nil || !st.NextSwitchAt.Equal(wantSwitch) || st.NextSwitchOn == nil || !*st.NextSwitchOn {
		t.Errorf("night status = %+v", st)
	}
}

func TestDisplayTestOffNeedsSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	wantProblem(t, e.do(http.MethodPost, testOffPath, nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.do(http.MethodPost, testOffPath, nil, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPost, testOffPath, nil, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")
	// Nothing of the refused requests switched the monitor off.
	if got := decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{})); !got.PowerOn {
		t.Error("a refused test-off switched the monitor off")
	}
}

func TestDisplayTestOffSwitchesOffForTenSecondsAndExtends(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}

	rec := e.do(http.MethodPost, testOffPath, nil, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if st := decode[DisplayStatus](t, rec); st.PowerOn || st.Reason != "test" {
		t.Errorf("answer = %+v", st)
	}
	state := func() KioskState { return decode[KioskState](t, e.do(http.MethodGet, kioskPath, nil, opts{})) }
	if state().PowerOn {
		t.Error("kiosk state still on right after test-off")
	}
	e.clock.add(9 * time.Second)
	if state().PowerOn {
		t.Error("on again after 9 s")
	}
	if st := decode[DisplayStatus](t, e.do(http.MethodGet, statusPath, nil, o)); st.PowerOn || st.Reason != "test" {
		t.Errorf("status during test = %+v", st)
	}
	// A second call extends the override by ten seconds from now.
	e.do(http.MethodPost, testOffPath, nil, o)
	e.clock.add(9 * time.Second)
	if state().PowerOn {
		t.Error("second call did not extend the override")
	}
	e.clock.add(time.Second)
	if !state().PowerOn {
		t.Error("still off after the override expired")
	}
	if st := decode[DisplayStatus](t, e.do(http.MethodGet, statusPath, nil, o)); !st.PowerOn || st.Reason != "always" {
		t.Errorf("status after the test = %+v", st)
	}
}
