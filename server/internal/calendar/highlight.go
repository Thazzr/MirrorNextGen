package calendar

import "time"

// HighlightTier is one switchable colour.
type HighlightTier struct {
	Enabled bool
	Color   string
}

// HighlightSoon colours items that start within Hours hours.
type HighlightSoon struct {
	Enabled bool
	Color   string
	Hours   int
}

// Highlight is the colouring of the time text of one section.
type Highlight struct {
	Enabled bool
	Ongoing HighlightTier
	Soon    HighlightSoon
	Today   HighlightTier
}

// HighlightColor returns the colour of an item's time text, or "" for none.
// Tiers by priority: running now, starting within the soon window, starting
// later today (local calendar day of loc). now and loc decide everything, so
// the function stays pure.
func HighlightColor(h Highlight, start, end time.Time, allDay bool, now time.Time, loc *time.Location) string {
	if !h.Enabled {
		return ""
	}
	if loc == nil {
		loc = time.UTC
	}
	ongoing := !start.After(now) && now.Before(end)
	if allDay {
		// An all-day event runs for the whole local day it covers.
		n, s, e := now.In(loc), start.In(loc), end.In(loc)
		ongoing = dayCount(s, n) >= 0 && dayCount(n, e) > 0
	}
	if ongoing {
		if h.Ongoing.Enabled {
			return h.Ongoing.Color
		}
		return ""
	}
	if !start.After(now) {
		return ""
	}
	if h.Soon.Enabled && start.Sub(now) <= time.Duration(h.Soon.Hours)*time.Hour {
		return h.Soon.Color
	}
	if h.Today.Enabled && dayCount(now.In(loc), start.In(loc)) == 0 {
		return h.Today.Color
	}
	return ""
}
