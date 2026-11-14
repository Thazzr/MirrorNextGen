package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thazzr/mirrornextgen/server/internal/display"
)

func TestOldFileWithOnlyDisplayLoadsAdditively(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	old := `{"schemaVersion":1,"display":{"rotation":90,"output":"DP-1","testPattern":true}}`
	if err := os.WriteFile(p, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if got.Display != (DisplaySettings{Rotation: 90, Output: "DP-1", TestPattern: true, Schedule: display.Schedule{OffTime: "23:00", OnTime: "06:00"}}) {
		t.Errorf("display lost: %+v", got.Display)
	}
	if got.Weather != (WeatherSettings{ShowIcons: true, RefreshMinutes: 15}) {
		t.Errorf("weather defaults missing: %+v", got.Weather)
	}
	if got.Wifi != (WifiSettings{Security: "WPA"}) {
		t.Errorf("wifi defaults missing: %+v", got.Wifi)
	}
	// Nothing was discarded.
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file moved aside: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("unexpected files: %v", entries)
	}
}

func TestPartialSectionKeepsOtherDefaults(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"weather":{"location":"Berlin","apiKey":"k"}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := Load(dir, quiet)
	w := s.Get().Weather
	if w.Location != "Berlin" || !w.ShowIcons || w.RefreshMinutes != 15 {
		t.Errorf("weather = %+v", w)
	}
}

func TestValidateWeather(t *testing.T) {
	tests := []struct {
		name  string
		w     WeatherSettings
		field string
		msg   string
	}{
		{"defaults", Defaults().Weather, "", ""},
		{"min refresh", WeatherSettings{RefreshMinutes: 10}, "", ""},
		{"max refresh", WeatherSettings{RefreshMinutes: 1440}, "", ""},
		{"refresh 9", WeatherSettings{RefreshMinutes: 9}, "weather.refreshMinutes", "Erlaubt sind 10 bis 1440 Minuten."},
		{"refresh 1441", WeatherSettings{RefreshMinutes: 1441}, "weather.refreshMinutes", "Erlaubt sind 10 bis 1440 Minuten."},
		{"location 100", WeatherSettings{RefreshMinutes: 15, Location: strings.Repeat("ä", 100)}, "", ""},
		{"location 101", WeatherSettings{RefreshMinutes: 15, Location: strings.Repeat("a", 101)}, "weather.location", "Höchstens 100 Zeichen."},
		{"key 200", WeatherSettings{RefreshMinutes: 15, APIKey: strings.Repeat("k", 200)}, "", ""},
		{"key 201", WeatherSettings{RefreshMinutes: 15, APIKey: strings.Repeat("k", 201)}, "weather.apiKey", "Höchstens 200 Zeichen."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := Defaults()
			st.Weather = tt.w
			checkField(t, st.Validate(), tt.field, tt.msg)
		})
	}
}

func TestValidateWifi(t *testing.T) {
	pw63 := strings.Repeat("p", 63)
	const pwMsg = "Das WLAN-Passwort muss 8 bis 63 Zeichen haben."
	tests := []struct {
		name  string
		w     WifiSettings
		field string
		msg   string
	}{
		{"disabled", WifiSettings{Security: "WPA"}, "", ""},
		{"wpa ok", WifiSettings{SSID: "Home", Security: "WPA", Password: "12345678"}, "", ""},
		{"wpa 63", WifiSettings{SSID: "Home", Security: "WPA", Password: pw63}, "", ""},
		{"wpa 64", WifiSettings{SSID: "Home", Security: "WPA", Password: pw63 + "p"}, "wifi.password", pwMsg},
		{"wpa 7", WifiSettings{SSID: "Home", Security: "WPA", Password: "1234567"}, "wifi.password", pwMsg},
		{"wpa none", WifiSettings{SSID: "Home", Security: "WPA"}, "wifi.password", pwMsg},
		{"wpa without ssid needs no password", WifiSettings{Security: "WPA"}, "", ""},
		{"open without password", WifiSettings{SSID: "Cafe", Security: "nopass"}, "", ""},
		{"ssid 32 bytes", WifiSettings{SSID: strings.Repeat("a", 32), Security: "nopass"}, "", ""},
		{"ssid 33 bytes", WifiSettings{SSID: strings.Repeat("a", 33), Security: "nopass"}, "wifi.ssid", "Höchstens 32 Bytes."},
		{"ssid counts bytes not runes", WifiSettings{SSID: strings.Repeat("ä", 17), Security: "nopass"}, "wifi.ssid", "Höchstens 32 Bytes."},
		{"security WEP", WifiSettings{SSID: "x", Security: "WEP"}, "wifi.security", "Erlaubt sind WPA oder offen."},
		{"security empty", WifiSettings{Security: ""}, "wifi.security", "Erlaubt sind WPA oder offen."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := Defaults()
			st.Wifi = tt.w
			checkField(t, st.Validate(), tt.field, tt.msg)
		})
	}
}

func checkField(t *testing.T, errs map[string]string, field, msg string) {
	t.Helper()
	if field == "" {
		if errs != nil {
			t.Fatalf("errs = %v, want valid", errs)
		}
		return
	}
	if errs[field] != msg || len(errs) != 1 {
		t.Fatalf("errs = %v, want only %s=%q", errs, field, msg)
	}
}

func TestWeatherSecretSemantics(t *testing.T) {
	s, _ := Load(t.TempDir(), quiet)
	base := WeatherUpdate{Location: " Berlin ", APIKey: " key-1 ", ShowIcons: true, RefreshMinutes: 20}
	got, errs, err := s.UpdateWeather(base)
	if err != nil || errs != nil {
		t.Fatal(errs, err)
	}
	if got.Location != "Berlin" || got.APIKey != "key-1" {
		t.Fatalf("trim: %+v", got)
	}

	// Empty key keeps the stored one.
	keep := base
	keep.APIKey = ""
	keep.RefreshMinutes = 30
	if got, _, _ = s.UpdateWeather(keep); got.APIKey != "key-1" || got.RefreshMinutes != 30 {
		t.Errorf("keep: %+v", got)
	}
	// A new key replaces it.
	repl := base
	repl.APIKey = "key-2"
	if got, _, _ = s.UpdateWeather(repl); got.APIKey != "key-2" {
		t.Errorf("replace: %+v", got)
	}
	// Both a new key and clear is a field error and changes nothing.
	both := base
	both.APIKey, both.ClearAPIKey = "key-3", true
	_, errs, _ = s.UpdateWeather(both)
	if errs["apiKey"] == "" {
		t.Errorf("conflict not reported: %v", errs)
	}
	if s.Get().Weather.APIKey != "key-2" {
		t.Errorf("conflict changed the key")
	}
	// Clear deletes it.
	clr := base
	clr.APIKey, clr.ClearAPIKey = "", true
	if got, _, _ = s.UpdateWeather(clr); got.APIKey != "" {
		t.Errorf("clear: %+v", got)
	}
}

func TestWifiSecretSemanticsAndPersistence(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, quiet)
	up := WifiUpdate{SSID: "Home", Security: "WPA", Password: "geheim123"}
	if _, errs, err := s.UpdateWifi(up); errs != nil || err != nil {
		t.Fatal(errs, err)
	}
	// Changing only the SSID keeps the password.
	got, errs, _ := s.UpdateWifi(WifiUpdate{SSID: "Home2", Security: "WPA"})
	if errs != nil || got.Password != "geheim123" || got.SSID != "Home2" {
		t.Fatalf("keep: %+v %v", got, errs)
	}
	// Survives a reload (the secret is persisted in the file).
	s2, _ := Load(dir, quiet)
	if s2.Get().Wifi.Password != "geheim123" {
		t.Errorf("reloaded: %+v", s2.Get().Wifi)
	}
	// Clearing a required password fails validation and changes nothing.
	_, errs, _ = s.UpdateWifi(WifiUpdate{SSID: "Home2", Security: "WPA", ClearPassword: true})
	if errs["password"] == "" {
		t.Errorf("clear with WPA must be rejected: %v", errs)
	}
	if s.Get().Wifi.Password != "geheim123" {
		t.Error("rejected clear changed the password")
	}
	// Switching to an open network and clearing works.
	got, errs, _ = s.UpdateWifi(WifiUpdate{SSID: "Home2", Security: "nopass", ClearPassword: true})
	if errs != nil || got.Password != "" || got.Security != "nopass" {
		t.Errorf("open: %+v %v", got, errs)
	}
	// Conflict.
	_, errs, _ = s.UpdateWifi(WifiUpdate{SSID: "x", Security: "nopass", Password: "abcdefgh", ClearPassword: true})
	if errs["password"] == "" {
		t.Errorf("conflict not reported: %v", errs)
	}
}

func TestSectionErrorsCarryNoSectionPrefix(t *testing.T) {
	s, _ := Load(t.TempDir(), quiet)
	_, errs, _ := s.UpdateWeather(WeatherUpdate{RefreshMinutes: 1})
	if len(errs) != 1 || errs["refreshMinutes"] == "" {
		t.Errorf("errs = %v (keys must carry no section prefix)", errs)
	}
}

func TestSettingsFileEncodingNamesSecrets(t *testing.T) {
	// The persisted file holds the secrets (mode 0600); this pins the JSON
	// names so a rename cannot silently drop them.
	data, _ := json.Marshal(Settings{Weather: WeatherSettings{APIKey: "abc"}, Wifi: WifiSettings{Password: "pw"}})
	for _, want := range []string{`"apiKey":"abc"`, `"password":"pw"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("%s missing in %s", want, data)
		}
	}
}

func TestOldDisplayWithoutScheduleGetsTheDefaultSchedule(t *testing.T) {
	dir := t.TempDir()
	old := `{"schemaVersion":1,"display":{"rotation":0,"output":"","testPattern":false}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Display.Schedule; got != (display.Schedule{Enabled: false, OffTime: "23:00", OnTime: "06:00"}) {
		t.Errorf("schedule = %+v", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("file moved aside: %v", entries)
	}
}

func TestPartialScheduleInFileKeepsTheOtherDefaults(t *testing.T) {
	dir := t.TempDir()
	old := `{"schemaVersion":1,"display":{"rotation":0,"output":"","testPattern":false,"schedule":{"enabled":true,"offTime":"22:00"}}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Display.Schedule; got != (display.Schedule{Enabled: true, OffTime: "22:00", OnTime: "06:00"}) {
		t.Errorf("schedule = %+v", got)
	}
}
