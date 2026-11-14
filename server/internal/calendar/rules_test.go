package calendar

import (
	"reflect"
	"testing"
)

func TestApply(t *testing.T) {
	def := IconRef{Kind: IconBootstrap, Name: "calendar"}
	star := IconRef{Kind: IconBootstrap, Name: "star"}
	cake := IconRef{Kind: IconUpload, File: "cake.svg"}

	tests := []struct {
		name       string
		rules      []Rule
		calendarID string
		title      string
		want       Result
	}{
		{
			name:  "no rules keeps everything",
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Title: "Zahnarzt", Icon: def},
		},
		{
			name:  "contains matches inside the title",
			rules: []Rule{{ID: "r1", MatchMode: MatchContains, Text: "arzt", Action: ActionHide}},
			title: "Zahnarzt Termin", calendarID: "c1",
			want: Result{Hidden: true, Title: "Zahnarzt Termin", Icon: def, Matched: []string{"r1"}},
		},
		{
			name:  "contains does not match a missing word",
			rules: []Rule{{ID: "r1", MatchMode: MatchContains, Text: "chor", Action: ActionHide}},
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Title: "Zahnarzt", Icon: def},
		},
		{
			name:  "startsWith matches only the beginning",
			rules: []Rule{{ID: "r1", MatchMode: MatchStartsWith, Text: "Zahn", Action: ActionIcon, Icon: star}},
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Title: "Zahnarzt", Icon: star, Matched: []string{"r1"}},
		},
		{
			name:  "startsWith rejects a match in the middle",
			rules: []Rule{{ID: "r1", MatchMode: MatchStartsWith, Text: "arzt", Action: ActionIcon, Icon: star}},
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Title: "Zahnarzt", Icon: def},
		},
		{
			name:  "equals needs the whole title",
			rules: []Rule{{ID: "r1", MatchMode: MatchEquals, Text: "Zahn", Action: ActionHide}},
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Title: "Zahnarzt", Icon: def},
		},
		{
			name:  "equals matches case-insensitively and trimmed",
			rules: []Rule{{ID: "r1", MatchMode: MatchEquals, Text: "  zahnarzt ", Action: ActionRename, RenameTo: "Arzt"}},
			title: " ZAHNARZT", calendarID: "c1",
			want: Result{Title: "Arzt", Icon: def, Matched: []string{"r1"}},
		},
		{
			name:  "umlauts are case-insensitive",
			rules: []Rule{{ID: "r1", MatchMode: MatchContains, Text: "ÄRZTIN", Action: ActionHide}},
			title: "Kinderärztin", calendarID: "c1",
			want: Result{Hidden: true, Title: "Kinderärztin", Icon: def, Matched: []string{"r1"}},
		},
		{
			name:  "calendar filter excludes other calendars",
			rules: []Rule{{ID: "r1", CalendarIDs: []string{"c2", "c3"}, MatchMode: MatchContains, Text: "arzt", Action: ActionHide}},
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Title: "Zahnarzt", Icon: def},
		},
		{
			name:  "calendar filter includes listed calendars",
			rules: []Rule{{ID: "r1", CalendarIDs: []string{"c2", "c1"}, MatchMode: MatchContains, Text: "arzt", Action: ActionHide}},
			title: "Zahnarzt", calendarID: "c1",
			want: Result{Hidden: true, Title: "Zahnarzt", Icon: def, Matched: []string{"r1"}},
		},
		{
			name:  "empty text applies to every title of the calendar",
			rules: []Rule{{ID: "r1", CalendarIDs: []string{"c1"}, MatchMode: MatchContains, Action: ActionIcon, Icon: star}},
			title: "Irgendwas", calendarID: "c1",
			want: Result{Title: "Irgendwas", Icon: star, Matched: []string{"r1"}},
		},
		{
			name: "first icon rule wins, later ones still match",
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "geburtstag", Action: ActionIcon, Icon: cake},
				{ID: "r2", MatchMode: MatchContains, Text: "oma", Action: ActionIcon, Icon: star},
			},
			title: "Geburtstag Oma", calendarID: "c1",
			want: Result{Title: "Geburtstag Oma", Icon: cake, Matched: []string{"r1", "r2"}},
		},
		{
			name: "first rename rule wins",
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "geburtstag", Action: ActionRename, RenameTo: "Feier"},
				{ID: "r2", MatchMode: MatchContains, Text: "oma", Action: ActionRename, RenameTo: "Besuch"},
			},
			title: "Geburtstag Oma", calendarID: "c1",
			want: Result{Title: "Feier", Icon: def, Matched: []string{"r1", "r2"}},
		},
		{
			name: "rename and icon are independent",
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "oma", Action: ActionRename, RenameTo: "Besuch"},
				{ID: "r2", MatchMode: MatchContains, Text: "oma", Action: ActionIcon, Icon: star},
			},
			title: "Geburtstag Oma", calendarID: "c1",
			want: Result{Title: "Besuch", Icon: star, Matched: []string{"r1", "r2"}},
		},
		{
			name: "rule matches the original title, not an earlier rename",
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "oma", Action: ActionRename, RenameTo: "Besuch"},
				{ID: "r2", MatchMode: MatchContains, Text: "besuch", Action: ActionIcon, Icon: star},
			},
			title: "Oma", calendarID: "c1",
			want: Result{Title: "Besuch", Icon: def, Matched: []string{"r1"}},
		},
		{
			name: "hide stops evaluation: later rules do not match",
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "oma", Action: ActionIcon, Icon: star},
				{ID: "r2", MatchMode: MatchContains, Text: "oma", Action: ActionHide},
				{ID: "r3", MatchMode: MatchContains, Text: "oma", Action: ActionRename, RenameTo: "X"},
			},
			title: "Oma", calendarID: "c1",
			want: Result{Hidden: true, Title: "Oma", Icon: star, Matched: []string{"r1", "r2"}},
		},
		{
			name: "empty rename target changes nothing",
			rules: []Rule{
				{ID: "r1", MatchMode: MatchContains, Text: "oma", Action: ActionRename, RenameTo: " "},
				{ID: "r2", MatchMode: MatchContains, Text: "oma", Action: ActionRename, RenameTo: "Besuch"},
			},
			title: "Oma", calendarID: "c1",
			want: Result{Title: "Besuch", Icon: def, Matched: []string{"r1", "r2"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Apply(tc.rules, tc.calendarID, tc.title, def)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
