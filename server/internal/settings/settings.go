// Package settings holds the persisted mirror settings (settings.json).
package settings

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// SchemaVersion is the version of the settings file format. A file with any
// other version is discarded, not migrated (nothing is deployed yet).
const SchemaVersion = 1

// Messages of the field errors, keyed by field path in Validate.
const (
	msgRotation = "Erlaubt sind 0°, 90°, 180° und 270°."
	msgOutput   = "Erlaubt sind 1 bis 32 Zeichen aus Buchstaben, Ziffern, Punkt, Bindestrich und Unterstrich."

	msgLocation       = "Höchstens 100 Zeichen."
	msgAPIKeyLength   = "Höchstens 200 Zeichen."
	msgRefresh        = "Erlaubt sind 10 bis 1440 Minuten."
	msgSSIDLength     = "Höchstens 32 Bytes."
	msgSecurity       = "Erlaubt sind WPA oder offen."
	msgWifiPassword   = "Das WLAN-Passwort muss 8 bis 63 Zeichen haben."
	msgSecretConflict = "Sende entweder einen neuen Wert oder lösche den gespeicherten, nicht beides."
)

// outputPattern is strict because the kiosk agent passes the value to
// wlr-randr as a command argument.
var outputPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// DisplaySettings control the mirror's screen.
type DisplaySettings struct {
	// Rotation in degrees clockwise: 0, 90, 180 or 270.
	Rotation int `json:"rotation"`
	// Output is a connector name such as HDMI-A-1; "" means automatic.
	Output string `json:"output"`
	// TestPattern shows the diagnose overlay on the mirror.
	TestPattern bool `json:"testPattern"`
}

// Wi-Fi security values.
const (
	SecurityWPA    = "WPA"
	SecurityNoPass = "nopass"
)

// WeatherSettings configure the weather widget. The API key is a secret: it
// is stored here but never leaves the server.
type WeatherSettings struct {
	// Location is a place name for OpenWeather's geocoder; "" disables weather.
	Location       string `json:"location"`
	APIKey         string `json:"apiKey"`
	ShowIcons      bool   `json:"showIcons"`
	RefreshMinutes int    `json:"refreshMinutes"`
}

// WifiSettings configure the guest Wi-Fi QR code. The password is a secret.
type WifiSettings struct {
	// SSID is the network name; "" disables the QR code.
	SSID     string `json:"ssid"`
	Security string `json:"security"`
	Password string `json:"password"`
}

// Settings is the content of settings.json. Later phases add sections; a
// file written before a section existed still loads (see parse).
type Settings struct {
	SchemaVersion int             `json:"schemaVersion"`
	Display       DisplaySettings `json:"display"`
	Weather       WeatherSettings `json:"weather"`
	Wifi          WifiSettings    `json:"wifi"`
}

// Defaults returns the settings used while no file exists.
func Defaults() Settings {
	return Settings{
		SchemaVersion: SchemaVersion,
		Weather:       WeatherSettings{ShowIcons: true, RefreshMinutes: 15},
		Wifi:          WifiSettings{Security: SecurityWPA},
	}
}

// Validate returns a German message per invalid field path; nil means valid.
func (s Settings) Validate() map[string]string {
	errs := map[string]string{}
	switch s.Display.Rotation {
	case 0, 90, 180, 270:
	default:
		errs["display.rotation"] = msgRotation
	}
	if s.Display.Output != "" && !outputPattern.MatchString(s.Display.Output) {
		errs["display.output"] = msgOutput
	}
	w := s.Weather
	if utf8.RuneCountInString(w.Location) > 100 {
		errs["weather.location"] = msgLocation
	}
	if utf8.RuneCountInString(w.APIKey) > 200 {
		errs["weather.apiKey"] = msgAPIKeyLength
	}
	if w.RefreshMinutes < 10 || w.RefreshMinutes > 1440 {
		errs["weather.refreshMinutes"] = msgRefresh
	}
	f := s.Wifi
	if len(f.SSID) > 32 {
		errs["wifi.ssid"] = msgSSIDLength
	}
	switch f.Security {
	case SecurityWPA, SecurityNoPass:
	default:
		errs["wifi.security"] = msgSecurity
	}
	pwLen := utf8.RuneCountInString(f.Password)
	if pwLen > 63 || (f.SSID != "" && f.Security == SecurityWPA && pwLen < 8) {
		errs["wifi.password"] = msgWifiPassword
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// WeatherUpdate is a change of the weather section. An empty APIKey keeps the
// stored key; ClearAPIKey deletes it.
type WeatherUpdate struct {
	Location       string
	APIKey         string
	ClearAPIKey    bool
	ShowIcons      bool
	RefreshMinutes int
}

// WifiUpdate is a change of the Wi-Fi section. An empty Password keeps the
// stored one; ClearPassword deletes it.
type WifiUpdate struct {
	SSID          string
	Security      string
	Password      string
	ClearPassword bool
}

// apply merges u into the current section; a conflicting request is reported
// as a field error.
func (u WeatherUpdate) apply(cur WeatherSettings) (WeatherSettings, map[string]string) {
	next := WeatherSettings{
		Location:       strings.TrimSpace(u.Location),
		APIKey:         cur.APIKey,
		ShowIcons:      u.ShowIcons,
		RefreshMinutes: u.RefreshMinutes,
	}
	key := strings.TrimSpace(u.APIKey)
	switch {
	case u.ClearAPIKey && key != "":
		return cur, map[string]string{"apiKey": msgSecretConflict}
	case u.ClearAPIKey:
		next.APIKey = ""
	case key != "":
		next.APIKey = key
	}
	return next, nil
}

func (u WifiUpdate) apply(cur WifiSettings) (WifiSettings, map[string]string) {
	next := WifiSettings{SSID: u.SSID, Security: u.Security, Password: cur.Password}
	switch {
	case u.ClearPassword && u.Password != "":
		return cur, map[string]string{"password": msgSecretConflict}
	case u.ClearPassword:
		next.Password = ""
	case u.Password != "":
		next.Password = u.Password
	}
	return next, nil
}
