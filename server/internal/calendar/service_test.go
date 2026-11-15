package calendar

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// serviceEnv serves the .ics fixtures over TLS; /fail answers 500.
func serviceEnv(t *testing.T) (*Service, string) {
	t.Helper()
	feeds, srv, clock, logs := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/garbage" {
			_, _ = w.Write([]byte("this is not a calendar"))
			return
		}
		b, err := os.ReadFile(filepath.Join("testdata", strings.TrimPrefix(r.URL.Path, "/")))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	_ = logs
	return NewService(ServiceOptions{Feeds: feeds, Now: clock.now, Location: berlin(t)}), srv.URL
}

func TestServiceListSection(t *testing.T) {
	svc, base := serviceEnv(t)
	cfg := Config{
		RefreshMinutes: 15,
		Calendars:      []Calendar{{ID: "familie", Name: "Familie", URL: base + "/google.ics", Icon: IconRef{Kind: IconBootstrap, Name: "house"}}},
		Sections:       []Section{{ID: "termine", Title: "Termine", Type: TypeList, CalendarIDs: []string{"familie"}, MaxEvents: 3, LookAheadDays: 14}},
		Rules:          []Rule{{ID: "umbenennen", MatchMode: MatchEquals, Text: "Standup", Action: ActionRename, RenameTo: "Morgenrunde"}},
	}
	res, ok := svc.Section(context.Background(), cfg, "termine")
	if !ok || res.Error != "" {
		t.Fatalf("ok=%v error=%q", ok, res.Error)
	}
	// Saturday 2026-11-14 13:00 Berlin: Monday standup (08:00) comes first.
	if len(res.Items) != 3 {
		t.Fatalf("items = %+v", res.Items)
	}
	first := res.Items[0]
	if first.Title != "Morgenrunde" || first.OriginalTitle != "Standup" || first.Icon.Name != "house" {
		t.Errorf("first = %+v", first)
	}
	if !first.Start.Equal(time.Date(2026, 11, 16, 8, 0, 0, 0, berlin(t))) {
		t.Errorf("first start = %v", first.Start)
	}
	if len(res.Candidates) < len(res.Items) {
		t.Errorf("candidates %d < items %d", len(res.Candidates), len(res.Items))
	}
	if _, ok := svc.Section(context.Background(), cfg, "nope"); ok {
		t.Error("unknown section found")
	}
}

func TestServiceShiftsSection(t *testing.T) {
	svc, base := serviceEnv(t)
	cfg := Config{
		RefreshMinutes: 15,
		Calendars:      []Calendar{{ID: "dienst", Name: "Dienst", URL: base + "/google.ics"}},
		Sections: []Section{{ID: "plan", Title: "Dienstplan", Type: TypeShifts, CalendarIDs: []string{"dienst"}, ShiftDays: 4, LookAheadDays: 7,
			Shifts: []Shift{{ID: "sync", Match: "Sync", MatchMode: MatchContains, Label: "Spätdienst", Start: "14:00", End: "22:00"}}}},
	}
	res, _ := svc.Section(context.Background(), cfg, "plan")
	got := make([]string, 0, len(res.Days))
	for _, d := range res.Days {
		got = append(got, d.DayLabel+": "+d.Title+" "+d.TimeRange)
	}
	want := []string{"Montag: Standup ", "Dienstag: Spätdienst 14:00–22:00"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %q\nwant   %q", got, want)
	}
	if len(res.Days) != 2 || res.Days[1].Free {
		t.Errorf("days = %+v", res.Days)
	}
}

func TestServicePartialAndTotalFailure(t *testing.T) {
	svc, base := serviceEnv(t)
	good := Calendar{ID: "familie", Name: "Familie", URL: base + "/google.ics"}
	bad := Calendar{ID: "kaputt", Name: "Arbeit", URL: base + "/fail?token=SECRET"}
	bad2 := Calendar{ID: "kaputt2", Name: "Verein", URL: base + "/fail2"}
	sec := func(ids ...string) Section {
		return Section{ID: "s", Title: "S", Type: TypeList, CalendarIDs: ids, MaxEvents: 5, LookAheadDays: 14}
	}
	tests := []struct {
		name      string
		section   Section
		wantError string
		wantItems bool
		wantCals  []CalendarError
	}{
		{"all good", sec("familie"), "", true, nil},
		{"one calendar failing", sec("kaputt"), "Kalender „Arbeit“ konnte nicht geladen werden.", false,
			[]CalendarError{{"kaputt", "Kalender „Arbeit“ konnte nicht geladen werden."}}},
		{"all of several failing", sec("kaputt", "kaputt2"), "Die Kalender konnten nicht geladen werden.", false,
			[]CalendarError{{"kaputt", "Kalender „Arbeit“ konnte nicht geladen werden."}, {"kaputt2", "Kalender „Verein“ konnte nicht geladen werden."}}},
		{"some failing", sec("familie", "kaputt"), "Einige Kalender konnten nicht geladen werden.", true,
			[]CalendarError{{"kaputt", "Kalender „Arbeit“ konnte nicht geladen werden."}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{RefreshMinutes: 15, Calendars: []Calendar{good, bad, bad2}, Sections: []Section{tt.section}}
			res, _ := svc.Section(context.Background(), cfg, "s")
			if res.Error != tt.wantError {
				t.Errorf("error = %q, want %q", res.Error, tt.wantError)
			}
			if (len(res.Items) > 0) != tt.wantItems {
				t.Errorf("items = %d, want items: %v", len(res.Items), tt.wantItems)
			}
			if strings.Contains(res.Error, "SECRET") {
				t.Error("error leaks the address")
			}
			_, calErrs := svc.Preview(context.Background(), cfg)
			if len(calErrs) != len(tt.wantCals) {
				t.Fatalf("calendar errors = %+v", calErrs)
			}
			for i := range calErrs {
				if calErrs[i] != tt.wantCals[i] {
					t.Errorf("calendar error %d = %+v, want %+v", i, calErrs[i], tt.wantCals[i])
				}
			}
		})
	}
}

func TestServiceUnparsableFeedCountsAsFailure(t *testing.T) {
	svc, base := serviceEnv(t)
	cfg := Config{
		RefreshMinutes: 15,
		Calendars:      []Calendar{{ID: "x", Name: "X", URL: base + "/garbage"}},
		Sections:       []Section{{ID: "s", Title: "S", Type: TypeList, CalendarIDs: []string{"x"}, MaxEvents: 5, LookAheadDays: 7}},
	}
	res, _ := svc.Section(context.Background(), cfg, "s")
	if res.Error != "Kalender „X“ konnte nicht geladen werden." {
		t.Errorf("error = %q", res.Error)
	}
}
