// Package display decides when the mirror's monitor is on: the night-schedule
// rules as pure functions of the clock, and the short "test off" override.
package display

import (
	"sort"
	"time"
)

// Schedule is the night schedule (Nachtruhe): the monitor is off from OffTime
// (inclusive) until OnTime (exclusive), in the time zone the caller passes.
// A period may cross midnight; equal times mean there is no quiet time.
type Schedule struct {
	Enabled bool   `json:"enabled"`
	OffTime string `json:"offTime"` // "HH:mm"
	OnTime  string `json:"onTime"`  // "HH:mm"
}

// ParseClock reads "HH:mm" (exactly five characters) as minutes after midnight.
func ParseClock(s string) (minutes int, ok bool) {
	if len(s) != 5 {
		return 0, false
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// Effective reports whether the schedule can ever switch the monitor off.
func (s Schedule) Effective() bool {
	off, okOff := ParseClock(s.OffTime)
	on, okOn := ParseClock(s.OnTime)
	return s.Enabled && okOff && okOn && off != on
}

type switchAt struct {
	at time.Time
	on bool
}

func wallMinutes(t time.Time) int { return t.Hour()*60 + t.Minute() }

// instantOf is the first instant of the local day (y, m, d) at which the wall
// clock shows minutes or later. A time that does not exist (the hour skipped
// by the switch to summer time) resolves to the moment the clock jumps over
// it; a time that exists twice resolves to whichever instant time.Date picks.
func instantOf(y int, m time.Month, d, minutes int, loc *time.Location) time.Time {
	t := time.Date(y, m, d, minutes/60, minutes%60, 0, 0, loc)
	if wallMinutes(t) == minutes {
		return t
	}
	// In the gap: step back to the first minute that is past the requested one.
	for {
		prev := t.Add(-time.Minute)
		if prev.Day() != t.Day() || wallMinutes(prev) < minutes {
			return t
		}
		t = prev
	}
}

// switches lists the schedule's switch instants around now, ordered by time:
// from yesterday to tomorrow in now's location.
func switches(s Schedule, now time.Time) []switchAt {
	off, _ := ParseClock(s.OffTime)
	on, _ := ParseClock(s.OnTime)
	loc := now.Location()
	y, m, d := now.Date()
	var out []switchAt
	for day := -1; day <= 1; day++ {
		dy, dm, dd := time.Date(y, m, d+day, 12, 0, 0, 0, loc).Date()
		out = append(out,
			switchAt{instantOf(dy, dm, dd, off, loc), false},
			switchAt{instantOf(dy, dm, dd, on, loc), true},
		)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.Before(out[j].at) })
	return out
}

// PowerOn reports whether the schedule wants the monitor on at now. The
// latest switch at or before now decides, so the off time itself is off and
// the on time itself is on, and the hour that repeats when the clocks go back
// cannot switch twice.
func PowerOn(s Schedule, now time.Time) bool {
	if !s.Effective() {
		return true
	}
	on := true
	for _, sw := range switches(s, now) {
		if sw.at.After(now) {
			break
		}
		on = sw.on
	}
	return on
}

// NextSwitch returns the first switch after now and whether it turns the
// monitor on. ok is false without an effective schedule.
func NextSwitch(s Schedule, now time.Time) (at time.Time, on bool, ok bool) {
	if !s.Effective() {
		return time.Time{}, false, false
	}
	for _, sw := range switches(s, now) {
		if sw.at.After(now) {
			return sw.at, sw.on, true
		}
	}
	return time.Time{}, false, false
}
