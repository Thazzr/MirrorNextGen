package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/thazzr/mirrornextgen/server/internal/wifi"
)

const (
	weatherPath     = "/api/v1/settings/weather"
	wifiPath        = "/api/v1/settings/wifi"
	dashWeatherPath = "/api/v1/dashboard/weather"
	dashWifiPath    = "/api/v1/dashboard/wifi"
	secretAPIKey    = "ow-SECRET-key-0123456789"
	secretWifiPass  = "wifi-SECRET-pass-42"
)

func ptr[T any](v T) *T { return &v }

func authed(t *testing.T, e *testEnv) opts {
	t.Helper()
	c := e.setup(t)
	return opts{cookie: c.cookie, csrf: c.csrf}
}

func TestWeatherAndWifiSettingsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	wb := WeatherSettingsUpdate{Location: "Berlin", ShowIcons: true, RefreshMinutes: 15, ApiKey: ptr(secretAPIKey)}
	fb := WifiSettingsUpdate{Ssid: "Home", Security: "WPA", Password: ptr(secretWifiPass)}
	for _, path := range []string{weatherPath, wifiPath} {
		body := any(wb)
		if path == wifiPath {
			body = fb
		}
		wantProblem(t, e.do(http.MethodGet, path, nil, opts{}), http.StatusUnauthorized, "")
		wantProblem(t, e.do(http.MethodPut, path, body, opts{}), http.StatusUnauthorized, "")
		wantProblem(t, e.do(http.MethodPut, path, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
		wantProblem(t, e.do(http.MethodPut, path, body, opts{cookie: c.cookie, csrf: "wrong"}), http.StatusForbidden, "")
	}
	// Nothing of the refused requests was stored.
	o := opts{cookie: c.cookie, csrf: c.csrf}
	if got := decode[WeatherSettings](t, e.do(http.MethodGet, weatherPath, nil, o)); got.Location != "" || got.HasApiKey {
		t.Errorf("refused PUT changed weather: %+v", got)
	}
	if got := decode[WifiSettings](t, e.do(http.MethodGet, wifiPath, nil, o)); got.Ssid != "" || got.HasPassword {
		t.Errorf("refused PUT changed wifi: %+v", got)
	}
}

func TestWeatherSettingsDefaultsAndSecretSemantics(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)

	got := decode[WeatherSettings](t, e.do(http.MethodGet, weatherPath, nil, o))
	if got != (WeatherSettings{ShowIcons: true, RefreshMinutes: 15}) {
		t.Errorf("defaults = %+v", got)
	}

	// Set a key: the answer and later reads report only that one is set.
	rec := e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Berlin", ApiKey: ptr(secretAPIKey), ShowIcons: false, RefreshMinutes: 30}, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	if got := decode[WeatherSettings](t, rec); got != (WeatherSettings{Location: "Berlin", HasApiKey: true, ShowIcons: false, RefreshMinutes: 30}) {
		t.Errorf("PUT answered %+v", got)
	}
	assertNoSecret := func(name string, rec interface{ String() string }) {
		t.Helper()
		if strings.Contains(rec.String(), secretAPIKey) {
			t.Errorf("%s leaks the API key: %s", name, rec.String())
		}
	}
	assertNoSecret("PUT body", rec.Body)
	get := e.do(http.MethodGet, weatherPath, nil, o)
	assertNoSecret("GET body", get.Body)
	if strings.Contains(get.Body.String(), `"apiKey"`) {
		t.Errorf("GET exposes an apiKey field: %s", get.Body)
	}

	// Absent or empty key keeps it.
	for name, key := range map[string]*string{"absent": nil, "empty": ptr("")} {
		rec := e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Hamburg", ApiKey: key, ShowIcons: true, RefreshMinutes: 20}, o)
		if got := decode[WeatherSettings](t, rec); !got.HasApiKey || got.Location != "Hamburg" {
			t.Errorf("%s key: %+v", name, got)
		}
	}
	// The kept key still works: the dashboard can use it (see the dashboard test).

	// Clear deletes it.
	rec = e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Hamburg", ClearApiKey: ptr(true), ShowIcons: true, RefreshMinutes: 20}, o)
	if got := decode[WeatherSettings](t, rec); got.HasApiKey {
		t.Errorf("clear: %+v", got)
	}
}

func TestWeatherSettingsFieldErrors(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	good := WeatherSettingsUpdate{Location: "Berlin", ApiKey: ptr("key"), ShowIcons: true, RefreshMinutes: 15}
	if rec := e.do(http.MethodPut, weatherPath, good, o); rec.Code != http.StatusOK {
		t.Fatalf("setup PUT: %d", rec.Code)
	}
	tests := []struct {
		name  string
		body  WeatherSettingsUpdate
		field string
		msg   string
	}{
		{"location too long", WeatherSettingsUpdate{Location: strings.Repeat("a", 101), ShowIcons: true, RefreshMinutes: 15}, "location", "Höchstens 100 Zeichen."},
		{"refresh too small", WeatherSettingsUpdate{Location: "x", ShowIcons: true, RefreshMinutes: 9}, "refreshMinutes", "Erlaubt sind 10 bis 1440 Minuten."},
		{"refresh too large", WeatherSettingsUpdate{Location: "x", ShowIcons: true, RefreshMinutes: 1441}, "refreshMinutes", "Erlaubt sind 10 bis 1440 Minuten."},
		{"key too long", WeatherSettingsUpdate{Location: "x", ApiKey: ptr(strings.Repeat("k", 201)), ShowIcons: true, RefreshMinutes: 15}, "apiKey", "Höchstens 200 Zeichen."},
		{"key and clear together", WeatherSettingsUpdate{Location: "x", ApiKey: ptr("new"), ClearApiKey: ptr(true), ShowIcons: true, RefreshMinutes: 15}, "apiKey", "Sende entweder einen neuen Wert oder lösche den gespeicherten, nicht beides."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := wantProblem(t, e.do(http.MethodPut, weatherPath, tt.body, o), http.StatusBadRequest, "")
			if p.Errors == nil || (*p.Errors)[tt.field] != tt.msg || len(*p.Errors) != 1 {
				t.Fatalf("errors = %v", p.Errors)
			}
			got := decode[WeatherSettings](t, e.do(http.MethodGet, weatherPath, nil, o))
			if got != (WeatherSettings{Location: "Berlin", HasApiKey: true, ShowIcons: true, RefreshMinutes: 15}) {
				t.Errorf("state changed to %+v", got)
			}
		})
	}
}

func TestWifiSettingsDefaultsAndSecretSemantics(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)

	if got := decode[WifiSettings](t, e.do(http.MethodGet, wifiPath, nil, o)); got != (WifiSettings{Security: "WPA"}) {
		t.Errorf("defaults = %+v", got)
	}
	rec := e.do(http.MethodPut, wifiPath, WifiSettingsUpdate{Ssid: "Home", Security: "WPA", Password: ptr(secretWifiPass)}, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	if got := decode[WifiSettings](t, rec); got != (WifiSettings{Ssid: "Home", Security: "WPA", HasPassword: true}) {
		t.Errorf("PUT answered %+v", got)
	}
	if strings.Contains(rec.Body.String(), secretWifiPass) {
		t.Errorf("PUT body leaks the password: %s", rec.Body)
	}
	if get := e.do(http.MethodGet, wifiPath, nil, o); strings.Contains(get.Body.String(), secretWifiPass) {
		t.Errorf("GET body leaks the password: %s", get.Body)
	}

	// Changing the SSID without a password keeps the password.
	// A slice, not a map: the last SSID ("Home-empty") must be deterministic.
	for _, c := range []struct {
		name string
		pw   *string
	}{{"absent", nil}, {"empty", ptr("")}} {
		rec := e.do(http.MethodPut, wifiPath, WifiSettingsUpdate{Ssid: "Home-" + c.name, Security: "WPA", Password: c.pw}, o)
		if got := decode[WifiSettings](t, rec); !got.HasPassword {
			t.Errorf("%s password: %+v", c.name, got)
		}
	}
	// The kept password is the one in the QR code.
	qr := decode[DashboardWifi](t, e.do(http.MethodGet, dashWifiPath, nil, opts{}))
	want, _ := wifi.Encode(wifi.Payload("Home-empty", "WPA", secretWifiPass))
	if qr.Qr == nil || !slices.Equal(qr.Qr.Rows, want.Rows) {
		t.Error("the QR code does not encode the kept password")
	}

	// Open network, clear the password.
	rec = e.do(http.MethodPut, wifiPath, WifiSettingsUpdate{Ssid: "Cafe", Security: "nopass", ClearPassword: ptr(true)}, o)
	if got := decode[WifiSettings](t, rec); got != (WifiSettings{Ssid: "Cafe", Security: "nopass"}) {
		t.Errorf("open network: %+v", got)
	}
}

func TestWifiSettingsFieldErrors(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	good := WifiSettingsUpdate{Ssid: "Home", Security: "WPA", Password: ptr("12345678")}
	if rec := e.do(http.MethodPut, wifiPath, good, o); rec.Code != http.StatusOK {
		t.Fatalf("setup PUT: %d", rec.Code)
	}
	tests := []struct {
		name  string
		body  WifiSettingsUpdate
		field string
		msg   string
	}{
		{"ssid too long", WifiSettingsUpdate{Ssid: strings.Repeat("a", 33), Security: "nopass"}, "ssid", "Höchstens 32 Bytes."},
		{"unknown security", WifiSettingsUpdate{Ssid: "x", Security: "WEP"}, "security", "Erlaubt sind WPA oder offen."},
		{"password too short", WifiSettingsUpdate{Ssid: "x", Security: "WPA", Password: ptr("1234567")}, "password", "Das WLAN-Passwort muss 8 bis 63 Zeichen haben."},
		{"password too long", WifiSettingsUpdate{Ssid: "x", Security: "WPA", Password: ptr(strings.Repeat("p", 64))}, "password", "Das WLAN-Passwort muss 8 bis 63 Zeichen haben."},
		{"wpa without any password", WifiSettingsUpdate{Ssid: "x", Security: "WPA", ClearPassword: ptr(true)}, "password", "Das WLAN-Passwort muss 8 bis 63 Zeichen haben."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := wantProblem(t, e.do(http.MethodPut, wifiPath, tt.body, o), http.StatusBadRequest, "")
			if p.Errors == nil || (*p.Errors)[tt.field] != tt.msg || len(*p.Errors) != 1 {
				t.Fatalf("errors = %v", p.Errors)
			}
			got := decode[WifiSettings](t, e.do(http.MethodGet, wifiPath, nil, o))
			if got != (WifiSettings{Ssid: "Home", Security: "WPA", HasPassword: true}) {
				t.Errorf("state changed to %+v", got)
			}
		})
	}
}

func TestDashboardEndpointsArePublic(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)
	for _, path := range []string{dashWeatherPath, dashWifiPath} {
		rec := e.do(http.MethodGet, path, nil, opts{})
		if rec.Code != http.StatusOK {
			t.Errorf("%s without a session: %d %s", path, rec.Code, rec.Body)
		}
	}
}

func TestDashboardWeatherDisabled(t *testing.T) {
	e := newTestEnv(t)
	rec := e.do(http.MethodGet, dashWeatherPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"configured":false`) || !strings.Contains(got, `"stale":false`) ||
		strings.Contains(got, `"data"`) || strings.Contains(got, `"error"`) {
		t.Errorf("body = %s", got)
	}
	if n := e.ow.calls.Load(); n != 0 {
		t.Errorf("%d upstream calls while disabled", n)
	}
	// A location without a key is still disabled.
	o := authed(t, e)
	e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Berlin", ShowIcons: true, RefreshMinutes: 15}, o)
	if got := decode[DashboardWeather](t, e.do(http.MethodGet, dashWeatherPath, nil, opts{})); got.Configured {
		t.Errorf("location without key must be disabled: %+v", got)
	}
}

func TestDashboardWeatherEndToEnd(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Berlin", ApiKey: ptr(secretAPIKey), ShowIcons: true, RefreshMinutes: 15}, o)

	rec := e.do(http.MethodGet, dashWeatherPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), secretAPIKey) {
		t.Errorf("dashboard weather leaks the API key")
	}
	got := decode[DashboardWeather](t, rec)
	if !got.Configured || got.Stale || got.Error != nil || got.Data == nil {
		t.Fatalf("result = %+v", got)
	}
	d := got.Data
	if d.Location != "Berlin" || d.TemperatureC != 8 || !d.ShowIcons || len(d.Hourly) != 9 || len(d.Forecast) != 3 {
		t.Errorf("data = %+v", d)
	}
	if d.Forecast[0].Date.Format("2006-01-02") != "2026-11-15" || d.Forecast[0].Condition != "Leichter Regen" {
		t.Errorf("forecast = %+v", d.Forecast[0])
	}
	if d.IconCode == nil || *d.IconCode != "04d" || d.UvIndex == nil || d.Sunrise == nil {
		t.Errorf("optional fields: %+v", d)
	}
	// Times travel as ISO 8601 with an offset, dates as yyyy-MM-dd.
	body := rec.Body.String()
	for _, want := range []string{`"time":"2026-11-14T00:00:00+01:00"`, `"date":"2026-11-15"`, `"sunrise":"2026-11-14T07:33:00+01:00"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s", want)
		}
	}

	// A second poll is served from the cache.
	calls := e.ow.calls.Load()
	e.do(http.MethodGet, dashWeatherPath, nil, opts{})
	if e.ow.calls.Load() != calls {
		t.Errorf("poll hit the upstream again: %d -> %d", calls, e.ow.calls.Load())
	}

	// Saving the settings drops the cache: the next poll fetches again.
	e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Berlin", ShowIcons: false, RefreshMinutes: 15}, o)
	got = decode[DashboardWeather](t, e.do(http.MethodGet, dashWeatherPath, nil, opts{}))
	if e.ow.calls.Load() == calls {
		t.Error("settings change did not trigger a refresh")
	}
	if got.Data == nil || got.Data.ShowIcons {
		t.Errorf("showIcons not applied: %+v", got.Data)
	}
}

func TestDashboardWeatherUpstreamFailureIsReportedInTheBody(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)
	e.ow.status.Store(401)
	e.do(http.MethodPut, weatherPath, WeatherSettingsUpdate{Location: "Berlin", ApiKey: ptr(secretAPIKey), ShowIcons: true, RefreshMinutes: 15}, o)
	rec := e.do(http.MethodGet, dashWeatherPath, nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: failures must not be HTTP errors", rec.Code)
	}
	got := decode[DashboardWeather](t, rec)
	if !got.Configured || got.Data != nil || got.Error == nil || *got.Error != "OpenWeather One Call 3.0 ist für diesen API-Key nicht freigeschaltet." {
		t.Errorf("result = %+v", got)
	}
}

func TestDashboardWifi(t *testing.T) {
	e := newTestEnv(t)
	o := authed(t, e)

	rec := e.do(http.MethodGet, dashWifiPath, nil, opts{})
	if got := strings.TrimSpace(rec.Body.String()); got != `{"configured":false}` {
		t.Errorf("disabled body = %s", got)
	}

	e.do(http.MethodPut, wifiPath, WifiSettingsUpdate{Ssid: "Gäste;WLAN", Security: "WPA", Password: ptr("pass:word,1")}, o)
	got := decode[DashboardWifi](t, e.do(http.MethodGet, dashWifiPath, nil, opts{}))
	if !got.Configured || got.Ssid == nil || *got.Ssid != "Gäste;WLAN" || got.Qr == nil {
		t.Fatalf("result = %+v", got)
	}
	want, err := wifi.Encode(`WIFI:T:WPA;S:Gäste\;WLAN;P:pass\:word\,1;;`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Qr.Size != want.Size || !slices.Equal(got.Qr.Rows, want.Rows) {
		t.Error("QR code does not encode the escaped payload")
	}
	if len(got.Qr.Rows) != got.Qr.Size {
		t.Errorf("%d rows for size %d", len(got.Qr.Rows), got.Qr.Size)
	}

	// An open network encodes no password, whatever was stored before.
	e.do(http.MethodPut, wifiPath, WifiSettingsUpdate{Ssid: "Cafe", Security: "nopass"}, o)
	got = decode[DashboardWifi](t, e.do(http.MethodGet, dashWifiPath, nil, opts{}))
	open, _ := wifi.Encode(wifi.Payload("Cafe", "nopass", ""))
	if got.Qr == nil || !slices.Equal(got.Qr.Rows, open.Rows) {
		t.Error("open network QR code differs from the nopass payload")
	}

	// Clearing the SSID disables the widget again.
	e.do(http.MethodPut, wifiPath, WifiSettingsUpdate{Ssid: "", Security: "nopass"}, o)
	if got := decode[DashboardWifi](t, e.do(http.MethodGet, dashWifiPath, nil, opts{})); got.Configured || got.Qr != nil {
		t.Errorf("cleared: %+v", got)
	}
}
