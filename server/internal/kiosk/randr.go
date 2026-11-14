package kiosk

import (
	"encoding/json"
	"errors"
	"fmt"
)

// State is the screen state kioskd applies. Phase 6 adds a PowerOn field;
// Plan compares field by field, so it only needs one more branch.
type State struct {
	Rotation int
	Output   string
}

// Transform maps a rotation in degrees to the wlr-randr transform name.
func Transform(rotation int) (string, error) {
	switch rotation {
	case 0:
		return "normal", nil
	case 90, 180, 270:
		return fmt.Sprint(rotation), nil
	}
	return "", fmt.Errorf("unsupported rotation %d", rotation)
}

type randrOutput struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// PickOutput chooses the output to configure from `wlr-randr --json`:
// the preferred one when given (an error if it does not exist), else the
// first enabled one, else the first one.
func PickOutput(outputsJSON []byte, preferred string) (string, error) {
	var outputs []randrOutput
	if err := json.Unmarshal(outputsJSON, &outputs); err != nil {
		return "", fmt.Errorf("parse wlr-randr output: %w", err)
	}
	if len(outputs) == 0 {
		return "", errors.New("wlr-randr reports no outputs")
	}
	if preferred != "" {
		for _, o := range outputs {
			if o.Name == preferred {
				return o.Name, nil
			}
		}
		return "", fmt.Errorf("output %q not found", preferred)
	}
	for _, o := range outputs {
		if o.Enabled {
			return o.Name, nil
		}
	}
	return outputs[0].Name, nil
}

// Plan returns the wlr-randr argument lists that bring the screen from
// applied to desired on outputName. It is empty when nothing changed; a nil
// applied (first run) always applies. An unsupported rotation plans nothing.
func Plan(desired State, applied *State, outputName string) [][]string {
	transform, err := Transform(desired.Rotation)
	if err != nil {
		return nil
	}
	if applied != nil && applied.Output == outputName && applied.Rotation == desired.Rotation {
		return nil
	}
	return [][]string{{"--output", outputName, "--transform", transform}}
}
