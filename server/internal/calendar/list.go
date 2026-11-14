package calendar

import (
	"sort"
	"time"
)

// SourcedOccurrence is an occurrence with the calendar it came from and that
// calendar's default icon.
type SourcedOccurrence struct {
	Occurrence
	CalendarID  string
	DefaultIcon IconRef
}

// ListInput configures one list ("Bereich"). MaxEvents <= 0 means no cut.
type ListInput struct {
	Occurrences   []SourcedOccurrence
	Rules         []Rule
	MaxEvents     int
	LookAheadDays int
}

// ListItem is one line of the dashboard list.
type ListItem struct {
	Title string
	// OriginalTitle is the title before the rules renamed it.
	OriginalTitle string
	Location      string
	TimeLabel     string
	Relative      string
	Icon          IconRef
	AllDay        bool
	Ongoing       bool
	Start         time.Time
	End           time.Time
	CalendarID    string
	MatchedRules  []string
}

// PreviewItem is a candidate for the admin preview: Hidden marks events a
// hide rule removed, Shown those that would appear on the mirror.
type PreviewItem struct {
	ListItem
	Hidden bool
	Shown  bool
}

// BuildList returns the events the mirror shows now, soonest first.
func BuildList(input ListInput, now time.Time, loc *time.Location) []ListItem {
	var out []ListItem
	for _, p := range BuildPreview(input, now, loc) {
		if p.Shown {
			out = append(out, p.ListItem)
		}
	}
	return out
}

// BuildPreview returns all candidates in the look-ahead window including the
// hidden ones, without the MaxEvents cut, and marks what would be shown.
func BuildPreview(input ListInput, now time.Time, loc *time.Location) []PreviewItem {
	if loc == nil {
		loc = time.UTC
	}
	limit := now.In(loc).AddDate(0, 0, input.LookAheadDays)

	var items []PreviewItem
	for _, so := range input.Occurrences {
		if !so.End.After(now) && !(so.End.Equal(so.Start) && !so.Start.Before(now)) {
			continue
		}
		if !so.Start.Before(limit) {
			continue
		}
		res := Apply(input.Rules, so.CalendarID, so.Title, so.DefaultIcon)
		items = append(items, PreviewItem{
			ListItem: ListItem{
				Title:         res.Title,
				OriginalTitle: so.Title,
				Location:      so.Location,
				TimeLabel:     TimeLabel(so.AllDay),
				Relative:      FormatRelativeTime(so.Start, so.End, so.AllDay, now, loc),
				Icon:          res.Icon,
				AllDay:        so.AllDay,
				Ongoing:       !so.Start.After(now) && now.Before(so.End),
				Start:         so.Start,
				End:           so.End,
				CalendarID:    so.CalendarID,
				MatchedRules:  res.Matched,
			},
			Hidden: res.Hidden,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Start.Equal(items[j].Start) {
			return items[i].Start.Before(items[j].Start)
		}
		return items[i].Title < items[j].Title
	})
	shown := 0
	for i := range items {
		if items[i].Hidden {
			continue
		}
		if input.MaxEvents > 0 && shown >= input.MaxEvents {
			continue
		}
		items[i].Shown = true
		shown++
	}
	return items
}
