package api

import (
	"context"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/thazzr/mirrornextgen/server/internal/calendar"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
)

func iconRefOut(kind, name, file string) IconRef {
	if kind == "" {
		kind = string(None)
	}
	return IconRef{Kind: IconRefKind(kind), Name: optional(name), File: optional(file)}
}

func toIconRef(r settings.IconRef) IconRef { return iconRefOut(r.Kind, r.Name, r.File) }

func fromIconRef(r IconRef) settings.IconRef {
	return settings.IconRef{Kind: string(r.Kind), Name: deref(r.Name), File: deref(r.File)}
}

func toCalendarSettings(c settings.CalendarSettings) CalendarSettings {
	out := CalendarSettings{
		RefreshMinutes: c.RefreshMinutes,
		Calendars:      make([]CalendarSource, 0, len(c.Calendars)),
		Sections:       make([]CalendarSection, 0, len(c.Sections)),
		Rules:          make([]CalendarRule, 0, len(c.Rules)),
	}
	for _, cal := range c.Calendars {
		// The address stays on the server; only a shortened hint leaves it.
		out.Calendars = append(out.Calendars, CalendarSource{
			Id: cal.ID, Name: cal.Name, Icon: toIconRef(cal.Icon),
			HasUrl: cal.URL != "", UrlHint: settings.URLHint(cal.URL),
		})
	}
	for _, sec := range c.Sections {
		o := CalendarSection{
			Id: sec.ID, Title: sec.Title, Type: CalendarSectionType(sec.Type), CalendarIds: sec.CalendarIDs,
			MaxEvents: sec.MaxEvents, LookAheadDays: sec.LookAheadDays, ShiftDays: sec.ShiftDays,
			Shifts: make([]CalendarShift, 0, len(sec.Shifts)),
		}
		for _, sh := range sec.Shifts {
			o.Shifts = append(o.Shifts, CalendarShift{
				Id: sh.ID, Match: sh.Match, MatchMode: CalendarShiftMatchMode(sh.MatchMode), Label: sh.Label,
				Start: sh.Start, End: sh.End, Icon: toIconRef(sh.Icon),
			})
		}
		out.Sections = append(out.Sections, o)
	}
	for _, r := range c.Rules {
		out.Rules = append(out.Rules, CalendarRule{
			Id: r.ID, CalendarIds: r.CalendarIDs, MatchMode: CalendarRuleMatchMode(r.MatchMode), Text: r.Text,
			Action: CalendarRuleAction(r.Action), Icon: toIconRef(r.Icon), RenameTo: r.RenameTo,
		})
	}
	return out
}

func toCalendarUpdate(b CalendarSettingsUpdate) settings.CalendarUpdate {
	u := settings.CalendarUpdate{RefreshMinutes: b.RefreshMinutes}
	for _, c := range b.Calendars {
		u.Calendars = append(u.Calendars, settings.CalendarSourceUpdate{ID: c.Id, Name: c.Name, URL: deref(c.Url), Icon: fromIconRef(c.Icon)})
	}
	for _, sec := range b.Sections {
		o := settings.CalendarSection{
			ID: sec.Id, Title: sec.Title, Type: string(sec.Type), CalendarIDs: sec.CalendarIds,
			MaxEvents: sec.MaxEvents, LookAheadDays: sec.LookAheadDays, ShiftDays: sec.ShiftDays,
		}
		for _, sh := range sec.Shifts {
			o.Shifts = append(o.Shifts, settings.CalendarShift{
				ID: sh.Id, Match: sh.Match, MatchMode: string(sh.MatchMode), Label: sh.Label,
				Start: sh.Start, End: sh.End, Icon: fromIconRef(sh.Icon),
			})
		}
		u.Sections = append(u.Sections, o)
	}
	for _, r := range b.Rules {
		u.Rules = append(u.Rules, settings.CalendarRule{
			ID: r.Id, CalendarIDs: r.CalendarIds, MatchMode: string(r.MatchMode), Text: r.Text,
			Action: string(r.Action), Icon: fromIconRef(r.Icon), RenameTo: r.RenameTo,
		})
	}
	return u
}

// calendarConfig converts the stored section into what the calendar service
// evaluates.
func calendarConfig(c settings.CalendarSettings) calendar.Config {
	ref := func(r settings.IconRef) calendar.IconRef {
		return calendar.IconRef{Kind: r.Kind, Name: r.Name, File: r.File}
	}
	cfg := calendar.Config{RefreshMinutes: c.RefreshMinutes}
	for _, cal := range c.Calendars {
		cfg.Calendars = append(cfg.Calendars, calendar.Calendar{ID: cal.ID, Name: cal.Name, URL: cal.URL, Icon: ref(cal.Icon)})
	}
	for _, sec := range c.Sections {
		o := calendar.Section{
			ID: sec.ID, Title: sec.Title, Type: sec.Type, CalendarIDs: sec.CalendarIDs,
			MaxEvents: sec.MaxEvents, LookAheadDays: sec.LookAheadDays, ShiftDays: sec.ShiftDays,
		}
		for _, sh := range sec.Shifts {
			o.Shifts = append(o.Shifts, calendar.Shift{
				ID: sh.ID, Match: sh.Match, MatchMode: sh.MatchMode, Label: sh.Label, Start: sh.Start, End: sh.End, Icon: ref(sh.Icon),
			})
		}
		cfg.Sections = append(cfg.Sections, o)
	}
	for _, r := range c.Rules {
		cfg.Rules = append(cfg.Rules, calendar.Rule{
			ID: r.ID, CalendarIDs: r.CalendarIDs, MatchMode: r.MatchMode, Text: r.Text,
			Action: r.Action, Icon: ref(r.Icon), RenameTo: r.RenameTo,
		})
	}
	return cfg
}

func calIcon(r calendar.IconRef) IconRef { return iconRefOut(r.Kind, r.Name, r.File) }

func toShiftDays(days []calendar.ShiftDay) []ShiftDay {
	out := make([]ShiftDay, 0, len(days))
	for _, d := range days {
		date, err := time.Parse(time.DateOnly, d.Date)
		if err != nil {
			continue // the engine formats the date itself; cannot happen
		}
		out = append(out, ShiftDay{Date: openapi_types.Date{Time: date}, DayLabel: d.DayLabel, Text: d.Text, Icon: calIcon(d.Icon), Free: d.Free})
	}
	return out
}

// GetCalendarSettings returns the calendar settings without the addresses.
func (s *Server) GetCalendarSettings(_ context.Context, _ GetCalendarSettingsRequestObject) (GetCalendarSettingsResponseObject, error) {
	return GetCalendarSettings200JSONResponse(toCalendarSettings(s.settings.Get().Calendar)), nil
}

// UpdateCalendarSettings stores new calendar settings and drops the cached
// feeds so the next dashboard poll uses them.
func (s *Server) UpdateCalendarSettings(_ context.Context, request UpdateCalendarSettingsRequestObject) (UpdateCalendarSettingsResponseObject, error) {
	// Held so a concurrent icon delete cannot slip between the icon check and
	// the write.
	s.iconMu.Lock()
	stored, errs, err := s.settings.UpdateCalendar(toCalendarUpdate(*request.Body))
	s.iconMu.Unlock()
	if err != nil {
		return nil, err
	}
	if errs != nil {
		return UpdateCalendarSettings400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	s.calendar.Invalidate()
	return UpdateCalendarSettings200JSONResponse(toCalendarSettings(stored)), nil
}

// PreviewCalendar evaluates an unsaved draft against the real feeds.
func (s *Server) PreviewCalendar(ctx context.Context, request PreviewCalendarRequestObject) (PreviewCalendarResponseObject, error) {
	draft, errs := s.settings.DraftCalendar(toCalendarUpdate(*request.Body))
	if errs != nil {
		return PreviewCalendar400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(badRequest(errs))}, nil
	}
	sections, calErrs := s.calendar.Preview(ctx, calendarConfig(draft))
	out := PreviewCalendar200JSONResponse{
		Sections:       make([]CalendarPreviewSection, 0, len(sections)),
		CalendarErrors: make([]CalendarError, 0, len(calErrs)),
	}
	for _, e := range calErrs {
		out.CalendarErrors = append(out.CalendarErrors, CalendarError{CalendarId: e.CalendarID, Message: e.Message})
	}
	for _, sec := range sections {
		o := CalendarPreviewSection{Id: sec.ID, Title: sec.Title, Type: CalendarPreviewSectionType(sec.Type), Error: optional(sec.Error)}
		switch {
		case sec.Type == calendar.TypeShifts && sec.Days != nil:
			days := toShiftDays(sec.Days)
			o.Days, o.Summary = &days, optional(sec.Summary)
		case sec.Type == calendar.TypeList:
			items := make([]PreviewItem, 0, len(sec.Candidates))
			for _, p := range sec.Candidates {
				rules := p.MatchedRules
				if rules == nil {
					rules = []string{}
				}
				items = append(items, PreviewItem{
					Title: p.Title, OriginalTitle: p.OriginalTitle, Location: optional(p.Location), TimeLabel: p.TimeLabel,
					Relative: p.Relative, Icon: calIcon(p.Icon), AllDay: p.AllDay, Ongoing: p.Ongoing,
					Hidden: p.Hidden, Shown: p.Shown, MatchedRules: rules, CalendarId: p.CalendarID, Start: p.Start,
				})
			}
			o.Items = &items
		}
		out.Sections = append(out.Sections, o)
	}
	return out, nil
}

// GetDashboardCalendarSection returns one section's events or shift days; a
// failing calendar travels in the body so the other areas stay visible.
func (s *Server) GetDashboardCalendarSection(ctx context.Context, request GetDashboardCalendarSectionRequestObject) (GetDashboardCalendarSectionResponseObject, error) {
	res, ok := s.calendar.Section(ctx, calendarConfig(s.settings.Get().Calendar), request.SectionId)
	if !ok {
		return GetDashboardCalendarSection404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse(problem(404, ""))}, nil
	}
	out := GetDashboardCalendarSection200JSONResponse{Id: res.ID, Title: res.Title, Type: DashboardCalendarSectionType(res.Type), Error: optional(res.Error)}
	if res.Type == calendar.TypeShifts {
		if res.Days != nil {
			days := toShiftDays(res.Days)
			out.Days = &days
		}
		return out, nil
	}
	items := make([]DashboardCalendarItem, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, DashboardCalendarItem{
			Title: it.Title, Location: optional(it.Location), TimeLabel: it.TimeLabel, Relative: it.Relative,
			Icon: calIcon(it.Icon), AllDay: it.AllDay, Ongoing: it.Ongoing,
		})
	}
	out.Items = &items
	return out, nil
}
