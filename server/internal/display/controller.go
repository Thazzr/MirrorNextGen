package display

import (
	"sync"
	"time"
)

// TestOffDuration is how long the admin's test button keeps the monitor off.
const TestOffDuration = 10 * time.Second

// Reason says why the monitor is in its current state.
type Reason string

// Reasons of a Status.
const (
	ReasonSchedule Reason = "schedule"
	ReasonTest     Reason = "test"
	ReasonAlways   Reason = "always"
)

// Status is the monitor state at one moment.
type Status struct {
	PowerOn      bool
	Reason       Reason
	NextSwitchAt *time.Time // nil without an effective schedule
	NextSwitchOn bool
	Now          time.Time // in the controller's location
	TimeZone     string
}

// Options configure a Controller.
type Options struct {
	// Schedule returns the stored night schedule.
	Schedule func() Schedule
	Now      func() time.Time
	Location *time.Location
}

// Controller combines the stored schedule with the in-memory test override.
// It is safe for concurrent use.
type Controller struct {
	opts Options

	mu           sync.Mutex
	testOffUntil time.Time
}

// NewController returns a Controller; a nil Now or Location means time.Now and UTC.
func NewController(o Options) *Controller {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Location == nil {
		o.Location = time.UTC
	}
	return &Controller{opts: o}
}

// TestOff switches the monitor off for TestOffDuration from now; a second
// call restarts the period. It returns the status right after the call.
func (c *Controller) TestOff() Status {
	now := c.opts.Now()
	c.mu.Lock()
	c.testOffUntil = now.Add(TestOffDuration)
	c.mu.Unlock()
	return c.Status()
}

// PowerOn reports whether the monitor should be on now.
func (c *Controller) PowerOn() bool { return c.Status().PowerOn }

// Status describes the monitor state now.
func (c *Controller) Status() Status {
	now := c.opts.Now().In(c.opts.Location)
	sched := c.opts.Schedule()
	st := Status{PowerOn: true, Reason: ReasonAlways, Now: now, TimeZone: c.opts.Location.String()}
	if sched.Effective() {
		st.Reason = ReasonSchedule
		st.PowerOn = PowerOn(sched, now)
		if at, on, ok := NextSwitch(sched, now); ok {
			st.NextSwitchAt = &at
			st.NextSwitchOn = on
		}
	}
	c.mu.Lock()
	testing := now.Before(c.testOffUntil)
	c.mu.Unlock()
	if testing {
		st.PowerOn = false
		st.Reason = ReasonTest
	}
	return st
}
