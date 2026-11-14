package kiosk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{URL: "http://mirror:8080/", PollInterval: 5 * time.Second, Rotation: "compositor", ExtraFlags: []string{}, User: "kiosk"}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadValues(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"KIOSK_URL":           "http://pi:9000",
		"KIOSK_POLL_INTERVAL": "2s",
		"KIOSK_DISABLE_GPU":   "true",
		"KIOSK_ROTATION":      "browser",
		"KIOSK_EXTRA_FLAGS":   "  --a=1   --b ",
		"KIOSK_USER":          "me",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{URL: "http://pi:9000/", PollInterval: 2 * time.Second, DisableGPU: true, Rotation: "browser",
		ExtraFlags: []string{"--a=1", "--b"}, User: "me"}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadErrorsNameVariable(t *testing.T) {
	tests := map[string]string{
		"KIOSK_URL":           "ftp://x",
		"KIOSK_POLL_INTERVAL": "500ms",
		"KIOSK_DISABLE_GPU":   "maybe",
		"KIOSK_ROTATION":      "sideways",
	}
	for name, value := range tests {
		_, err := Load(env(map[string]string{name: value}))
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s=%q: error %v does not name the variable", name, value, err)
		}
	}
	_, err := Load(env(map[string]string{"KIOSK_POLL_INTERVAL": "soon"}))
	if err == nil || !strings.Contains(err.Error(), "KIOSK_POLL_INTERVAL") {
		t.Errorf("unparsable duration: %v", err)
	}
}

func TestBrowserURL(t *testing.T) {
	if got := BrowserURL(Config{URL: "http://mirror:8080/", Rotation: "compositor"}); got != "http://mirror:8080/" {
		t.Errorf("compositor: %q", got)
	}
	if got := BrowserURL(Config{URL: "http://mirror:8080/", Rotation: "browser"}); got != "http://mirror:8080/?rotation=browser" {
		t.Errorf("browser: %q", got)
	}
}

func TestChromiumArgs(t *testing.T) {
	base := ChromiumArgs(Config{}, "http://x/")
	if base[0] != "--kiosk" || base[len(base)-1] != "http://x/" {
		t.Fatalf("unexpected framing: %v", base)
	}
	for _, a := range base {
		if a == "--disable-gpu" {
			t.Fatal("--disable-gpu set without configuration")
		}
	}
	args := ChromiumArgs(Config{DisableGPU: true, ExtraFlags: []string{"--foo"}}, "http://x/")
	n := len(args)
	if args[n-1] != "http://x/" || args[n-2] != "--foo" || args[n-3] != "--disable-gpu" {
		t.Fatalf("order wrong: %v", args[n-4:])
	}
}

func TestCageArgs(t *testing.T) {
	got := CageArgs([]string{"--kiosk", "http://x/"})
	want := []string{"-d", "-s", "-m", "last", "--", "chromium", "--kiosk", "http://x/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestTransform(t *testing.T) {
	for rot, want := range map[int]string{0: "normal", 90: "90", 180: "180", 270: "270"} {
		got, err := Transform(rot)
		if err != nil || got != want {
			t.Errorf("Transform(%d) = %q, %v", rot, got, err)
		}
	}
	for _, rot := range []int{45, -90, 360} {
		if _, err := Transform(rot); err == nil {
			t.Errorf("Transform(%d) accepted", rot)
		}
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPickOutput(t *testing.T) {
	single, two := fixture(t, "outputs_single.json"), fixture(t, "outputs_two.json")
	tests := []struct {
		name      string
		json      []byte
		preferred string
		last      string
		want      string
		wantErr   bool
	}{
		{"single auto", single, "", "", "HDMI-A-1", false},
		{"two auto picks enabled", two, "", "", "HDMI-A-2", false},
		{"preferred present", two, "HDMI-A-1", "", "HDMI-A-1", false},
		{"preferred missing", single, "DP-1", "", "", true},
		{"all disabled picks first", []byte(`[{"name":"A","enabled":false},{"name":"B","enabled":false}]`), "", "", "A", false},
		{"empty", []byte(`[]`), "", "", "", true},
		{"garbage", []byte(`nope`), "", "", "", true},
		{"last still listed but disabled", []byte(`[{"name":"A","enabled":false},{"name":"B","enabled":false}]`), "", "B", "B", false},
		{"last wins over another enabled output", []byte(`[{"name":"A","enabled":true},{"name":"B","enabled":false}]`), "", "B", "B", false},
		{"last no longer listed falls back to first enabled", []byte(`[{"name":"A","enabled":false},{"name":"B","enabled":true}]`), "", "C", "B", false},
		{"preferred wins over last", two, "HDMI-A-1", "HDMI-A-2", "HDMI-A-1", false},
	}
	for _, tt := range tests {
		got, err := PickOutput(tt.json, tt.preferred, tt.last)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: got %q, %v", tt.name, got, err)
		}
	}
}

func TestPlan(t *testing.T) {
	on := func(rot int) State { return State{Rotation: rot, PowerOn: true} }
	off := func(rot int) State { return State{Rotation: rot} }
	appliedOn := func(rot int, out string) *State { return &State{Rotation: rot, Output: out, PowerOn: true} }
	appliedOff := func(rot int, out string) *State { return &State{Rotation: rot, Output: out} }
	args := func(out string, rest ...string) []string { return append([]string{"--output", out}, rest...) }
	const hdmi = "HDMI-A-1"
	tests := []struct {
		name    string
		desired State
		applied *State
		output  string
		rotate  bool
		want    [][]string
	}{
		{"first run on", on(90), nil, hdmi, true, [][]string{args(hdmi, "--on"), args(hdmi, "--transform", "90")}},
		{"first run on normal", on(0), nil, hdmi, true, [][]string{args(hdmi, "--on"), args(hdmi, "--transform", "normal")}},
		{"first run off", off(90), nil, hdmi, true, [][]string{args(hdmi, "--off")}},
		{"unchanged on", on(90), appliedOn(90, hdmi), hdmi, true, nil},
		{"unchanged off", off(90), appliedOff(90, hdmi), hdmi, true, nil},
		{"rotation changed while on", on(180), appliedOn(90, hdmi), hdmi, true, [][]string{args(hdmi, "--transform", "180")}},
		{"output changed while on", on(90), appliedOn(90, hdmi), "HDMI-A-2", true, [][]string{args("HDMI-A-2", "--on"), args("HDMI-A-2", "--transform", "90")}},
		{"output changed while off", off(90), appliedOff(90, hdmi), "HDMI-A-2", true, [][]string{args("HDMI-A-2", "--off")}},
		{"power goes off", off(90), appliedOn(90, hdmi), hdmi, true, [][]string{args(hdmi, "--off")}},
		{"power goes on re-applies the unchanged transform", on(90), appliedOff(90, hdmi), hdmi, true, [][]string{args(hdmi, "--on"), args(hdmi, "--transform", "90")}},
		{"rotation changed while off sends nothing", off(180), appliedOff(90, hdmi), hdmi, true, nil},
		{"remembered rotation is applied when power goes on", on(180), appliedOff(90, hdmi), hdmi, true, [][]string{args(hdmi, "--on"), args(hdmi, "--transform", "180")}},
		{"power and rotation change together to off", off(180), appliedOn(90, hdmi), hdmi, true, [][]string{args(hdmi, "--off")}},
		{"invalid rotation", on(45), nil, hdmi, true, nil},
		{"invalid rotation does not matter without rotation", on(45), nil, hdmi, false, [][]string{args(hdmi, "--on")}},
		{"browser mode first run on", on(90), nil, hdmi, false, [][]string{args(hdmi, "--on")}},
		{"browser mode first run off", off(90), nil, hdmi, false, [][]string{args(hdmi, "--off")}},
		{"browser mode power goes off", off(90), appliedOn(90, hdmi), hdmi, false, [][]string{args(hdmi, "--off")}},
		{"browser mode power goes on without transform", on(90), appliedOff(90, hdmi), hdmi, false, [][]string{args(hdmi, "--on")}},
		{"browser mode rotation change is ignored", on(180), appliedOn(90, hdmi), hdmi, false, nil},
	}
	for _, tt := range tests {
		if got := Plan(tt.desired, tt.applied, tt.output, tt.rotate); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFetchState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/kiosk/state" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"rotation":270,"output":"HDMI-A-2","testPattern":true,"powerOn":true}`)
	}))
	defer srv.Close()
	got, err := FetchState(context.Background(), srv.Client(), srv.URL+"/")
	if err != nil || got != (State{Rotation: 270, Output: "HDMI-A-2", PowerOn: true}) {
		t.Fatalf("got %+v, %v", got, err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) }))
	defer bad.Close()
	if _, err := FetchState(context.Background(), bad.Client(), bad.URL+"/"); err == nil {
		t.Fatal("500 accepted")
	}
}

func TestFetchStatePowerOn(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"powerOn false", `{"rotation":0,"output":"","powerOn":false}`, false},
		{"powerOn true", `{"rotation":0,"output":"","powerOn":true}`, true},
		{"a server without the field means on", `{"rotation":0,"output":"","testPattern":false}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tt.body) }))
			defer srv.Close()
			got, err := FetchState(context.Background(), srv.Client(), srv.URL+"/")
			if err != nil || got.PowerOn != tt.want {
				t.Fatalf("got %+v, %v; want PowerOn %v", got, err, tt.want)
			}
		})
	}
}

func TestWaitHealthy(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) < 3 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := WaitHealthy(context.Background(), srv.Client(), srv.URL+"/", time.Millisecond, time.Hour, log); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("health called %d times, want 3", calls.Load())
	}
}

func TestWaitHealthyStopsWithContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 503) }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := WaitHealthy(ctx, srv.Client(), srv.URL+"/", time.Millisecond, time.Hour, log); err == nil {
		t.Fatal("expected context error")
	}
}

type fakeCmd struct {
	mu          sync.Mutex
	outputsJSON []byte
	failApply   bool
	calls       [][]string
}

func (f *fakeCmd) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{name}, args...))
	if len(args) == 1 && args[0] == "--json" {
		return f.outputsJSON, nil
	}
	if f.failApply {
		return nil, errors.New("boom")
	}
	return nil, nil
}

func (f *fakeCmd) applies() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if len(c) > 1 && c[1] == "--output" {
			n++
		}
	}
	return n
}

func newAgent(state *State, fetchErr *error, cmd *fakeCmd) *Agent {
	return &Agent{
		Fetch: func(context.Context) (State, error) {
			if *fetchErr != nil {
				return State{}, *fetchErr
			}
			return *state, nil
		},
		Commander: cmd,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestAgentTick(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json")}
	state := State{Rotation: 90, PowerOn: true}
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	ctx := context.Background()

	// The first pass switches the output on and then rotates it.
	if err := a.Tick(ctx); err != nil || cmd.applies() != 2 {
		t.Fatalf("first tick: err=%v applies=%d", err, cmd.applies())
	}
	last := cmd.calls[len(cmd.calls)-1]
	if !reflect.DeepEqual(last, []string{"wlr-randr", "--output", "HDMI-A-1", "--transform", "90"}) {
		t.Fatalf("unexpected command %v", last)
	}
	if err := a.Tick(ctx); err != nil || cmd.applies() != 2 {
		t.Fatalf("unchanged tick re-applied: err=%v applies=%d", err, cmd.applies())
	}

	// A fetch error keeps the applied state and applies nothing.
	fetchErr = errors.New("down")
	if err := a.Tick(ctx); err == nil || cmd.applies() != 2 {
		t.Fatalf("fetch error: err=%v applies=%d", err, cmd.applies())
	}
	fetchErr = nil
	if err := a.Tick(ctx); err != nil || cmd.applies() != 2 {
		t.Fatalf("recovery re-applied the unchanged state: err=%v applies=%d", err, cmd.applies())
	}

	state.Rotation = 180
	if err := a.Tick(ctx); err != nil || cmd.applies() != 3 {
		t.Fatalf("change tick: err=%v applies=%d", err, cmd.applies())
	}
}

func TestAgentRetriesAfterFailedApply(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json"), failApply: true}
	state := State{Rotation: 270, PowerOn: true}
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	if err := a.Tick(context.Background()); err == nil {
		t.Fatal("failed apply reported success")
	}
	cmd.failApply = false
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	// One failed command, then the whole plan again (--on, --transform).
	if cmd.applies() != 3 {
		t.Fatalf("applies = %d, want a retry (3)", cmd.applies())
	}
	if err := a.Tick(context.Background()); err != nil || cmd.applies() != 3 {
		t.Fatalf("applied state not remembered: err=%v applies=%d", err, cmd.applies())
	}
}

func TestAgentRejectsInvalidRotation(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json")}
	state := State{Rotation: 45, PowerOn: true}
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	if err := a.Tick(context.Background()); err == nil || cmd.applies() != 0 {
		t.Fatalf("err=%v applies=%d", err, cmd.applies())
	}
}

func TestAgentRunFollowsTicker(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json")}
	var mu sync.Mutex
	rotation := 90
	a := &Agent{
		Fetch: func(context.Context) (State, error) {
			mu.Lock()
			defer mu.Unlock()
			return State{Rotation: rotation, PowerOn: true}, nil
		},
		Commander: cmd,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx, ticks); close(done) }()

	waitFor := func(n int) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for cmd.applies() < n {
			if time.Now().After(deadline) {
				t.Fatalf("applies stuck below %d", n)
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitFor(2) // immediate first pass: --on and --transform
	mu.Lock()
	rotation = 180
	mu.Unlock()
	ticks <- time.Time{}
	waitFor(3)
	cancel()
	<-done
}

// takeApplies returns the wlr-randr arguments (without "--json" listings) of
// every apply so far and forgets them.
func (f *fakeCmd) takeApplies() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out [][]string
	for _, c := range f.calls {
		if len(c) > 1 && c[1] == "--output" {
			out = append(out, c[1:])
		}
	}
	f.calls = nil
	return out
}

type step struct {
	name  string
	state State
	err   error // fetch error
	want  [][]string
}

func runSteps(t *testing.T, skipRotation bool, steps []step) {
	t.Helper()
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json")}
	var state State
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	a.SkipRotation = skipRotation
	for _, st := range steps {
		state, fetchErr = st.state, st.err
		err := a.Tick(context.Background())
		if (err != nil) != (st.err != nil) {
			t.Fatalf("%s: err = %v", st.name, err)
		}
		if got := cmd.takeApplies(); !reflect.DeepEqual(got, st.want) {
			t.Errorf("%s: commands %v, want %v", st.name, got, st.want)
		}
	}
}

func TestAgentPowerTransitions(t *testing.T) {
	const o = "HDMI-A-1"
	runSteps(t, false, []step{
		{"first run while off", State{Rotation: 90}, nil, [][]string{{"--output", o, "--off"}}},
		{"still off", State{Rotation: 90}, nil, nil},
		{"rotation changes while off", State{Rotation: 180}, nil, nil},
		{"fetch error while off changes nothing", State{}, errors.New("down"), nil},
		{"on applies the remembered rotation", State{Rotation: 180, PowerOn: true}, nil, [][]string{{"--output", o, "--on"}, {"--output", o, "--transform", "180"}}},
		{"still on", State{Rotation: 180, PowerOn: true}, nil, nil},
		{"rotation changes while on", State{Rotation: 270, PowerOn: true}, nil, [][]string{{"--output", o, "--transform", "270"}}},
		{"off", State{Rotation: 270}, nil, [][]string{{"--output", o, "--off"}}},
		{"on again re-applies the same rotation", State{Rotation: 270, PowerOn: true}, nil, [][]string{{"--output", o, "--on"}, {"--output", o, "--transform", "270"}}},
	})
}

func TestAgentPowerTransitionsInBrowserRotationMode(t *testing.T) {
	const o = "HDMI-A-1"
	runSteps(t, true, []step{
		{"first run on", State{Rotation: 90, PowerOn: true}, nil, [][]string{{"--output", o, "--on"}}},
		{"rotation changes are the dashboard's business", State{Rotation: 180, PowerOn: true}, nil, nil},
		{"off", State{Rotation: 180}, nil, [][]string{{"--output", o, "--off"}}},
		{"on without a transform", State{Rotation: 180, PowerOn: true}, nil, [][]string{{"--output", o, "--on"}}},
		{"an unsupported rotation does not matter", State{Rotation: 45, PowerOn: true}, nil, nil},
	})
}

func TestAgentRetriesFailedPowerOn(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json")}
	state := State{Rotation: 90}
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	cmd.takeApplies()
	state.PowerOn = true
	cmd.failApply = true
	if err := a.Tick(context.Background()); err == nil {
		t.Fatal("failed --on reported success")
	}
	cmd.takeApplies()
	cmd.failApply = false
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"--output", "HDMI-A-1", "--on"}, {"--output", "HDMI-A-1", "--transform", "90"}}
	if got := cmd.takeApplies(); !reflect.DeepEqual(got, want) {
		t.Errorf("retry commands %v, want %v", got, want)
	}
}

// stateCmd is a commander whose `--json` answer follows the on/off commands it
// has seen, like wlr-randr does: an output switched off stays listed as
// disabled.
type stateCmd struct {
	order   []string
	enabled map[string]bool
	applied [][]string
}

func (c *stateCmd) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	if len(args) == 1 && args[0] == "--json" {
		var parts []string
		for _, n := range c.order {
			parts = append(parts, fmt.Sprintf(`{"name":%q,"enabled":%t}`, n, c.enabled[n]))
		}
		return []byte("[" + strings.Join(parts, ",") + "]"), nil
	}
	c.applied = append(c.applied, args)
	switch args[2] {
	case "--off":
		c.enabled[args[1]] = false
	case "--on":
		c.enabled[args[1]] = true
	}
	return nil, nil
}

func TestAgentKeepsOutputWhileMonitorIsOff(t *testing.T) {
	cmd := &stateCmd{order: []string{"HDMI-A-1", "HDMI-A-2"}, enabled: map[string]bool{"HDMI-A-2": true}}
	state := State{Rotation: 90, PowerOn: true}
	var fetchErr error
	a := &Agent{
		Fetch:     func(context.Context) (State, error) { return state, fetchErr },
		Commander: cmd,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx := context.Background()
	tick := func(s State) {
		t.Helper()
		state = s
		if err := a.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	tick(State{Rotation: 90, PowerOn: true})
	tick(State{Rotation: 90})
	tick(State{Rotation: 90}) // a further tick while off must not wander to HDMI-A-1
	tick(State{Rotation: 90, PowerOn: true})

	want := [][]string{
		{"--output", "HDMI-A-2", "--on"},
		{"--output", "HDMI-A-2", "--transform", "90"},
		{"--output", "HDMI-A-2", "--off"},
		{"--output", "HDMI-A-2", "--on"},
		{"--output", "HDMI-A-2", "--transform", "90"},
	}
	if !reflect.DeepEqual(cmd.applied, want) {
		t.Fatalf("commands %v, want %v", cmd.applied, want)
	}
}
