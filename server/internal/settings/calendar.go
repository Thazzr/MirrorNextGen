package settings

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Calendar section types and shift/rule vocabularies.
const (
	SectionList   = "list"
	SectionShifts = "shifts"
)

// Defaults a zero value in an update falls back to.
const (
	defaultRefreshMinutes = 15
	defaultMaxEvents      = 8
	defaultLookAheadDays  = 14
	defaultShiftDays      = 5
)

// Limits that keep the settings file and the dashboard reasonable.
const (
	maxCalendars = 20
	maxSections  = 10
	maxRules     = 100
	maxShifts    = 20
	maxURLLength = 2000
)

// Messages of the calendar field errors.
const (
	msgCalRefresh    = "Erlaubt sind 5 bis 1440 Minuten."
	msgCalID         = "Erlaubt sind 1 bis 40 Zeichen: Kleinbuchstaben, Ziffern und Bindestrich."
	msgCalIDTaken    = "Diese ID ist schon vergeben."
	msgCalName       = "Der Name muss 1 bis 60 Zeichen haben."
	msgCalURLMissing = "Bitte gib die Adresse des Kalenders an."
	msgCalURLShape   = "Die Adresse muss mit http://, https://, webcal:// oder webcals:// beginnen."
	msgCalURLLength  = "Höchstens 2000 Zeichen."
	msgCalTooMany    = "Zu viele Einträge."
	msgSecType       = "Erlaubt sind Liste und Schichten."
	msgSecCalendars  = "Wähle mindestens einen Kalender."
	msgSecMaxEvents  = "Erlaubt sind 1 bis 30 Termine."
	msgSecLookAhead  = "Erlaubt sind 1 bis 60 Tage."
	msgSecShiftDays  = "Erlaubt sind 1 bis 14 Einträge."
	msgSecShiftsOnly = "Schichten gibt es nur bei Bereichen vom Typ „Schichten“."
	msgShiftMatch    = "Das Stichwort muss 1 bis 40 Zeichen haben."
	msgShiftMode     = "Erlaubt sind „gleich“, „beginnt mit“ und „enthält“."
	msgShiftLabel    = "Der Name muss 1 bis 40 Zeichen haben."
	msgShiftBoth     = "Gib Beginn und Ende an oder lass beide leer."
	msgShiftTime     = "Erwartet wird eine Uhrzeit im Format HH:mm."
	msgRuleMode      = "Erlaubt sind „enthält“, „beginnt mit“ und „ist gleich“."
	msgRuleText      = "Höchstens 80 Zeichen."
	msgRuleAction    = "Erlaubt sind Ausblenden, Symbol setzen und Umbenennen."
	msgRuleIcon      = "Wähle ein Symbol."
	msgRuleRename    = "Der neue Titel muss 1 bis 80 Zeichen haben."
	msgIconKind      = "Erlaubt sind kein Symbol, ein Bootstrap-Symbol und eine hochgeladene Datei."
	msgIconName      = "Erlaubt sind 1 bis 40 Zeichen: Kleinbuchstaben, Ziffern und Bindestrich."
	msgIconFile      = "Dieses Symbol gibt es nicht."
	msgHighColor     = "Erlaubt ist eine Farbe im Format #rrggbb."
	msgHighHours     = "Erlaubt sind 1 bis 48 Stunden."
)

var (
	calIDPattern   = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	colorPattern   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	iconNamePatten = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
)

// IconRef points at a symbol: none, a Bootstrap icon name or an uploaded file.
type IconRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	File string `json:"file"`
}

// CalendarSource is one iCal feed. The URL is a secret: it may carry a
// private token and never leaves the server.
type CalendarSource struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	URL  string  `json:"url"`
	Icon IconRef `json:"icon"`
}

// CalendarShift maps a title keyword to a named shift.
type CalendarShift struct {
	ID        string  `json:"id"`
	Match     string  `json:"match"`
	MatchMode string  `json:"matchMode"`
	Label     string  `json:"label"`
	Start     string  `json:"start"`
	End       string  `json:"end"`
	Icon      IconRef `json:"icon"`
}

// Default colours of the time highlight.
const (
	defaultOngoingColor = "#f6b73c"
	defaultSoonColor    = "#ff8a65"
	defaultTodayColor   = "#8fd3ff"
	defaultSoonHours    = 2
)

// HighlightTier is one switchable colour of the time highlight.
type HighlightTier struct {
	Enabled bool   `json:"enabled"`
	Color   string `json:"color"`
}

// HighlightSoon colours items that start within Hours hours.
type HighlightSoon struct {
	Enabled bool   `json:"enabled"`
	Color   string `json:"color"`
	Hours   int    `json:"hours"`
}

// CalendarHighlight colours the time text of a section by how soon an item
// starts: running now, within a few hours, later today.
type CalendarHighlight struct {
	Enabled bool          `json:"enabled"`
	Ongoing HighlightTier `json:"ongoing"`
	Soon    HighlightSoon `json:"soon"`
	Today   HighlightTier `json:"today"`
}

// DefaultHighlight is what a new section and a section stored without a
// highlight get: only the running item is coloured.
func DefaultHighlight() CalendarHighlight {
	return CalendarHighlight{
		Enabled: true,
		Ongoing: HighlightTier{Enabled: true, Color: defaultOngoingColor},
		Soon:    HighlightSoon{Color: defaultSoonColor, Hours: defaultSoonHours},
		Today:   HighlightTier{Color: defaultTodayColor},
	}
}

// CalendarSection is one area of the mirror and one module instance.
type CalendarSection struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Type          string   `json:"type"`
	CalendarIDs   []string `json:"calendarIds"`
	MaxEvents     int      `json:"maxEvents"`
	LookAheadDays int      `json:"lookAheadDays"`
	ShiftDays     int      `json:"shiftDays"`
	// ShowFreeDays also lists days without a shift; absent in older files, so false.
	ShowFreeDays bool              `json:"showFreeDays"`
	Shifts       []CalendarShift   `json:"shifts"`
	Highlight    CalendarHighlight `json:"highlight"`
}

// UnmarshalJSON decodes on top of the default highlight, so a section stored
// before the highlight existed (or with only part of it) keeps the defaults
// instead of an all-false zero value.
func (s *CalendarSection) UnmarshalJSON(data []byte) error {
	type plain CalendarSection
	p := plain{Highlight: DefaultHighlight()}
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*s = CalendarSection(p)
	return nil
}

// CalendarRule is one entry of the ordered rule list.
type CalendarRule struct {
	ID          string   `json:"id"`
	CalendarIDs []string `json:"calendarIds"`
	MatchMode   string   `json:"matchMode"`
	Text        string   `json:"text"`
	Action      string   `json:"action"`
	Icon        IconRef  `json:"icon"`
	RenameTo    string   `json:"renameTo"`
}

// CalendarSettings configure the calendar modules.
type CalendarSettings struct {
	RefreshMinutes int               `json:"refreshMinutes"`
	Calendars      []CalendarSource  `json:"calendars"`
	Sections       []CalendarSection `json:"sections"`
	Rules          []CalendarRule    `json:"rules"`
}

func defaultCalendar() CalendarSettings {
	return CalendarSettings{
		RefreshMinutes: defaultRefreshMinutes,
		Calendars:      []CalendarSource{},
		Sections:       []CalendarSection{},
		Rules:          []CalendarRule{},
	}
}

// clone deep-copies c and replaces nil lists by empty ones (JSON must carry
// [] rather than null).
func (c CalendarSettings) clone() CalendarSettings {
	out := CalendarSettings{RefreshMinutes: c.RefreshMinutes}
	out.Calendars = append([]CalendarSource{}, c.Calendars...)
	out.Sections = make([]CalendarSection, len(c.Sections))
	for i, sec := range c.Sections {
		sec.CalendarIDs = append([]string{}, sec.CalendarIDs...)
		sec.Shifts = append([]CalendarShift{}, sec.Shifts...)
		out.Sections[i] = sec
	}
	out.Rules = make([]CalendarRule, len(c.Rules))
	for i, r := range c.Rules {
		r.CalendarIDs = append([]string{}, r.CalendarIDs...)
		out.Rules[i] = r
	}
	return out
}

func (c CalendarSettings) equal(o CalendarSettings) bool { return reflect.DeepEqual(c, o) }

// UsesIcon reports whether any calendar, shift or rule references the
// uploaded icon.
func (c CalendarSettings) UsesIcon(id string) bool {
	uses := func(r IconRef) bool { return r.Kind == "upload" && r.File == id }
	for _, cal := range c.Calendars {
		if uses(cal.Icon) {
			return true
		}
	}
	for _, sec := range c.Sections {
		for _, sh := range sec.Shifts {
			if uses(sh.Icon) {
				return true
			}
		}
	}
	for _, r := range c.Rules {
		if uses(r.Icon) {
			return true
		}
	}
	return false
}

// URLHint shortens a calendar address for display: scheme, host and the last
// path segment cut to 24 characters, for example
// "https://calendar.google.com/…/basic.ics". Query and userinfo (where tokens
// live) are never part of it.
func URLHint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	hint := u.Scheme + "://" + u.Hostname()
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	last := segs[len(segs)-1]
	if last == "" {
		return hint
	}
	if r := []rune(last); len(r) > 24 {
		last = string(r[:23]) + "…"
	}
	if len(segs) > 1 {
		return hint + "/…/" + last
	}
	return hint + "/" + last
}

// validURL checks scheme and host without echoing the address.
func validURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "webcal", "webcals":
		return true
	}
	return false
}

func validIcon(r IconRef, iconExists func(string) bool) string {
	switch r.Kind {
	case "none":
	case "bootstrap":
		if !iconNamePatten.MatchString(r.Name) {
			return msgIconName
		}
	case "upload":
		if iconExists != nil && !iconExists(r.File) {
			return msgIconFile
		}
		if r.File == "" {
			return msgIconFile
		}
	default:
		return msgIconKind
	}
	return ""
}

func rangeErr(n, lo, hi int) bool { return n < lo || n > hi }

// validate reports field errors with the "calendar." prefix. iconExists may
// be nil to skip the check of uploaded icon ids (loading a file).
func (c CalendarSettings) validate(errs map[string]string, iconExists func(string) bool) {
	if rangeErr(c.RefreshMinutes, 5, 1440) {
		errs["calendar.refreshMinutes"] = msgCalRefresh
	}
	if len(c.Calendars) > maxCalendars {
		errs["calendar.calendars"] = msgCalTooMany
	}
	if len(c.Sections) > maxSections {
		errs["calendar.sections"] = msgCalTooMany
	}
	if len(c.Rules) > maxRules {
		errs["calendar.rules"] = msgCalTooMany
	}

	calIDs := map[string]bool{}
	for i, cal := range c.Calendars {
		p := fmt.Sprintf("calendar.calendars.%d.", i)
		switch {
		case !calIDPattern.MatchString(cal.ID):
			errs[p+"id"] = msgCalID
		case calIDs[cal.ID]:
			errs[p+"id"] = msgCalIDTaken
		}
		calIDs[cal.ID] = true
		if n := utf8.RuneCountInString(strings.TrimSpace(cal.Name)); n < 1 || n > 60 {
			errs[p+"name"] = msgCalName
		}
		switch {
		case cal.URL == "":
			errs[p+"url"] = msgCalURLMissing
		case len(cal.URL) > maxURLLength:
			errs[p+"url"] = msgCalURLLength
		case !validURL(cal.URL):
			errs[p+"url"] = msgCalURLShape
		}
		if msg := validIcon(cal.Icon, iconExists); msg != "" {
			errs[p+"icon"] = msg
		}
	}

	secIDs := map[string]bool{}
	for i, sec := range c.Sections {
		p := fmt.Sprintf("calendar.sections.%d.", i)
		switch {
		case !calIDPattern.MatchString(sec.ID):
			errs[p+"id"] = msgCalID
		case secIDs[sec.ID]:
			errs[p+"id"] = msgCalIDTaken
		}
		secIDs[sec.ID] = true
		if n := utf8.RuneCountInString(strings.TrimSpace(sec.Title)); n < 1 || n > 60 {
			errs[p+"title"] = msgCalName
		}
		if sec.Type != SectionList && sec.Type != SectionShifts {
			errs[p+"type"] = msgSecType
		}
		if len(sec.CalendarIDs) == 0 {
			errs[p+"calendarIds"] = msgSecCalendars
		}
		for _, id := range sec.CalendarIDs {
			if !calIDs[id] {
				errs[p+"calendarIds"] = fmt.Sprintf("Den Kalender „%s“ gibt es nicht.", id)
				break
			}
		}
		if rangeErr(sec.MaxEvents, 1, 30) {
			errs[p+"maxEvents"] = msgSecMaxEvents
		}
		if rangeErr(sec.LookAheadDays, 1, 60) {
			errs[p+"lookAheadDays"] = msgSecLookAhead
		}
		if rangeErr(sec.ShiftDays, 1, 14) {
			errs[p+"shiftDays"] = msgSecShiftDays
		}
		if sec.Type != SectionShifts && len(sec.Shifts) > 0 {
			errs[p+"shifts"] = msgSecShiftsOnly
		}
		if len(sec.Shifts) > maxShifts {
			errs[p+"shifts"] = msgCalTooMany
		}
		validateShifts(errs, p, sec.Shifts, iconExists)
		sec.Highlight.validate(errs, p+"highlight.")
	}

	ruleIDs := map[string]bool{}
	for i, r := range c.Rules {
		p := fmt.Sprintf("calendar.rules.%d.", i)
		switch {
		case !calIDPattern.MatchString(r.ID):
			errs[p+"id"] = msgCalID
		case ruleIDs[r.ID]:
			errs[p+"id"] = msgCalIDTaken
		}
		ruleIDs[r.ID] = true
		for _, id := range r.CalendarIDs {
			if !calIDs[id] {
				errs[p+"calendarIds"] = fmt.Sprintf("Den Kalender „%s“ gibt es nicht.", id)
				break
			}
		}
		if !slices.Contains([]string{"contains", "startsWith", "equals"}, r.MatchMode) {
			errs[p+"matchMode"] = msgRuleMode
		}
		if utf8.RuneCountInString(r.Text) > 80 {
			errs[p+"text"] = msgRuleText
		}
		switch r.Action {
		case "hide":
		case "icon":
			if r.Icon.Kind == "" || r.Icon.Kind == "none" {
				errs[p+"icon"] = msgRuleIcon
			}
		case "rename":
		default:
			errs[p+"action"] = msgRuleAction
		}
		if _, taken := errs[p+"icon"]; !taken {
			if msg := validIcon(r.Icon, iconExists); msg != "" {
				errs[p+"icon"] = msg
			}
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(r.RenameTo)); (r.Action == "rename" && n < 1) || n > 80 {
			errs[p+"renameTo"] = msgRuleRename
		}
	}
}

func (h CalendarHighlight) validate(errs map[string]string, p string) {
	for key, color := range map[string]string{"ongoing.color": h.Ongoing.Color, "soon.color": h.Soon.Color, "today.color": h.Today.Color} {
		if !colorPattern.MatchString(color) {
			errs[p+key] = msgHighColor
		}
	}
	if rangeErr(h.Soon.Hours, 1, 48) {
		errs[p+"soon.hours"] = msgHighHours
	}
}

func validateShifts(errs map[string]string, secPrefix string, shifts []CalendarShift, iconExists func(string) bool) {
	ids := map[string]bool{}
	for j, sh := range shifts {
		p := fmt.Sprintf("%sshifts.%d.", secPrefix, j)
		switch {
		case !calIDPattern.MatchString(sh.ID):
			errs[p+"id"] = msgCalID
		case ids[sh.ID]:
			errs[p+"id"] = msgCalIDTaken
		}
		ids[sh.ID] = true
		if n := utf8.RuneCountInString(strings.TrimSpace(sh.Match)); n < 1 || utf8.RuneCountInString(sh.Match) > 40 {
			errs[p+"match"] = msgShiftMatch
		}
		if !slices.Contains([]string{"equals", "startsWith", "contains"}, sh.MatchMode) {
			errs[p+"matchMode"] = msgShiftMode
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(sh.Label)); n < 1 || n > 40 {
			errs[p+"label"] = msgShiftLabel
		}
		validateShiftTimes(errs, p, sh)
		if msg := validIcon(sh.Icon, iconExists); msg != "" {
			errs[p+"icon"] = msg
		}
	}
}

func validateShiftTimes(errs map[string]string, p string, sh CalendarShift) {
	check := func(field, v string) {
		if v == "" {
			return
		}
		if _, err := time.Parse("15:04", v); err != nil || len(v) != 5 {
			errs[p+field] = msgShiftTime
		}
	}
	check("start", sh.Start)
	check("end", sh.End)
	switch {
	case sh.Start == "" && sh.End != "":
		if _, bad := errs[p+"start"]; !bad {
			errs[p+"start"] = msgShiftBoth
		}
	case sh.Start != "" && sh.End == "":
		errs[p+"end"] = msgShiftBoth
	}
}

// CalendarSourceUpdate is a calendar in an update. An empty URL keeps the
// stored address of the calendar with the same ID.
type CalendarSourceUpdate struct {
	ID   string
	Name string
	URL  string
	Icon IconRef
}

// CalendarUpdate is a change of the calendar section.
type CalendarUpdate struct {
	RefreshMinutes int
	Calendars      []CalendarSourceUpdate
	Sections       []CalendarSection
	Rules          []CalendarRule
}

// apply merges u into the stored section. Zero numbers fall back to their
// defaults; a missing URL of a new calendar stays empty and is reported by
// validation.
func (u CalendarUpdate) apply(cur CalendarSettings) CalendarSettings {
	stored := map[string]string{}
	for _, c := range cur.Calendars {
		stored[c.ID] = c.URL
	}
	next := CalendarSettings{RefreshMinutes: u.RefreshMinutes}
	if next.RefreshMinutes == 0 {
		next.RefreshMinutes = defaultRefreshMinutes
	}
	for _, c := range u.Calendars {
		raw := strings.TrimSpace(c.URL)
		if raw == "" {
			raw = stored[c.ID]
		}
		next.Calendars = append(next.Calendars, CalendarSource{ID: c.ID, Name: strings.TrimSpace(c.Name), URL: raw, Icon: normIcon(c.Icon)})
	}
	next.Sections = slices.Clone(u.Sections)
	for i := range next.Sections {
		sec := &next.Sections[i]
		sec.Title = strings.TrimSpace(sec.Title)
		sec.MaxEvents = orDefault(sec.MaxEvents, defaultMaxEvents)
		sec.LookAheadDays = orDefault(sec.LookAheadDays, defaultLookAheadDays)
		sec.ShiftDays = orDefault(sec.ShiftDays, defaultShiftDays)
		sec.Shifts = slices.Clone(sec.Shifts)
		for j := range sec.Shifts {
			sec.Shifts[j].Icon = normIcon(sec.Shifts[j].Icon)
		}
	}
	next.Rules = slices.Clone(u.Rules)
	for i := range next.Rules {
		next.Rules[i].Icon = normIcon(next.Rules[i].Icon)
	}
	return next.clone()
}

// normIcon turns an absent kind into "none".
func normIcon(r IconRef) IconRef {
	if r.Kind == "" {
		r.Kind = "none"
	}
	return r
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
