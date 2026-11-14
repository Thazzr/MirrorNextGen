package kiosk

import (
	"encoding/json"
	"errors"
	"fmt"
)

// State is the screen state kioskd applies.
type State struct {
	Rotation int
	Output   string
	// PowerOn is false while the monitor is to be switched off.
	PowerOn bool
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

// PickOutput chooses the output to configure from `wlr-randr --json`, in this
// order: the preferred one when given (an error if it does not exist); the
// output chosen last time while it is still listed, enabled or not, so that an
// output switched off by the schedule is not swapped for another one; the
// first enabled one; the first one.
func PickOutput(outputsJSON []byte, preferred, last string) (string, error) {
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
	if last != "" {
		for _, o := range outputs {
			if o.Name == last {
				return o.Name, nil
			}
		}
	}
	for _, o := range outputs {
		if o.Enabled {
			return o.Name, nil
		}
	}
	return outputs[0].Name, nil
}

// Plan returns the wlr-randr argument lists that bring the screen from
// applied to desired on outputName. It is empty when nothing changed. A nil
// applied (first run) or a different output applies everything: the power
// state, and when rotate is set the transform.
//
//   - Turning the monitor on runs --on and then re-applies the transform, so a
//     compositor that forgot it while the output was off gets it back.
//   - While the monitor is off no transform is sent. A rotation that changes
//     meanwhile is not lost: desired carries it, and the next "on" applies it.
//   - rotate false (the dashboard rotates itself) leaves the transform alone;
//     power is still switched.
//
// An unsupported rotation plans nothing when rotate is set.
func Plan(desired State, applied *State, outputName string, rotate bool) [][]string {
	var transform string
	if rotate {
		t, err := Transform(desired.Rotation)
		if err != nil {
			return nil
		}
		transform = t
	}
	fresh := applied == nil || applied.Output != outputName
	powerChanged := fresh || applied.PowerOn != desired.PowerOn
	var plan [][]string
	switch {
	case !desired.PowerOn:
		if powerChanged {
			plan = append(plan, []string{"--output", outputName, "--off"})
		}
	case powerChanged:
		plan = append(plan, []string{"--output", outputName, "--on"})
		if rotate {
			plan = append(plan, []string{"--output", outputName, "--transform", transform})
		}
	case rotate && applied.Rotation != desired.Rotation:
		plan = append(plan, []string{"--output", outputName, "--transform", transform})
	}
	return plan
}
