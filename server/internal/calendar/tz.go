package calendar

import (
	"strings"
	"sync"
	"time"
)

// windowsZones maps the zone names Outlook and Exchange write into TZID to
// IANA names. Keys are lower case.
var windowsZones = map[string]string{
	"w. europe standard time":        "Europe/Berlin",
	"romance standard time":          "Europe/Paris",
	"gmt standard time":              "Europe/London",
	"central europe standard time":   "Europe/Budapest",
	"central european standard time": "Europe/Warsaw",
	"e. europe standard time":        "Europe/Chisinau",
	"fle standard time":              "Europe/Kyiv",
	"gtb standard time":              "Europe/Bucharest",
	"russian standard time":          "Europe/Moscow",
	"turkey standard time":           "Europe/Istanbul",
	"greenwich standard time":        "Atlantic/Reykjavik",
	"utc":                            "UTC",
	"gmt":                            "UTC",
	"eastern standard time":          "America/New_York",
	"central standard time":          "America/Chicago",
	"mountain standard time":         "America/Denver",
	"pacific standard time":          "America/Los_Angeles",
	"tokyo standard time":            "Asia/Tokyo",
	"china standard time":            "Asia/Shanghai",
	"india standard time":            "Asia/Kolkata",
	"aus eastern standard time":      "Australia/Sydney",
	"w. australia standard time":     "Australia/Perth",
	"new zealand standard time":      "Pacific/Auckland",
}

var (
	zoneMu    sync.Mutex
	zoneCache = map[string]*time.Location{} // nil value = known to be unknown
)

// resolveLocation turns a TZID value into a location. Unknown or empty names
// give fallback, never an error: a feed with a zone we cannot name still
// shows its events, at the wall-clock time the feed wrote.
func resolveLocation(tzid string, fallback *time.Location) *time.Location {
	tzid = strings.Trim(strings.TrimSpace(tzid), `"`)
	if tzid == "" {
		return fallback
	}
	zoneMu.Lock()
	defer zoneMu.Unlock()
	if l, ok := zoneCache[tzid]; ok {
		if l == nil {
			return fallback
		}
		return l
	}
	l := lookupZone(tzid)
	zoneCache[tzid] = l
	if l == nil {
		return fallback
	}
	return l
}

func lookupZone(tzid string) *time.Location {
	if name, ok := windowsZones[strings.ToLower(tzid)]; ok {
		if l, err := time.LoadLocation(name); err == nil {
			return l
		}
		return nil
	}
	if strings.EqualFold(tzid, "Local") {
		return nil // never the host zone: the dashboard's zone is configured
	}
	if l, err := time.LoadLocation(tzid); err == nil {
		return l
	}
	// Globally unique TZIDs such as "/mozilla.org/20050126_1/Europe/Berlin":
	// try the trailing path segments.
	parts := strings.Split(strings.Trim(tzid, "/"), "/")
	for n := 2; n >= 1; n-- {
		if len(parts) <= n {
			continue
		}
		if l, err := time.LoadLocation(strings.Join(parts[len(parts)-n:], "/")); err == nil {
			return l
		}
	}
	return nil
}
