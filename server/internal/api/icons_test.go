package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const iconsPath = "/api/v1/icons"

const goodSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><path d="M0 0h8v8z"/></svg>`

var goodPNG = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)

func upload(e *testEnv, c creds, data []byte, fileName string, fields map[string]string) *httptest.ResponseRecorder {
	return e.doMultipart(iconsPath, data, fileName, fields, opts{cookie: c.cookie, csrf: c.csrf})
}

func uploadSVG(t *testing.T, e *testEnv, c creds) string {
	t.Helper()
	rec := upload(e, c, []byte(goodSVG), "sonne.svg", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload %d: %s", rec.Code, rec.Body)
	}
	return decode[Icon](t, rec).Id
}

func TestIconUploadListAndFile(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie}

	if got := decode[[]Icon](t, e.do(http.MethodGet, iconsPath, nil, o)); got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v", got)
	}
	rec := upload(e, c, []byte(goodSVG), "sonne.svg", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("svg: %d %s", rec.Code, rec.Body)
	}
	svg := decode[Icon](t, rec)
	if svg.Name != "sonne" || svg.ContentType != "image/svg+xml" || svg.Size != len(goodSVG) || len(svg.Id) != 16 {
		t.Errorf("svg icon = %+v", svg)
	}
	rec = upload(e, c, goodPNG, "x.bin", map[string]string{"name": "Haus"})
	png := decode[Icon](t, rec)
	if rec.Code != http.StatusCreated || png.Name != "Haus" || png.ContentType != "image/png" {
		t.Fatalf("png: %d %+v", rec.Code, png)
	}
	if got := decode[[]Icon](t, e.do(http.MethodGet, iconsPath, nil, o)); len(got) != 2 || got[0] != svg || got[1] != png {
		t.Errorf("list = %+v", got)
	}

	// The file is public and locked down.
	for _, tt := range []struct {
		icon Icon
		ct   string
		body []byte
	}{{svg, "image/svg+xml", []byte(goodSVG)}, {png, "image/png", goodPNG}} {
		rec := e.do(http.MethodGet, iconsPath+"/"+tt.icon.Id+"/file", nil, opts{})
		if rec.Code != http.StatusOK {
			t.Fatalf("file %d", rec.Code)
		}
		h := rec.Header()
		if h.Get("Content-Type") != tt.ct ||
			h.Get("Content-Security-Policy") != "default-src 'none'; style-src 'unsafe-inline'; sandbox" ||
			h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("Cache-Control") != "public, max-age=86400" {
			t.Errorf("headers for %s = %v", tt.ct, h)
		}
		if string(rec.Body.Bytes()) != string(tt.body) {
			t.Errorf("body for %s differs", tt.ct)
		}
	}
	wantProblem(t, e.do(http.MethodGet, iconsPath+"/0000000000000000/file", nil, opts{}), http.StatusNotFound, "")
	wantProblem(t, e.do(http.MethodGet, iconsPath+"/..%2F..%2Fsettings.json/file", nil, opts{}), http.StatusNotFound, "")
}

func TestIconUploadRejections(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	mal := func(inner string) []byte {
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">` + inner + `</svg>`)
	}
	tests := []struct {
		name   string
		data   []byte
		fields map[string]string
		field  string
		msg    string
	}{
		{"wrong type", []byte("GIF89a..."), nil, "file", "Erlaubt sind SVG und PNG."},
		{"too large", append(append([]byte{}, goodPNG...), make([]byte, 256*1024)...), nil, "file", "Höchstens 256 KB."},
		{"no file", nil, nil, "file", "Bitte wähle eine Datei."},
		{"name too long", []byte(goodSVG), map[string]string{"name": strings.Repeat("a", 61)}, "name", "Der Name muss 1 bis 60 Zeichen haben."},
		{"name over the 1024 byte read limit", []byte(goodSVG), map[string]string{"name": strings.Repeat("a", 1025)}, "name", "Der Name muss 1 bis 60 Zeichen haben."},
		{"script", mal(`<script>alert(1)</script>`), nil, "file", "Die SVG-Datei enthält nicht erlaubte Inhalte."},
		{"foreignObject", mal(`<foreignObject/>`), nil, "file", "Die SVG-Datei enthält nicht erlaubte Inhalte."},
		{"onclick", mal(`<g onclick="x()"/>`), nil, "file", "Die SVG-Datei enthält nicht erlaubte Inhalte."},
		{"external href", mal(`<image href="https://evil/x.png"/>`), nil, "file", "Die SVG-Datei enthält nicht erlaubte Inhalte."},
		{"doctype", []byte(`<!DOCTYPE svg [<!ENTITY a "b">]>` + goodSVG), nil, "file", "Die SVG-Datei enthält nicht erlaubte Inhalte."},
		{"style import", mal(`<style>@import "x.css";</style>`), nil, "file", "Die SVG-Datei enthält nicht erlaubte Inhalte."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := upload(e, c, tt.data, "x.svg", tt.fields)
			p := wantProblem(t, rec, http.StatusBadRequest, "")
			if got := (*p.Errors)[tt.field]; got != tt.msg {
				t.Errorf("errors = %v, want %s: %q", *p.Errors, tt.field, tt.msg)
			}
		})
	}
	if got := decode[[]Icon](t, e.do(http.MethodGet, iconsPath, nil, opts{cookie: c.cookie})); len(got) != 0 {
		t.Errorf("a refused upload was stored: %+v", got)
	}
	// Needs a session and CSRF.
	wantProblem(t, e.doMultipart(iconsPath, []byte(goodSVG), "a.svg", nil, opts{}), http.StatusUnauthorized, "")
	wantProblem(t, e.doMultipart(iconsPath, []byte(goodSVG), "a.svg", nil, opts{cookie: c.cookie}), http.StatusForbidden, "")
}

func TestIconDelete(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	id := uploadSVG(t, e, c)
	file := func() string { return filepath.Join(e.iconsDir, id+".svg") }
	if _, err := os.Stat(file()); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}

	body := calBody(e)
	body.Calendars[0].Icon = IconRef{Kind: Upload, File: sptr(id)}
	putCalendar(t, e, c, body)
	p := wantProblem(t, e.do(http.MethodDelete, iconsPath+"/"+id, nil, o), http.StatusConflict, "")
	if p.Detail == nil || *p.Detail != "Das Symbol wird noch verwendet." {
		t.Errorf("problem = %+v", p)
	}
	if _, err := os.Stat(file()); err != nil {
		t.Error("an icon in use was removed")
	}

	body.Calendars[0].Icon = none()
	putCalendar(t, e, c, body)
	wantProblem(t, e.do(http.MethodDelete, iconsPath+"/"+id, nil, opts{cookie: c.cookie}), http.StatusForbidden, "")
	if rec := e.do(http.MethodDelete, iconsPath+"/"+id, nil, o); rec.Code != http.StatusNoContent {
		t.Fatalf("delete unused: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(file()); !os.IsNotExist(err) {
		t.Errorf("file still there: %v", err)
	}
	wantProblem(t, e.do(http.MethodGet, iconsPath+"/"+id+"/file", nil, opts{}), http.StatusNotFound, "")
	wantProblem(t, e.do(http.MethodDelete, iconsPath+"/"+id, nil, o), http.StatusNotFound, "")
}

func TestIconInUseByShiftAndRuleBlocksDelete(t *testing.T) {
	e := newTestEnv(t)
	c := e.setup(t)
	o := opts{cookie: c.cookie, csrf: c.csrf}
	id := uploadSVG(t, e, c)
	body := calBody(e)
	body.Sections[1].Shifts[0].Icon = IconRef{Kind: Upload, File: sptr(id)}
	putCalendar(t, e, c, body)
	wantProblem(t, e.do(http.MethodDelete, iconsPath+"/"+id, nil, o), http.StatusConflict, "")
	body.Sections[1].Shifts[0].Icon = none()
	body.Rules = []CalendarRule{{Id: "r", CalendarIds: []string{}, MatchMode: "contains", Text: "x", Action: "icon", Icon: IconRef{Kind: Upload, File: sptr(id)}}}
	putCalendar(t, e, c, body)
	wantProblem(t, e.do(http.MethodDelete, iconsPath+"/"+id, nil, o), http.StatusConflict, "")
}
