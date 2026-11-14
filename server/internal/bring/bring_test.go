package bring

import (
	"reflect"
	"testing"
)

func TestIconSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Milch", "milch"},
		{"  Milch  ", "milch"},
		{"Äpfel", "aepfel"},
		{"Öl", "oel"},
		{"Müsli", "muesli"},
		{"Maße", "masse"},
		{"ÄÖÜ", "aeoeue"},
		{"Salz & Pfeffer", "salz___pfeffer"},
		{"Salz&Pfeffer", "salz_pfeffer"},
		{"Olivenöl extra", "olivenoel_extra"},
		{"Käse (mild)", "kaese__mild"},
		{"Crème fraîche", "cr_me_fra_che"},
		{"7up", "7up"},
		{"_lead_", "lead"},
		{"🥛", ""},
		{"  ", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := IconSlug(tt.in); got != tt.want {
				t.Errorf("IconSlug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIconURL(t *testing.T) {
	icons := map[string]string{"milch": "Vollmilch", "kaffee": " "}
	tests := []struct {
		name, want string
	}{
		{"Brot", "https://web.getbring.com/assets/images/items/brot.png"},
		{"Äpfel", "https://web.getbring.com/assets/images/items/aepfel.png"},
		// The user's picture choice wins over the name; lookup ignores case.
		{"Milch", "https://web.getbring.com/assets/images/items/vollmilch.png"},
		{" MILCH ", "https://web.getbring.com/assets/images/items/vollmilch.png"},
		// A blank choice falls back to the name.
		{"Kaffee", "https://web.getbring.com/assets/images/items/kaffee.png"},
		// No slug: no address, the widget shows the name instead.
		{"🥛", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IconURL(tt.name, icons); got != tt.want {
				t.Errorf("IconURL(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
	if got := IconURL("Brot", nil); got == "" {
		t.Error("nil map must work")
	}
}

func TestSelect(t *testing.T) {
	in := []Item{
		{"Milch", "2 Liter"},
		{" milch ", " 2 LITER "}, // same after trim and case folding
		{"Milch", "1 Liter"},     // other specification, kept
		{"Milch", ""},            // no specification, kept
		{"", "ohne Namen"},       // dropped
		{"   ", ""},              // dropped
		{"Brot", ""},
		{"BROT", ""}, // duplicate
		{"Eier", "10"},
	}
	tests := []struct {
		name string
		max  int
		want []Item
	}{
		{"all", 50, []Item{{"Milch", "2 Liter"}, {"Milch", "1 Liter"}, {"Milch", ""}, {"Brot", ""}, {"Eier", "10"}}},
		{"cut to 2", 2, []Item{{"Milch", "2 Liter"}, {"Milch", "1 Liter"}}},
		// Duplicates do not use up the budget.
		{"cut counts kept items", 4, []Item{{"Milch", "2 Liter"}, {"Milch", "1 Liter"}, {"Milch", ""}, {"Brot", ""}}},
		{"one", 1, []Item{{"Milch", "2 Liter"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Select(in, tt.max); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Select = %v, want %v", got, tt.want)
			}
		})
	}
	if got := Select(nil, 5); got == nil || len(got) != 0 {
		t.Errorf("Select(nil) = %#v, want an empty non-nil slice", got)
	}
}
