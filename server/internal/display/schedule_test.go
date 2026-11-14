package display

import (
	"testing"
	"time"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// at parses a UTC instant and returns it in Berlin.
func at(t *testing.T, utc string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, utc)
	if err != nil {
		t.Fatal(err)
	}
	return v.In(berlin(t))
}

func local(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04:05", s, berlin(t))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestParseClock(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"00:00", 0, true},
		{"23:59", 1439, true},
		{"06:30", 390, true},
		{"24:00", 0, false},
		{"6:30", 0, false},
		{"06:60", 0, false},
		{"06:30:00", 0, false},
		{"", 0, false},
		{"ab:cd", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseClock(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseClock(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestPowerOn(t *testing.T) {
	night := Schedule{Enabled: true, OffTime: "23:00", OnTime: "06:00"}
	sameDay := Schedule{Enabled: true, OffTime: "01:00", OnTime: "05:00"}
	tests := []struct {
		name   string
		s      Schedule
		now    string
		wantOn bool
	}{
		{"night: evening before off", night, "2026-11-14 22:59:59", true},
		{"night: exactly off time is off", night, "2026-11-14 23:00:00", false},
		{"night: after midnight is off", night, "2026-11-15 00:30:00", false},
		{"night: just before on time is off", night, "2026-11-15 05:59:59", false},
		{"night: exactly on time is on", night, "2026-11-15 06:00:00", true},
		{"night: midday is on", night, "2026-11-15 12:00:00", true},
		{"same day: before off", sameDay, "2026-11-14 00:59:59", true},
		{"same day: exactly off", sameDay, "2026-11-14 01:00:00", false},
		{"same day: just before on", sameDay, "2026-11-14 04:59:59", false},
		{"same day: exactly on", sameDay, "2026-11-14 05:00:00", true},
		{"same day: evening", sameDay, "2026-11-14 23:30:00", true},
		{"equal times: never off", Schedule{Enabled: true, OffTime: "23:00", OnTime: "23:00"}, "2026-11-14 23:00:00", true},
		{"disabled: never off", Schedule{Enabled: false, OffTime: "23:00", OnTime: "06:00"}, "2026-11-15 01:00:00", true},
		{"invalid time: never off", Schedule{Enabled: true, OffTime: "late", OnTime: "06:00"}, "2026-11-15 01:00:00", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PowerOn(tt.s, local(t, tt.now)); got != tt.wantOn {
				t.Errorf("PowerOn at %s = %v, want %v", tt.now, got, tt.wantOn)
			}
		})
	}
}

func TestNextSwitch(t *testing.T) {
	night := Schedule{Enabled: true, OffTime: "23:00", OnTime: "06:00"}
	tests := []struct {
		name   string
		s      Schedule
		now    time.Time
		wantAt time.Time
		wantOn bool
		wantOK bool
	}{
		{"evening: off tonight", night, local(t, "2026-11-14 20:00:00"), local(t, "2026-11-14 23:00:00"), false, true},
		{"exactly off time: next is on", night, local(t, "2026-11-14 23:00:00"), local(t, "2026-11-15 06:00:00"), true, true},
		{"after midnight: on this morning", night, local(t, "2026-11-15 01:00:00"), local(t, "2026-11-15 06:00:00"), true, true},
		{"exactly on time: next is off", night, local(t, "2026-11-15 06:00:00"), local(t, "2026-11-15 23:00:00"), false, true},
		{"morning: off tonight", night, local(t, "2026-11-15 08:00:00"), local(t, "2026-11-15 23:00:00"), false, true},
		{"equal times", Schedule{Enabled: true, OffTime: "07:00", OnTime: "07:00"}, local(t, "2026-11-15 08:00:00"), time.Time{}, false, false},
		{"disabled", Schedule{OffTime: "23:00", OnTime: "06:00"}, local(t, "2026-11-15 08:00:00"), time.Time{}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAt, gotOn, ok := NextSwitch(tt.s, tt.now)
			if ok != tt.wantOK || gotOn != tt.wantOn || !gotAt.Equal(tt.wantAt) {
				t.Errorf("NextSwitch = %v, %v, %v; want %v, %v, %v", gotAt, gotOn, ok, tt.wantAt, tt.wantOn, tt.wantOK)
			}
		})
	}
}

// changes counts the power changes of s minute by minute from start to end.
func changes(t *testing.T, s Schedule, start, end string) (n int, states string) {
	t.Helper()
	cur := at(t, start)
	last := PowerOn(s, cur)
	for ; cur.Before(at(t, end)); cur = cur.Add(time.Minute) {
		if on := PowerOn(s, cur); on != last {
			n++
			last = on
			if on {
				states += "on "
			} else {
				states += "off "
			}
		}
	}
	return n, states
}

func TestPowerOnSpringForward(t *testing.T) {
	// 2026-03-29: at 01:00 UTC the clock jumps from 02:00 CET to 03:00 CEST,
	// so 02:30 does not exist.
	t.Run("on time inside the gap switches when the clock jumps", func(t *testing.T) {
		s := Schedule{Enabled: true, OffTime: "23:00", OnTime: "02:30"}
		if PowerOn(s, at(t, "2026-03-29T00:59:00Z")) { // 01:59 CET
			t.Error("on at 01:59 CET, want off")
		}
		if !PowerOn(s, at(t, "2026-03-29T01:00:00Z")) { // 03:00 CEST
			t.Error("off at 03:00 CEST, want on")
		}
		got, on, ok := NextSwitch(s, at(t, "2026-03-29T00:00:00Z"))
		if !ok || !on || !got.Equal(at(t, "2026-03-29T01:00:00Z")) {
			t.Errorf("NextSwitch = %v, %v, %v; want 03:00 CEST on", got, on, ok)
		}
	})
	t.Run("off time inside the gap switches when the clock jumps", func(t *testing.T) {
		s := Schedule{Enabled: true, OffTime: "02:30", OnTime: "06:00"}
		if !PowerOn(s, at(t, "2026-03-29T00:59:00Z")) {
			t.Error("off at 01:59 CET, want on")
		}
		if PowerOn(s, at(t, "2026-03-29T01:00:00Z")) {
			t.Error("on at 03:00 CEST, want off")
		}
		if !PowerOn(s, at(t, "2026-03-29T04:00:00Z")) { // 06:00 CEST
			t.Error("off at 06:00 CEST, want on")
		}
	})
	t.Run("one change each way over the day", func(t *testing.T) {
		s := Schedule{Enabled: true, OffTime: "02:30", OnTime: "06:00"}
		n, states := changes(t, s, "2026-03-28T20:00:00Z", "2026-03-29T10:00:00Z")
		if n != 2 || states != "off on " {
			t.Errorf("changes = %d (%s), want off then on", n, states)
		}
	})
}

func TestPowerOnFallBack(t *testing.T) {
	// 2026-10-25: at 01:00 UTC the clock goes from 03:00 CEST back to 02:00
	// CET, so 02:30 happens twice (00:30 UTC and 01:30 UTC).
	t.Run("off time that exists twice switches once", func(t *testing.T) {
		s := Schedule{Enabled: true, OffTime: "02:30", OnTime: "06:00"}
		n, states := changes(t, s, "2026-10-24T20:00:00Z", "2026-10-25T10:00:00Z")
		if n != 2 || states != "off on " {
			t.Errorf("changes = %d (%s), want one off and one on", n, states)
		}
		if PowerOn(s, at(t, "2026-10-25T01:45:00Z")) {
			t.Error("on at 02:45 CET, want off")
		}
		if !PowerOn(s, at(t, "2026-10-25T05:00:00Z")) { // 06:00 CET
			t.Error("off at 06:00 CET, want on")
		}
	})
	t.Run("on time that exists twice switches once", func(t *testing.T) {
		s := Schedule{Enabled: true, OffTime: "23:00", OnTime: "02:30"}
		n, states := changes(t, s, "2026-10-24T22:00:00Z", "2026-10-25T10:00:00Z")
		if n != 1 || states != "on " {
			t.Errorf("changes = %d (%s), want a single on", n, states)
		}
	})
	t.Run("next switch is the same instant PowerOn flips at", func(t *testing.T) {
		s := Schedule{Enabled: true, OffTime: "02:30", OnTime: "06:00"}
		now := at(t, "2026-10-24T22:00:00Z")
		sw, on, ok := NextSwitch(s, now)
		if !ok || on {
			t.Fatalf("NextSwitch = %v, %v, %v; want an off switch", sw, on, ok)
		}
		if !PowerOn(s, sw.Add(-time.Second)) || PowerOn(s, sw) {
			t.Errorf("PowerOn does not flip at the announced switch %v", sw)
		}
	})
}
