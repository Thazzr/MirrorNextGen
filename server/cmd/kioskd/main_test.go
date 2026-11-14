package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProc struct {
	name   string
	events *[]string
	mu     *sync.Mutex
	done   chan struct{}
	once   sync.Once
	// exitsOnTerminate false simulates a process that ignores SIGTERM.
	exitsOnTerminate bool
}

func newFake(name string, events *[]string, mu *sync.Mutex, exitsOnTerminate bool) *fakeProc {
	return &fakeProc{name: name, events: events, mu: mu, done: make(chan struct{}), exitsOnTerminate: exitsOnTerminate}
}

func (f *fakeProc) Name() string          { return f.name }
func (f *fakeProc) Done() <-chan struct{} { return f.done }
func (f *fakeProc) record(s string) {
	f.mu.Lock()
	*f.events = append(*f.events, s)
	f.mu.Unlock()
}
func (f *fakeProc) exit() { f.once.Do(func() { close(f.done) }) }
func (f *fakeProc) Terminate() {
	f.record("term " + f.name)
	if f.exitsOnTerminate {
		f.exit()
	}
}
func (f *fakeProc) Kill() { f.record("kill " + f.name); f.exit() }

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestSuperviseShutdownOrder(t *testing.T) {
	var events []string
	var mu sync.Mutex
	cage := newFake("cage", &events, &mu, true)
	seatd := newFake("seatd", &events, &mu, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if code := supervise(ctx, quietLog(), cage, seatd); code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
	if got := strings.Join(events, ","); got != "term cage,term seatd" {
		t.Fatalf("order: %s", got)
	}
}

func TestSuperviseChildExitIsFailure(t *testing.T) {
	for _, dead := range []string{"cage", "seatd"} {
		var events []string
		var mu sync.Mutex
		cage := newFake("cage", &events, &mu, true)
		seatd := newFake("seatd", &events, &mu, true)
		if dead == "cage" {
			cage.exit()
		} else {
			seatd.exit()
		}
		if code := supervise(context.Background(), quietLog(), cage, seatd); code != 1 {
			t.Errorf("%s died: exit code %d, want 1", dead, code)
		}
		// The survivor must be stopped, the dead one left alone.
		got := strings.Join(events, ",")
		want := "term seatd"
		if dead == "seatd" {
			want = "term cage"
		}
		if got != want {
			t.Errorf("%s died: events %q, want %q", dead, got, want)
		}
	}
}

func TestStopProcKillsAfterGrace(t *testing.T) {
	var events []string
	var mu sync.Mutex
	p := newFake("cage", &events, &mu, false)
	finished := make(chan struct{})
	go func() {
		stopProc(p, 20*time.Millisecond, quietLog())
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("stopProc did not escalate to SIGKILL after the grace period")
	}
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(events, ","); got != "term cage,kill cage" {
		t.Fatalf("events: %s", got)
	}
}

func TestBaseEnv(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("WLR_RENDERER", "")
	t.Setenv("WLR_DRM_DEVICES", "")
	t.Setenv("TZ", "")
	got := baseEnv()
	want := []string{
		"PATH=/usr/bin:/bin",
		"XDG_RUNTIME_DIR=/run/kiosk",
		"LIBSEAT_BACKEND=seatd",
		"WLR_LIBINPUT_NO_DEVICES=1",
		"HOME=/tmp",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty optionals: got %v, want %v", got, want)
	}

	t.Setenv("WLR_RENDERER", "pixman")
	t.Setenv("WLR_DRM_DEVICES", "/dev/dri/card1")
	t.Setenv("TZ", "Europe/Berlin")
	got = baseEnv()
	for _, e := range []string{"WLR_RENDERER=pixman", "WLR_DRM_DEVICES=/dev/dri/card1", "TZ=Europe/Berlin"} {
		if !slices.Contains(got, e) {
			t.Errorf("%s missing from %v", e, got)
		}
	}
}

func TestWaitForPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sock")
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(path, nil, 0o600)
	}()
	if err := waitForPath(context.Background(), path, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := waitForPath(context.Background(), filepath.Join(t.TempDir(), "never"), 150*time.Millisecond); err == nil {
		t.Fatal("missing path reported as present")
	}
}

func TestLineLogger(t *testing.T) {
	var out bytes.Buffer
	l := &lineLogger{log: slog.New(slog.NewTextHandler(&out, nil)), name: "cage"}
	_, _ = l.Write([]byte("first\nsec"))
	_, _ = l.Write([]byte("ond\n\n"))
	got := out.String()
	if strings.Count(got, "process=cage") != 2 || !strings.Contains(got, "msg=second") {
		t.Fatalf("unexpected log:\n%s", got)
	}
}

func TestRunRejectsBadConfig(t *testing.T) {
	env := func(k string) string {
		if k == "KIOSK_ROTATION" {
			return "bogus"
		}
		return ""
	}
	if code := run(context.Background(), env, io.Discard); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
}
