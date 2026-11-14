package display

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newController(t *testing.T, s Schedule, c *clock) *Controller {
	t.Helper()
	return NewController(Options{Schedule: func() Schedule { return s }, Now: c.now, Location: berlin(t)})
}

func TestStatusAlwaysOn(t *testing.T) {
	c := &clock{local(t, "2026-11-14 12:00:00")}
	for _, s := range []Schedule{
		{Enabled: false, OffTime: "23:00", OnTime: "06:00"},
		{Enabled: true, OffTime: "07:00", OnTime: "07:00"},
	} {
		st := newController(t, s, c).Status()
		if !st.PowerOn || st.Reason != ReasonAlways || st.NextSwitchAt != nil {
			t.Errorf("%+v: status = %+v", s, st)
		}
		if st.TimeZone != "Europe/Berlin" || !st.Now.Equal(c.t) {
			t.Errorf("zone/now = %q %v", st.TimeZone, st.Now)
		}
	}
}

func TestStatusFollowsSchedule(t *testing.T) {
	s := Schedule{Enabled: true, OffTime: "23:00", OnTime: "06:00"}
	c := &clock{local(t, "2026-11-14 12:00:00")}
	ctl := newController(t, s, c)

	st := ctl.Status()
	if !st.PowerOn || st.Reason != ReasonSchedule || st.NextSwitchAt == nil || st.NextSwitchOn ||
		!st.NextSwitchAt.Equal(local(t, "2026-11-14 23:00:00")) {
		t.Errorf("daytime status = %+v", st)
	}
	c.t = local(t, "2026-11-15 01:00:00")
	st = ctl.Status()
	if st.PowerOn || st.Reason != ReasonSchedule || !st.NextSwitchOn || !st.NextSwitchAt.Equal(local(t, "2026-11-15 06:00:00")) {
		t.Errorf("night status = %+v", st)
	}
	if ctl.PowerOn() {
		t.Error("PowerOn() true at night")
	}
}

func TestTestOffExpiresAndExtends(t *testing.T) {
	c := &clock{local(t, "2026-11-14 12:00:00")}
	ctl := newController(t, Schedule{}, c)
	if !ctl.PowerOn() {
		t.Fatal("off before the test")
	}

	st := ctl.TestOff()
	if st.PowerOn || st.Reason != ReasonTest {
		t.Fatalf("status right after TestOff = %+v", st)
	}
	c.t = c.t.Add(9 * time.Second)
	if ctl.PowerOn() {
		t.Error("on again after 9 s")
	}
	// A second call restarts the ten seconds.
	ctl.TestOff()
	c.t = c.t.Add(9 * time.Second)
	if ctl.PowerOn() {
		t.Error("second call did not extend the override")
	}
	c.t = c.t.Add(time.Second) // 10 s after the second call
	st = ctl.Status()
	if !st.PowerOn || st.Reason != ReasonAlways {
		t.Errorf("status after expiry = %+v", st)
	}
}

func TestTestOffFallsBackToTheScheduleAfterwards(t *testing.T) {
	s := Schedule{Enabled: true, OffTime: "23:00", OnTime: "06:00"}
	c := &clock{local(t, "2026-11-15 01:00:00")} // schedule says off
	ctl := newController(t, s, c)
	ctl.TestOff()
	if st := ctl.Status(); st.PowerOn || st.Reason != ReasonTest {
		t.Fatalf("during the test: %+v", st)
	}
	c.t = c.t.Add(TestOffDuration)
	if st := ctl.Status(); st.PowerOn || st.Reason != ReasonSchedule {
		t.Errorf("after the test the schedule must apply again: %+v", st)
	}
}
