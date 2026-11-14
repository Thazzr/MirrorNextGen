package calendar

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Shift maps a title keyword to a named shift. Start and End ("HH:mm") are
// optional; when both are set they replace the event's own times.
type Shift struct {
	ID        string
	Match     string
	MatchMode string // equals|startsWith|contains
	Label     string
	Start     string
	End       string
	Icon      IconRef
}

// ShiftDay is one day of the compact shift display.
type ShiftDay struct {
	Date     string // yyyy-MM-dd
	DayLabel string
	Text     string
	Icon     IconRef
	Free     bool
}

func parseHM(s string) (h, m int, ok bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}

// formatRange renders "06–14 Uhr" or, with minutes, "06:30–14:15 Uhr".
func formatRange(sh, sm, eh, em int) string {
	if sm == 0 && em == 0 {
		return fmt.Sprintf("%02d–%02d Uhr", sh, eh)
	}
	return fmt.Sprintf("%02d:%02d–%02d:%02d Uhr", sh, sm, eh, em)
}

func (s Shift) matches(title string) bool {
	if strings.TrimSpace(s.Match) == "" {
		return false // a shift needs a keyword; empty would claim every event
	}
	return textMatches(s.MatchMode, title, s.Match)
}

// BuildShifts builds the display for today and the following days-1 local
// days. A shift is recognised on the original title or, failing that, on the
// title after rename rules.
func BuildShifts(occs []SourcedOccurrence, rules []Rule, shifts []Shift, days int, now time.Time, loc *time.Location) []ShiftDay {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)

	type visible struct {
		SourcedOccurrence
		res Result
	}
	var vis []visible
	for _, so := range occs {
		res := Apply(rules, so.CalendarID, so.Title, so.DefaultIcon)
		if !res.Hidden {
			vis = append(vis, visible{so, res})
		}
	}
	sort.SliceStable(vis, func(i, j int) bool { return vis[i].Start.Before(vis[j].Start) })

	var out []ShiftDay
	for i := 0; i < days; i++ {
		dayStart := time.Date(now.Year(), now.Month(), now.Day()+i, 0, 0, 0, 0, loc)
		win := Window{From: dayStart, To: time.Date(now.Year(), now.Month(), now.Day()+i+1, 0, 0, 0, 0, loc)}
		d := ShiftDay{Date: dayStart.Format("2006-01-02")}
		switch i {
		case 0:
			d.DayLabel = "Heute"
		case 1:
			d.DayLabel = "Morgen"
		default:
			d.DayLabel = FormatDayOfWeek(dayStart)
		}

		var day []visible
		for _, v := range vis {
			if overlaps(v.Start, v.End, win) {
				day = append(day, v)
			}
		}
		if len(day) == 0 {
			d.Text, d.Free = "frei", true
			out = append(out, d)
			continue
		}

		matched := false
	search:
		for _, v := range day {
			for _, sh := range shifts {
				if !sh.matches(v.Title) && !sh.matches(v.res.Title) {
					continue
				}
				d.Text = sh.Label
				d.Icon = v.res.Icon
				if sh.Icon.set() {
					d.Icon = sh.Icon
				}
				if r := shiftRange(sh, v.Occurrence, loc); r != "" {
					d.Text += " " + r
				}
				matched = true
				break search
			}
		}
		if !matched {
			d.Text, d.Icon = day[0].res.Title, day[0].res.Icon
		}
		out = append(out, d)
	}
	return out
}

func shiftRange(sh Shift, o Occurrence, loc *time.Location) string {
	if h1, m1, ok1 := parseHM(sh.Start); ok1 {
		if h2, m2, ok2 := parseHM(sh.End); ok2 {
			return formatRange(h1, m1, h2, m2)
		}
	}
	if o.AllDay || !o.End.After(o.Start) {
		return ""
	}
	s, e := o.Start.In(loc), o.End.In(loc)
	return formatRange(s.Hour(), s.Minute(), e.Hour(), e.Minute())
}

// ShiftSummary joins the days to "Heute: Frühdienst 06–14 Uhr · Morgen: frei".
func ShiftSummary(days []ShiftDay) string {
	parts := make([]string, 0, len(days))
	for _, d := range days {
		parts = append(parts, d.DayLabel+": "+d.Text)
	}
	return strings.Join(parts, " · ")
}
