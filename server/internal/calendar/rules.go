package calendar

import "strings"

// Icon kinds, rule match modes and rule actions.
const (
	IconBootstrap = "bootstrap"
	IconUpload    = "upload"
	IconNone      = "none"

	MatchContains   = "contains"
	MatchStartsWith = "startsWith"
	MatchEquals     = "equals"

	ActionHide   = "hide"
	ActionIcon   = "icon"
	ActionRename = "rename"
)

// IconRef points at a symbol: a Bootstrap icon name or an uploaded file.
type IconRef struct {
	Kind string
	Name string
	File string
}

// Rule is one entry of the ordered rule list. An empty CalendarIDs applies to
// every calendar; an empty Text matches every title.
type Rule struct {
	ID          string
	CalendarIDs []string
	MatchMode   string
	Text        string
	Action      string
	Icon        IconRef
	RenameTo    string
}

// Result is what the rules make of one event.
type Result struct {
	Hidden  bool
	Title   string
	Icon    IconRef
	Matched []string // IDs of every rule that matched, in order
}

func (i IconRef) set() bool { return i.Kind != "" && i.Kind != IconNone }

// textMatches compares case-insensitively on trimmed text. An empty needle
// matches any title.
func textMatches(mode, title, needle string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	switch mode {
	case MatchEquals:
		return title == needle
	case MatchStartsWith:
		return strings.HasPrefix(title, needle)
	default:
		return strings.Contains(title, needle)
	}
}

func (r Rule) appliesTo(calendarID, title string) bool {
	if len(r.CalendarIDs) > 0 {
		found := false
		for _, id := range r.CalendarIDs {
			if id == calendarID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return textMatches(r.MatchMode, title, r.Text)
}

// Apply runs the rules in order against one event. The first icon rule and the
// first rename rule win; a hide rule ends the evaluation.
func Apply(rules []Rule, calendarID, title string, defaultIcon IconRef) Result {
	res := Result{Title: title, Icon: defaultIcon}
	iconSet, renamed := false, false
	for _, r := range rules {
		if !r.appliesTo(calendarID, title) {
			continue
		}
		res.Matched = append(res.Matched, r.ID)
		switch r.Action {
		case ActionHide:
			res.Hidden = true
			return res
		case ActionIcon:
			if !iconSet {
				res.Icon, iconSet = r.Icon, true
			}
		case ActionRename:
			if !renamed && strings.TrimSpace(r.RenameTo) != "" {
				res.Title, renamed = r.RenameTo, true
			}
		}
	}
	return res
}
