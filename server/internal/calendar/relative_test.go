package calendar

import (
	"testing"
	"time"
)

func TestFormatRelativeTime(t *testing.T) {
	loc := berlin(t)
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, loc) }
	now := at(2026, 3, 10, 12, 0) // Tuesday
	tests := []struct {
		name       string
		start, end time.Time
		allDay     bool
		now        time.Time
		want       string
	}{
		// ongoing
		{"ongoing all-day, 1 day left", at(2026, 3, 9, 0, 0), at(2026, 3, 11, 0, 0), true, now, "Noch 1 Tag"},
		{"ongoing all-day today only", at(2026, 3, 10, 0, 0), at(2026, 3, 11, 0, 0), true, now, "Noch 1 Tag"},
		{"ongoing all-day, several days", at(2026, 3, 9, 0, 0), at(2026, 3, 12, 0, 0), true, now, "Noch 2 Tage"},
		{"ongoing timed past midnight", at(2026, 3, 10, 11, 0), at(2026, 3, 11, 2, 0), false, now, "Noch 1 Tag"},
		{"ongoing, 2.5 hours rounds up", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 14, 30), false, now, "Noch 3 Stunden"},
		{"ongoing, exactly 1 hour", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 13, 0), false, now, "Noch 1 Stunde"},
		{"ongoing, 1h01 rounds to 2", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 13, 1), false, now, "Noch 2 Stunden"},
		{"ongoing, 30 minutes", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 12, 30), false, now, "Noch 30 Minuten"},
		{"ongoing, 30 seconds", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 12, 0).Add(30 * time.Second), false, now, "Noch 1 Minute"},
		{"ongoing, 10m30s rounds up", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 12, 10).Add(30 * time.Second), false, now, "Noch 11 Minuten"},
		{"starting right now counts as ongoing", at(2026, 3, 10, 12, 0), at(2026, 3, 10, 12, 45), false, now, "Noch 45 Minuten"},
		{"end instant is not ongoing", at(2026, 3, 10, 11, 0), at(2026, 3, 10, 12, 0), false, now, "Heute um 11:00"},
		// future
		{"today timed", at(2026, 3, 10, 15, 0), at(2026, 3, 10, 16, 0), false, now, "Heute um 15:00"},
		{"tomorrow timed", at(2026, 3, 11, 14, 30), at(2026, 3, 11, 15, 0), false, now, "Morgen um 14:30"},
		{"tomorrow all-day", at(2026, 3, 11, 0, 0), at(2026, 3, 12, 0, 0), true, now, "Morgen"},
		{"weekday timed", at(2026, 3, 12, 9, 0), at(2026, 3, 12, 10, 0), false, now, "Donnerstag um 09:00"},
		{"weekday all-day", at(2026, 3, 13, 0, 0), at(2026, 3, 14, 0, 0), true, now, "Freitag"},
		{"6 days ahead is still a weekday", at(2026, 3, 16, 9, 0), at(2026, 3, 16, 10, 0), false, now, "Montag um 09:00"},
		{"7 days ahead timed", at(2026, 3, 17, 9, 0), at(2026, 3, 17, 10, 0), false, now, "17. März um 09:00"},
		{"7 days ahead all-day", at(2026, 3, 17, 0, 0), at(2026, 3, 18, 0, 0), true, now, "17. März"},
		{"abbreviated month with dot", at(2026, 4, 2, 0, 0), at(2026, 4, 3, 0, 0), true, now, "2. Apr."},
		{"september is Sept.", at(2026, 9, 3, 0, 0), at(2026, 9, 4, 0, 0), true, now, "3. Sept."},
		{"year change shows the year", at(2027, 1, 5, 0, 0), at(2027, 1, 6, 0, 0), true, now, "5. Jan. 2027"},
		{"year change timed", at(2027, 1, 5, 8, 0), at(2027, 1, 5, 9, 0), false, now, "5. Jan. 2027 um 08:00"},
		// past
		{"yesterday timed", at(2026, 3, 9, 9, 0), at(2026, 3, 9, 10, 0), false, now, "Gestern um 09:00"},
		{"yesterday all-day", at(2026, 3, 9, 0, 0), at(2026, 3, 10, 0, 0), true, now, "Gestern"},
		// day differences are calendar days, also across DST
		{"DST: 7 calendar days are 167 hours", at(2026, 3, 29, 8, 0), at(2026, 3, 29, 9, 0), false, at(2026, 3, 22, 12, 0), "29. März um 08:00"},
		{"DST: 6 calendar days stay a weekday", at(2026, 3, 28, 8, 0), at(2026, 3, 28, 9, 0), false, at(2026, 3, 22, 12, 0), "Samstag um 08:00"},
		{"DST: tomorrow an hour away", at(2026, 3, 29, 0, 30), at(2026, 3, 29, 1, 0), false, at(2026, 3, 28, 23, 30), "Morgen um 00:30"},
		{"DST fall back: 24h+ is still today", at(2026, 10, 25, 23, 0), at(2026, 10, 25, 23, 30), false, at(2026, 10, 25, 0, 30), "Heute um 23:00"},
		// instants are shown in the dashboard zone
		{"UTC instants shown in local time", time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC), time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC), false, now, "Heute um 15:00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatRelativeTime(tc.start, tc.end, tc.allDay, tc.now, loc); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatDayOfWeekAndLongDate(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	for i, want := range []string{"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag"} {
		if got := FormatDayOfWeek(time.Date(2026, 3, 9+i, 0, 0, 0, 0, loc)); got != want {
			t.Errorf("weekday %d: %q, want %q", i, got, want)
		}
	}
	months := []string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez."}
	for i, want := range months {
		if got := FormatLongDate(time.Date(2026, time.Month(i+1), 7, 0, 0, 0, 0, loc), now); got != "7. "+want {
			t.Errorf("month %d: %q", i+1, got)
		}
	}
	if got := FormatLongDate(time.Date(2025, 12, 31, 0, 0, 0, 0, loc), now); got != "31. Dez. 2025" {
		t.Errorf("other year: %q", got)
	}
}

func TestTimeLabel(t *testing.T) {
	if TimeLabel(true) != "Ganztägig" || TimeLabel(false) != "" {
		t.Error("time label wrong")
	}
}
