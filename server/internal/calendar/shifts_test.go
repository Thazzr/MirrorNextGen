package calendar

import (
	"reflect"
	"testing"
	"time"
)

var testShifts = []Shift{
	{ID: "f", Match: "F", MatchMode: MatchEquals, Label: "Frühdienst", Start: "06:00", End: "14:00", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}},
	{ID: "s", Match: "Spät", MatchMode: MatchStartsWith, Label: "Spätdienst", Start: "14:00", End: "22:30"},
	{ID: "n", Match: "nacht", MatchMode: MatchContains, Label: "Nachtdienst"},
}

// Free-days mode keeps the consecutive-days behaviour.
func TestBuildShifts(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 10, 7, 0, 0, 0, loc) // Tuesday
	at := func(d, h, m int) time.Time { return time.Date(2026, 3, d, h, m, 0, 0, loc) }
	def := IconRef{Kind: IconBootstrap, Name: "calendar"}
	mk := func(title string, start, end time.Time, allDay bool) SourcedOccurrence {
		return SourcedOccurrence{
			Occurrence:  Occurrence{UID: title, Title: title, Start: start, End: end, AllDay: allDay},
			CalendarID:  "dienst",
			DefaultIcon: def,
		}
	}

	tests := []struct {
		name   string
		occs   []SourcedOccurrence
		rules  []Rule
		shifts []Shift
		days   int
		want   []ShiftDay
		icons  []IconRef // expected icon per day when checked
	}{
		{
			name:   "equals match with definition times, free tomorrow",
			occs:   []SourcedOccurrence{mk("F", at(10, 6, 0), at(10, 14, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Title: "Frühdienst", TimeRange: "06–14", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}},
				{Date: "2026-03-11", DayLabel: "Morgen", Title: "frei", Free: true},
			},
		},
		{
			name:   "equals is case-insensitive and needs the whole title",
			occs:   []SourcedOccurrence{mk("f", at(10, 6, 0), at(10, 14, 0), false), mk("Frei", at(11, 8, 0), at(11, 9, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Title: "Frühdienst", TimeRange: "06–14", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}},
				{Date: "2026-03-11", DayLabel: "Morgen", Title: "Frei", Icon: def},
			},
		},
		{
			name:   "startsWith with minutes in the definition",
			occs:   []SourcedOccurrence{mk("Spätschicht Station 3", at(10, 14, 0), at(10, 22, 0), false)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Spätdienst", TimeRange: "14:00–22:30", Icon: def}},
		},
		{
			name:   "contains match uses the occurrence's own times",
			occs:   []SourcedOccurrence{mk("Dienst Nachtbereitschaft", at(10, 22, 0), at(11, 6, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Title: "Nachtdienst", TimeRange: "22–06", Icon: def},
				{Date: "2026-03-11", DayLabel: "Morgen", Title: "Nachtdienst", TimeRange: "22–06", Icon: def},
			},
		},
		{
			name:   "occurrence times with minutes",
			occs:   []SourcedOccurrence{mk("Nacht", at(10, 6, 30), at(10, 14, 15), false)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Nachtdienst", TimeRange: "06:30–14:15", Icon: def}},
		},
		{
			name:   "all-day shift without definition times has no range",
			occs:   []SourcedOccurrence{mk("Nacht", at(10, 0, 0), at(11, 0, 0), true)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Nachtdienst", Icon: def}},
		},
		{
			name:   "all-day shift with definition times uses them",
			occs:   []SourcedOccurrence{mk("F", at(10, 0, 0), at(11, 0, 0), true)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Frühdienst", TimeRange: "06–14", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}}},
		},
		{
			name:   "multi-day all-day shift shows on every day it covers",
			occs:   []SourcedOccurrence{mk("Nacht", at(10, 0, 0), at(12, 0, 0), true)},
			shifts: testShifts, days: 3,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Title: "Nachtdienst", Icon: def},
				{Date: "2026-03-11", DayLabel: "Morgen", Title: "Nachtdienst", Icon: def},
				{Date: "2026-03-12", DayLabel: "Donnerstag", Title: "frei", Free: true},
			},
		},
		{
			name:   "unmatched title is shown as is",
			occs:   []SourcedOccurrence{mk("Zahnarzt", at(10, 9, 0), at(10, 10, 0), false)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Zahnarzt", Icon: def}},
		},
		{
			name: "unmatched title is renamed and iconised by rules",
			occs: []SourcedOccurrence{mk("Zahnarzt", at(10, 9, 0), at(10, 10, 0), false)},
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "zahn", Action: ActionRename, RenameTo: "Arzt"},
				{ID: "r2", MatchMode: MatchContains, Text: "zahn", Action: ActionIcon, Icon: IconRef{Kind: IconBootstrap, Name: "heart"}},
			},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Arzt", Icon: IconRef{Kind: IconBootstrap, Name: "heart"}}},
		},
		{
			name: "a shift is also recognised after a rename rule",
			occs: []SourcedOccurrence{mk("Dienst A", at(10, 6, 0), at(10, 14, 0), false)},
			rules: []Rule{
				{ID: "r1", MatchMode: MatchEquals, Text: "Dienst A", Action: ActionRename, RenameTo: "F"},
			},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Frühdienst", TimeRange: "06–14", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}}},
		},
		{
			name:   "hide rule removes the event, day becomes free",
			occs:   []SourcedOccurrence{mk("F", at(10, 6, 0), at(10, 14, 0), false)},
			rules:  []Rule{{ID: "h", MatchMode: MatchEquals, Text: "F", Action: ActionHide}},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "frei", Free: true}},
		},
		{
			name: "first matching event by start wins, even if a later one is a better definition",
			occs: []SourcedOccurrence{
				mk("Spät", at(10, 14, 0), at(10, 22, 0), false),
				mk("F", at(10, 6, 0), at(10, 14, 0), false),
				mk("Zahnarzt", at(10, 5, 0), at(10, 5, 30), false),
			},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Frühdienst", TimeRange: "06–14", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}}},
		},
		{
			name: "definitions are tried in order",
			occs: []SourcedOccurrence{mk("Nachtfrüh", at(10, 4, 0), at(10, 6, 0), false)},
			shifts: []Shift{
				{ID: "a", Match: "nacht", MatchMode: MatchContains, Label: "A"},
				{ID: "b", Match: "nachtfrüh", MatchMode: MatchEquals, Label: "B"},
			},
			days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "A", TimeRange: "04–06", Icon: def}},
		},
		{
			name:   "definition with an empty keyword never matches",
			occs:   []SourcedOccurrence{mk("Irgendwas", at(10, 9, 0), at(10, 10, 0), false)},
			shifts: []Shift{{ID: "x", Match: "", MatchMode: MatchContains, Label: "X"}},
			days:   1,
			want:   []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Title: "Irgendwas", Icon: def}},
		},
		{
			name:   "event ending exactly at midnight does not spill onto the next day",
			occs:   []SourcedOccurrence{mk("Spät", at(10, 16, 0), at(11, 0, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Title: "Spätdienst", TimeRange: "14:00–22:30", Icon: def},
				{Date: "2026-03-11", DayLabel: "Morgen", Title: "frei", Free: true},
			},
		},
		{
			name:   "later days are labelled with the weekday",
			shifts: testShifts, days: 5,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Title: "frei", Free: true},
				{Date: "2026-03-11", DayLabel: "Morgen", Title: "frei", Free: true},
				{Date: "2026-03-12", DayLabel: "Donnerstag", Title: "frei", Free: true},
				{Date: "2026-03-13", DayLabel: "Freitag", Title: "frei", Free: true},
				{Date: "2026-03-14", DayLabel: "Samstag", Title: "frei", Free: true},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildShifts(tc.occs, tc.rules, tc.shifts, ShiftOptions{ShowFreeDays: true, Rows: tc.days, HorizonDays: 60}, now, loc)
			for i := range got {
				got[i].Ongoing = false // covered by TestBuildShiftsWorkDays
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestBuildShiftsAcrossDST(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 28, 20, 0, 0, 0, loc)
	occs := []SourcedOccurrence{{
		Occurrence: Occurrence{Title: "F", Start: time.Date(2026, 3, 29, 0, 0, 0, 0, loc), End: time.Date(2026, 3, 30, 0, 0, 0, 0, loc), AllDay: true},
		CalendarID: "c",
	}}
	got := BuildShifts(occs, nil, testShifts, ShiftOptions{ShowFreeDays: true, Rows: 3, HorizonDays: 60}, now, loc)
	dates := []string{got[0].Date, got[1].Date, got[2].Date}
	if !reflect.DeepEqual(dates, []string{"2026-03-28", "2026-03-29", "2026-03-30"}) {
		t.Errorf("dates %v", dates)
	}
	if got[0].Free != true || got[1].Title != "Frühdienst" || got[2].Free != true {
		t.Errorf("all-day shift on the 23-hour day must cover exactly that day: %+v", got)
	}
}

func TestShiftDayLabel(t *testing.T) {
	loc := berlin(t)
	today := time.Date(2026, 11, 16, 0, 0, 0, 0, loc) // Monday
	tests := []struct {
		add  int
		want string
	}{
		{0, "Heute"}, {1, "Morgen"}, {2, "Mittwoch"}, {6, "Sonntag"}, {7, "Mo, 23.11."}, {8, "Di, 24.11."}, {20, "So, 06.12."},
	}
	for _, tc := range tests {
		d := time.Date(2026, 11, 16+tc.add, 0, 0, 0, 0, loc)
		if got := ShiftDayLabel(d, today); got != tc.want {
			t.Errorf("+%d: got %q, want %q", tc.add, got, tc.want)
		}
	}
	// Across the DST change (2026-10-25) a day is still a calendar day.
	t0 := time.Date(2026, 10, 24, 0, 0, 0, 0, loc)
	if got := ShiftDayLabel(time.Date(2026, 10, 25, 0, 0, 0, 0, loc), t0); got != "Morgen" {
		t.Errorf("DST day: %q", got)
	}
	if got := ShiftDayLabel(time.Date(2026, 10, 31, 0, 0, 0, 0, loc), t0); got != "Sa, 31.10." {
		t.Errorf("DST +7: %q", got)
	}
}

func TestBuildShiftsWorkDays(t *testing.T) {
	loc := berlin(t)
	def := IconRef{Kind: IconBootstrap, Name: "calendar"}
	at := func(m, d, h, mi int) time.Time { return time.Date(2026, time.Month(m), d, h, mi, 0, 0, loc) }
	mk := func(title string, start, end time.Time, allDay bool) SourcedOccurrence {
		return SourcedOccurrence{Occurrence: Occurrence{UID: title + start.String(), Title: title, Start: start, End: end, AllDay: allDay}, CalendarID: "c", DefaultIcon: def}
	}
	// Saturday 2026-10-24 morning; DST ends on Sunday 10-25.
	now := time.Date(2026, 10, 24, 7, 0, 0, 0, loc)
	week := []SourcedOccurrence{
		mk("F", at(10, 27, 6, 0), at(10, 27, 14, 0), false),     // Tue
		mk("Spät", at(10, 29, 14, 0), at(10, 29, 22, 0), false), // Thu
		mk("Nacht", at(11, 2, 22, 0), at(11, 3, 6, 0), false),   // Mon, beyond a 7-day horizon
		mk("F", at(10, 30, 6, 0), at(10, 30, 14, 0), false),     // Fri
	}
	dates := func(days []ShiftDay) []string {
		var out []string
		for _, d := range days {
			out = append(out, d.Date)
		}
		return out
	}
	run := func(occs []SourcedOccurrence, o ShiftOptions, now time.Time) []ShiftDay {
		return BuildShifts(occs, nil, testShifts, o, now, loc)
	}

	t.Run("free days are hidden and the horizon is respected", func(t *testing.T) {
		got := run(week, ShiftOptions{Rows: 5, HorizonDays: 7}, now) // days 10-24..10-30
		if want := []string{"2026-10-27", "2026-10-29", "2026-10-30"}; !reflect.DeepEqual(dates(got), want) {
			t.Fatalf("dates %v, want %v", dates(got), want)
		}
		if got[0].DayLabel != "Dienstag" || got[0].Title != "Frühdienst" || got[0].TimeRange != "06–14" || got[0].Free {
			t.Errorf("row 0 = %+v", got[0])
		}
		if got[1].DayLabel != "Donnerstag" || got[1].TimeRange != "14:00–22:30" {
			t.Errorf("row 1 = %+v", got[1])
		}
	})
	t.Run("a shift beyond the horizon is not shown", func(t *testing.T) {
		got := run(week, ShiftOptions{Rows: 9, HorizonDays: 9}, now) // 10-24 .. 11-01
		if len(got) != 3 {
			t.Errorf("dates %v", dates(got))
		}
		got = run(week, ShiftOptions{Rows: 9, HorizonDays: 10}, now) // reaches 11-02
		if len(got) != 4 || got[3].Date != "2026-11-02" || got[3].DayLabel != "Mo, 02.11." {
			t.Errorf("dates %v", dates(got))
		}
	})
	t.Run("the row limit is respected", func(t *testing.T) {
		got := run(week, ShiftOptions{Rows: 2, HorizonDays: 30}, now)
		if want := []string{"2026-10-27", "2026-10-29"}; !reflect.DeepEqual(dates(got), want) {
			t.Errorf("dates %v", dates(got))
		}
	})
	t.Run("no work days gives an empty, non-nil list", func(t *testing.T) {
		got := run(nil, ShiftOptions{Rows: 5, HorizonDays: 7}, now)
		if got == nil || len(got) != 0 {
			t.Errorf("got %#v", got)
		}
	})
	t.Run("free days shown gives consecutive rows", func(t *testing.T) {
		got := run(week, ShiftOptions{ShowFreeDays: true, Rows: 4, HorizonDays: 60}, now)
		if want := []string{"2026-10-24", "2026-10-25", "2026-10-26", "2026-10-27"}; !reflect.DeepEqual(dates(got), want) {
			t.Fatalf("dates %v", dates(got))
		}
		if !got[0].Free || got[0].Title != "frei" || got[0].DayLabel != "Heute" || got[3].Free {
			t.Errorf("rows %+v", got)
		}
	})

	today := func(h int) time.Time { return at(10, 24, h, 0) }
	todayShift := []SourcedOccurrence{mk("F", at(10, 24, 6, 0), at(10, 24, 14, 0), false)}
	t.Run("today's ended shift is skipped", func(t *testing.T) {
		got := run(append(todayShift, week...), ShiftOptions{Rows: 5, HorizonDays: 7}, today(15))
		if got[0].Date != "2026-10-27" {
			t.Errorf("dates %v", dates(got))
		}
		// With free days shown the row stays, but is no longer ongoing.
		got = run(todayShift, ShiftOptions{ShowFreeDays: true, Rows: 1, HorizonDays: 7}, today(15))
		if len(got) != 1 || got[0].Ongoing || got[0].Title != "Frühdienst" {
			t.Errorf("rows %+v", got)
		}
	})
	t.Run("a running shift is ongoing, a coming one is not", func(t *testing.T) {
		got := run(todayShift, ShiftOptions{Rows: 5, HorizonDays: 7}, today(9))
		if len(got) != 1 || !got[0].Ongoing || got[0].DayLabel != "Heute" {
			t.Errorf("rows %+v", got)
		}
		got = run(todayShift, ShiftOptions{Rows: 5, HorizonDays: 7}, today(5))
		if len(got) != 1 || got[0].Ongoing {
			t.Errorf("rows %+v", got)
		}
	})
	t.Run("an all-day shift today is shown and ongoing", func(t *testing.T) {
		occs := []SourcedOccurrence{mk("Nacht", at(10, 24, 0, 0), at(10, 25, 0, 0), true)}
		got := run(occs, ShiftOptions{Rows: 5, HorizonDays: 7}, today(23))
		if len(got) != 1 || !got[0].Ongoing || got[0].TimeRange != "" {
			t.Errorf("rows %+v", got)
		}
	})
	t.Run("a hidden event does not make a work day", func(t *testing.T) {
		rules := []Rule{{ID: "h", MatchMode: MatchEquals, Text: "F", Action: ActionHide}}
		got := BuildShifts(week[:1], rules, testShifts, ShiftOptions{Rows: 5, HorizonDays: 7}, now, loc)
		if len(got) != 0 {
			t.Errorf("rows %+v", got)
		}
	})
}

func TestFormatRangeHasNoSuffix(t *testing.T) {
	if got := formatRange(6, 0, 14, 0); got != "06–14" {
		t.Errorf("got %q", got)
	}
	if got := formatRange(6, 30, 19, 0); got != "06:30–19:00" {
		t.Errorf("got %q", got)
	}
}
