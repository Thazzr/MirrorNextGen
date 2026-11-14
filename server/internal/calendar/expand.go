// Package calendar holds the pure calendar logic of the mirror: expanding
// iCalendar data into occurrences, applying rules, and building the lists and
// shift summaries the dashboard shows. Nothing in here does I/O or reads the
// clock; callers pass bytes, a window and "now".
package calendar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
)

const (
	// maxInstancesPerSeries bounds the instances one series may contribute.
	maxInstancesPerSeries = 5000
	// maxIterationsPerSeries stops runaway rules (e.g. SECONDLY since years)
	// that would otherwise spin before reaching the window.
	maxIterationsPerSeries = 200000
)

// Window is the half-open time range [From, To). A zero bound is open.
type Window struct {
	From, To time.Time
}

// Occurrence is one concrete event instance.
type Occurrence struct {
	UID      string
	Title    string
	Location string
	Start    time.Time
	End      time.Time
	AllDay   bool
}

// base64Shorthand matches the vCard-style "ENCODING=B" some feeds write.
var base64Shorthand = regexp.MustCompile(`(?i)ENCODING\s*=\s*"?B"?([;:])`)

// ErrNoCalendar is returned when the data holds no iCalendar object.
var ErrNoCalendar = errors.New("no calendar data")

type dateVal struct {
	t      time.Time
	allDay bool
}

type vevent struct {
	uid, title, location string
	start, end           time.Time
	allDay               bool
	hasStart, hasEnd     bool
	cancelled            bool
	rrule                string
	rdates, exdates      []dateVal
	rid                  *dateVal
}

// Expand returns every occurrence overlapping window, sorted by start. loc is
// used for floating times, dates and unknown zones. Individual broken events
// are skipped; unparsable data is an error.
func Expand(data []byte, window Window, loc *time.Location) ([]Occurrence, error) {
	if loc == nil {
		loc = time.UTC
	}
	comps, err := decodeEvents(data)
	if err != nil {
		return nil, err
	}

	var events []*vevent
	for _, c := range comps {
		if ev, err := parseEvent(c, loc); err == nil {
			events = append(events, ev)
		}
	}

	masters := map[string][]*vevent{}
	overrides := map[string][]*vevent{}
	for _, ev := range events {
		if ev.rid != nil {
			overrides[ev.uid] = append(overrides[ev.uid], ev)
		} else {
			masters[ev.uid] = append(masters[ev.uid], ev)
		}
	}

	var out []Occurrence
	// Overrides replace instances by original start; collect those keys first.
	suppressed := map[string]map[string]bool{}
	for uid, list := range overrides {
		keys := map[string]bool{}
		for _, o := range list {
			keys[instKey(o.rid.t, o.rid.allDay)] = true
		}
		suppressed[uid] = keys
	}

	for _, ev := range events {
		if ev.rid != nil || ev.cancelled {
			continue
		}
		if ev.rrule == "" && len(ev.rdates) == 0 {
			occ := ev.occurrence(ev.start, ev.end)
			if overlaps(occ.Start, occ.End, window) {
				out = append(out, occ)
			}
			continue
		}
		out = append(out, expandSeries(ev, window, loc, suppressed[ev.uid])...)
	}

	for uid, list := range overrides {
		for _, o := range list {
			if o.cancelled {
				continue
			}
			var master *vevent
			if ms := masters[uid]; len(ms) > 0 {
				master = ms[0]
			}
			start, end := o.start, o.end
			if !o.hasStart {
				start, o.allDay = o.rid.t, o.rid.allDay
			}
			if master != nil {
				if o.title == "" {
					o.title = master.title
				}
				if o.location == "" {
					o.location = master.location
				}
				if !o.hasEnd {
					end = shiftEnd(master, start, o.allDay, loc)
				}
			} else if !o.hasEnd {
				end = defaultEnd(start, o.allDay)
			}
			occ := o.occurrence(start, end)
			if overlaps(occ.Start, occ.End, window) {
				out = append(out, occ)
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Start.Equal(b.Start) {
			return a.Start.Before(b.Start)
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		return a.UID < b.UID
	})
	return out, nil
}

func (e *vevent) occurrence(start, end time.Time) Occurrence {
	title := e.title
	if title == "" {
		title = "Termin"
	}
	return Occurrence{UID: e.uid, Title: title, Location: e.location, Start: start, End: end, AllDay: e.allDay}
}

func overlaps(start, end time.Time, w Window) bool {
	if !w.To.IsZero() && !start.Before(w.To) {
		return false
	}
	if w.From.IsZero() || end.After(w.From) {
		return true
	}
	// A zero-length event counts when it starts inside the window.
	return end.Equal(start) && !start.Before(w.From)
}

func defaultEnd(start time.Time, allDay bool) time.Time {
	if allDay {
		return time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, start.Location())
	}
	return start
}

// shiftEnd gives the end of an instance starting at start with the master's
// duration (calendar days for all-day, elapsed time otherwise).
func shiftEnd(m *vevent, start time.Time, allDay bool, loc *time.Location) time.Time {
	if allDay {
		days := dayCount(m.start, m.end)
		if days < 1 {
			days = 1
		}
		return time.Date(start.Year(), start.Month(), start.Day()+days, 0, 0, 0, 0, loc)
	}
	return start.Add(m.end.Sub(m.start))
}

// dayCount is the number of civil days from a to b.
func dayCount(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(db.Sub(da) / (24 * time.Hour))
}

func instKey(t time.Time, allDay bool) string {
	if allDay {
		return "d" + t.Format("2006-01-02")
	}
	return "t" + strconv.FormatInt(t.Unix(), 10)
}

func expandSeries(m *vevent, w Window, loc *time.Location, suppressed map[string]bool) []Occurrence {
	// All-day rules run on UTC midnights so a DST gap can never swallow or
	// shift a date; timed rules run in DTSTART's own zone so wall-clock times
	// survive DST changes.
	dtstart := m.start
	if m.allDay {
		dtstart = time.Date(m.start.Year(), m.start.Month(), m.start.Day(), 0, 0, 0, 0, time.UTC)
	}
	toInstance := func(t time.Time) time.Time {
		if m.allDay {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
		return t
	}

	var starts []time.Time
	counted := 0
	add := func(inst time.Time) bool {
		end := shiftEnd(m, inst, m.allDay, loc)
		if !overlaps(inst, end, w) {
			return true
		}
		counted++
		if counted > maxInstancesPerSeries {
			return false
		}
		starts = append(starts, inst)
		return true
	}

	if m.rrule != "" {
		if ro, err := rrule.StrToROptionInLocation(m.rrule, dtstart.Location()); err == nil {
			ro.Dtstart = dtstart
			if !m.allDay && hasDateOnlyUntil(m.rrule) {
				// A date UNTIL covers its whole day, not just its midnight.
				u := ro.Until
				ro.Until = time.Date(u.Year(), u.Month(), u.Day(), 23, 59, 59, 0, u.Location())
			}
			if rule, err := rrule.NewRRule(*ro); err == nil {
				next := rule.Iterator()
				for i := 0; i < maxIterationsPerSeries; i++ {
					t, ok := next()
					if !ok {
						break
					}
					inst := toInstance(t)
					if !w.To.IsZero() && !inst.Before(w.To) {
						break
					}
					if !add(inst) {
						break
					}
				}
			}
		}
	}
	// DTSTART is always the first instance, even if the rule would not hit it.
	add(m.start)
	for _, r := range m.rdates {
		t := r.t
		if r.allDay && !m.allDay {
			t = time.Date(t.Year(), t.Month(), t.Day(), m.start.Hour(), m.start.Minute(), m.start.Second(), 0, m.start.Location())
		} else if !r.allDay && m.allDay {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
		add(t)
	}

	seen := map[string]bool{}
	var out []Occurrence
	for _, s := range starts {
		key := instKey(s, m.allDay)
		if seen[key] {
			continue
		}
		seen[key] = true
		if suppressed[key] || (!m.allDay && suppressed["d"+s.In(m.start.Location()).Format("2006-01-02")]) {
			continue
		}
		if excluded(m, s) {
			continue
		}
		out = append(out, m.occurrence(s, shiftEnd(m, s, m.allDay, loc)))
	}
	return out
}

func excluded(m *vevent, inst time.Time) bool {
	for _, ex := range m.exdates {
		switch {
		case ex.allDay == m.allDay:
			if instKey(ex.t, ex.allDay) == instKey(inst, m.allDay) {
				return true
			}
		case ex.allDay: // date EXDATE against a timed series: whole day
			if ex.t.Format("2006-01-02") == inst.In(ex.t.Location()).Format("2006-01-02") {
				return true
			}
		}
	}
	return false
}

func hasDateOnlyUntil(rule string) bool {
	for _, part := range strings.Split(rule, ";") {
		k, v, ok := strings.Cut(part, "=")
		if ok && strings.EqualFold(strings.TrimSpace(k), "UNTIL") {
			return len(strings.TrimSpace(v)) == 8
		}
	}
	return false
}

func decodeEvents(data []byte) (events []*ical.Component, err error) {
	defer func() {
		if r := recover(); r != nil {
			events, err = nil, fmt.Errorf("parse calendar: %v", r)
		}
	}()
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	data = base64Shorthand.ReplaceAll(data, []byte("ENCODING=BASE64$1"))
	dec := ical.NewDecoder(bytes.NewReader(data))
	n := 0
	for {
		cal, err := dec.Decode()
		if err != nil {
			if n > 0 && errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.EOF) {
				return nil, ErrNoCalendar
			}
			return nil, fmt.Errorf("parse calendar: %w", err)
		}
		n++
		for _, c := range cal.Children {
			if c.Name == ical.CompEvent {
				events = append(events, c)
			}
		}
	}
	return events, nil
}

func parseEvent(c *ical.Component, loc *time.Location) (*vevent, error) {
	ev := &vevent{}
	if p := c.Props.Get(ical.PropUID); p != nil {
		ev.uid = strings.TrimSpace(p.Value)
	}
	if p := c.Props.Get(ical.PropSummary); p != nil {
		ev.title = strings.TrimSpace(unescapeText(p.Value))
	}
	if p := c.Props.Get(ical.PropLocation); p != nil {
		ev.location = strings.TrimSpace(unescapeText(p.Value))
	}
	if p := c.Props.Get(ical.PropStatus); p != nil {
		ev.cancelled = strings.EqualFold(strings.TrimSpace(p.Value), "CANCELLED")
	}
	if p := c.Props.Get(ical.PropRecurrenceID); p != nil {
		d, err := parseTime(p, loc)
		if err != nil {
			return nil, fmt.Errorf("recurrence-id: %w", err)
		}
		ev.rid = &d
	}
	if p := c.Props.Get(ical.PropDateTimeStart); p != nil {
		d, err := parseTime(p, loc)
		if err != nil {
			return nil, fmt.Errorf("dtstart: %w", err)
		}
		ev.start, ev.allDay, ev.hasStart = d.t, d.allDay, true
	} else if ev.rid == nil {
		return nil, errors.New("missing dtstart")
	}

	switch {
	case c.Props.Get(ical.PropDateTimeEnd) != nil:
		d, err := parseTime(c.Props.Get(ical.PropDateTimeEnd), loc)
		if err != nil {
			return nil, fmt.Errorf("dtend: %w", err)
		}
		ev.end, ev.hasEnd = d.t, true
	case c.Props.Get(ical.PropDuration) != nil:
		dur, err := parseDuration(c.Props.Get(ical.PropDuration).Value)
		if err != nil {
			return nil, fmt.Errorf("duration: %w", err)
		}
		if ev.allDay {
			days := int((dur + 24*time.Hour - 1) / (24 * time.Hour))
			ev.end = time.Date(ev.start.Year(), ev.start.Month(), ev.start.Day()+days, 0, 0, 0, 0, ev.start.Location())
		} else {
			ev.end = ev.start.Add(dur)
		}
		ev.hasEnd = true
	case ev.hasStart:
		ev.end = defaultEnd(ev.start, ev.allDay)
	}
	if ev.hasStart && ev.hasEnd && ev.allDay && !ev.end.After(ev.start) {
		ev.end = defaultEnd(ev.start, true)
	}
	if ev.hasStart && ev.end.Before(ev.start) {
		ev.end = ev.start
	}

	if p := c.Props.Get(ical.PropRecurrenceRule); p != nil && ev.rid == nil {
		ev.rrule = strings.TrimSpace(p.Value)
	}
	for _, p := range c.Props.Values(ical.PropRecurrenceDates) {
		ev.rdates = append(ev.rdates, parseList(p, loc)...)
	}
	for _, p := range c.Props.Values(ical.PropExceptionDates) {
		ev.exdates = append(ev.exdates, parseList(p, loc)...)
	}
	return ev, nil
}

// parseList reads a comma separated RDATE/EXDATE value, skipping bad entries.
func parseList(p ical.Prop, loc *time.Location) []dateVal {
	var out []dateVal
	for _, part := range strings.Split(p.Value, ",") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "/"); i >= 0 { // PERIOD: keep the start
			part = part[:i]
		}
		if part == "" {
			continue
		}
		single := p
		single.Value = part
		if d, err := parseTime(&single, loc); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// parseTime reads a DATE or DATE-TIME property value.
func parseTime(p *ical.Prop, loc *time.Location) (dateVal, error) {
	v := strings.TrimSpace(p.Value)
	valueParam := strings.ToUpper(p.Params.Get(ical.ParamValue))
	if valueParam == "DATE" || (valueParam != "DATE-TIME" && len(v) == 8) {
		t, err := time.ParseInLocation("20060102", v, loc)
		if err != nil {
			return dateVal{}, err
		}
		return dateVal{t: t, allDay: true}, nil
	}
	if strings.HasSuffix(v, "Z") || strings.HasSuffix(v, "z") {
		t, err := time.ParseInLocation("20060102T150405", v[:len(v)-1], time.UTC)
		return dateVal{t: t}, err
	}
	zone := resolveLocation(p.Params.Get(ical.PropTimezoneID), loc)
	t, err := time.ParseInLocation("20060102T150405", v, zone)
	return dateVal{t: t}, err
}

var durationRe = regexp.MustCompile(`^([+-])?P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

func parseDuration(s string) (time.Duration, error) {
	m := durationRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(s)))
	if m == nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	num := func(i int) time.Duration {
		n, _ := strconv.Atoi(m[i])
		return time.Duration(n)
	}
	d := num(2)*7*24*time.Hour + num(3)*24*time.Hour + num(4)*time.Hour + num(5)*time.Minute + num(6)*time.Second
	if m[1] == "-" {
		d = -d
	}
	return d, nil
}

// unescapeText resolves iCalendar TEXT escapes; unlike go-ical's Text() it
// tolerates unknown escapes such as "\:" that real feeds contain.
func unescapeText(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			sb.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n', 'N':
			sb.WriteByte('\n')
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}
