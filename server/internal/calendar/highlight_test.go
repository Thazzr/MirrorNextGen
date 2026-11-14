package calendar

import (
	"testing"
	"time"
)

const (
	cOngoing = "#f6b73c"
	cSoon    = "#ff8a65"
	cToday   = "#8fd3ff"
)

func allTiers() Highlight {
	return Highlight{
		Enabled: true,
		Ongoing: HighlightTier{Enabled: true, Color: cOngoing},
		Soon:    HighlightSoon{Enabled: true, Color: cSoon, Hours: 2},
		Today:   HighlightTier{Enabled: true, Color: cToday},
	}
}

func TestHighlightColor(t *testing.T) {
	loc := berlin(t)
	// Ordinary day, 10:00 local.
	now := time.Date(2026, 3, 10, 10, 0, 0, 0, loc)
	at := func(d, h, m int) time.Time { return time.Date(2026, 3, d, h, m, 0, 0, loc) }
	with := func(f func(h *Highlight)) Highlight {
		h := allTiers()
		f(&h)
		return h
	}
	// Day of the spring DST switch: 02:00 CET jumps to 03:00 CEST.
	dstNow := time.Date(2026, 3, 29, 0, 30, 0, 0, loc)

	tests := []struct {
		name       string
		h          Highlight
		start, end time.Time
		allDay     bool
		now        time.Time
		want       string
	}{
		{"master off", with(func(h *Highlight) { h.Enabled = false }), at(10, 9, 0), at(10, 11, 0), false, now, ""},
		{"ongoing", allTiers(), at(10, 9, 0), at(10, 11, 0), false, now, cOngoing},
		{"ongoing starts exactly now", allTiers(), at(10, 10, 0), at(10, 11, 0), false, now, cOngoing},
		{"ends exactly now is over", allTiers(), at(10, 9, 0), at(10, 10, 0), false, now, ""},
		{"ongoing tier off, running item gets nothing", with(func(h *Highlight) { h.Ongoing.Enabled = false }), at(10, 9, 0), at(10, 11, 0), false, now, ""},
		{"soon exactly hours ahead", allTiers(), at(10, 12, 0), at(10, 13, 0), false, now, cSoon},
		{"soon one minute too far, still today", allTiers(), at(10, 12, 1), at(10, 13, 0), false, now, cToday},
		{"soon one minute too far, today off", with(func(h *Highlight) { h.Today.Enabled = false }), at(10, 12, 1), at(10, 13, 0), false, now, ""},
		{"soon off falls through to today", with(func(h *Highlight) { h.Soon.Enabled = false }), at(10, 11, 0), at(10, 12, 0), false, now, cToday},
		{"soon off and today off", with(func(h *Highlight) { h.Soon.Enabled, h.Today.Enabled = false, false }), at(10, 11, 0), at(10, 12, 0), false, now, ""},
		{"later today beyond soon", allTiers(), at(10, 18, 0), at(10, 19, 0), false, now, cToday},
		{"tomorrow is not today", allTiers(), at(11, 9, 0), at(11, 10, 0), false, now, ""},
		{"tomorrow 00:30 within hours is soon, not today",
			allTiers(), at(11, 0, 30), at(11, 1, 0), false, time.Date(2026, 3, 10, 23, 0, 0, 0, loc), cSoon},
		{"tomorrow 00:30 within hours, soon off, is not today",
			with(func(h *Highlight) { h.Soon.Enabled = false }), at(11, 0, 30), at(11, 1, 0), false, time.Date(2026, 3, 10, 23, 0, 0, 0, loc), ""},
		{"priority ongoing over soon and today", allTiers(), at(10, 9, 0), at(10, 12, 0), false, now, cOngoing},
		{"already over", allTiers(), at(10, 7, 0), at(10, 8, 0), false, now, ""},
		{"all-day today is ongoing", allTiers(), at(10, 0, 0), at(11, 0, 0), true, now, cOngoing},
		{"all-day today, ongoing off", with(func(h *Highlight) { h.Ongoing.Enabled = false }), at(10, 0, 0), at(11, 0, 0), true, now, ""},
		{"multi-day all-day in its middle is ongoing", allTiers(), at(9, 0, 0), at(12, 0, 0), true, now, cOngoing},
		{"all-day tomorrow within soon window", allTiers(), at(11, 0, 0), at(12, 0, 0), true, time.Date(2026, 3, 10, 23, 0, 0, 0, loc), cSoon},
		{"all-day yesterday is over", allTiers(), at(9, 0, 0), at(10, 0, 0), true, now, ""},
		// DST day: from 00:30 CET to 03:30 CEST are two real hours.
		{"dst: soon counts real hours", allTiers(), time.Date(2026, 3, 29, 3, 30, 0, 0, loc), time.Date(2026, 3, 29, 4, 0, 0, 0, loc), false, dstNow, cSoon},
		{"dst: one minute beyond is only today", allTiers(), time.Date(2026, 3, 29, 3, 31, 0, 0, loc), time.Date(2026, 3, 29, 4, 0, 0, 0, loc), false, dstNow, cToday},
		{"dst: all-day on the switch day is ongoing", allTiers(), time.Date(2026, 3, 29, 0, 0, 0, 0, loc), time.Date(2026, 3, 30, 0, 0, 0, 0, loc), true, dstNow, cOngoing},
		{"nil location falls back to UTC", allTiers(), at(10, 9, 0), at(10, 11, 0), false, now, cOngoing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := loc
			if tt.name == "nil location falls back to UTC" {
				l = nil
			}
			if got := HighlightColor(tt.h, tt.start, tt.end, tt.allDay, tt.now, l); got != tt.want {
				t.Errorf("HighlightColor = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildListCarriesHighlightColor(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, loc)
	in := ListInput{Occurrences: listFixture(loc), LookAheadDays: 7, Highlight: allTiers()}
	got := map[string]string{}
	for _, it := range BuildList(in, now, loc) {
		got[it.Title] = it.HighlightColor
	}
	want := map[string]string{"Ongoing": cOngoing, "Later": cToday, "Alpha": cToday, "Hidden thing": cSoon, "Tomorrow": ""}
	for title, c := range want {
		if g, ok := got[title]; !ok || g != c {
			t.Errorf("%s: colour %q (present %v), want %q", title, g, ok, c)
		}
	}
	in.Highlight.Enabled = false
	for _, it := range BuildList(in, now, loc) {
		if it.HighlightColor != "" {
			t.Errorf("%s coloured with master off", it.Title)
		}
	}
}

func TestBuildShiftsCarriesHighlightColor(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 10, 7, 0, 0, 0, loc) // Tuesday
	at := func(d, h, m int) time.Time { return time.Date(2026, 3, d, h, m, 0, 0, loc) }
	mk := func(title string, start, end time.Time) SourcedOccurrence {
		return SourcedOccurrence{Occurrence: Occurrence{UID: title, Title: title, Start: start, End: end}, CalendarID: "dienst"}
	}
	occs := []SourcedOccurrence{
		mk("F", at(10, 6, 0), at(10, 14, 0)), // running now
		mk("Spät", at(11, 14, 0), at(11, 22, 30)),
	}
	h := allTiers()
	h.Soon.Hours = 30 // tomorrow 14:00 is 31 hours away; the clock times of the definition do not matter
	opt := ShiftOptions{ShowFreeDays: true, Rows: 3, Highlight: h}
	days := BuildShifts(occs, nil, testShifts, opt, now, loc)
	if len(days) != 3 {
		t.Fatalf("days = %+v", days)
	}
	if days[0].HighlightColor != cOngoing {
		t.Errorf("running shift = %q", days[0].HighlightColor)
	}
	if days[1].HighlightColor != "" {
		t.Errorf("tomorrow beyond the soon window = %q", days[1].HighlightColor)
	}
	if !days[2].Free || days[2].HighlightColor != "" {
		t.Errorf("free row = %+v", days[2])
	}

	opt.Highlight.Soon.Hours = 31
	days = BuildShifts(occs, nil, testShifts, opt, now, loc)
	if days[1].HighlightColor != cSoon {
		t.Errorf("tomorrow within 31 h = %q", days[1].HighlightColor)
	}

	opt.Highlight.Enabled = false
	for _, d := range BuildShifts(occs, nil, testShifts, opt, now, loc) {
		if d.HighlightColor != "" {
			t.Errorf("row coloured with master off: %+v", d)
		}
	}
}
