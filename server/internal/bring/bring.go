// Package bring reads the open items of a Bring! shopping list and turns them
// into what the dashboard shows: de-duplicated, cut to a maximum and with the
// address of Bring!'s picture for each item.
package bring

import (
	"regexp"
	"strings"
)

// iconBaseURL is where Bring!'s web app serves its item pictures.
const iconBaseURL = "https://web.getbring.com/assets/images/items/"

// Item is one open entry of the list as Bring! reports it.
type Item struct {
	Name          string
	Specification string
}

// slugReplacer applies the replacements that precede the generic character
// filter; the input is lower-cased first.
var slugReplacer = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", " ", "_")

var slugOther = regexp.MustCompile(`[^a-z0-9_]`)

// IconSlug returns the file name stem of Bring!'s picture for an item name or
// catalogue id: "Äpfel & Birnen" becomes "aepfel___birnen" (every other
// character turns into an underscore, the ends are trimmed).
func IconSlug(value string) string {
	s := slugReplacer.Replace(strings.ToLower(strings.TrimSpace(value)))
	return strings.Trim(slugOther.ReplaceAllString(s, "_"), "_")
}

// IconURL returns the picture address of an item. userIcons maps the
// catalogue id (case-insensitive, lower-cased keys) to the picture id the user
// chose; without an entry the item's own name decides. It is empty when no
// slug can be derived (an emoji name, for example), so the widget falls back
// to the name itself.
func IconURL(name string, userIcons map[string]string) string {
	icon := name
	if id := strings.TrimSpace(userIcons[strings.ToLower(strings.TrimSpace(name))]); id != "" {
		icon = id
	}
	slug := IconSlug(icon)
	if slug == "" {
		return ""
	}
	return iconBaseURL + slug + ".png"
}

// Select trims the items, drops nameless ones and duplicates (same name and
// specification, case-insensitive) and keeps at most maxItems of them in
// their original order.
func Select(items []Item, maxItems int) []Item {
	out := []Item{}
	seen := map[string]bool{}
	for _, it := range items {
		name, spec := strings.TrimSpace(it.Name), strings.TrimSpace(it.Specification)
		if name == "" {
			continue
		}
		key := strings.ToLower(name) + "\x1f" + strings.ToLower(spec)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Item{Name: name, Specification: spec})
		if len(out) >= maxItems {
			break
		}
	}
	return out
}
