package api

import (
	"net/http"
	"testing"

	"github.com/thazzr/mirrornextgen/server/internal/icons"
)

func uploadPNG(t *testing.T, e *testEnv, c creds) string {
	t.Helper()
	rec := upload(e, c, goodPNG, "haus.png", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload %d: %s", rec.Code, rec.Body)
	}
	return decode[Icon](t, rec).Id
}

func ctOf(r IconRef) string {
	if r.ContentType == nil {
		return ""
	}
	return *r.ContentType
}

func TestUploadedIconsCarryTheirContentTypeInResponses(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	svg, png := uploadSVG(t, e, c), uploadPNG(t, e, c)

	body := calBody(e)
	body.Calendars[0].Icon = IconRef{Kind: Upload, File: sptr(svg)}
	body.Sections[1].Shifts[0].Icon = IconRef{Kind: Upload, File: sptr(png)}
	body.Rules = []CalendarRule{{Id: "r1", CalendarIds: []string{"familie"}, MatchMode: "contains", Text: "Standup", Action: CalendarRuleActionIcon, Icon: IconRef{Kind: Upload, File: sptr(svg)}}}
	putCalendar(t, e, c, body)

	// Settings GET: calendar, shift and rule icons.
	got := decode[CalendarSettings](t, e.do(http.MethodGet, calSettingsPath, nil, opts{cookie: c.cookie}))
	if ctOf(got.Calendars[0].Icon) != "image/svg+xml" || ctOf(got.Sections[1].Shifts[0].Icon) != "image/png" || ctOf(got.Rules[0].Icon) != "image/svg+xml" {
		t.Errorf("settings = %+v", got)
	}
	if got.Calendars[1].Icon.ContentType != nil {
		t.Error("an icon of kind none carries no content type")
	}

	// Dashboard list item (public).
	list := decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/termine", nil, opts{}))
	if len(*list.Items) == 0 || ctOf((*list.Items)[0].Icon) != "image/svg+xml" {
		t.Errorf("list item = %+v", *list.Items)
	}
	// Dashboard shift row.
	shifts := decode[DashboardCalendarSection](t, e.do(http.MethodGet, "/api/v1/dashboard/calendar/plan", nil, opts{}))
	days := *shifts.Days
	if ctOf(days[3].Icon) != "image/png" {
		t.Errorf("shift day = %+v", days[3])
	}
	// Preview.
	pv := decode[CalendarPreview](t, e.do(http.MethodPost, calPreviewPath, body, opts{cookie: c.cookie, csrf: c.csrf}))
	found := false
	for _, sec := range pv.Sections {
		if sec.Items != nil {
			for _, it := range *sec.Items {
				if it.Icon.Kind == Upload {
					found = true
					if ctOf(it.Icon) != "image/svg+xml" {
						t.Errorf("preview item = %+v", it.Icon)
					}
				}
			}
		}
	}
	if !found {
		t.Error("preview has no upload icon")
	}
}

func TestIconRefOutLeavesUnknownIdsWithoutContentType(t *testing.T) {
	store, err := icons.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{icons: store}
	if r := s.iconRefOut("upload", "", "abcdefgh12345678"); r.ContentType != nil {
		t.Errorf("unknown id = %+v", r)
	}
	if r := s.iconRefOut("bootstrap", "house", ""); r.ContentType != nil {
		t.Errorf("bootstrap = %+v", r)
	}
}
