package calendar

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// wrap puts VEVENT bodies into a minimal calendar.
func wrap(events ...string) []byte {
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//EN\n")
	for _, e := range events {
		sb.WriteString("BEGIN:VEVENT\n" + strings.TrimSpace(e) + "\nEND:VEVENT\n")
	}
	sb.WriteString("END:VCALENDAR\n")
	return []byte(sb.String())
}

// summarize renders occurrences in Berlin time so tables stay readable.
func summarize(occs []Occurrence, loc *time.Location) []string {
	out := make([]string, 0, len(occs))
	for _, o := range occs {
		s, e := o.Start.In(loc), o.End.In(loc)
		if o.AllDay {
			out = append(out, s.Format("2006-01-02")+".."+e.Format("2006-01-02")+" allday "+o.Title)
		} else {
			out = append(out, s.Format("2006-01-02 15:04")+"-"+e.Format("15:04")+" "+o.Title)
		}
	}
	return out
}

func day(loc *time.Location, y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func TestExpandFixtures(t *testing.T) {
	loc := berlin(t)
	tests := []struct {
		name string
		file string
		from time.Time
		to   time.Time
		want []string
	}{
		{
			name: "google: exdate, override moved out, override moved in, cancelled override",
			file: "google.ics",
			from: day(loc, 2026, 3, 9), to: day(loc, 2026, 3, 24),
			want: []string{
				"2026-03-10 10:00-10:30 Dienstags-Sync",
				"2026-03-20 15:00-16:00 Standup (vorgezogen)",
				"2026-03-23 08:00-09:00 Standup",
			},
		},
		{
			name: "google: override moved into the later window, original suppressed",
			file: "google.ics",
			from: day(loc, 2026, 3, 24), to: day(loc, 2026, 4, 1),
			want: []string{
				"2026-03-24 10:00-10:30 Dienstags-Sync",
				"2026-03-25 10:00-11:00 Standup (verschoben)",
				"2026-03-31 10:00-10:30 Dienstags-Sync",
			},
		},
		{
			name: "outlook: windows zone names, UTC, floating and unknown zone",
			file: "outlook.ics",
			from: day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{
				"2026-03-10 09:00-10:00 Call Chisinau",
				"2026-03-10 10:00-11:00 Call Budapest",
				"2026-03-10 10:00-11:00 Call Paris",
				"2026-03-10 10:00-11:00 Call Unbekannt",
				"2026-03-10 11:00-12:00 Call London",
				"2026-03-10 11:00-12:00 Call UTC-Name",
				"2026-03-10 14:00-15:00 Teammeeting",
				"2026-03-10 14:00-15:00 Webinar",
				"2026-03-10 15:00-16:00 Call New York",
				"2026-03-10 18:00-19:00 Call Los Angeles",
				"2026-03-10 18:00-19:00 Gleitende Zeit",
			},
		},
		{
			name: "icloud: all-day, multi-day, duration, monthly bymonthday, rdate",
			file: "icloud.ics",
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 1),
			want: []string{
				"2026-03-05 19:30-21:30 Chor",
				"2026-03-10 18:00-19:00 Yoga",
				"2026-03-14..2026-03-15 allday Geburtstag Oma",
				"2026-03-16..2026-03-20 allday Urlaub",
				"2026-03-17 18:00-19:00 Yoga",
				"2026-03-20..2026-03-23 allday Kurztrip",
				"2026-03-20 19:30-21:30 Chor",
				"2026-03-21 18:00-19:00 Yoga",
				"2026-03-24 18:00-19:00 Yoga",
				"2026-03-29..2026-03-30 allday Zeitumstellung",
			},
		},
		{
			name: "icloud: multi-day event overlapping the window from before",
			file: "icloud.ics",
			from: day(loc, 2026, 3, 18), to: day(loc, 2026, 3, 19),
			want: []string{"2026-03-16..2026-03-20 allday Urlaub"},
		},
		{
			name: "event ending exactly at window start is excluded",
			file: "icloud.ics",
			from: day(loc, 2026, 3, 20), to: day(loc, 2026, 3, 21),
			want: []string{
				"2026-03-20..2026-03-23 allday Kurztrip",
				"2026-03-20 19:30-21:30 Chor",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			occs, err := Expand(fixture(t, tc.file), Window{tc.from, tc.to}, loc)
			if err != nil {
				t.Fatal(err)
			}
			if got := summarize(occs, loc); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
			}
		})
	}
}

func TestExpandFixtureDetails(t *testing.T) {
	loc := berlin(t)
	win := Window{day(loc, 2026, 3, 23), day(loc, 2026, 3, 24)}
	occs, err := Expand(fixture(t, "google.ics"), win, loc)
	if err != nil || len(occs) != 1 {
		t.Fatalf("occs=%v err=%v", occs, err)
	}
	if o := occs[0]; o.Location != "Büro, Raum 4" || o.UID != "weekly-standup@google.com" || o.AllDay {
		t.Errorf("unexpected occurrence %+v", o)
	}

	occs, err = Expand(fixture(t, "icloud.ics"), Window{day(loc, 2026, 3, 29), day(loc, 2026, 3, 30)}, loc)
	if err != nil || len(occs) != 1 {
		t.Fatalf("occs=%v err=%v", occs, err)
	}
	o := occs[0]
	if !o.AllDay || !o.Start.Equal(day(loc, 2026, 3, 29)) || !o.End.Equal(day(loc, 2026, 3, 30)) {
		t.Errorf("all-day on the DST day must span local midnights, got %v - %v", o.Start, o.End)
	}
	if o.End.Sub(o.Start) != 23*time.Hour {
		t.Errorf("DST day has 23 hours, got %v", o.End.Sub(o.Start))
	}
}

func TestExpandFloatingAndUnknownZoneUseLoc(t *testing.T) {
	utc := time.UTC
	occs, err := Expand(fixture(t, "outlook.ics"), Window{day(utc, 2026, 3, 10), day(utc, 2026, 3, 11)}, utc)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]time.Time{
		"Gleitende Zeit": time.Date(2026, 3, 10, 18, 0, 0, 0, utc),
		"Call Unbekannt": time.Date(2026, 3, 10, 10, 0, 0, 0, utc),
		"Webinar":        time.Date(2026, 3, 10, 13, 0, 0, 0, utc),
	}
	for _, o := range occs {
		if w, ok := want[o.Title]; ok {
			if !o.Start.Equal(w) {
				t.Errorf("%s: start %v, want %v", o.Title, o.Start, w)
			}
			delete(want, o.Title)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing %v", want)
	}
}

func TestExpandRecurrence(t *testing.T) {
	loc := berlin(t)
	tests := []struct {
		name     string
		event    string
		from, to time.Time
		want     []string
	}{
		{
			name: "COUNT limits instances",
			event: `UID:c1
SUMMARY:Zählt
DTSTART;TZID=Europe/Berlin:20260310T090000
DTEND;TZID=Europe/Berlin:20260310T100000
RRULE:FREQ=DAILY;COUNT=3`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 1),
			want: []string{"2026-03-10 09:00-10:00 Zählt", "2026-03-11 09:00-10:00 Zählt", "2026-03-12 09:00-10:00 Zählt"},
		},
		{
			name: "UNTIL in UTC is inclusive",
			event: `UID:u1
SUMMARY:Bis
DTSTART;TZID=Europe/Berlin:20260303T100000
DTEND;TZID=Europe/Berlin:20260303T110000
RRULE:FREQ=WEEKLY;UNTIL=20260317T090000Z`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 30),
			want: []string{"2026-03-03 10:00-11:00 Bis", "2026-03-10 10:00-11:00 Bis", "2026-03-17 10:00-11:00 Bis"},
		},
		{
			name: "date-only UNTIL covers the whole last day",
			event: `UID:u2
SUMMARY:Bis
DTSTART;TZID=Europe/Berlin:20260303T100000
DTEND;TZID=Europe/Berlin:20260303T110000
RRULE:FREQ=WEEKLY;UNTIL=20260317`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 30),
			want: []string{"2026-03-03 10:00-11:00 Bis", "2026-03-10 10:00-11:00 Bis", "2026-03-17 10:00-11:00 Bis"},
		},
		{
			name: "INTERVAL and BYDAY",
			event: `UID:i1
SUMMARY:Alle zwei Wochen
DTSTART;TZID=Europe/Berlin:20260302T180000
DTEND;TZID=Europe/Berlin:20260302T190000
RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 3, 22),
			want: []string{
				"2026-03-02 18:00-19:00 Alle zwei Wochen",
				"2026-03-04 18:00-19:00 Alle zwei Wochen",
				"2026-03-16 18:00-19:00 Alle zwei Wochen",
				"2026-03-18 18:00-19:00 Alle zwei Wochen",
			},
		},
		{
			name: "several EXDATE properties and comma lists, date-time and date",
			event: `UID:x1
SUMMARY:Ausnahmen
DTSTART;TZID=Europe/Berlin:20260302T080000
DTEND;TZID=Europe/Berlin:20260302T090000
RRULE:FREQ=DAILY;COUNT=6
EXDATE;TZID=Europe/Berlin:20260303T080000,20260304T080000
EXDATE:20260305T070000Z
EXDATE;VALUE=DATE:20260306`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 1),
			want: []string{"2026-03-02 08:00-09:00 Ausnahmen", "2026-03-07 08:00-09:00 Ausnahmen"},
		},
		{
			name: "RDATE adds date-times to a rule-less event",
			event: `UID:r1
SUMMARY:Extra
DTSTART;TZID=Europe/Berlin:20260302T080000
DTEND;TZID=Europe/Berlin:20260302T090000
RDATE;TZID=Europe/Berlin:20260310T080000,20260311T080000
RDATE:20260312T070000Z`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 1),
			want: []string{
				"2026-03-02 08:00-09:00 Extra", "2026-03-10 08:00-09:00 Extra",
				"2026-03-11 08:00-09:00 Extra", "2026-03-12 08:00-09:00 Extra",
			},
		},
		{
			name: "all-day weekly series keeps local midnight across DST",
			event: `UID:a1
SUMMARY:Müll
DTSTART;VALUE=DATE:20260323
DTEND;VALUE=DATE:20260324
RRULE:FREQ=WEEKLY;COUNT=3`,
			from: day(loc, 2026, 3, 1), to: day(loc, 2026, 4, 30),
			want: []string{
				"2026-03-23..2026-03-24 allday Müll",
				"2026-03-30..2026-03-31 allday Müll",
				"2026-04-06..2026-04-07 allday Müll",
			},
		},
		{
			name: "series starting long before the window",
			event: `UID:o1
SUMMARY:Alt
DTSTART;TZID=Europe/Berlin:20150105T080000
DTEND;TZID=Europe/Berlin:20150105T090000
RRULE:FREQ=DAILY`,
			from: day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 08:00-09:00 Alt"},
		},
		{
			name: "series instance overlapping window start from before",
			event: `UID:n1
SUMMARY:Nachtschicht
DTSTART;TZID=Europe/Berlin:20260309T220000
DTEND;TZID=Europe/Berlin:20260310T060000
RRULE:FREQ=DAILY;COUNT=3`,
			from: day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-09 22:00-06:00 Nachtschicht", "2026-03-10 22:00-06:00 Nachtschicht"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			occs, err := Expand(wrap(tc.event), Window{tc.from, tc.to}, loc)
			if err != nil {
				t.Fatal(err)
			}
			if got := summarize(occs, loc); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
			}
		})
	}
}

func TestExpandKeepsWallClockAcrossDST(t *testing.T) {
	loc := berlin(t)
	tests := []struct {
		name       string
		dtstart    string
		from, to   time.Time
		wantLocal  []string
		wantOffset []int // hours east of UTC
	}{
		{
			name:    "spring forward 2026-03-29",
			dtstart: "20260302T080000",
			from:    day(loc, 2026, 3, 20), to: day(loc, 2026, 4, 10),
			wantLocal:  []string{"2026-03-23 08:00", "2026-03-30 08:00", "2026-04-06 08:00"},
			wantOffset: []int{1, 2, 2},
		},
		{
			name:    "fall back 2026-10-25",
			dtstart: "20261005T080000",
			from:    day(loc, 2026, 10, 15), to: day(loc, 2026, 11, 5),
			wantLocal:  []string{"2026-10-19 08:00", "2026-10-26 08:00", "2026-11-02 08:00"},
			wantOffset: []int{2, 1, 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := "UID:dst\nSUMMARY:Wecker\nDTSTART;TZID=Europe/Berlin:" + tc.dtstart +
				"\nDTEND;TZID=Europe/Berlin:" + strings.Replace(tc.dtstart, "T08", "T09", 1) +
				"\nRRULE:FREQ=WEEKLY;BYDAY=MO"
			occs, err := Expand(wrap(ev), Window{tc.from, tc.to}, loc)
			if err != nil {
				t.Fatal(err)
			}
			if len(occs) != len(tc.wantLocal) {
				t.Fatalf("got %d occurrences: %v", len(occs), summarize(occs, loc))
			}
			for i, o := range occs {
				if got := o.Start.In(loc).Format("2006-01-02 15:04"); got != tc.wantLocal[i] {
					t.Errorf("occurrence %d at %s, want %s", i, got, tc.wantLocal[i])
				}
				if _, off := o.Start.Zone(); off != tc.wantOffset[i]*3600 {
					t.Errorf("occurrence %d offset %d, want %d h", i, off, tc.wantOffset[i])
				}
				if o.End.Sub(o.Start) != time.Hour {
					t.Errorf("occurrence %d duration %v", i, o.End.Sub(o.Start))
				}
			}
		})
	}
}

func TestExpandGuardLimitsInstances(t *testing.T) {
	loc := berlin(t)
	ev := `UID:g1
SUMMARY:Dauerfeuer
DTSTART:20260301T000000Z
DTEND:20260301T000030Z
RRULE:FREQ=MINUTELY`
	occs, err := Expand(wrap(ev), Window{day(loc, 2026, 3, 1), day(loc, 2026, 5, 1)}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(occs) != maxInstancesPerSeries {
		t.Errorf("got %d instances, want the guard's %d", len(occs), maxInstancesPerSeries)
	}
}

func TestExpandGuardIgnoresPreWindowInstances(t *testing.T) {
	loc := berlin(t)
	// ~5900 daily instances lie before the window, more than the guard allows
	// per series, yet below the iteration cap.
	ev := `UID:old
SUMMARY:Uralt
DTSTART;TZID=Europe/Berlin:20100105T080000
DTEND;TZID=Europe/Berlin:20100105T090000
RRULE:FREQ=DAILY`
	occs, err := Expand(wrap(ev), Window{day(loc, 2026, 3, 10), day(loc, 2026, 3, 12)}, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-03-10 08:00-09:00 Uralt", "2026-03-11 08:00-09:00 Uralt"}
	if got := summarize(occs, loc); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandWindowsZoneMapIsUsed(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	occs, err := Expand(fixture(t, "outlook.ics"), Window{day(ny, 2026, 3, 9), day(ny, 2026, 3, 12)}, ny)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range occs {
		if o.Title == "Teammeeting" {
			if want := time.Date(2026, 3, 10, 14, 0, 0, 0, berlin(t)); !o.Start.Equal(want) {
				t.Errorf("start %v, want %v", o.Start, want)
			}
			return
		}
	}
	t.Error("Teammeeting not found")
}

func TestExpandSimpleEvents(t *testing.T) {
	loc := berlin(t)
	tests := []struct {
		name     string
		event    string
		from, to time.Time
		want     []string
	}{
		{
			name:  "ends exactly at window start: excluded",
			event: "UID:e1\nSUMMARY:Vorbei\nDTSTART;TZID=Europe/Berlin:20260310T100000\nDTEND;TZID=Europe/Berlin:20260310T110000",
			from:  time.Date(2026, 3, 10, 11, 0, 0, 0, loc), to: day(loc, 2026, 3, 11),
		},
		{
			name:  "ends one minute after window start: included",
			event: "UID:e2\nSUMMARY:Läuft\nDTSTART;TZID=Europe/Berlin:20260310T100000\nDTEND;TZID=Europe/Berlin:20260310T110000",
			from:  time.Date(2026, 3, 10, 10, 59, 0, 0, loc), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 10:00-11:00 Läuft"},
		},
		{
			name:  "starts exactly at window end: excluded",
			event: "UID:e3\nSUMMARY:Später\nDTSTART;TZID=Europe/Berlin:20260311T000000\nDTEND;TZID=Europe/Berlin:20260311T010000",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
		},
		{
			name:  "timed event without DTEND has zero length",
			event: "UID:e4\nSUMMARY:Punkt\nDTSTART;TZID=Europe/Berlin:20260310T100000",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 10:00-10:00 Punkt"},
		},
		{
			name:  "timed event with DURATION",
			event: "UID:e5\nSUMMARY:Dauer\nDTSTART;TZID=Europe/Berlin:20260310T100000\nDURATION:PT1H30M",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 10:00-11:30 Dauer"},
		},
		{
			name:  "all-day without DTEND lasts one day",
			event: "UID:e6\nSUMMARY:Tag\nDTSTART;VALUE=DATE:20260310",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10..2026-03-11 allday Tag"},
		},
		{
			name:  "all-day ending at window start (exclusive DTEND): excluded",
			event: "UID:e7\nSUMMARY:Gestern\nDTSTART;VALUE=DATE:20260309\nDTEND;VALUE=DATE:20260310",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
		},
		{
			name:  "cancelled standalone event is dropped",
			event: "UID:e8\nSUMMARY:Weg\nSTATUS:CANCELLED\nDTSTART;TZID=Europe/Berlin:20260310T100000\nDTEND;TZID=Europe/Berlin:20260310T110000",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
		},
		{
			name:  "cancelled master drops the whole series",
			event: "UID:e9\nSUMMARY:Weg\nSTATUS:CANCELLED\nDTSTART;TZID=Europe/Berlin:20260310T100000\nDTEND;TZID=Europe/Berlin:20260310T110000\nRRULE:FREQ=DAILY",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 20),
		},
		{
			name:  "missing summary gets a default title",
			event: "UID:e10\nDTSTART;TZID=Europe/Berlin:20260310T100000\nDTEND;TZID=Europe/Berlin:20260310T110000",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 10:00-11:00 Termin"},
		},
		{
			name:  "text escapes incl. unknown ones",
			event: `UID:e11` + "\n" + `SUMMARY:Arzt\, Dr. Müller\; 10\:00` + "\n" + `DTSTART;TZID=Europe/Berlin:20260310T100000` + "\n" + `DTEND;TZID=Europe/Berlin:20260310T110000`,
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 10:00-11:00 Arzt, Dr. Müller; 10:00"},
		},
		{
			name:  "quoted TZID and path style TZID",
			event: "UID:e12\nSUMMARY:Pfad\nDTSTART;TZID=\"/mozilla.org/20050126_1/Europe/Berlin\":20260310T100000\nDTEND;TZID=\"/mozilla.org/20050126_1/Europe/Berlin\":20260310T110000",
			from:  day(loc, 2026, 3, 10), to: day(loc, 2026, 3, 11),
			want: []string{"2026-03-10 10:00-11:00 Pfad"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			occs, err := Expand(wrap(tc.event), Window{tc.from, tc.to}, loc)
			if err != nil {
				t.Fatal(err)
			}
			got := summarize(occs, loc)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExpandOverrideWithoutMasterStandsAlone(t *testing.T) {
	loc := berlin(t)
	ev := `UID:orphan
SUMMARY:Einzelinstanz
RECURRENCE-ID;TZID=Europe/Berlin:20260310T100000
DTSTART;TZID=Europe/Berlin:20260310T150000
DTEND;TZID=Europe/Berlin:20260310T160000`
	occs, err := Expand(wrap(ev), Window{day(loc, 2026, 3, 10), day(loc, 2026, 3, 11)}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := summarize(occs, loc); !reflect.DeepEqual(got, []string{"2026-03-10 15:00-16:00 Einzelinstanz"}) {
		t.Errorf("got %v", got)
	}
}

func TestExpandAllDayOverrideMatchesByDate(t *testing.T) {
	loc := berlin(t)
	master := `UID:ad
SUMMARY:Müll
DTSTART;VALUE=DATE:20260310
DTEND;VALUE=DATE:20260311
RRULE:FREQ=DAILY;COUNT=3`
	override := `UID:ad
SUMMARY:Müll verschoben
RECURRENCE-ID;VALUE=DATE:20260311
DTSTART;VALUE=DATE:20260320
DTEND;VALUE=DATE:20260321`
	occs, err := Expand(wrap(master, override), Window{day(loc, 2026, 3, 1), day(loc, 2026, 4, 1)}, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2026-03-10..2026-03-11 allday Müll",
		"2026-03-12..2026-03-13 allday Müll",
		"2026-03-20..2026-03-21 allday Müll verschoben",
	}
	if got := summarize(occs, loc); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandInputFormats(t *testing.T) {
	loc := berlin(t)
	win := Window{day(loc, 2026, 3, 1), day(loc, 2026, 4, 1)}
	tests := []struct {
		name string
		data string
		want []string
	}{
		{
			name: "CRLF line endings and folded lines (space and tab)",
			data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:crlf\r\nSUMMARY:Ein sehr lan\r\n ger Titel\r\nLOCATION:Haupt\r\n\tstraße 1\r\nDTSTART;TZID=Europe/Berlin:20260310T100000\r\nDTEND;TZID=Europe/Berlin:20260310T110000\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
			want: []string{"2026-03-10 10:00-11:00 Ein sehr langer Titel"},
		},
		{
			name: "LF only without trailing newline",
			data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:lf\nSUMMARY:LF\nDTSTART:20260310T100000Z\nDTEND:20260310T110000Z\nEND:VEVENT\nEND:VCALENDAR",
			want: []string{"2026-03-10 11:00-12:00 LF"},
		},
		{
			name: "byte order mark",
			data: "\xef\xbb\xbfBEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:bom\nSUMMARY:BOM\nDTSTART:20260310T100000Z\nDTEND:20260310T110000Z\nEND:VEVENT\nEND:VCALENDAR\n",
			want: []string{"2026-03-10 11:00-12:00 BOM"},
		},
		{
			name: "broken event is skipped, the others stay",
			data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:bad\nSUMMARY:Kaputt\nDTSTART:keinDatum\nEND:VEVENT\nBEGIN:VEVENT\nUID:nostart\nSUMMARY:Ohne Start\nEND:VEVENT\nBEGIN:VEVENT\nUID:badrule\nSUMMARY:Regelkaputt\nDTSTART:20260310T120000Z\nRRULE:FREQ=GIBTSNICHT\nEND:VEVENT\nBEGIN:VEVENT\nUID:ok\nSUMMARY:Gut\nDTSTART:20260310T100000Z\nDTEND:20260310T110000Z\nEND:VEVENT\nEND:VCALENDAR\n",
			want: []string{"2026-03-10 11:00-12:00 Gut", "2026-03-10 13:00-13:00 Regelkaputt"},
		},
		{
			name: "two calendars in one stream",
			data: "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:a\nSUMMARY:A\nDTSTART:20260310T100000Z\nEND:VEVENT\nEND:VCALENDAR\nBEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:b\nSUMMARY:B\nDTSTART:20260310T110000Z\nEND:VEVENT\nEND:VCALENDAR\n",
			want: []string{"2026-03-10 11:00-11:00 A", "2026-03-10 12:00-12:00 B"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			occs, err := Expand([]byte(tc.data), win, loc)
			if err != nil {
				t.Fatal(err)
			}
			if got := summarize(occs, loc); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExpandErrors(t *testing.T) {
	loc := berlin(t)
	win := Window{day(loc, 2026, 3, 1), day(loc, 2026, 4, 1)}
	for name, data := range map[string]string{
		"empty":          "",
		"whitespace":     "  \n\n",
		"garbage":        "dies ist kein kalender",
		"html page":      "<html><body>Login</body></html>",
		"wrong toplevel": "BEGIN:VEVENT\nUID:x\nEND:VEVENT\n",
		"truncated":      "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:x\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Expand([]byte(data), win, loc); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestBase64ShorthandIsNormalized(t *testing.T) {
	data := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:b64\nSUMMARY:Anhang\n" +
		"ATTACH;ENCODING=B;VALUE=BINARY:SGFsbG8=\n" +
		"ATTACH;encoding = \"b\";VALUE=BINARY:SGFsbG8=\n" +
		"DTSTART:20260310T100000Z\nEND:VEVENT\nEND:VCALENDAR\n"
	comps, err := decodeEvents([]byte(data))
	if err != nil || len(comps) != 1 {
		t.Fatalf("comps=%v err=%v", comps, err)
	}
	for i, p := range comps[0].Props.Values("ATTACH") {
		if got := p.Params.Get("ENCODING"); got != "BASE64" {
			t.Errorf("ATTACH %d: ENCODING=%q, want BASE64", i, got)
		}
	}
}

func TestResolveLocation(t *testing.T) {
	loc := time.UTC
	tests := []struct{ tzid, want string }{
		{"W. Europe Standard Time", "Europe/Berlin"},
		{"Romance Standard Time", "Europe/Paris"},
		{"GMT Standard Time", "Europe/London"},
		{"Central Europe Standard Time", "Europe/Budapest"},
		{"E. Europe Standard Time", "Europe/Chisinau"},
		{"UTC", "UTC"},
		{"Eastern Standard Time", "America/New_York"},
		{"Pacific Standard Time", "America/Los_Angeles"},
		{"Europe/Berlin", "Europe/Berlin"},
		{`"Europe/Berlin"`, "Europe/Berlin"},
		{"/softwarestudio.org/Tzfile/Europe/Berlin", "Europe/Berlin"},
		{"Nirgendwo/Unbekannt", "UTC"},
		{"Local", "UTC"},
		{"", "UTC"},
	}
	for _, tc := range tests {
		if got := resolveLocation(tc.tzid, loc).String(); got != tc.want {
			t.Errorf("%q -> %q, want %q", tc.tzid, got, tc.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		bad  bool
	}{
		{"PT1H", time.Hour, false},
		{"PT1H30M", 90 * time.Minute, false},
		{"P1D", 24 * time.Hour, false},
		{"P1W", 7 * 24 * time.Hour, false},
		{"P1DT2H", 26 * time.Hour, false},
		{"-PT15M", -15 * time.Minute, false},
		{"PT45S", 45 * time.Second, false},
		{"1H", 0, true},
	}
	for _, tc := range tests {
		got, err := parseDuration(tc.in)
		if (err != nil) != tc.bad || got != tc.want {
			t.Errorf("%q -> %v, %v", tc.in, got, err)
		}
	}
}
