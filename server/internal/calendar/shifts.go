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

// ShiftDay is one row of the shift display.
type ShiftDay struct {
	Date      string // yyyy-MM-dd
	DayLabel  string
	Title     string
	TimeRange string // "06:00–14:00" or "06:30–19:00"; empty when unknown
	Icon      IconRef
	Free      bool
	Ongoing   bool
	// HighlightColor tints the time text; empty for none and for free days.
	HighlightColor string
}

// ShiftOptions are the section settings BuildShifts needs.
type ShiftOptions struct {
	ShowFreeDays bool
	Rows         int // number of rows to show
	HorizonDays  int // how many days from today are searched for work days
	Highlight    Highlight
}

func parseHM(s string) (h, m int, ok bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}

// formatRange always renders "HH:MM–HH:MM" (e.g. "11:00–19:00"), never
// dropping zero minutes, and without a trailing "Uhr".
func formatRange(sh, sm, eh, em int) string {
	return fmt.Sprintf("%02d:%02d–%02d:%02d", sh, sm, eh, em)
}

func (s Shift) matches(title string) bool {
	if strings.TrimSpace(s.Match) == "" {
		return false // a shift needs a keyword; empty would claim every event
	}
	return textMatches(s.MatchMode, title, s.Match)
}

var germanDaysShort = [...]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}

// ShiftDayLabel names the local day relative to today: "Heute", "Morgen", the
// weekday for the following days up to six days ahead, and from there on
// "Mo, 23.11.".
func ShiftDayLabel(day, today time.Time) string {
	switch diff := dayCount(today, day); {
	case diff == 0:
		return "Heute"
	case diff == 1:
		return "Morgen"
	case diff > 1 && diff < 7:
		return FormatDayOfWeek(day)
	default:
		return fmt.Sprintf("%s, %02d.%02d.", germanDaysShort[day.Weekday()], day.Day(), int(day.Month()))
	}
}

// BuildShifts builds the shift rows. Without ShowFreeDays it collects the
// first Rows days within the horizon that have a visible event; with it, Rows
// consecutive days from today including free ones. A shift is recognised on
// the original title or, failing that, on the title after rename rules.
func BuildShifts(occs []SourcedOccurrence, rules []Rule, shifts []Shift, opt ShiftOptions, now time.Time, loc *time.Location) []ShiftDay {
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

	days := opt.Rows
	if !opt.ShowFreeDays {
		days = opt.HorizonDays
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	out := []ShiftDay{}
	for i := 0; i < days && len(out) < opt.Rows; i++ {
		dayStart := time.Date(now.Year(), now.Month(), now.Day()+i, 0, 0, 0, 0, loc)
		win := Window{From: dayStart, To: time.Date(now.Year(), now.Month(), now.Day()+i+1, 0, 0, 0, 0, loc)}
		d := ShiftDay{Date: dayStart.Format("2006-01-02"), DayLabel: ShiftDayLabel(dayStart, today)}

		var day []visible
		for _, v := range vis {
			if overlaps(v.Start, v.End, win) {
				day = append(day, v)
			}
		}
		if len(day) == 0 {
			if opt.ShowFreeDays {
				d.Title, d.Free = "frei", true
				out = append(out, d)
			}
			continue
		}

		chosen := day[0]
		matched := false
	search:
		for _, v := range day {
			for _, sh := range shifts {
				if !sh.matches(v.Title) && !sh.matches(v.res.Title) {
					continue
				}
				d.Title = sh.Label
				d.Icon = v.res.Icon
				if sh.Icon.set() {
					d.Icon = sh.Icon
				}
				d.TimeRange = shiftRange(sh, v.Occurrence, loc)
				chosen = v
				matched = true
				break search
			}
		}
		if !matched {
			d.Title, d.Icon = chosen.res.Title, chosen.res.Icon
		}
		if i == 0 {
			// All-day events last the whole day, so they never count as ended.
			ended := !chosen.AllDay && !chosen.End.After(now)
			if ended && !opt.ShowFreeDays {
				continue
			}
			d.Ongoing = chosen.AllDay || (!ended && !chosen.Start.After(now))
		}
		d.HighlightColor = HighlightColor(opt.Highlight, chosen.Start, chosen.End, chosen.AllDay, now, loc)
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
