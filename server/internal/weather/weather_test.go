package weather

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const testKey = "k3y-SECRET-9876"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// upstream is a fake OpenWeather. Each endpoint can be overridden per test.
type upstream struct {
	t      *testing.T
	server *httptest.Server

	mu        sync.Mutex
	calls     map[string]int
	queries   map[string][]map[string]string
	geocode   func(w http.ResponseWriter, r *http.Request)
	oneCall   func(w http.ResponseWriter, r *http.Request)
	timeMach  func(w http.ResponseWriter, r *http.Request)
	timeCalls []string // dt values
}

func newUpstream(t *testing.T) *upstream {
	u := &upstream{t: t, calls: map[string]int{}, queries: map[string][]map[string]string{}}
	serve := func(name string) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture(t, name))
		}
	}
	u.geocode = serve("geocode_berlin.json")
	u.oneCall = serve("onecall_berlin.json")
	u.timeMach = serve("timemachine_clear.json")
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := map[string]string{}
		for k, v := range r.URL.Query() {
			q[k] = v[0]
		}
		u.mu.Lock()
		u.calls[r.URL.Path]++
		u.queries[r.URL.Path] = append(u.queries[r.URL.Path], q)
		if r.URL.Path == "/data/3.0/onecall/timemachine" {
			u.timeCalls = append(u.timeCalls, q["dt"])
		}
		geocode, oneCall, timeMach := u.geocode, u.oneCall, u.timeMach
		u.mu.Unlock()
		switch r.URL.Path {
		case "/geo/1.0/direct":
			geocode(w, r)
		case "/data/3.0/onecall":
			oneCall(w, r)
		case "/data/3.0/onecall/timemachine":
			timeMach(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (u *upstream) count(path string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls[path]
}

func (u *upstream) set(f func(u *upstream)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	f(u)
}

func status(code int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = io.WriteString(w, `{"cod":`+http.StatusText(code)+`}`)
	}
}

const (
	pathGeo  = "/geo/1.0/direct"
	pathOne  = "/data/3.0/onecall"
	pathTime = "/data/3.0/onecall/timemachine"
)

type env struct {
	svc   *Service
	up    *upstream
	clock *fakeClock
	log   *bytes.Buffer
	logMu *sync.Mutex

	cfgMu sync.Mutex
	cfg   Config
}

func (e *env) setCfg(f func(c *Config)) {
	e.cfgMu.Lock()
	defer e.cfgMu.Unlock()
	f(&e.cfg)
}

type lockedWriter struct {
	mu *sync.Mutex
	b  *bytes.Buffer
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (e *env) logText() string {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	return e.log.String()
}

// newEnv starts at 2026-11-14 12:00 Europe/Berlin, the instant the fixtures
// were written for.
func newEnv(t *testing.T) *env {
	t.Helper()
	loc := berlin(t)
	e := &env{
		up:    newUpstream(t),
		clock: &fakeClock{t: time.Date(2026, 11, 14, 12, 0, 0, 0, loc)},
		log:   &bytes.Buffer{},
		logMu: &sync.Mutex{},
		cfg:   Config{Location: "Berlin", APIKey: testKey, ShowIcons: true, RefreshMinutes: 15},
	}
	logger := slog.New(slog.NewTextHandler(lockedWriter{e.logMu, e.log}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e.svc = New(Options{
		HTTPClient: e.up.server.Client(),
		BaseURL:    e.up.server.URL,
		Config: func() Config {
			e.cfgMu.Lock()
			defer e.cfgMu.Unlock()
			return e.cfg
		},
		Now:      e.clock.now,
		Location: loc,
		Logger:   logger,
	})
	return e
}

func TestDisabledWithoutLocationOrKey(t *testing.T) {
	for name, mod := range map[string]func(c *Config){
		"no location": func(c *Config) { c.Location = "" },
		"no key":      func(c *Config) { c.APIKey = "" },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.setCfg(mod)
			got := e.svc.Get(context.Background())
			if got.Configured || got.Data != nil || got.Error != "" || got.Stale {
				t.Errorf("result = %+v", got)
			}
			if n := e.up.count(pathGeo) + e.up.count(pathOne); n != 0 {
				t.Errorf("%d upstream calls while disabled", n)
			}
		})
	}
}

func TestSuccessfulRefresh(t *testing.T) {
	e := newEnv(t)
	got := e.svc.Get(context.Background())
	if !got.Configured || got.Stale || got.Error != "" || got.Data == nil || !got.ShowIcons {
		t.Fatalf("result = %+v", got)
	}
	d := got.Data
	if d.Location != "Berlin" || d.TemperatureC != 8 || d.FeelsLikeC != 6 || d.Humidity != 81 {
		t.Errorf("current values: %+v", d)
	}
	if d.Condition != "Überwiegend bewölkt" || d.IconCode != "04d" || d.WindKph != 12 || d.UVIndex != 0 {
		t.Errorf("condition/wind: %+v", d)
	}
	if !d.UpdatedAt.Equal(e.clock.now()) {
		t.Errorf("updatedAt = %v", d.UpdatedAt)
	}
	if d.Sunrise == nil || d.Sunrise.Format(time.RFC3339) != "2026-11-14T07:33:00+01:00" {
		t.Errorf("sunrise = %v", d.Sunrise)
	}
	if d.Sunset == nil || d.Sunset.Format("15:04") != "16:52" {
		t.Errorf("sunset = %v", d.Sunset)
	}
	// Nearest chart point (12:00) has a probability of 10 %.
	if d.PrecipitationProbability != 10 {
		t.Errorf("precipitation = %d", d.PrecipitationProbability)
	}
	if d.Alert != "Regen ab ca. 18:00 Uhr" {
		t.Errorf("alert = %q", d.Alert)
	}

	if len(d.Hourly) != 9 {
		t.Fatalf("%d chart points", len(d.Hourly))
	}
	var hours []string
	for _, p := range d.Hourly {
		hours = append(hours, p.Time.Format("02 15:04"))
	}
	if strings.Join(hours, ",") != "14 00:00,14 03:00,14 06:00,14 09:00,14 12:00,14 15:00,14 18:00,14 21:00,15 00:00" {
		t.Errorf("chart hours = %v", hours)
	}
	if d.Hourly[4].TemperatureC != 8 || d.Hourly[6].Condition != "Leichter Regen" || d.Hourly[6].PrecipitationProbability != 60 || d.Hourly[6].PrecipitationMm != 0.3 {
		t.Errorf("forecast points: %+v %+v", d.Hourly[4], d.Hourly[6])
	}
	// Past points come from the time machine, which knows no probability.
	if d.Hourly[0].Condition != "Klarer Himmel" || d.Hourly[0].PrecipitationProbability != 0 {
		t.Errorf("past point: %+v", d.Hourly[0])
	}
	if n := e.up.count(pathTime); n != 4 {
		t.Errorf("%d time machine calls, want 4 (00:00 03:00 06:00 09:00)", n)
	}

	if len(d.Forecast) != 3 {
		t.Fatalf("%d forecast days", len(d.Forecast))
	}
	want := ForecastDay{Date: "2026-11-15", Condition: "Leichter Regen", IconCode: "10d", LowC: 6, HighC: 10,
		PrecipitationProbability: 80, WindKph: 18, PrecipitationMm: 1.8}
	if d.Forecast[0] != want {
		t.Errorf("forecast[0] = %+v, want %+v", d.Forecast[0], want)
	}
	if d.Forecast[1].Date != "2026-11-16" || d.Forecast[1].Condition != "Klarer Himmel" || d.Forecast[1].LowC != -1 {
		t.Errorf("forecast[1] = %+v", d.Forecast[1])
	}
	if d.Forecast[2].Condition != "Gewitter" || d.Forecast[2].PrecipitationMm != 12.5 {
		t.Errorf("forecast[2] = %+v", d.Forecast[2])
	}
}

func TestUpstreamRequests(t *testing.T) {
	e := newEnv(t)
	e.setCfg(func(c *Config) { c.Location = "Köln & Umgebung, DE" })
	e.svc.Get(context.Background())

	geo := e.up.queries[pathGeo][0]
	if geo["q"] != "Köln & Umgebung, DE" || geo["limit"] != "1" || geo["appid"] != testKey {
		t.Errorf("geocoding query = %v", geo)
	}
	one := e.up.queries[pathOne][0]
	for k, want := range map[string]string{"lat": "52.5170365", "lon": "13.3888599", "units": "metric", "lang": "de", "exclude": "minutely,alerts", "appid": testKey} {
		if one[k] != want {
			t.Errorf("one call %s = %q, want %q", k, one[k], want)
		}
	}
	tm := e.up.queries[pathTime]
	if len(tm) == 0 {
		t.Fatal("no time machine call")
	}
	if tm[0]["units"] != "metric" || tm[0]["lang"] != "de" || tm[0]["appid"] != testKey {
		t.Errorf("time machine query = %v", tm[0])
	}
	// 00:00 local on 2026-11-14 is 23:00 UTC on the 13th.
	wantDT := time.Date(2026, 11, 13, 23, 0, 0, 0, time.UTC).Unix()
	if tm[0]["dt"] != strconv.FormatInt(wantDT, 10) {
		t.Errorf("first dt = %s, want %d", tm[0]["dt"], wantDT)
	}
}

func TestLocationNotFound(t *testing.T) {
	e := newEnv(t)
	e.up.set(func(u *upstream) {
		u.geocode = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(fixture(t, "geocode_empty.json")) }
	})
	got := e.svc.Get(context.Background())
	if !got.Configured || got.Data != nil || got.Error != "Der Ort wurde nicht gefunden." {
		t.Errorf("result = %+v", got)
	}
	if e.up.count(pathOne) != 0 {
		t.Error("one call was requested without a location")
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name  string
		setup func(u *upstream)
		want  string
	}{
		{"one call 401", func(u *upstream) { u.oneCall = status(401) }, "OpenWeather One Call 3.0 ist für diesen API-Key nicht freigeschaltet."},
		{"one call 403", func(u *upstream) { u.oneCall = status(403) }, "OpenWeather One Call 3.0 ist für diesen API-Key nicht freigeschaltet."},
		{"geocoding 401", func(u *upstream) { u.geocode = status(401) }, "OpenWeather One Call 3.0 ist für diesen API-Key nicht freigeschaltet."},
		{"one call 500", func(u *upstream) { u.oneCall = status(500) }, "Die Wetterdaten konnten nicht geladen werden."},
		{"geocoding 500", func(u *upstream) { u.geocode = status(500) }, "Die Wetterdaten konnten nicht geladen werden."},
		{"one call 429", func(u *upstream) { u.oneCall = status(429) }, "Die Wetterdaten konnten nicht geladen werden."},
		{"broken JSON", func(u *upstream) {
			u.oneCall = func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "{not json") }
		}, "Die Wetterdaten konnten nicht geladen werden."},
		{"answer without current", func(u *upstream) {
			u.oneCall = func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"hourly":[],"daily":[]}`) }
		}, "Die Wetterdaten konnten nicht geladen werden."},
		{"no usable hourly points", func(u *upstream) {
			u.oneCall = func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"current":{"temp":1,"weather":[]},"hourly":[],"daily":[]}`)
			}
			u.timeMach = func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"data":[]}`) }
		}, "Die Wetterdaten konnten nicht geladen werden."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.up.set(tt.setup)
			got := e.svc.Get(context.Background())
			if !got.Configured || got.Data != nil || got.Stale || got.Error != tt.want {
				t.Errorf("result = %+v, want error %q", got, tt.want)
			}
			if strings.Contains(got.Error, testKey) || strings.Contains(e.logText(), testKey) {
				t.Errorf("API key leaked.\nresult: %s\nlog: %s", got.Error, e.logText())
			}
			if !strings.Contains(e.logText(), "weather refresh failed") {
				t.Errorf("failure was not logged: %s", e.logText())
			}
		})
	}
}

func TestAPIKeyNeverInErrorsOrLogsWhenUpstreamIsDown(t *testing.T) {
	e := newEnv(t)
	e.up.server.Close() // connection refused; net/http's error text contains the URL
	got := e.svc.Get(context.Background())
	if got.Error != "Die Wetterdaten konnten nicht geladen werden." {
		t.Errorf("result = %+v", got)
	}
	if log := e.logText(); strings.Contains(log, testKey) || !strings.Contains(log, "weather refresh failed") {
		t.Errorf("log = %s", log)
	}
}

func TestStaleDataRule(t *testing.T) {
	e := newEnv(t)
	first := e.svc.Get(context.Background())
	if first.Data == nil {
		t.Fatalf("setup: %+v", first)
	}
	e.up.set(func(u *upstream) { u.oneCall = status(500) })

	// Two hours later the refresh fails: the old data is shown, marked stale,
	// without an error.
	e.clock.advance(2 * time.Hour)
	got := e.svc.Get(context.Background())
	if got.Data == nil || !got.Stale || got.Error != "" {
		t.Errorf("after 2 h: %+v", got)
	}
	if got.Data != nil && !got.Data.UpdatedAt.Equal(first.Data.UpdatedAt) {
		t.Errorf("stale data changed: %v", got.Data.UpdatedAt)
	}

	// Just below the limit it is still stale data ...
	e.clock.advance(59 * time.Minute) // 2 h 59 min
	if got := e.svc.Get(context.Background()); got.Data == nil || !got.Stale {
		t.Errorf("after 2 h 59 min: %+v", got)
	}
	// ... four hours after the last good refresh only the error remains.
	e.clock.advance(61 * time.Minute) // 4 h
	got = e.svc.Get(context.Background())
	if got.Data != nil || got.Stale || got.Error != "Die Wetterdaten konnten nicht geladen werden." {
		t.Errorf("after 4 h: %+v", got)
	}

	// The upstream recovers: fresh data, no stale flag.
	e.up.set(func(u *upstream) {
		u.oneCall = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(fixture(t, "onecall_berlin.json")) }
	})
	e.clock.advance(2 * time.Minute)
	got = e.svc.Get(context.Background())
	if got.Data == nil || got.Stale || got.Error != "" {
		t.Errorf("after recovery: %+v", got)
	}
}

func TestStaleBoundaryIsThreeHours(t *testing.T) {
	e := newEnv(t)
	e.svc.Get(context.Background())
	e.up.set(func(u *upstream) { u.oneCall = status(500) })
	e.clock.advance(3 * time.Hour)
	if got := e.svc.Get(context.Background()); got.Data != nil || got.Error == "" {
		t.Errorf("exactly 3 h old data must not be shown: %+v", got)
	}
}

func TestRefreshIntervalIsRespected(t *testing.T) {
	e := newEnv(t)
	e.svc.Get(context.Background())
	if e.up.count(pathOne) != 1 || e.up.count(pathGeo) != 1 {
		t.Fatalf("first Get: geo %d one %d", e.up.count(pathGeo), e.up.count(pathOne))
	}

	e.clock.advance(14*time.Minute + 59*time.Second)
	e.svc.Get(context.Background())
	if e.up.count(pathOne) != 1 {
		t.Errorf("refreshed before the interval: %d calls", e.up.count(pathOne))
	}

	e.clock.advance(time.Second) // 15 minutes
	got := e.svc.Get(context.Background())
	if e.up.count(pathOne) != 2 {
		t.Errorf("no refresh after the interval: %d calls", e.up.count(pathOne))
	}
	if !got.Data.UpdatedAt.Equal(e.clock.now()) {
		t.Errorf("updatedAt not renewed: %v", got.Data.UpdatedAt)
	}
	// The geocoding result and the past points are cached across refreshes.
	if e.up.count(pathGeo) != 1 {
		t.Errorf("geocoded again: %d", e.up.count(pathGeo))
	}
	if e.up.count(pathTime) != 4 {
		t.Errorf("past points asked again: %d time machine calls", e.up.count(pathTime))
	}
}

func TestRefreshIntervalHasATenMinuteFloor(t *testing.T) {
	e := newEnv(t)
	e.setCfg(func(c *Config) { c.RefreshMinutes = 1 })
	e.svc.Get(context.Background())
	e.clock.advance(9 * time.Minute)
	e.svc.Get(context.Background())
	if e.up.count(pathOne) != 1 {
		t.Errorf("refreshed after 9 minutes despite the floor: %d", e.up.count(pathOne))
	}
	e.clock.advance(time.Minute)
	e.svc.Get(context.Background())
	if e.up.count(pathOne) != 2 {
		t.Errorf("no refresh after 10 minutes: %d", e.up.count(pathOne))
	}
}

func TestLongIntervalIsHonoured(t *testing.T) {
	e := newEnv(t)
	e.setCfg(func(c *Config) { c.RefreshMinutes = 60 })
	e.svc.Get(context.Background())
	e.clock.advance(59 * time.Minute)
	e.svc.Get(context.Background())
	if e.up.count(pathOne) != 1 {
		t.Errorf("refreshed early: %d", e.up.count(pathOne))
	}
}

func TestFailuresAreNotRetriedOnEveryPoll(t *testing.T) {
	e := newEnv(t)
	e.up.set(func(u *upstream) { u.oneCall = status(500) })
	for range 5 {
		e.svc.Get(context.Background())
		e.clock.advance(10 * time.Second)
	}
	if n := e.up.count(pathOne); n != 1 {
		t.Errorf("%d upstream calls within a minute of a failure, want 1", n)
	}
	e.clock.advance(time.Minute)
	e.svc.Get(context.Background())
	if n := e.up.count(pathOne); n != 2 {
		t.Errorf("no retry after the pause: %d calls", n)
	}
}

func TestShowIconsFollowsTheSettingsWithoutARefresh(t *testing.T) {
	e := newEnv(t)
	if !e.svc.Get(context.Background()).ShowIcons {
		t.Fatal("showIcons should be true")
	}
	e.setCfg(func(c *Config) { c.ShowIcons = false })
	got := e.svc.Get(context.Background())
	if got.ShowIcons || got.Data == nil {
		t.Errorf("result = %+v", got)
	}
	if e.up.count(pathOne) != 1 {
		t.Errorf("a display setting caused a refresh: %d", e.up.count(pathOne))
	}
}

func TestSingleFlight(t *testing.T) {
	e := newEnv(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	e.up.set(func(u *upstream) {
		u.oneCall = func(w http.ResponseWriter, _ *http.Request) {
			once.Do(func() { close(entered) })
			<-release
			_, _ = w.Write(fixture(t, "onecall_berlin.json"))
		}
	})

	const callers = 20
	results := make([]Result, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.svc.Get(context.Background())
		}()
	}
	<-entered
	time.Sleep(100 * time.Millisecond) // let the other callers reach the wait
	close(release)
	wg.Wait()

	if n := e.up.count(pathOne); n != 1 {
		t.Errorf("%d One Call requests for %d concurrent callers, want 1", n, callers)
	}
	if n := e.up.count(pathGeo); n != 1 {
		t.Errorf("%d geocoding requests, want 1", n)
	}
	for i, r := range results {
		if r.Data == nil || r.Error != "" {
			t.Errorf("caller %d: %+v", i, r)
		}
	}
}

func TestWaitingCallerHonoursItsContext(t *testing.T) {
	e := newEnv(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	e.up.set(func(u *upstream) {
		u.oneCall = func(w http.ResponseWriter, _ *http.Request) {
			once.Do(func() { close(entered) })
			<-release
			_, _ = w.Write(fixture(t, "onecall_berlin.json"))
		}
	})
	leader := make(chan Result, 1)
	go func() { leader <- e.svc.Get(context.Background()) }()
	<-entered

	ctx, cancel := context.WithCancel(context.Background())
	waiter := make(chan Result, 1)
	go func() { waiter <- e.svc.Get(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case r := <-waiter:
		if r.Data != nil || r.Error == "" {
			t.Errorf("cancelled waiter: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled caller kept waiting")
	}
	close(release)
	if r := <-leader; r.Data == nil {
		t.Errorf("leader: %+v", r)
	}
}

func TestSettingsChangeInvalidates(t *testing.T) {
	tests := []struct {
		name string
		mod  func(e *env)
	}{
		{"location", func(e *env) { e.setCfg(func(c *Config) { c.Location = "Hamburg" }) }},
		{"api key", func(e *env) { e.setCfg(func(c *Config) { c.APIKey = "other-key" }) }},
		{"explicit Invalidate", func(e *env) { e.svc.Invalidate() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.svc.Get(context.Background())
			tt.mod(e)
			e.clock.advance(time.Minute) // well inside the refresh interval
			got := e.svc.Get(context.Background())
			if got.Data == nil {
				t.Fatalf("result = %+v", got)
			}
			if n := e.up.count(pathOne); n != 2 {
				t.Errorf("%d One Call requests, want 2 (refresh after the change)", n)
			}
		})
	}
}

func TestSettingsChangeDropsOldDataAndGeocoding(t *testing.T) {
	e := newEnv(t)
	e.svc.Get(context.Background())
	e.setCfg(func(c *Config) { c.Location = "Hamburg" })
	e.up.set(func(u *upstream) {
		u.geocode = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(fixture(t, "geocode_empty.json")) }
	})
	e.clock.advance(time.Minute)
	got := e.svc.Get(context.Background())
	// The new location is geocoded afresh (not taken from the cache), and the
	// old place's data is not offered as stale data for it.
	if got.Data != nil || got.Error != "Der Ort wurde nicht gefunden." {
		t.Errorf("result = %+v", got)
	}
	if e.up.count(pathGeo) != 2 {
		t.Errorf("geocoding calls = %d, want 2", e.up.count(pathGeo))
	}
}

func TestInvalidateDuringRefreshDiscardsTheOldResult(t *testing.T) {
	e := newEnv(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	e.up.set(func(u *upstream) {
		u.oneCall = func(w http.ResponseWriter, _ *http.Request) {
			first := false
			once.Do(func() { first = true; close(entered) })
			if first {
				<-release
			}
			_, _ = w.Write(fixture(t, "onecall_berlin.json"))
		}
	})
	done := make(chan Result, 1)
	go func() { done <- e.svc.Get(context.Background()) }()
	<-entered
	e.svc.Invalidate()
	close(release)
	if r := <-done; r.Data == nil {
		t.Fatalf("result = %+v", r)
	}
	if n := e.up.count(pathOne); n != 2 {
		t.Errorf("%d One Call requests, want 2: the refresh that finished after the change must not be kept", n)
	}
}

func TestResultsAreSafeForConcurrentReaders(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%7 == 0 {
				e.clock.advance(time.Minute)
			}
			if i%11 == 0 {
				e.svc.Invalidate()
			}
			r := e.svc.Get(context.Background())
			if r.Data != nil {
				_ = len(r.Data.Hourly) + len(r.Data.Forecast)
			}
		}()
	}
	wg.Wait()
}

func TestNewDefaults(t *testing.T) {
	s := New(Options{Config: func() Config { return Config{} }})
	if s.baseURL != DefaultBaseURL || s.http == nil || s.http.Timeout == 0 || s.now == nil || s.loc == nil || s.log == nil {
		t.Errorf("defaults not applied: %+v", s)
	}
}
