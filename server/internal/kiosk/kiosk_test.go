package kiosk

import (
	"context"
	"errors"
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
		want      string
		wantErr   bool
	}{
		{"single auto", single, "", "HDMI-A-1", false},
		{"two auto picks enabled", two, "", "HDMI-A-2", false},
		{"preferred present", two, "HDMI-A-1", "HDMI-A-1", false},
		{"preferred missing", single, "DP-1", "", true},
		{"all disabled picks first", []byte(`[{"name":"A","enabled":false},{"name":"B","enabled":false}]`), "", "A", false},
		{"empty", []byte(`[]`), "", "", true},
		{"garbage", []byte(`nope`), "", "", true},
	}
	for _, tt := range tests {
		got, err := PickOutput(tt.json, tt.preferred)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: got %q, %v", tt.name, got, err)
		}
	}
}

func TestPlan(t *testing.T) {
	tests := []struct {
		name    string
		desired State
		applied *State
		output  string
		want    [][]string
	}{
		{"first run", State{Rotation: 90}, nil, "HDMI-A-1", [][]string{{"--output", "HDMI-A-1", "--transform", "90"}}},
		{"first run normal", State{Rotation: 0}, nil, "HDMI-A-1", [][]string{{"--output", "HDMI-A-1", "--transform", "normal"}}},
		{"unchanged", State{Rotation: 90}, &State{Rotation: 90, Output: "HDMI-A-1"}, "HDMI-A-1", nil},
		{"rotation changed", State{Rotation: 180}, &State{Rotation: 90, Output: "HDMI-A-1"}, "HDMI-A-1", [][]string{{"--output", "HDMI-A-1", "--transform", "180"}}},
		{"output changed", State{Rotation: 90}, &State{Rotation: 90, Output: "HDMI-A-1"}, "HDMI-A-2", [][]string{{"--output", "HDMI-A-2", "--transform", "90"}}},
		{"invalid rotation", State{Rotation: 45}, nil, "HDMI-A-1", nil},
	}
	for _, tt := range tests {
		if got := Plan(tt.desired, tt.applied, tt.output); !reflect.DeepEqual(got, tt.want) {
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
		_, _ = io.WriteString(w, `{"rotation":270,"output":"HDMI-A-2","testPattern":true}`)
	}))
	defer srv.Close()
	got, err := FetchState(context.Background(), srv.Client(), srv.URL+"/")
	if err != nil || got != (State{Rotation: 270, Output: "HDMI-A-2"}) {
		t.Fatalf("got %+v, %v", got, err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) }))
	defer bad.Close()
	if _, err := FetchState(context.Background(), bad.Client(), bad.URL+"/"); err == nil {
		t.Fatal("500 accepted")
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
	state := State{Rotation: 90}
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	ctx := context.Background()

	if err := a.Tick(ctx); err != nil || cmd.applies() != 1 {
		t.Fatalf("first tick: err=%v applies=%d", err, cmd.applies())
	}
	last := cmd.calls[len(cmd.calls)-1]
	if !reflect.DeepEqual(last, []string{"wlr-randr", "--output", "HDMI-A-1", "--transform", "90"}) {
		t.Fatalf("unexpected command %v", last)
	}
	if err := a.Tick(ctx); err != nil || cmd.applies() != 1 {
		t.Fatalf("unchanged tick re-applied: err=%v applies=%d", err, cmd.applies())
	}

	// A fetch error keeps the applied state and applies nothing.
	fetchErr = errors.New("down")
	if err := a.Tick(ctx); err == nil || cmd.applies() != 1 {
		t.Fatalf("fetch error: err=%v applies=%d", err, cmd.applies())
	}
	fetchErr = nil
	if err := a.Tick(ctx); err != nil || cmd.applies() != 1 {
		t.Fatalf("recovery re-applied the unchanged state: err=%v applies=%d", err, cmd.applies())
	}

	state.Rotation = 180
	if err := a.Tick(ctx); err != nil || cmd.applies() != 2 {
		t.Fatalf("change tick: err=%v applies=%d", err, cmd.applies())
	}
}

func TestAgentRetriesAfterFailedApply(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json"), failApply: true}
	state := State{Rotation: 270}
	var fetchErr error
	a := newAgent(&state, &fetchErr, cmd)
	if err := a.Tick(context.Background()); err == nil {
		t.Fatal("failed apply reported success")
	}
	cmd.failApply = false
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cmd.applies() != 2 {
		t.Fatalf("applies = %d, want a retry (2)", cmd.applies())
	}
	if err := a.Tick(context.Background()); err != nil || cmd.applies() != 2 {
		t.Fatalf("applied state not remembered: err=%v applies=%d", err, cmd.applies())
	}
}

func TestAgentRejectsInvalidRotation(t *testing.T) {
	cmd := &fakeCmd{outputsJSON: fixture(t, "outputs_single.json")}
	state := State{Rotation: 45}
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
			return State{Rotation: rotation}, nil
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
	waitFor(1) // immediate first pass
	mu.Lock()
	rotation = 180
	mu.Unlock()
	ticks <- time.Time{}
	waitFor(2)
	cancel()
	<-done
}
