package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

const (
	calSettingsPath = "/api/v1/settings/calendar"
	calPreviewPath  = "/api/v1/settings/calendar/preview"
)

func none() IconRef { return IconRef{Kind: None} }

func sptr(s string) *string { return &s }

// calBody is a draft with a list section on a Google fixture and a shifts
// section on the same feed; urls maps calendar ids to the secret address.
func calBody(e *testEnv) CalendarSettingsUpdate {
	return CalendarSettingsUpdate{
		RefreshMinutes: 15,
		Calendars: []CalendarSourceUpdate{
			{Id: "familie", Name: "Familie", Icon: IconRef{Kind: Bootstrap, Name: sptr("house")}, Url: sptr(e.feed.url("google.ics"))},
			{Id: "dienst", Name: "Dienst", Icon: none(), Url: sptr(e.feed.url("google.ics"))},
		},
		Sections: []CalendarSection{
			{Id: "termine", Title: "Termine", Type: CalendarSectionType("list"), CalendarIds: []string{"familie"}, MaxEvents: 3, LookAheadDays: 14, ShiftDays: 2, Shifts: []CalendarShift{}, Highlight: defaultHighlightBody()},
			{Id: "plan", Title: "Dienstplan", Type: CalendarSectionType("shifts"), CalendarIds: []string{"dienst"}, MaxEvents: 8, LookAheadDays: 14, ShiftDays: 4, ShowFreeDays: true, Highlight: defaultHighlightBody(), Shifts: []CalendarShift{
				{Id: "sync", Match: "Sync", MatchMode: "contains", Label: "Spätdienst", Start: "14:00", End: "22:00", Icon: IconRef{Kind: Bootstrap, Name: sptr("moon")}},
			}},
		},
		Rules: []CalendarRule{},
	}
}

func defaultHighlightBody() CalendarHighlight {
	return CalendarHighlight{
		Enabled: true,
		Ongoing: HighlightTier{Enabled: true, Color: "#f6b73c"},
		Soon:    HighlightSoon{Color: "#ff8a65", Hours: 2},
		Today:   HighlightTier{Color: "#8fd3ff"},
	}
}

func putCalendar(t *testing.T, e *testEnv, c creds, body CalendarSettingsUpdate) {
	t.Helper()
	rec := e.do(http.MethodPut, calSettingsPath, body, opts{cookie: c.cookie, csrf: c.csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status %d: %s", rec.Code, rec.Body)
	}
}

func TestCalendarSettingsNeedSessionAndCSRF(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := calBody(e)
	for _, p := range []struct{ method, path string }{{"GET", calSettingsPath}, {"PUT", calSettingsPath}, {"POST", calPreviewPath}} {
		wantProblem(t, e.do(p.method, p.path, body, opts{}), http.StatusUnauthorized, "")
	}
	wantProblem(t, e.do(http.MethodPut, calSettingsPath, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
	wantProblem(t, e.do(http.MethodPost, calPreviewPath, body, opts{cookie: c.cookie}), http.StatusForbidden, "")
	if got := decode[CalendarSettings](t, e.do(http.MethodGet, calSettingsPath, nil, opts{cookie: c.cookie})); len(got.Calendars) != 0 {
		t.Errorf("refused PUT stored something: %+v", got)
	}
}

func TestCalendarSettingsRoundTripNeverContainsTheURL(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}

	rec := e.do(http.MethodGet, calSettingsPath, nil, o)
	if got := decode[CalendarSettings](t, rec); got.RefreshMinutes != 15 || got.Calendars == nil || got.Sections == nil || got.Rules == nil {
		t.Errorf("defaults = %+v (%s)", got, rec.Body)
	}

	body := calBody(e)
	body.Calendars[0].Url = sptr("https://calendar.google.com/calendar/ical/abc/private-TOKEN123/basic.ics")
	putRec := e.do(http.MethodPut, calSettingsPath, body, o)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT %d: %s", putRec.Code, putRec.Body)
	}
	for name, raw := range map[string]string{"PUT": putRec.Body.String(), "GET": e.do(http.MethodGet, calSettingsPath, nil, o).Body.String()} {
		for _, secret := range []string{"TOKEN123", "private-", "secret-token", "ical/abc", "?token"} {
			if strings.Contains(raw, secret) {
				t.Errorf("%s answer contains %q: %s", name, secret, raw)
			}
		}
		var generic map[string]any
		_ = json.Unmarshal([]byte(raw), &generic)
		cals := generic["calendars"].([]any)
		if _, has := cals[0].(map[string]any)["url"]; has {
			t.Errorf("%s answer has a url field", name)
		}
	}
	got := decode[CalendarSettings](t, e.do(http.MethodGet, calSettingsPath, nil, o))
	if got.Calendars[0].UrlHint != "https://calendar.google.com/…/basic.ics" || !got.Calendars[0].HasUrl {
		t.Errorf("calendar 0 = %+v", got.Calendars[0])
	}
	if len(got.Sections) != 2 || got.Sections[1].Shifts[0].Start != "14:00" {
		t.Errorf("sections = %+v", got.Sections)
	}
}

func TestCalendarSettingsKeepReplaceAndNewWithoutURL(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	putCalendar(t, e, c, calBody(e))

	// Absent and empty url keep the stored address: the preview still loads.
	body := calBody(e)
	body.Calendars[0].Url = nil
	body.Calendars[1].Url = sptr("")
	putCalendar(t, e, c, body)
	prev := decode[CalendarPreview](t, e.do(http.MethodPost, calPreviewPath, body, o))
	if len(prev.CalendarErrors) != 0 || prev.Sections[0].Items == nil || len(*prev.Sections[0].Items) == 0 {
		t.Fatalf("stored URL not kept: %+v", prev)
	}

	// A replaced address is used (the new one fails).
	body.Calendars[0].Url = sptr(e.feed.server.URL + "/fail")
	putCalendar(t, e, c, body)
	body.Calendars[0].Url = nil
	prev = decode[CalendarPreview](t, e.do(http.MethodPost, calPreviewPath, body, o))
	if len(prev.CalendarErrors) != 1 || prev.CalendarErrors[0].CalendarId != "familie" {
		t.Errorf("replaced URL not stored: %+v", prev.CalendarErrors)
	}

	// A new id without url names the field.
	body = calBody(e)
	body.Calendars = append(body.Calendars, CalendarSourceUpdate{Id: "neu", Name: "Neu", Icon: none()})
	p := wantProblem(t, e.do(http.MethodPut, calSettingsPath, body, o), http.StatusBadRequest, "")
	if (*p.Errors)["calendars.2.url"] != "Bitte gib die Adresse des Kalenders an." {
		t.Errorf("errors = %v", *p.Errors)
	}
}

func TestCalendarSettingsFieldErrorsCarryIndexPaths(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := calBody(e)
	body.Sections[1].Title = ""
	body.Sections[1].Shifts[0].Start = ""
	body.Rules = []CalendarRule{{Id: "a", CalendarIds: []string{}, MatchMode: "contains", Text: strings.Repeat("x", 81), Action: "hide", Icon: none()}}
	p := wantProblem(t, e.do(http.MethodPut, calSettingsPath, body, opts{cookie: c.cookie, csrf: c.csrf}), http.StatusBadRequest, "")
	for _, path := range []string{"sections.1.title", "sections.1.shifts.0.start", "rules.0.text"} {
		if (*p.Errors)[path] == "" {
			t.Errorf("no error for %s: %v", path, *p.Errors)
		}
	}
}

func TestCalendarSettingsRejectUnknownIconFile(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	body := calBody(e)
	body.Calendars[0].Icon = IconRef{Kind: Upload, File: sptr("abcdefgh12345678")}
	p := wantProblem(t, e.do(http.MethodPut, calSettingsPath, body, o), http.StatusBadRequest, "")
	if (*p.Errors)["calendars.0.icon"] != "Dieses Symbol gibt es nicht." {
		t.Errorf("errors = %v", *p.Errors)
	}
	id := uploadSVG(t, e, c)
	body.Calendars[0].Icon = IconRef{Kind: Upload, File: sptr(id)}
	putCalendar(t, e, c, body)
}

func layoutOf(t *testing.T, e *testEnv, c creds) Layout {
	t.Helper()
	return decode[Layout](t, e.do(http.MethodGet, "/api/v1/settings/layout", nil, opts{cookie: c.cookie}))
}

func TestCalendarSectionsJoinAndLeaveTheLayoutAndModules(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie}
	putCalendar(t, e, c, calBody(e))
	if got := layoutOf(t, e, c).TopLeft; !slices.Equal(got, []string{"clock", "calendar:termine", "calendar:plan"}) {
		t.Errorf("topLeft = %v", got)
	}
	var names []string
	for _, m := range decode[[]ModuleInfo](t, e.do(http.MethodGet, "/api/v1/modules", nil, o)) {
		names = append(names, m.Key+"="+m.Name)
	}
	for _, want := range []string{"calendar:termine=Kalender: Termine", "calendar:plan=Schichten: Dienstplan"} {
		if !slices.Contains(names, want) {
			t.Errorf("modules %v lack %q", names, want)
		}
	}
	body := calBody(e)
	body.Sections = body.Sections[:1]
	putCalendar(t, e, c, body)
	if got := layoutOf(t, e, c).TopLeft; !slices.Equal(got, []string{"clock", "calendar:termine"}) {
		t.Errorf("topLeft after removal = %v", got)
	}
	if got := decode[Layout](t, e.do(http.MethodGet, "/api/v1/dashboard/layout", nil, opts{})).TopLeft; !slices.Equal(got, []string{"clock", "calendar:termine"}) {
		t.Errorf("dashboard layout = %v", got)
	}
}

func TestCalendarPreviewShowsRuleEffectsOfAnUnsavedDraft(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	stored := calBody(e)
	putCalendar(t, e, c, stored)

	draft := calBody(e)
	for i := range draft.Calendars {
		draft.Calendars[i].Url = nil // the stored addresses are used
	}
	draft.Sections[0].MaxEvents = 30
	draft.Rules = []CalendarRule{
		{Id: "weg", CalendarIds: []string{}, MatchMode: "equals", Text: "Dienstags-Sync", Action: "hide", Icon: none()},
		{Id: "neu", CalendarIds: []string{"familie"}, MatchMode: "startsWith", Text: "Standup", Action: "rename", Icon: none(), RenameTo: "Runde"},
	}
	rec := e.do(http.MethodPost, calPreviewPath, draft, o)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	prev := decode[CalendarPreview](t, rec)
	items := *prev.Sections[0].Items
	var hidden, renamed, plain int
	for _, it := range items {
		switch {
		case it.OriginalTitle == "Dienstags-Sync":
			hidden++
			if !it.Hidden || it.Shown || !slices.Equal(it.MatchedRules, []string{"weg"}) {
				t.Errorf("hidden item = %+v", it)
			}
		case it.Title == "Runde":
			renamed++
			if it.Hidden || !it.Shown || !slices.Equal(it.MatchedRules, []string{"neu"}) || !strings.HasPrefix(it.OriginalTitle, "Standup") || it.CalendarId != "familie" {
				t.Errorf("renamed item = %+v", it)
			}
		default:
			plain++
		}
	}
	if hidden == 0 || renamed == 0 {
		t.Fatalf("preview lacks rule effects: hidden %d renamed %d of %d", hidden, renamed, len(items))
	}
	if prev.Sections[1].Type != "shifts" || prev.Sections[1].Days == nil {
		t.Fatalf("shifts preview = %+v", prev.Sections[1])
	}
	// The draft was not stored: the dashboard still shows the unrenamed list.
	dash := decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{}))
	for _, it := range *dash.Items {
		if it.Title == "Runde" {
			t.Error("the draft rule leaked into the stored settings")
		}
	}
	// The preview shows the very text the dashboard shows for the same event
	// (a title can recur on several days, so compare against all its candidates).
	for _, d := range *dash.Items {
		found := false
		for _, it := range items {
			if it.OriginalTitle == d.Title && it.TimeLabel == d.TimeLabel && it.Relative == d.Relative && d.Relative != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("no preview item with the dashboard text %q for %q", d.Relative, d.Title)
		}
	}
	// Invalid drafts answer like the PUT.
	draft.Sections[0].CalendarIds = []string{"nope"}
	p := wantProblem(t, e.do(http.MethodPost, calPreviewPath, draft, o), http.StatusBadRequest, "")
	if (*p.Errors)["sections.0.calendarIds"] == "" {
		t.Errorf("errors = %v", *p.Errors)
	}
}

func TestDashboardCalendarSections(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	putCalendar(t, e, c, calBody(e))
	// No session at all: the section endpoints are public.
	rec := e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	list := decode[DashboardCalendarSection](t, rec)
	if list.Type != "list" || list.Title != "Termine" || list.Error != nil || list.Items == nil || len(*list.Items) != 3 {
		t.Fatalf("list = %+v", list)
	}
	first := (*list.Items)[0]
	if first.Title != "Standup" || first.Relative == "" || first.Icon.Kind != Bootstrap || *first.Icon.Name != "house" || first.Location == nil || *first.Location != "Büro, Raum 4" {
		t.Errorf("first = %+v", first)
	}
	if list.Days != nil {
		t.Error("a list carries days")
	}

	rec = e.do(http.MethodGet, "/api/v1/dashboard/calendar/plan", nil, opts{})
	shifts := decode[DashboardCalendarSection](t, rec)
	if shifts.Type != "shifts" || shifts.Days == nil || len(*shifts.Days) != 4 || shifts.Items != nil {
		t.Fatalf("shifts = %s", rec.Body)
	}
	days := *shifts.Days
	if days[0].DayLabel != "Heute" || !days[0].Free || days[0].Title != "frei" || days[0].Ongoing || days[0].TimeRange != nil {
		t.Errorf("day 0 = %+v", days[0])
	}
	if days[3].Title != "Spätdienst" || days[3].TimeRange == nil || *days[3].TimeRange != "14–22" || days[3].Free || days[3].Icon.Kind != Bootstrap || *days[3].Icon.Name != "moon" || days[3].Date.Format("2006-01-02") != "2026-11-17" {
		t.Errorf("day 3 = %+v", days[3])
	}
	if strings.Contains(rec.Body.String(), "secret-token") {
		t.Error("answer contains the calendar address")
	}

	p := wantProblem(t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/gibtsnicht", nil, opts{}), http.StatusNotFound, "")
	if p.Status != 404 {
		t.Errorf("problem = %+v", p)
	}
}

func TestDashboardCalendarSectionReportsFailuresInTheBody(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := calBody(e)
	body.Calendars[0].Url = sptr(e.feed.server.URL + "/fail?token=secret-token")
	putCalendar(t, e, c, body)
	rec := e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	got := decode[DashboardCalendarSection](t, rec)
	if got.Error == nil || *got.Error != "Kalender „Familie“ konnte nicht geladen werden." || len(*got.Items) != 0 {
		t.Errorf("section = %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "secret-token") || strings.Contains(e.logs.String(), "secret-token") {
		t.Errorf("address leaked: body %s, log %s", rec.Body, e.logs)
	}
	// The other section still works: the failure stays in its area.
	other := decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/plan", nil, opts{}))
	if other.Error != nil {
		t.Errorf("plan = %+v", other)
	}
}

func TestCalendarUpdateInvalidatesTheFeedCache(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	putCalendar(t, e, c, calBody(e))
	e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{})
	e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{})
	before := e.feed.calls.Load()
	if before != 1 {
		t.Fatalf("feed calls = %d, want 1 (cached)", before)
	}
	putCalendar(t, e, c, calBody(e))
	e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{})
	if got := e.feed.calls.Load(); got != before+1 {
		t.Errorf("feed calls = %d, want %d after the update", got, before+1)
	}
}

func TestShowFreeDaysRoundTripsAndChangesTheDashboard(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie}
	body := calBody(e)
	putCalendar(t, e, c, body)
	if got := decode[CalendarSettings](t, e.do(http.MethodGet, calSettingsPath, nil, o)); !got.Sections[1].ShowFreeDays {
		t.Fatal("showFreeDays = true was not stored")
	}

	// Off: only work days, ShiftDays is the row count and no row is free.
	body.Sections[1].ShowFreeDays = false
	putCalendar(t, e, c, body)
	if got := decode[CalendarSettings](t, e.do(http.MethodGet, calSettingsPath, nil, o)); got.Sections[1].ShowFreeDays {
		t.Fatal("showFreeDays = false was not stored")
	}
	rec := e.do(http.MethodGet, "/api/v1/dashboard/calendar/plan", nil, opts{})
	days := *decode[DashboardCalendarSection](t, rec).Days
	if len(days) == 0 || len(days) > 4 {
		t.Fatalf("rows = %d: %s", len(days), rec.Body)
	}
	for _, d := range days {
		if d.Free || d.Title == "frei" {
			t.Errorf("free row although free days are off: %+v", d)
		}
	}
	if !strings.Contains(rec.Body.String(), `"ongoing"`) {
		t.Error("answer lacks ongoing")
	}
}

func TestDashboardCarriesHighlightColors(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	body := calBody(e)
	// The test clock is Sat 2026-11-14 13:00 Berlin: Monday's standup (08:00)
	// starts in 43 hours, not today, so only a 48-hour soon window colours it.
	soon := HighlightSoon{Enabled: true, Color: "#ff8a65", Hours: 48}
	for i := range body.Sections {
		body.Sections[i].Highlight.Soon = soon
	}
	putCalendar(t, e, c, body)

	list := decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{}))
	first := (*list.Items)[0]
	if first.Title != "Standup" || first.HighlightColor == nil || *first.HighlightColor != "#ff8a65" {
		t.Errorf("list item = %+v", first)
	}
	days := *decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/plan", nil, opts{})).Days
	for _, i := range []int{0, 1} {
		if !days[i].Free || days[i].HighlightColor != nil {
			t.Errorf("free row %d = %+v", i, days[i])
		}
	}
	if days[2].Free || days[2].HighlightColor == nil || *days[2].HighlightColor != "#ff8a65" {
		t.Errorf("monday row = %+v", days[2])
	}

	// Master switch off: nothing is coloured any more.
	for i := range body.Sections {
		body.Sections[i].Highlight.Enabled = false
	}
	putCalendar(t, e, c, body)
	list = decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{}))
	for _, it := range *list.Items {
		if it.HighlightColor != nil {
			t.Errorf("coloured although off: %+v", it)
		}
	}
	days = *decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/plan", nil, opts{})).Days
	for _, d := range days {
		if d.HighlightColor != nil {
			t.Errorf("row coloured although off: %+v", d)
		}
	}
}

func TestCalendarHighlightRoundTripsAndReportsFieldErrors(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	body := calBody(e)
	body.Sections[0].Highlight = CalendarHighlight{
		Enabled: false,
		Ongoing: HighlightTier{Enabled: false, Color: "#ffffff"},
		Soon:    HighlightSoon{Enabled: true, Color: "#7ee787", Hours: 6},
		Today:   HighlightTier{Enabled: true, Color: "#ff5c5c"},
	}
	putCalendar(t, e, c, body)
	got := decode[CalendarSettings](t, e.do(http.MethodGet, calSettingsPath, nil, opts{cookie: c.cookie}))
	if got.Sections[0].Highlight != body.Sections[0].Highlight {
		t.Errorf("highlight = %+v", got.Sections[0].Highlight)
	}

	body.Sections[1].Highlight.Soon.Hours = 49
	body.Sections[1].Highlight.Today.Color = "rot"
	p := wantProblem(t, e.do(http.MethodPut, calSettingsPath, body, o), http.StatusBadRequest, "")
	if (*p.Errors)["sections.1.highlight.soon.hours"] != "Erlaubt sind 1 bis 48 Stunden." ||
		(*p.Errors)["sections.1.highlight.today.color"] != "Erlaubt ist eine Farbe im Format #rrggbb." {
		t.Errorf("errors = %v", *p.Errors)
	}
}
