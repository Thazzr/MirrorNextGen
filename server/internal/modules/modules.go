// Package modules is the directory of the mirror's modules: it knows which
// module keys exist, so the layout can be validated against them. It depends
// on no other internal package (settings imports it), so the data that
// instance-based modules need arrives through Input.
package modules

import "slices"

// Info names one available module.
type Info struct {
	Key  string
	Name string
}

// Section names one calendar section; its module key is "calendar:<ID>".
type Section struct {
	ID    string
	Title string
	// Shifts marks a section of type shifts.
	Shifts bool
}

// Input is the part of the settings that decides which module instances
// exist.
type Input struct {
	CalendarSections []Section
}

func calendarInstances(in Input) []Info {
	out := make([]Info, 0, len(in.CalendarSections))
	for _, sec := range in.CalendarSections {
		prefix := "Kalender: "
		if sec.Shifts {
			prefix = "Schichten: "
		}
		out = append(out, Info{Key: "calendar:" + sec.ID, Name: prefix + sec.Title})
	}
	return out
}

// definition is either a single key (key + name) or a prefix with instances.
type definition struct {
	key  string
	name string
	// instances lists the keys of a prefix module for the given input; used
	// when key is empty.
	instances func(Input) []Info
}

func (d definition) list(in Input) []Info {
	if d.key != "" {
		return []Info{{Key: d.key, Name: d.name}}
	}
	return d.instances(in)
}

var directory = []definition{
	{key: "clock", name: "Uhr"},
	{key: "weather", name: "Wetter"},
	{instances: calendarInstances},
	{key: "wifi", name: "WLAN-QR-Code"},
	{key: "bring", name: "Bring!-Einkaufsliste"},
}

// Available returns all modules that currently exist, in directory order.
func Available(in Input) []Info {
	out := []Info{}
	for _, d := range directory {
		out = append(out, d.list(in)...)
	}
	return out
}

// Keys returns just the keys of Available.
func Keys(in Input) []string {
	infos := Available(in)
	keys := make([]string, len(infos))
	for i, m := range infos {
		keys[i] = m.Key
	}
	return keys
}

// IsAvailable reports whether key names an existing module.
func IsAvailable(in Input, key string) bool {
	return slices.Contains(Keys(in), key)
}

// Name returns the German name of an available module key.
func Name(in Input, key string) (string, bool) {
	for _, m := range Available(in) {
		if m.Key == key {
			return m.Name, true
		}
	}
	return "", false
}
