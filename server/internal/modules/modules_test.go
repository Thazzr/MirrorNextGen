package modules

import (
	"slices"
	"testing"
)

func TestAvailableListsKeysWithGermanNames(t *testing.T) {
	want := []Info{{"clock", "Uhr"}, {"weather", "Wetter"}, {"wifi", "WLAN-QR-Code"}}
	if got := Available(Input{}); !slices.Equal(got, want) {
		t.Errorf("Available = %v, want %v", got, want)
	}
	if got := Keys(Input{}); !slices.Equal(got, []string{"clock", "weather", "wifi"}) {
		t.Errorf("Keys = %v", got)
	}
}

func TestIsAvailableAndName(t *testing.T) {
	tests := []struct {
		key  string
		ok   bool
		name string
	}{
		{"clock", true, "Uhr"},
		{"wifi", true, "WLAN-QR-Code"},
		{"nope", false, ""},
		{"", false, ""},
		{"Clock", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsAvailable(Input{}, tt.key); got != tt.ok {
				t.Errorf("IsAvailable = %v", got)
			}
			name, ok := Name(Input{}, tt.key)
			if ok != tt.ok || name != tt.name {
				t.Errorf("Name = %q, %v", name, ok)
			}
		})
	}
}

func TestPrefixDefinitionListsInstances(t *testing.T) {
	d := definition{instances: func(Input) []Info {
		return []Info{{"calendar:a", "Familie"}, {"calendar:b", "Arbeit"}}
	}}
	if got := d.list(Input{}); len(got) != 2 || got[1].Key != "calendar:b" {
		t.Errorf("list = %v", got)
	}
}
