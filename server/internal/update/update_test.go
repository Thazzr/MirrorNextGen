package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func writeStatus(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, statusFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeRequest drops a request made at the fixed now.
func writeRequest(t *testing.T, dir string) { writeRequestAt(t, dir, now) }

func writeRequestAt(t *testing.T, dir string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, requestFile), []byte(at.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUnavailableWithoutDirectory(t *testing.T) {
	s := New("")
	if st := s.Status(now); st.Available {
		t.Errorf("status = %+v", st)
	}
	if _, err := s.Request(now); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestStatus(t *testing.T) {
	// Valid JSON padded with trailing spaces to exactly maxStatusBytes and one
	// byte more: a reader that merely truncates would still parse the latter.
	const body = `{"state":"failed","message":"m"}`
	pad := func(total int) string { return body + strings.Repeat(" ", total-len(body)) }
	big := pad(maxStatusBytes + 1)
	exact := pad(maxStatusBytes)
	tests := []struct {
		name      string
		content   string // "" = no file
		pending   bool
		wantState State
		wantMsg   string
	}{
		{"missing file is idle", "", false, StateIdle, ""},
		{"running", `{"state":"running","startedAt":"2026-11-14T11:55:00+01:00"}`, false, StateRunning, ""},
		{"succeeded", `{"state":"succeeded","startedAt":"2026-11-14T11:00:00+01:00","finishedAt":"2026-11-14T11:05:00+01:00","message":"Aktualisiert."}`, false, StateSucceeded, "Aktualisiert."},
		{"failed", `{"state":"failed","message":"pull failed"}`, false, StateFailed, "pull failed"},
		{"invalid json", `{not json`, false, StateUnknown, msgUnreadable},
		{"unknown state", `{"state":"exploded"}`, false, StateUnknown, msgUnreadable},
		{"idle is not a host state", `{"state":"idle"}`, false, StateUnknown, msgUnreadable},
		{"exactly 64 KiB is parsed", exact, false, StateFailed, "m"},
		{"64 KiB plus one byte is unknown", big, false, StateUnknown, msgUnreadable},
		{"pending with no status", "", true, StateIdle, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.content != "" {
				writeStatus(t, dir, tt.content)
			}
			if tt.pending {
				writeRequest(t, dir)
			}
			st := New(dir).Status(now)
			if !st.Available || st.State != tt.wantState || st.Message != tt.wantMsg || st.Pending != tt.pending {
				t.Errorf("status = %+v", st)
			}
		})
	}
}

func TestStatusParsesTimes(t *testing.T) {
	dir := t.TempDir()
	writeStatus(t, dir, `{"state":"succeeded","startedAt":"2026-11-14T11:00:00+01:00","finishedAt":"2026-11-14T11:05:00+01:00"}`)
	st := New(dir).Status(now)
	if st.StartedAt == nil || !st.StartedAt.Equal(time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("startedAt = %v", st.StartedAt)
	}
	if st.FinishedAt == nil || !st.FinishedAt.Equal(time.Date(2026, 11, 14, 10, 5, 0, 0, time.UTC)) {
		t.Errorf("finishedAt = %v", st.FinishedAt)
	}
}

func TestRequestWritesAtomicallyAndReportsPending(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir).Request(now)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Pending {
		t.Errorf("status = %+v", st)
	}
	b, err := os.ReadFile(filepath.Join(dir, requestFile))
	if err != nil || strings.TrimSpace(string(b)) != "2026-11-14T12:00:00Z" {
		t.Errorf("request = %q, %v", b, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("leftover files: %v", entries)
	}
}

func TestRequestRefusals(t *testing.T) {
	tests := []struct {
		name    string
		pending bool
		status  string
		wantErr error
	}{
		{"pending", true, "", ErrBusy},
		{"running young", false, `{"state":"running","startedAt":"2026-11-14T11:40:00Z"}`, ErrBusy},
		{"running without start time", false, `{"state":"running"}`, ErrBusy},
		{"running stale", false, `{"state":"running","startedAt":"2026-11-14T11:29:00Z"}`, nil},
		{"running exactly at the stale boundary", false, `{"state":"running","startedAt":"2026-11-14T11:30:00Z"}`, nil},
		{"running just under the stale boundary", false, `{"state":"running","startedAt":"2026-11-14T11:30:01Z"}`, ErrBusy},
		{"succeeded", false, `{"state":"succeeded","startedAt":"2026-11-14T11:55:00Z"}`, nil},
		{"failed", false, `{"state":"failed"}`, nil},
		{"unreadable status", false, `garbage`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.pending {
				writeRequest(t, dir)
			}
			if tt.status != "" {
				writeStatus(t, dir, tt.status)
			}
			_, err := New(dir).Request(now)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequestFileIsWorldReadableRFC3339(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir).Request(now); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, requestFile))
	if err != nil {
		t.Fatal(err)
	}
	// The host service runs as another user and must be able to read it.
	if info.Mode().Perm()&0o044 != 0o044 {
		t.Errorf("mode = %v", info.Mode())
	}
	b, _ := os.ReadFile(filepath.Join(dir, requestFile))
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b))); err != nil {
		t.Errorf("content %q: %v", b, err)
	}
}

func TestStalledRequest(t *testing.T) {
	tests := []struct {
		name        string
		age         time.Duration
		status      string
		wantStalled bool
		wantBlocked bool
	}{
		{"fresh", time.Minute, "", false, true},
		{"exactly two minutes", StalledAfter, "", false, true},
		{"just over two minutes", StalledAfter + time.Second, "", true, false},
		{"old but the host is running", time.Hour, `{"state":"running","startedAt":"2026-11-14T11:59:00Z"}`, false, true},
		{"old, last run failed", time.Hour, `{"state":"failed"}`, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRequestAt(t, dir, now.Add(-tt.age))
			if tt.status != "" {
				writeStatus(t, dir, tt.status)
			}
			st := New(dir).Status(now)
			if !st.Pending || st.Stalled != tt.wantStalled {
				t.Errorf("status = %+v", st)
			}
			if got := Blocked(st, now); got != tt.wantBlocked {
				t.Errorf("Blocked = %v, want %v", got, tt.wantBlocked)
			}
		})
	}
}

func TestStalledRequestIsOverwrittenByANewOne(t *testing.T) {
	dir := t.TempDir()
	writeRequestAt(t, dir, now.Add(-time.Hour))
	st, err := New(dir).Request(now)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Pending || st.Stalled {
		t.Errorf("status = %+v", st)
	}
	b, _ := os.ReadFile(filepath.Join(dir, requestFile))
	if strings.TrimSpace(string(b)) != "2026-11-14T12:00:00Z" {
		t.Errorf("request = %q", b)
	}
}

func TestStalledFallsBackToModTimeWithoutATimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, requestFile)
	if err := os.WriteFile(path, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if st := New(dir).Status(now); !st.Stalled {
		t.Errorf("status = %+v", st)
	}
}

func TestRequestIntoMissingDirectoryIsNotWritable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := New(missing).Request(now)
	if !errors.Is(err, ErrNotWritable) || !strings.Contains(err.Error(), missing) {
		t.Errorf("err = %v", err)
	}
}

func TestRequestIntoReadOnlyDirectoryIsNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions are not enforced for root")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := New(dir).Request(now); !errors.Is(err, ErrNotWritable) {
		t.Errorf("err = %v", err)
	}
}
