// Package settings holds the persisted mirror settings (settings.json).
package settings

import "regexp"

// SchemaVersion is the version of the settings file format. A file with any
// other version is discarded, not migrated (nothing is deployed yet).
const SchemaVersion = 1

// Messages of the field errors, keyed by field path in Validate.
const (
	msgRotation = "Erlaubt sind 0°, 90°, 180° und 270°."
	msgOutput   = "Erlaubt sind 1 bis 32 Zeichen aus Buchstaben, Ziffern, Punkt, Bindestrich und Unterstrich."
)

// outputPattern is strict because the kiosk agent passes the value to
// wlr-randr as a command argument.
var outputPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// DisplaySettings control the mirror's screen.
type DisplaySettings struct {
	// Rotation in degrees clockwise: 0, 90, 180 or 270.
	Rotation int `json:"rotation"`
	// Output is a connector name such as HDMI-A-1; "" means automatic.
	Output string `json:"output"`
	// TestPattern shows the diagnose overlay on the mirror.
	TestPattern bool `json:"testPattern"`
}

// Settings is the content of settings.json. Later phases add sections.
type Settings struct {
	SchemaVersion int             `json:"schemaVersion"`
	Display       DisplaySettings `json:"display"`
}

// Defaults returns the settings used while no file exists.
func Defaults() Settings {
	return Settings{SchemaVersion: SchemaVersion}
}

// Validate returns a German message per invalid field path; nil means valid.
func (s Settings) Validate() map[string]string {
	errs := map[string]string{}
	switch s.Display.Rotation {
	case 0, 90, 180, 270:
	default:
		errs["display.rotation"] = msgRotation
	}
	if s.Display.Output != "" && !outputPattern.MatchString(s.Display.Output) {
		errs["display.output"] = msgOutput
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
