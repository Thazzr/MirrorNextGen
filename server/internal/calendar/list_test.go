package calendar

import (
	"reflect"
	"testing"
	"time"
)

func sourced(loc *time.Location, cal, title string, start, end time.Time, allDay bool) SourcedOccurrence {
	return SourcedOccurrence{
		Occurrence:  Occurrence{UID: title, Title: title, Start: start, End: end, AllDay: allDay},
		CalendarID:  cal,
		DefaultIcon: IconRef{Kind: IconBootstrap, Name: "icon-" + cal},
	}
}

func listFixture(loc *time.Location) []SourcedOccurrence {
	at := func(d, h, m int) time.Time { return time.Date(2026, 3, d, h, m, 0, 0, loc) }
	return []SourcedOccurrence{
		sourced(loc, "c1", "Ended", at(10, 9, 0), at(10, 10, 0), false),
		sourced(loc, "c1", "Ends now", at(10, 11, 0), at(10, 12, 0), false),
		sourced(loc, "c1", "Ongoing", at(10, 11, 0), at(10, 13, 0), false),
		sourced(loc, "c2", "Later", at(10, 15, 0), at(10, 16, 0), false),
		sourced(loc, "c1", "Alpha", at(10, 15, 0), at(10, 16, 0), false),
		sourced(loc, "c1", "Hidden thing", at(10, 14, 0), at(10, 15, 0), false),
		sourced(loc, "c1", "Tomorrow", at(11, 0, 0), at(12, 0, 0), true),
		sourced(loc, "c1", "Far away", at(20, 9, 0), at(20, 10, 0), false),
		sourced(loc, "c1", "At cutoff", at(17, 12, 0), at(17, 13, 0), false),
		sourced(loc, "c1", "Just before cutoff", at(17, 11, 59), at(17, 13, 0), false),
		sourced(loc, "c1", "Instant now", at(10, 12, 0), at(10, 12, 0), false),
		sourced(loc, "c1", "Instant past", at(10, 11, 59), at(10, 11, 59), false),
	}
}

func titles(items []ListItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func TestBuildList(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, loc)
	rules := []Rule{
		{ID: "hide", MatchMode: MatchContains, Text: "hidden", Action: ActionHide},
		{ID: "ren", MatchMode: MatchEquals, Text: "later", Action: ActionRename, RenameTo: "Spaeter"},
	}
	tests := []struct {
		name  string
		input ListInput
		want  []string
	}{
		{
			name:  "drops ended, hidden and out-of-window events; sorts by start then title",
			input: ListInput{Occurrences: listFixture(loc), Rules: rules, MaxEvents: 0, LookAheadDays: 7},
			want:  []string{"Ongoing", "Instant now", "Alpha", "Spaeter", "Tomorrow", "Just before cutoff"},
		},
		{
			name:  "MaxEvents cuts after sorting",
			input: ListInput{Occurrences: listFixture(loc), Rules: rules, MaxEvents: 3, LookAheadDays: 7},
			want:  []string{"Ongoing", "Instant now", "Alpha"},
		},
		{
			name:  "hidden events do not use up the cut",
			input: ListInput{Occurrences: listFixture(loc), Rules: rules, MaxEvents: 4, LookAheadDays: 7},
			want:  []string{"Ongoing", "Instant now", "Alpha", "Spaeter"},
		},
		{
			name:  "short look-ahead",
			input: ListInput{Occurrences: listFixture(loc), Rules: rules, MaxEvents: 10, LookAheadDays: 1},
			want:  []string{"Ongoing", "Instant now", "Alpha", "Spaeter", "Tomorrow"},
		},
		{
			name:  "no occurrences",
			input: ListInput{MaxEvents: 5, LookAheadDays: 7},
			want:  []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := titles(BuildList(tc.input, now, loc))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildListItemFields(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, loc)
	rules := []Rule{
		{ID: "ico", CalendarIDs: []string{"c2"}, MatchMode: MatchContains, Text: "later", Action: ActionIcon, Icon: IconRef{Kind: IconBootstrap, Name: "star"}},
	}
	items := BuildList(ListInput{Occurrences: listFixture(loc), Rules: rules, LookAheadDays: 7}, now, loc)
	byTitle := map[string]ListItem{}
	for _, it := range items {
		byTitle[it.Title] = it
	}

	on := byTitle["Ongoing"]
	if !on.Ongoing || on.Relative != "Noch 1 Stunde" || on.TimeLabel != "" || on.AllDay || on.CalendarID != "c1" {
		t.Errorf("ongoing item wrong: %+v", on)
	}
	if on.Icon != (IconRef{Kind: IconBootstrap, Name: "icon-c1"}) {
		t.Errorf("calendar default icon not used: %+v", on.Icon)
	}
	if !on.Start.Equal(time.Date(2026, 3, 10, 11, 0, 0, 0, loc)) {
		t.Errorf("start %v", on.Start)
	}

	tm := byTitle["Tomorrow"]
	if !tm.AllDay || tm.TimeLabel != "Ganztägig" || tm.Relative != "Morgen" || tm.Ongoing {
		t.Errorf("all-day item wrong: %+v", tm)
	}

	// A zero-length event starting exactly now is listed but not ongoing.
	if in, ok := byTitle["Instant now"]; !ok || in.Ongoing {
		t.Errorf("instant at now: %+v (listed %v)", in, ok)
	}
	for _, p := range BuildPreview(ListInput{Occurrences: listFixture(loc), LookAheadDays: 7}, now, loc) {
		if p.Title == "Instant now" && p.Ongoing {
			t.Error("preview marks the instant as ongoing")
		}
	}

	later := byTitle["Later"]
	if later.Icon.Name != "star" || !reflect.DeepEqual(later.MatchedRules, []string{"ico"}) || later.Relative != "Heute um 15:00" {
		t.Errorf("rule item wrong: %+v", later)
	}
	if got := byTitle["Alpha"].MatchedRules; len(got) != 0 {
		t.Errorf("rule limited to c2 matched c1: %v", got)
	}
}

func TestBuildPreviewMarksHiddenAndCut(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, loc)
	rules := []Rule{{ID: "hide", MatchMode: MatchContains, Text: "hidden", Action: ActionHide}}
	preview := BuildPreview(ListInput{Occurrences: listFixture(loc), Rules: rules, MaxEvents: 3, LookAheadDays: 1}, now, loc)

	type row struct {
		Title         string
		Hidden, Shown bool
	}
	var got []row
	for _, p := range preview {
		got = append(got, row{p.Title, p.Hidden, p.Shown})
	}
	want := []row{
		{"Ongoing", false, true},
		{"Instant now", false, true},
		{"Hidden thing", true, false},
		{"Alpha", false, true},
		{"Later", false, false}, // cut by MaxEvents
		{"Tomorrow", false, false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	for _, p := range preview {
		if p.Title == "Hidden thing" && !reflect.DeepEqual(p.MatchedRules, []string{"hide"}) {
			t.Errorf("hidden item lacks its rule: %v", p.MatchedRules)
		}
	}
}
