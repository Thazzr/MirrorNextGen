package settings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thazzr/mirrornextgen/server/internal/modules"
)

func goodCalendar() CalendarSettings {
	none := IconRef{Kind: "none"}
	return CalendarSettings{
		RefreshMinutes: 15,
		Calendars: []CalendarSource{
			{ID: "familie", Name: "Familie", URL: "https://calendar.google.com/calendar/ical/x/basic.ics", Icon: IconRef{Kind: "bootstrap", Name: "house"}},
			{ID: "dienst", Name: "Dienst", URL: "webcal://example.org/dienst.ics", Icon: none},
		},
		Sections: []CalendarSection{
			{ID: "termine", Title: "Termine", Type: "list", CalendarIDs: []string{"familie"}, MaxEvents: 8, LookAheadDays: 14, ShiftDays: 2, Shifts: []CalendarShift{}, Highlight: DefaultHighlight()},
			{ID: "schichten", Title: "Dienstplan", Type: "shifts", CalendarIDs: []string{"dienst"}, MaxEvents: 8, LookAheadDays: 14, ShiftDays: 2, Highlight: DefaultHighlight(), Shifts: []CalendarShift{
				{ID: "frueh", Match: "F", MatchMode: "equals", Label: "Frühdienst", Start: "06:00", End: "14:00", Icon: none},
			}},
		},
		Rules: []CalendarRule{
			{ID: "r1", CalendarIDs: []string{}, MatchMode: "contains", Text: "Müll", Action: "icon", Icon: IconRef{Kind: "bootstrap", Name: "trash"}},
		},
	}
}

func withCalendar(c CalendarSettings) Settings {
	s := Defaults()
	s.Calendar = c
	s.Layout = Layout{TopLeft: []string{"clock", "calendar:termine", "calendar:schichten"}, TopRight: []string{}, Middle: []string{}, BottomLeft: []string{}, BottomRight: []string{}}
	return s
}

func TestCalendarDefaultsAreValid(t *testing.T) {
	if errs := Defaults().Validate(); errs != nil {
		t.Fatalf("defaults invalid: %v", errs)
	}
	if got := Defaults().Calendar.RefreshMinutes; got != 15 {
		t.Errorf("refreshMinutes default = %d", got)
	}
	if errs := withCalendar(goodCalendar()).Validate(); errs != nil {
		t.Fatalf("good calendar invalid: %v", errs)
	}
}

func TestValidateCalendar(t *testing.T) {
	long := strings.Repeat("x", 61)
	tests := []struct {
		name   string
		mutate func(c *CalendarSettings)
		path   string
		msg    string
	}{
		{"refresh too low", func(c *CalendarSettings) { c.RefreshMinutes = 4 }, "calendar.refreshMinutes", msgCalRefresh},
		{"refresh too high", func(c *CalendarSettings) { c.RefreshMinutes = 1441 }, "calendar.refreshMinutes", msgCalRefresh},
		{"calendar id pattern", func(c *CalendarSettings) { c.Calendars[1].ID = "Dienst!" }, "calendar.calendars.1.id", msgCalID},
		{"calendar id duplicate", func(c *CalendarSettings) { c.Calendars[1].ID = "familie" }, "calendar.calendars.1.id", msgCalIDTaken},
		{"calendar name empty", func(c *CalendarSettings) { c.Calendars[0].Name = " " }, "calendar.calendars.0.name", msgCalName},
		{"calendar name long", func(c *CalendarSettings) { c.Calendars[0].Name = long }, "calendar.calendars.0.name", msgCalName},
		{"calendar url missing", func(c *CalendarSettings) { c.Calendars[1].URL = "" }, "calendar.calendars.1.url", msgCalURLMissing},
		{"calendar url scheme", func(c *CalendarSettings) { c.Calendars[0].URL = "ftp://example.org/a.ics" }, "calendar.calendars.0.url", msgCalURLShape},
		{"calendar url no host", func(c *CalendarSettings) { c.Calendars[0].URL = "https:///a.ics" }, "calendar.calendars.0.url", msgCalURLShape},
		{"calendar url long", func(c *CalendarSettings) { c.Calendars[0].URL = "https://example.org/" + strings.Repeat("a", 2000) }, "calendar.calendars.0.url", msgCalURLLength},
		{"calendar icon kind", func(c *CalendarSettings) { c.Calendars[0].Icon = IconRef{Kind: "emoji"} }, "calendar.calendars.0.icon", msgIconKind},
		{"calendar icon bootstrap name", func(c *CalendarSettings) { c.Calendars[0].Icon = IconRef{Kind: "bootstrap", Name: "Haus Eins"} }, "calendar.calendars.0.icon", msgIconName},
		{"section id pattern", func(c *CalendarSettings) { c.Sections[0].ID = "a b" }, "calendar.sections.0.id", msgCalID},
		{"section id duplicate", func(c *CalendarSettings) { c.Sections[1].ID = "termine" }, "calendar.sections.1.id", msgCalIDTaken},
		{"section title empty", func(c *CalendarSettings) { c.Sections[1].Title = "" }, "calendar.sections.1.title", msgCalName},
		{"section type", func(c *CalendarSettings) { c.Sections[0].Type = "grid" }, "calendar.sections.0.type", msgSecType},
		{"section no calendars", func(c *CalendarSettings) { c.Sections[0].CalendarIDs = nil }, "calendar.sections.0.calendarIds", msgSecCalendars},
		{"section unknown calendar", func(c *CalendarSettings) { c.Sections[0].CalendarIDs = []string{"gibtsnicht"} }, "calendar.sections.0.calendarIds", "Den Kalender „gibtsnicht“ gibt es nicht."},
		{"section maxEvents low", func(c *CalendarSettings) { c.Sections[0].MaxEvents = 0 }, "calendar.sections.0.maxEvents", msgSecMaxEvents},
		{"section maxEvents high", func(c *CalendarSettings) { c.Sections[0].MaxEvents = 31 }, "calendar.sections.0.maxEvents", msgSecMaxEvents},
		{"highlight ongoing color", func(c *CalendarSettings) { c.Sections[0].Highlight.Ongoing.Color = "#12345" }, "calendar.sections.0.highlight.ongoing.color", msgHighColor},
		{"highlight soon color", func(c *CalendarSettings) { c.Sections[1].Highlight.Soon.Color = "orange" }, "calendar.sections.1.highlight.soon.color", msgHighColor},
		{"highlight today color", func(c *CalendarSettings) { c.Sections[0].Highlight.Today.Color = "#gggggg" }, "calendar.sections.0.highlight.today.color", msgHighColor},
		{"highlight hours low", func(c *CalendarSettings) { c.Sections[0].Highlight.Soon.Hours = 0 }, "calendar.sections.0.highlight.soon.hours", msgHighHours},
		{"highlight hours high", func(c *CalendarSettings) { c.Sections[0].Highlight.Soon.Hours = 49 }, "calendar.sections.0.highlight.soon.hours", msgHighHours},
		{"section lookAhead high", func(c *CalendarSettings) { c.Sections[0].LookAheadDays = 61 }, "calendar.sections.0.lookAheadDays", msgSecLookAhead},
		{"section shiftDays high", func(c *CalendarSettings) { c.Sections[1].ShiftDays = 15 }, "calendar.sections.1.shiftDays", msgSecShiftDays},
		{"section shiftDays negative", func(c *CalendarSettings) { c.Sections[1].ShiftDays = -1 }, "calendar.sections.1.shiftDays", msgSecShiftDays},
		{"shifts only for type shifts", func(c *CalendarSettings) { c.Sections[0].Shifts = c.Sections[1].Shifts }, "calendar.sections.0.shifts", msgSecShiftsOnly},
		{"shift id pattern", func(c *CalendarSettings) { c.Sections[1].Shifts[0].ID = "F!" }, "calendar.sections.1.shifts.0.id", msgCalID},
		{"shift id duplicate", func(c *CalendarSettings) {
			c.Sections[1].Shifts = append(c.Sections[1].Shifts, c.Sections[1].Shifts[0])
		}, "calendar.sections.1.shifts.1.id", msgCalIDTaken},
		{"shift match empty", func(c *CalendarSettings) { c.Sections[1].Shifts[0].Match = "" }, "calendar.sections.1.shifts.0.match", msgShiftMatch},
		{"shift match long", func(c *CalendarSettings) { c.Sections[1].Shifts[0].Match = strings.Repeat("a", 41) }, "calendar.sections.1.shifts.0.match", msgShiftMatch},
		{"shift mode", func(c *CalendarSettings) { c.Sections[1].Shifts[0].MatchMode = "regex" }, "calendar.sections.1.shifts.0.matchMode", msgShiftMode},
		{"shift label empty", func(c *CalendarSettings) { c.Sections[1].Shifts[0].Label = "" }, "calendar.sections.1.shifts.0.label", msgShiftLabel},
		{"shift start without end", func(c *CalendarSettings) { c.Sections[1].Shifts[0].End = "" }, "calendar.sections.1.shifts.0.end", msgShiftBoth},
		{"shift end without start", func(c *CalendarSettings) { c.Sections[1].Shifts[0].Start = "" }, "calendar.sections.1.shifts.0.start", msgShiftBoth},
		{"shift start format", func(c *CalendarSettings) { c.Sections[1].Shifts[0].Start = "6:00" }, "calendar.sections.1.shifts.0.start", msgShiftTime},
		{"shift end invalid", func(c *CalendarSettings) { c.Sections[1].Shifts[0].End = "25:00" }, "calendar.sections.1.shifts.0.end", msgShiftTime},
		{"shift icon", func(c *CalendarSettings) { c.Sections[1].Shifts[0].Icon = IconRef{Kind: "upload"} }, "calendar.sections.1.shifts.0.icon", msgIconFile},
		{"rule id duplicate", func(c *CalendarSettings) {
			c.Rules = append(c.Rules, CalendarRule{ID: "r1", MatchMode: "equals", Action: "hide"})
		}, "calendar.rules.1.id", msgCalIDTaken},
		{"rule unknown calendar", func(c *CalendarSettings) { c.Rules[0].CalendarIDs = []string{"nope"} }, "calendar.rules.0.calendarIds", "Den Kalender „nope“ gibt es nicht."},
		{"rule mode", func(c *CalendarSettings) { c.Rules[0].MatchMode = "regex" }, "calendar.rules.0.matchMode", msgRuleMode},
		{"rule text long", func(c *CalendarSettings) { c.Rules[0].Text = strings.Repeat("a", 81) }, "calendar.rules.0.text", msgRuleText},
		{"rule action", func(c *CalendarSettings) { c.Rules[0].Action = "delete" }, "calendar.rules.0.action", msgRuleAction},
		{"rule icon action needs icon", func(c *CalendarSettings) { c.Rules[0].Icon = IconRef{Kind: "none"} }, "calendar.rules.0.icon", msgRuleIcon},
		{"rule rename needs title", func(c *CalendarSettings) {
			c.Rules[0].Action, c.Rules[0].RenameTo = "rename", ""
		}, "calendar.rules.0.renameTo", msgRuleRename},
		{"rule rename long", func(c *CalendarSettings) {
			c.Rules[0].Action, c.Rules[0].RenameTo = "rename", strings.Repeat("a", 81)
		}, "calendar.rules.0.renameTo", msgRuleRename},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := goodCalendar().clone()
			tt.mutate(&c)
			errs := withCalendar(c).Validate()
			if got := errs[tt.path]; got != tt.msg {
				t.Errorf("errs[%q] = %q, want %q (all: %v)", tt.path, got, tt.msg, errs)
			}
		})
	}
}

func TestRuleIconOnlyRequiredForIconAction(t *testing.T) {
	c := goodCalendar()
	c.Rules[0] = CalendarRule{ID: "r1", MatchMode: "contains", Action: "hide", Icon: IconRef{Kind: "none"}}
	if errs := withCalendar(c).Validate(); errs != nil {
		t.Errorf("hide rule without icon rejected: %v", errs)
	}
}

func TestUploadedIconIdsAreCheckedAgainstTheStore(t *testing.T) {
	c := goodCalendar()
	c.Calendars[0].Icon = IconRef{Kind: "upload", File: "abcdefgh12345678"}
	s := withCalendar(c)
	if errs := s.validate(func(string) bool { return true }); errs != nil {
		t.Errorf("existing icon rejected: %v", errs)
	}
	errs := s.validate(func(string) bool { return false })
	if errs["calendar.calendars.0.icon"] != msgIconFile {
		t.Errorf("missing icon not reported: %v", errs)
	}
	// Loading a file does not check (nil checker).
	if errs := s.Validate(); errs != nil {
		t.Errorf("Validate without checker: %v", errs)
	}
}

func TestURLHint(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://calendar.google.com/calendar/ical/abc%40gmail.com/private-1234567890/basic.ics", "https://calendar.google.com/…/basic.ics"},
		{"webcal://example.org/a.ics?token=SECRET", "webcal://example.org/a.ics"},
		{"https://example.org/", "https://example.org"},
		{"https://example.org/x/" + strings.Repeat("a", 40) + ".ics", "https://example.org/…/" + strings.Repeat("a", 23) + "…"},
		{"https://user:pw@example.org:8443/dir/f.ics", "https://example.org/…/f.ics"},
		{"garbage", ""},
	}
	for _, tt := range tests {
		got := URLHint(tt.in)
		if got != tt.want {
			t.Errorf("URLHint(%q) = %q, want %q", tt.in, got, tt.want)
		}
		for _, secret := range []string{"SECRET", "pw@", "token"} {
			if strings.Contains(got, secret) {
				t.Errorf("hint %q leaks %q", got, secret)
			}
		}
	}
}

func updateFrom(c CalendarSettings, urlFor func(id string) string) CalendarUpdate {
	u := CalendarUpdate{RefreshMinutes: c.RefreshMinutes, Sections: c.Sections, Rules: c.Rules}
	for _, cal := range c.Calendars {
		u.Calendars = append(u.Calendars, CalendarSourceUpdate{ID: cal.ID, Name: cal.Name, URL: urlFor(cal.ID), Icon: cal.Icon})
	}
	return u
}

func TestUpdateCalendarSecretURLRules(t *testing.T) {
	st, err := Load(t.TempDir(), quiet)
	if err != nil {
		t.Fatal(err)
	}
	good := goodCalendar()
	orig := map[string]string{"familie": good.Calendars[0].URL, "dienst": good.Calendars[1].URL}
	if _, errs, err := st.UpdateCalendar(updateFrom(good, func(id string) string { return orig[id] })); err != nil || errs != nil {
		t.Fatalf("initial update: %v %v", errs, err)
	}

	// Empty URL keeps the stored one.
	cur, errs, err := st.UpdateCalendar(updateFrom(good, func(string) string { return "" }))
	if err != nil || errs != nil {
		t.Fatalf("keep: %v %v", errs, err)
	}
	for i, c := range cur.Calendars {
		if c.URL != orig[c.ID] {
			t.Errorf("calendar %d lost its URL: %q", i, c.URL)
		}
	}
	// A new URL replaces only its own calendar.
	cur, errs, _ = st.UpdateCalendar(updateFrom(good, func(id string) string {
		if id == "dienst" {
			return " https://example.org/neu.ics "
		}
		return ""
	}))
	if errs != nil {
		t.Fatalf("replace: %v", errs)
	}
	if cur.Calendars[1].URL != "https://example.org/neu.ics" || cur.Calendars[0].URL != orig["familie"] {
		t.Errorf("replace result: %+v", cur.Calendars)
	}
	// A new id without URL is an error at its index, and nothing is stored.
	next := good.clone()
	next.Calendars = append(next.Calendars, CalendarSource{ID: "neu", Name: "Neu", Icon: IconRef{Kind: "none"}})
	_, errs, _ = st.UpdateCalendar(updateFrom(next, func(string) string { return "" }))
	if errs["calendars.2.url"] != "Bitte gib die Adresse des Kalenders an." {
		t.Errorf("errs = %v", errs)
	}
	if len(st.Get().Calendar.Calendars) != 2 {
		t.Error("a refused update changed the stored calendars")
	}
	// A bad scheme is refused without echoing the address.
	_, errs, _ = st.UpdateCalendar(updateFrom(good, func(id string) string {
		if id == "familie" {
			return "gopher://example.org/SECRET"
		}
		return ""
	}))
	if errs["calendars.0.url"] != msgCalURLShape || strings.Contains(errs["calendars.0.url"], "SECRET") {
		t.Errorf("errs = %v", errs)
	}
}

func TestUpdateCalendarFillsDefaultsAndNormalizesIcons(t *testing.T) {
	st, _ := Load(t.TempDir(), quiet)
	u := CalendarUpdate{
		Calendars: []CalendarSourceUpdate{{ID: "a", Name: "A", URL: "https://example.org/a.ics"}},
		Sections:  []CalendarSection{{ID: "s", Title: "S", Type: "list", CalendarIDs: []string{"a"}, Highlight: DefaultHighlight()}},
	}
	cur, errs, err := st.UpdateCalendar(u)
	if err != nil || errs != nil {
		t.Fatalf("%v %v", errs, err)
	}
	sec := cur.Sections[0]
	if cur.RefreshMinutes != 15 || sec.MaxEvents != 8 || sec.LookAheadDays != 14 || sec.ShiftDays != 5 || sec.ShowFreeDays {
		t.Errorf("defaults: refresh %d, %+v", cur.RefreshMinutes, sec)
	}
	if cur.Calendars[0].Icon.Kind != "none" {
		t.Errorf("icon kind = %q", cur.Calendars[0].Icon.Kind)
	}
}

func TestUpdateCalendarCouplesTheLayout(t *testing.T) {
	st, _ := Load(t.TempDir(), quiet)
	cal := []CalendarSourceUpdate{{ID: "a", Name: "A", URL: "https://example.org/a.ics"}}
	sec := func(ids ...string) []CalendarSection {
		var out []CalendarSection
		for _, id := range ids {
			out = append(out, CalendarSection{ID: id, Title: id, Type: "list", CalendarIDs: []string{"a"}, Highlight: DefaultHighlight()})
		}
		return out
	}
	if _, errs, err := st.UpdateCalendar(CalendarUpdate{Calendars: cal, Sections: sec("familie")}); err != nil || errs != nil {
		t.Fatalf("%v %v", errs, err)
	}
	if got := st.Get().Layout.TopLeft; !slices.Equal(got, []string{"clock", "calendar:familie"}) {
		t.Errorf("topLeft after adding = %v", got)
	}
	// The user moves the section to the middle; a second section is appended
	// to topLeft, the first stays where the user put it.
	l := st.Get().Layout
	l.TopLeft, l.Middle = []string{"clock"}, []string{"calendar:familie"}
	if _, errs, _ := st.UpdateLayout(l); errs != nil {
		t.Fatal(errs)
	}
	if _, errs, _ := st.UpdateCalendar(CalendarUpdate{Calendars: cal, Sections: sec("familie", "arbeit")}); errs != nil {
		t.Fatal(errs)
	}
	got := st.Get().Layout
	if !slices.Equal(got.TopLeft, []string{"clock", "calendar:arbeit"}) || !slices.Equal(got.Middle, []string{"calendar:familie"}) {
		t.Errorf("layout = %+v", got)
	}
	// Removing a section removes its key, wherever it stands.
	if _, errs, _ := st.UpdateCalendar(CalendarUpdate{Calendars: cal, Sections: sec("arbeit")}); errs != nil {
		t.Fatal(errs)
	}
	got = st.Get().Layout
	if len(got.Middle) != 0 || !slices.Equal(got.TopLeft, []string{"clock", "calendar:arbeit"}) {
		t.Errorf("layout after removal = %+v", got)
	}
	// A refused update leaves the layout alone.
	if _, errs, _ := st.UpdateCalendar(CalendarUpdate{Calendars: cal, Sections: sec("arbeit", "Bad Id")}); errs == nil {
		t.Fatal("bad id accepted")
	}
	if got := st.Get().Layout.TopLeft; !slices.Equal(got, []string{"clock", "calendar:arbeit"}) {
		t.Errorf("layout after refusal = %v", got)
	}
}

func TestCalendarModulesAreListedWithNames(t *testing.T) {
	c := goodCalendar()
	s := withCalendar(c)
	got := modules.Available(s.ModuleInput())
	want := []modules.Info{
		{Key: "clock", Name: "Uhr"},
		{Key: "weather", Name: "Wetter"},
		{Key: "camera", Name: "Kamera"},
		{Key: "calendar:termine", Name: "Kalender: Termine"},
		{Key: "calendar:schichten", Name: "Schichten: Dienstplan"},
		{Key: "wifi", Name: "WLAN-QR-Code"},
		{Key: "bring", Name: "Bring!-Einkaufsliste"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Available = %v", got)
	}
	if !s.ModuleAvailable("calendar:termine") || s.ModuleAvailable("calendar:nope") {
		t.Error("ModuleAvailable does not follow the sections")
	}
}

func TestLoadDropsLayoutKeyOfMissingSection(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"layout":{"topLeft":["clock","calendar:gone"],"topRight":[],"middle":[],"bottomLeft":[],"bottomRight":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get().Layout.TopLeft; !slices.Equal(got, []string{"clock"}) {
		t.Errorf("topLeft = %v", got)
	}
}

func TestDraftCalendarUsesStoredURLsAndStoresNothing(t *testing.T) {
	st, _ := Load(t.TempDir(), quiet)
	good := goodCalendar()
	if _, errs, _ := st.UpdateCalendar(updateFrom(good, func(id string) string {
		if id == "familie" {
			return good.Calendars[0].URL
		}
		return good.Calendars[1].URL
	})); errs != nil {
		t.Fatal(errs)
	}
	before := st.Get()
	draft := good.clone()
	draft.Rules[0].Text = "Neu"
	got, errs := st.DraftCalendar(updateFrom(draft, func(string) string { return "" }))
	if errs != nil {
		t.Fatal(errs)
	}
	if got.Calendars[0].URL != good.Calendars[0].URL || got.Rules[0].Text != "Neu" {
		t.Errorf("draft = %+v", got)
	}
	if !st.Get().Equal(before) {
		t.Error("draft changed the stored settings")
	}
	draft.Calendars = append(draft.Calendars, CalendarSource{ID: "x", Name: "X", Icon: IconRef{Kind: "none"}})
	if _, errs := st.DraftCalendar(updateFrom(draft, func(string) string { return "" })); errs["calendars.2.url"] == "" {
		t.Errorf("draft errors = %v", errs)
	}
}

func TestUsesIcon(t *testing.T) {
	up := IconRef{Kind: "upload", File: "abcdefgh12345678"}
	for name, mutate := range map[string]func(c *CalendarSettings){
		"calendar": func(c *CalendarSettings) { c.Calendars[0].Icon = up },
		"shift":    func(c *CalendarSettings) { c.Sections[1].Shifts[0].Icon = up },
		"rule":     func(c *CalendarSettings) { c.Rules[0].Icon = up },
	} {
		c := goodCalendar().clone()
		if c.UsesIcon(up.File) {
			t.Fatalf("%s: unused icon reported as used", name)
		}
		mutate(&c)
		if !c.UsesIcon(up.File) {
			t.Errorf("%s: use not detected", name)
		}
		if c.UsesIcon("zzzzzzzzzzzzzzzz") {
			t.Errorf("%s: other id reported as used", name)
		}
	}
}

func TestCalendarSurvivesSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	st, _ := Load(dir, quiet)
	good := goodCalendar()
	if _, errs, _ := st.UpdateCalendar(updateFrom(good, func(id string) string {
		if id == "familie" {
			return good.Calendars[0].URL
		}
		return good.Calendars[1].URL
	})); errs != nil {
		t.Fatal(errs)
	}
	again, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Get().Equal(st.Get()) || len(again.Get().Calendar.Sections) != 2 {
		t.Errorf("reloaded = %+v", again.Get().Calendar)
	}
}

func TestShiftDaysAcceptsTheFullRange(t *testing.T) {
	for _, n := range []int{1, 5, 14} {
		c := goodCalendar()
		c.Sections[1].ShiftDays = n
		errs := map[string]string{}
		c.validate(errs, nil)
		if len(errs) != 0 {
			t.Errorf("shiftDays %d: %v", n, errs)
		}
	}
}

func TestLoadSectionWithoutShowFreeDaysDefaultsToFalse(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"calendar":{"refreshMinutes":15,"calendars":[{"id":"d","name":"D","url":"https://example.org/d.ics","icon":{"kind":"none","name":"","file":""}}],` +
		`"sections":[{"id":"plan","title":"Plan","type":"shifts","calendarIds":["d"],"maxEvents":8,"lookAheadDays":14,"shiftDays":3,"shifts":[]}],"rules":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	secs := st.Get().Calendar.Sections
	if len(secs) != 1 || secs[0].ShowFreeDays || secs[0].ShiftDays != 3 {
		t.Errorf("sections = %+v", secs)
	}
}

func TestShowFreeDaysSurvivesUpdateAndLoad(t *testing.T) {
	dir := t.TempDir()
	st, _ := Load(dir, quiet)
	good := goodCalendar()
	good.Sections[1].ShowFreeDays = true
	if _, errs, _ := st.UpdateCalendar(updateFrom(good, func(id string) string {
		if id == "familie" {
			return good.Calendars[0].URL
		}
		return good.Calendars[1].URL
	})); errs != nil {
		t.Fatal(errs)
	}
	again, _ := Load(dir, quiet)
	if !again.Get().Calendar.Sections[1].ShowFreeDays {
		t.Error("showFreeDays lost")
	}
}

func TestDefaultHighlight(t *testing.T) {
	h := DefaultHighlight()
	want := CalendarHighlight{
		Enabled: true,
		Ongoing: HighlightTier{Enabled: true, Color: "#f6b73c"},
		Soon:    HighlightSoon{Enabled: false, Color: "#ff8a65", Hours: 2},
		Today:   HighlightTier{Enabled: false, Color: "#8fd3ff"},
	}
	if h != want {
		t.Errorf("DefaultHighlight = %+v", h)
	}
	for _, c := range []string{"#f6b73c", "#FF8A65", "#000000"} {
		if !colorPattern.MatchString(c) {
			t.Errorf("%s rejected", c)
		}
	}
}

func TestLoadSectionWithoutHighlightGetsDefaults(t *testing.T) {
	dir := t.TempDir()
	content := `{"schemaVersion":1,"calendar":{"refreshMinutes":15,"calendars":[{"id":"d","name":"D","url":"https://example.org/d.ics","icon":{"kind":"none","name":"","file":""}}],` +
		`"sections":[{"id":"plan","title":"Plan","type":"shifts","calendarIds":["d"],"maxEvents":8,"lookAheadDays":14,"shiftDays":3,"shifts":[]},` +
		`{"id":"teil","title":"Teil","type":"list","calendarIds":["d"],"maxEvents":8,"lookAheadDays":14,"shiftDays":3,"shifts":[],"highlight":{"enabled":true,"soon":{"enabled":true,"hours":5}}}],"rules":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := filepath.Glob(filepath.Join(dir, "settings.json.discarded-*")); len(entries) != 0 {
		t.Fatalf("file was discarded: %v", entries)
	}
	secs := st.Get().Calendar.Sections
	if len(secs) != 2 {
		t.Fatalf("sections = %+v", secs)
	}
	if secs[0].Highlight != DefaultHighlight() {
		t.Errorf("section without highlight = %+v, want defaults", secs[0].Highlight)
	}
	// A partial highlight keeps the defaults for what it leaves out.
	want := DefaultHighlight()
	want.Soon.Enabled, want.Soon.Hours = true, 5
	if secs[1].Highlight != want {
		t.Errorf("partial highlight = %+v, want %+v", secs[1].Highlight, want)
	}
}

func TestHighlightSurvivesUpdateAndLoad(t *testing.T) {
	dir := t.TempDir()
	st, _ := Load(dir, quiet)
	good := goodCalendar()
	good.Sections[0].Highlight = CalendarHighlight{
		Enabled: false,
		Ongoing: HighlightTier{Enabled: false, Color: "#ffffff"},
		Soon:    HighlightSoon{Enabled: true, Color: "#7ee787", Hours: 12},
		Today:   HighlightTier{Enabled: true, Color: "#ff5c5c"},
	}
	if _, errs, _ := st.UpdateCalendar(updateFrom(good, func(id string) string {
		if id == "familie" {
			return good.Calendars[0].URL
		}
		return good.Calendars[1].URL
	})); errs != nil {
		t.Fatal(errs)
	}
	again, err := Load(dir, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Get().Calendar.Sections[0].Highlight; got != good.Sections[0].Highlight {
		t.Errorf("reloaded highlight = %+v", got)
	}
}
