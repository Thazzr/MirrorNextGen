package calendar

import (
	"fmt"
	"math"
	"time"
)

var (
	germanDays   = [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}
	germanMonths = [...]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez."}
)

// FormatDayOfWeek gives the German weekday name of t.
func FormatDayOfWeek(t time.Time) string { return germanDays[t.Weekday()] }

// FormatLongDate gives "d. MMM", plus the year when it differs from now's.
func FormatLongDate(t, now time.Time) string {
	s := fmt.Sprintf("%d. %s", t.Day(), germanMonths[t.Month()-1])
	if t.Year() != now.Year() {
		s += fmt.Sprintf(" %d", t.Year())
	}
	return s
}

// TimeLabel is "Ganztägig" for all-day events, otherwise empty.
func TimeLabel(allDay bool) string {
	if allDay {
		return "Ganztägig"
	}
	return ""
}

// FormatRelativeTime describes when an event happens relative to now, in
// local calendar days of loc (so a DST day does not skew the difference).
func FormatRelativeTime(start, end time.Time, allDay bool, now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	start, end, now = start.In(loc), end.In(loc), now.In(loc)

	if !start.After(now) && now.Before(end) {
		remaining := end.Sub(now)
		if dayCount(now, end) > 0 {
			days := dayCount(now, end)
			if days == 1 {
				return "Noch 1 Tag"
			}
			return fmt.Sprintf("Noch %d Tage", days)
		}
		if remaining >= time.Hour {
			hours := max(1, int(math.Ceil(remaining.Hours())))
			if hours == 1 {
				return "Noch 1 Stunde"
			}
			return fmt.Sprintf("Noch %d Stunden", hours)
		}
		minutes := max(1, int(math.Ceil(remaining.Minutes())))
		if minutes == 1 {
			return "Noch 1 Minute"
		}
		return fmt.Sprintf("Noch %d Minuten", minutes)
	}

	diff := dayCount(now, start)
	if diff >= 7 {
		if allDay {
			return FormatLongDate(start, now)
		}
		return fmt.Sprintf("%s um %s", FormatLongDate(start, now), start.Format("15:04"))
	}
	day := FormatDayOfWeek(start)
	switch diff {
	case -1:
		day = "Gestern"
	case 0:
		day = "Heute"
	case 1:
		day = "Morgen"
	}
	if allDay {
		return day
	}
	return fmt.Sprintf("%s um %s", day, start.Format("15:04"))
}
