package calendar

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Messages shown on the dashboard; the details of a failure go to the log.
const (
	MsgSomeFailed = "Einige Kalender konnten nicht geladen werden."
	MsgAllFailed  = "Die Kalender konnten nicht geladen werden."
)

// MsgCalendarFailed is the error of one calendar.
func MsgCalendarFailed(name string) string {
	return fmt.Sprintf("Kalender „%s“ konnte nicht geladen werden.", name)
}

// Section types.
const (
	TypeList   = "list"
	TypeShifts = "shifts"
)

// Calendar is one feed in the configuration.
type Calendar struct {
	ID   string
	Name string
	URL  string
	Icon IconRef
}

// Section is one area of the mirror.
type Section struct {
	ID            string
	Title         string
	Type          string
	CalendarIDs   []string
	MaxEvents     int
	LookAheadDays int
	ShiftDays     int
	ShowFreeDays  bool
	Shifts        []Shift
}

// Config is the part of the settings the service evaluates.
type Config struct {
	RefreshMinutes int
	Calendars      []Calendar
	Sections       []Section
	Rules          []Rule
}

// SectionResult is one evaluated section. Error is set when calendars failed;
// Items/Days are empty when all of them did.
type SectionResult struct {
	ID    string
	Title string
	Type  string
	Error string
	// Items are the events the mirror shows (list type).
	Items []ListItem
	// Candidates are all events of the window with rule results (list type),
	// for the admin preview.
	Candidates []PreviewItem
	Days       []ShiftDay
}

// CalendarError names a calendar that could not be loaded.
type CalendarError struct {
	CalendarID string
	Message    string
}

// Service turns feeds and configuration into dashboard sections.
type Service struct {
	feeds *Feeds
	now   func() time.Time
	loc   *time.Location
	log   *slog.Logger
}

// ServiceOptions are the collaborators of a Service.
type ServiceOptions struct {
	Feeds *Feeds
	// Now defaults to time.Now; Location to time.Local.
	Now      func() time.Time
	Location *time.Location
	Logger   *slog.Logger
}

// NewService returns a Service.
func NewService(o ServiceOptions) *Service {
	s := &Service{feeds: o.Feeds, now: o.Now, loc: o.Location, log: o.Logger}
	if s.now == nil {
		s.now = time.Now
	}
	if s.loc == nil {
		s.loc = time.Local
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	return s
}

// Invalidate drops the cached feeds; the settings handler calls it after a
// change.
func (s *Service) Invalidate() { s.feeds.Invalidate() }

// Section evaluates one section; ok is false for an unknown id.
func (s *Service) Section(ctx context.Context, cfg Config, id string) (SectionResult, bool) {
	for _, sec := range cfg.Sections {
		if sec.ID == id {
			res, _ := s.evaluate(ctx, cfg, sec)
			return res, true
		}
	}
	return SectionResult{}, false
}

// Preview evaluates every section of cfg and lists the calendars that failed.
func (s *Service) Preview(ctx context.Context, cfg Config) ([]SectionResult, []CalendarError) {
	var out []SectionResult
	failed := map[string]string{}
	var order []string
	for _, sec := range cfg.Sections {
		res, errs := s.evaluate(ctx, cfg, sec)
		out = append(out, res)
		for _, e := range errs {
			if _, seen := failed[e.CalendarID]; !seen {
				order = append(order, e.CalendarID)
			}
			failed[e.CalendarID] = e.Message
		}
	}
	var calErrs []CalendarError
	for _, id := range order {
		calErrs = append(calErrs, CalendarError{CalendarID: id, Message: failed[id]})
	}
	return out, calErrs
}

type feedResult struct {
	cal  Calendar
	occs []Occurrence
	err  error
}

func (s *Service) evaluate(ctx context.Context, cfg Config, sec Section) (SectionResult, []CalendarError) {
	now := s.now()
	res := SectionResult{ID: sec.ID, Title: sec.Title, Type: sec.Type}

	var win Window
	today := time.Date(now.In(s.loc).Year(), now.In(s.loc).Month(), now.In(s.loc).Day(), 0, 0, 0, 0, s.loc)
	if sec.Type == TypeShifts {
		win = Window{From: today, To: today.AddDate(0, 0, shiftHorizon(sec))}
	} else {
		win = Window{From: now, To: now.In(s.loc).AddDate(0, 0, max(sec.LookAheadDays, 1))}
	}
	interval := time.Duration(max(cfg.RefreshMinutes, 1)) * time.Minute

	var cals []Calendar
	for _, id := range sec.CalendarIDs {
		for _, c := range cfg.Calendars {
			if c.ID == id {
				cals = append(cals, c)
			}
		}
	}
	results := make([]feedResult, len(cals))
	var wg sync.WaitGroup
	for i, c := range cals {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = feedResult{cal: c}
			data, err := s.feeds.Fetch(ctx, c.URL, interval)
			if err != nil {
				results[i].err = err
				return
			}
			occs, err := Expand(data, win, s.loc)
			if err != nil {
				s.log.Warn("calendar feed could not be parsed", "host", hostOf(c.URL), "error", err)
				results[i].err = err
				return
			}
			results[i].occs = occs
		}()
	}
	wg.Wait()

	var sourced []SourcedOccurrence
	var errs []CalendarError
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, CalendarError{CalendarID: r.cal.ID, Message: MsgCalendarFailed(r.cal.Name)})
			continue
		}
		for _, o := range r.occs {
			sourced = append(sourced, SourcedOccurrence{Occurrence: o, CalendarID: r.cal.ID, DefaultIcon: r.cal.Icon})
		}
	}
	switch {
	case len(errs) > 0 && len(errs) == len(cals):
		res.Error = MsgAllFailed
		if len(cals) == 1 {
			res.Error = errs[0].Message
		}
		return res, errs
	case len(errs) > 0:
		res.Error = MsgSomeFailed
	}

	if sec.Type == TypeShifts {
		res.Days = BuildShifts(sourced, cfg.Rules, sec.Shifts, shiftOptions(sec), now, s.loc)
		return res, errs
	}
	res.Candidates = BuildPreview(ListInput{Occurrences: sourced, Rules: cfg.Rules, MaxEvents: sec.MaxEvents, LookAheadDays: sec.LookAheadDays}, now, s.loc)
	for _, p := range res.Candidates {
		if p.Shown {
			res.Items = append(res.Items, p.ListItem)
		}
	}
	return res, errs
}

// shiftOptions maps the section settings to the shift builder's options.
func shiftOptions(sec Section) ShiftOptions {
	return ShiftOptions{ShowFreeDays: sec.ShowFreeDays, Rows: max(sec.ShiftDays, 1), HorizonDays: max(sec.LookAheadDays, 1)}
}

// shiftHorizon is how many days from today a shift section needs expanded:
// the search horizon, or the row count when free days fill consecutive rows.
func shiftHorizon(sec Section) int {
	o := shiftOptions(sec)
	if o.ShowFreeDays {
		return o.Rows
	}
	return o.HorizonDays
}
