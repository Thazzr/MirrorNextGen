// Package update is the mirror's side of the update button. The mirror
// container never updates itself: it drops a request file into a directory it
// shares with the host, a systemd path unit on the host runs the updater
// script, and the script reports progress in a status file next to it
// (deploy/host/mirror-update.sh). No command is run from here.
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/fsutil"
)

const (
	requestFile = "update-request"
	statusFile  = "update-status.json"

	// maxStatusBytes bounds the status file read; the host script writes a few
	// hundred bytes (a five-line log tail).
	maxStatusBytes = 64 << 10
	// StaleRunning is how long a "running" status blocks a new request. A
	// host script that crashed leaves "running" behind forever otherwise.
	StaleRunning = 30 * time.Minute
	// StalledAfter is how long a request may wait for the host's updater
	// before it counts as not picked up (service not installed).
	StalledAfter = 2 * time.Minute

	msgUnreadable = "Der Status des Updates ist nicht lesbar."
)

// State is the progress of the updater as the host reported it.
type State string

// The states of an update.
const (
	StateIdle      State = "idle"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateUnknown   State = "unknown"
)

// ErrUnavailable means no control directory is configured.
var ErrUnavailable = errors.New("updates by button are not set up")

// ErrBusy means an update is already requested or running.
var ErrBusy = errors.New("an update is already pending or running")

// ErrNotWritable means the control directory is missing or not writable.
var ErrNotWritable = errors.New("control directory is not writable")

// Status is what the admin sees.
type Status struct {
	Available bool
	Pending   bool
	// Stalled: a request has waited longer than StalledAfter and the host is
	// not running an update. It no longer blocks new requests.
	Stalled    bool
	State      State
	StartedAt  *time.Time
	FinishedAt *time.Time
	Message    string
}

// Service reads and writes the control directory. An empty directory means
// the feature is unavailable.
type Service struct {
	dir string
}

// New returns a Service for the control directory dir ("" = unavailable).
func New(dir string) *Service { return &Service{dir: dir} }

// Available reports whether a control directory is configured.
func (s *Service) Available() bool { return s != nil && s.dir != "" }

// hostStatus is the JSON the host script writes.
type hostStatus struct {
	State      string `json:"state"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Message    string `json:"message"`
}

// Status reads the host's status file and the request marker.
func (s *Service) Status(now time.Time) Status {
	if !s.Available() {
		return Status{State: StateIdle}
	}
	st := s.readHostStatus()
	if reqAt, ok := s.requestTime(); ok {
		st.Pending = true
		st.Stalled = st.State != StateRunning && now.Sub(reqAt) > StalledAfter
	}
	return st
}

// requestTime reports whether a request file exists and when it was made: the
// timestamp in its content, else the file's modification time.
func (s *Service) requestTime() (time.Time, bool) {
	path := filepath.Join(s.dir, requestFile)
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	if b, err := os.ReadFile(path); err == nil && len(b) <= 256 {
		if t := parseTime(strings.TrimSpace(string(b))); t != nil {
			return *t, true
		}
	}
	return info.ModTime(), true
}

func (s *Service) readHostStatus() Status {
	st := Status{Available: true, State: StateIdle}
	f, err := os.Open(filepath.Join(s.dir, statusFile))
	if errors.Is(err, os.ErrNotExist) {
		return st
	}
	if err != nil {
		return unknown(st)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxStatusBytes+1))
	if err != nil || len(data) > maxStatusBytes {
		return unknown(st)
	}
	var hs hostStatus
	if err := json.Unmarshal(data, &hs); err != nil {
		return unknown(st)
	}
	switch State(hs.State) {
	case StateRunning, StateSucceeded, StateFailed:
		st.State = State(hs.State)
	default:
		return unknown(st)
	}
	st.StartedAt = parseTime(hs.StartedAt)
	st.FinishedAt = parseTime(hs.FinishedAt)
	st.Message = hs.Message
	return st
}

func unknown(st Status) Status {
	st.State = StateUnknown
	st.Message = msgUnreadable
	return st
}

func parseTime(v string) *time.Time {
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	return &t
}

// Blocked reports whether a new request must be refused at now: one is
// pending and not stalled, or the host reports "running" since less than StaleRunning.
func Blocked(st Status, now time.Time) bool {
	if st.Pending && !st.Stalled {
		return true
	}
	if st.State != StateRunning {
		return false
	}
	// Without a start time the age is unknown; trust "running" rather than
	// start a second update on top of it.
	return st.StartedAt == nil || now.Sub(*st.StartedAt) < StaleRunning
}

// Request asks the host to update. It returns ErrUnavailable without a
// control directory and ErrBusy while an update is pending or running.
func (s *Service) Request(now time.Time) (Status, error) {
	if !s.Available() {
		return Status{}, ErrUnavailable
	}
	if st := s.Status(now); Blocked(st, now) {
		return st, ErrBusy
	}
	content := []byte(now.Format(time.RFC3339) + "\n")
	// World-readable: the host service runs as another user.
	if err := fsutil.WriteFileAtomic(filepath.Join(s.dir, requestFile), content, 0o644); err != nil {
		return Status{}, fmt.Errorf("write update request in %s: %w: %w", s.dir, ErrNotWritable, err)
	}
	return s.Status(now), nil
}
