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
				{Date: "2026-03-10", DayLabel: "Heute", Text: "Frühdienst 06–14 Uhr", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}},
				{Date: "2026-03-11", DayLabel: "Morgen", Text: "frei", Free: true},
			},
		},
		{
			name:   "equals is case-insensitive and needs the whole title",
			occs:   []SourcedOccurrence{mk("f", at(10, 6, 0), at(10, 14, 0), false), mk("Frei", at(11, 8, 0), at(11, 9, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Text: "Frühdienst 06–14 Uhr", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}},
				{Date: "2026-03-11", DayLabel: "Morgen", Text: "Frei", Icon: def},
			},
		},
		{
			name:   "startsWith with minutes in the definition",
			occs:   []SourcedOccurrence{mk("Spätschicht Station 3", at(10, 14, 0), at(10, 22, 0), false)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Spätdienst 14:00–22:30 Uhr", Icon: def}},
		},
		{
			name:   "contains match uses the occurrence's own times",
			occs:   []SourcedOccurrence{mk("Dienst Nachtbereitschaft", at(10, 22, 0), at(11, 6, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Text: "Nachtdienst 22–06 Uhr", Icon: def},
				{Date: "2026-03-11", DayLabel: "Morgen", Text: "Nachtdienst 22–06 Uhr", Icon: def},
			},
		},
		{
			name:   "occurrence times with minutes",
			occs:   []SourcedOccurrence{mk("Nacht", at(10, 6, 30), at(10, 14, 15), false)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Nachtdienst 06:30–14:15 Uhr", Icon: def}},
		},
		{
			name:   "all-day shift without definition times has no range",
			occs:   []SourcedOccurrence{mk("Nacht", at(10, 0, 0), at(11, 0, 0), true)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Nachtdienst", Icon: def}},
		},
		{
			name:   "all-day shift with definition times uses them",
			occs:   []SourcedOccurrence{mk("F", at(10, 0, 0), at(11, 0, 0), true)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Frühdienst 06–14 Uhr", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}}},
		},
		{
			name:   "multi-day all-day shift shows on every day it covers",
			occs:   []SourcedOccurrence{mk("Nacht", at(10, 0, 0), at(12, 0, 0), true)},
			shifts: testShifts, days: 3,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Text: "Nachtdienst", Icon: def},
				{Date: "2026-03-11", DayLabel: "Morgen", Text: "Nachtdienst", Icon: def},
				{Date: "2026-03-12", DayLabel: "Donnerstag", Text: "frei", Free: true},
			},
		},
		{
			name:   "unmatched title is shown as is",
			occs:   []SourcedOccurrence{mk("Zahnarzt", at(10, 9, 0), at(10, 10, 0), false)},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Zahnarzt", Icon: def}},
		},
		{
			name: "unmatched title is renamed and iconised by rules",
			occs: []SourcedOccurrence{mk("Zahnarzt", at(10, 9, 0), at(10, 10, 0), false)},
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "zahn", Action: ActionRename, RenameTo: "Arzt"},
				{ID: "r2", MatchMode: MatchContains, Text: "zahn", Action: ActionIcon, Icon: IconRef{Kind: IconBootstrap, Name: "heart"}},
			},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Arzt", Icon: IconRef{Kind: IconBootstrap, Name: "heart"}}},
		},
		{
			name: "a shift is also recognised after a rename rule",
			occs: []SourcedOccurrence{mk("Dienst A", at(10, 6, 0), at(10, 14, 0), false)},
			rules: []Rule{
				{ID: "r1", MatchMode: MatchEquals, Text: "Dienst A", Action: ActionRename, RenameTo: "F"},
			},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Frühdienst 06–14 Uhr", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}}},
		},
		{
			name:   "hide rule removes the event, day becomes free",
			occs:   []SourcedOccurrence{mk("F", at(10, 6, 0), at(10, 14, 0), false)},
			rules:  []Rule{{ID: "h", MatchMode: MatchEquals, Text: "F", Action: ActionHide}},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "frei", Free: true}},
		},
		{
			name: "first matching event by start wins, even if a later one is a better definition",
			occs: []SourcedOccurrence{
				mk("Spät", at(10, 14, 0), at(10, 22, 0), false),
				mk("F", at(10, 6, 0), at(10, 14, 0), false),
				mk("Zahnarzt", at(10, 5, 0), at(10, 5, 30), false),
			},
			shifts: testShifts, days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Frühdienst 06–14 Uhr", Icon: IconRef{Kind: IconBootstrap, Name: "sunrise"}}},
		},
		{
			name: "definitions are tried in order",
			occs: []SourcedOccurrence{mk("Nachtfrüh", at(10, 4, 0), at(10, 6, 0), false)},
			shifts: []Shift{
				{ID: "a", Match: "nacht", MatchMode: MatchContains, Label: "A"},
				{ID: "b", Match: "nachtfrüh", MatchMode: MatchEquals, Label: "B"},
			},
			days: 1,
			want: []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "A 04–06 Uhr", Icon: def}},
		},
		{
			name:   "definition with an empty keyword never matches",
			occs:   []SourcedOccurrence{mk("Irgendwas", at(10, 9, 0), at(10, 10, 0), false)},
			shifts: []Shift{{ID: "x", Match: "", MatchMode: MatchContains, Label: "X"}},
			days:   1,
			want:   []ShiftDay{{Date: "2026-03-10", DayLabel: "Heute", Text: "Irgendwas", Icon: def}},
		},
		{
			name:   "event ending exactly at midnight does not spill onto the next day",
			occs:   []SourcedOccurrence{mk("Spät", at(10, 16, 0), at(11, 0, 0), false)},
			shifts: testShifts, days: 2,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Text: "Spätdienst 14:00–22:30 Uhr", Icon: def},
				{Date: "2026-03-11", DayLabel: "Morgen", Text: "frei", Free: true},
			},
		},
		{
			name:   "later days are labelled with the weekday",
			shifts: testShifts, days: 5,
			want: []ShiftDay{
				{Date: "2026-03-10", DayLabel: "Heute", Text: "frei", Free: true},
				{Date: "2026-03-11", DayLabel: "Morgen", Text: "frei", Free: true},
				{Date: "2026-03-12", DayLabel: "Donnerstag", Text: "frei", Free: true},
				{Date: "2026-03-13", DayLabel: "Freitag", Text: "frei", Free: true},
				{Date: "2026-03-14", DayLabel: "Samstag", Text: "frei", Free: true},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildShifts(tc.occs, tc.rules, tc.shifts, tc.days, now, loc)
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
	got := BuildShifts(occs, nil, testShifts, 3, now, loc)
	dates := []string{got[0].Date, got[1].Date, got[2].Date}
	if !reflect.DeepEqual(dates, []string{"2026-03-28", "2026-03-29", "2026-03-30"}) {
		t.Errorf("dates %v", dates)
	}
	if got[0].Free != true || got[1].Text != "Frühdienst 06–14 Uhr" || got[2].Free != true {
		t.Errorf("all-day shift on the 23-hour day must cover exactly that day: %+v", got)
	}
}

func TestShiftSummary(t *testing.T) {
	days := []ShiftDay{
		{DayLabel: "Heute", Text: "Frühdienst 06–14 Uhr"},
		{DayLabel: "Morgen", Text: "frei"},
	}
	if got, want := ShiftSummary(days), "Heute: Frühdienst 06–14 Uhr · Morgen: frei"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := ShiftSummary(nil); got != "" {
		t.Errorf("empty summary %q", got)
	}
	if got := ShiftSummary(days[:1]); got != "Heute: Frühdienst 06–14 Uhr" {
		t.Errorf("single day %q", got)
	}
}
