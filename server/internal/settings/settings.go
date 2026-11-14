// Package settings holds the persisted mirror settings (settings.json).
package settings

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/thazzr/mirrornextgen/server/internal/modules"
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
	msgMaxModules     = "Höchstens 12 Module je Region."
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

// maxModulesPerRegion bounds a layout region; the dashboard has no room for more.
const maxModulesPerRegion = 12

// Layout places modules on the five regions of the dashboard. Each region is
// an ordered list of module keys; a module in no region is switched off.
type Layout struct {
	TopLeft     []string `json:"topLeft"`
	TopRight    []string `json:"topRight"`
	Middle      []string `json:"middle"`
	BottomLeft  []string `json:"bottomLeft"`
	BottomRight []string `json:"bottomRight"`
}

// LayoutRegions are the region names in layout order, also the field names
// of validation errors.
var LayoutRegions = []string{"topLeft", "topRight", "middle", "bottomLeft", "bottomRight"}

// regions returns pointers to the lists in LayoutRegions order.
func (l *Layout) regions() []*[]string {
	return []*[]string{&l.TopLeft, &l.TopRight, &l.Middle, &l.BottomLeft, &l.BottomRight}
}

// normalized returns a deep copy in which no region is nil (JSON must carry
// [] rather than null).
func (l Layout) normalized() Layout {
	out := Layout{}
	src, dst := l.regions(), out.regions()
	for i := range src {
		*dst[i] = append([]string{}, *src[i]...)
	}
	return out
}

// Filtered returns a copy of the layout without the keys keep rejects.
func (l Layout) Filtered(keep func(key string) bool) Layout {
	out := Layout{}
	src, dst := l.regions(), out.regions()
	for i := range src {
		*dst[i] = []string{}
		for _, k := range *src[i] {
			if keep(k) {
				*dst[i] = append(*dst[i], k)
			}
		}
	}
	return out
}

// Equal reports whether both layouts hold the same lists.
func (l Layout) Equal(o Layout) bool {
	a, b := l.regions(), o.regions()
	for i := range a {
		if !slices.Equal(*a[i], *b[i]) {
			return false
		}
	}
	return true
}

// DefaultLayout is the layout while none is stored.
func DefaultLayout() Layout {
	return Layout{
		TopLeft:     []string{"clock"},
		TopRight:    []string{"weather"},
		Middle:      []string{},
		BottomLeft:  []string{},
		BottomRight: []string{"wifi"},
	}
}

// Settings is the content of settings.json. Later phases add sections; a
// file written before a section existed still loads (see parse). Layout
// holds slices, so Settings is compared with Equal, not ==.
type Settings struct {
	SchemaVersion int             `json:"schemaVersion"`
	Display       DisplaySettings `json:"display"`
	Weather       WeatherSettings `json:"weather"`
	Wifi          WifiSettings    `json:"wifi"`
	Layout        Layout          `json:"layout"`
}

// moduleInput hands the modules directory what it needs to know about the
// settings (nothing yet; calendar sections will come here).
func (s Settings) moduleInput() modules.Input { return modules.Input{} }

// ModuleAvailable reports whether key names a module that exists under s.
func (s Settings) ModuleAvailable(key string) bool {
	return modules.IsAvailable(s.moduleInput(), key)
}

// Equal reports whether both settings hold the same content.
func (s Settings) Equal(o Settings) bool {
	return s.SchemaVersion == o.SchemaVersion && s.Display == o.Display &&
		s.Weather == o.Weather && s.Wifi == o.Wifi && s.Layout.Equal(o.Layout)
}

// clone deep-copies the parts of s that are slices.
func (s Settings) clone() Settings {
	s.Layout = s.Layout.normalized()
	return s
}

// validateLayout reports field errors keyed by region name. A duplicate is
// reported on the region of its second occurrence.
func (s Settings) validateLayout(errs map[string]string) {
	in := s.moduleInput()
	seen := map[string]bool{}
	l := s.Layout
	for i, region := range l.regions() {
		name := LayoutRegions[i]
		if len(*region) > maxModulesPerRegion {
			errs["layout."+name] = msgMaxModules
		}
		for _, key := range *region {
			var msg string
			switch {
			case !modules.IsAvailable(in, key):
				msg = fmt.Sprintf("Unbekanntes Modul „%s“.", key)
			case seen[key]:
				msg = fmt.Sprintf("„%s“ steht mehrfach in der Anordnung.", key)
			}
			seen[key] = true
			if msg != "" {
				if _, taken := errs["layout."+name]; !taken {
					errs["layout."+name] = msg
				}
			}
		}
	}
}

// Defaults returns the settings used while no file exists.
func Defaults() Settings {
	return Settings{
		SchemaVersion: SchemaVersion,
		Weather:       WeatherSettings{ShowIcons: true, RefreshMinutes: 15},
		Wifi:          WifiSettings{Security: SecurityWPA},
		Layout:        DefaultLayout(),
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
	s.validateLayout(errs)
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
