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

	"github.com/thazzr/mirrornextgen/server/internal/kiosk"
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

func TestCursorEnv(t *testing.T) {
	log := quietLog()
	dir := filepath.Join(t.TempDir(), "cursors")

	got := cursorEnv(log, kiosk.Config{}, dir)
	want := []string{"XCURSOR_PATH=" + dir, "XCURSOR_THEME=default", "XCURSOR_SIZE=24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default: got %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "default", "cursors", "default")); err != nil {
		t.Errorf("theme not written: %v", err)
	}

	otherDir := filepath.Join(t.TempDir(), "cursors")
	if got := cursorEnv(log, kiosk.Config{ShowCursor: true}, otherDir); len(got) != 0 {
		t.Errorf("ShowCursor: got %v, want none", got)
	}
	if _, err := os.Stat(otherDir); err == nil {
		t.Error("ShowCursor: theme was written anyway")
	}
}

func TestCursorEnvWriteFailureWarnsAndContinues(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	if got := cursorEnv(log, kiosk.Config{}, filepath.Join(file, "sub")); len(got) != 0 {
		t.Errorf("got %v, want no environment", got)
	}
	if !strings.Contains(buf.String(), `"level":"WARN"`) {
		t.Errorf("no warning logged: %s", buf.String())
	}
}

func TestStartProcPassesExtraEnv(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	p, err := startProc(log, "envtest", nil, []string{"XCURSOR_THEME=default"}, "env")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("env did not exit")
	}
	if !strings.Contains(buf.String(), "XCURSOR_THEME=default") {
		t.Errorf("extra env missing from child output: %s", buf.String())
	}
}
