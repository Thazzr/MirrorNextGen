package camera

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const secretURL = "rtsp://admin:geheim-passwort@192.168.1.20:554/stream1?token=t0k3n"

type recorded struct {
	method string
	query  string // raw query
	path   string
}

// fakeGo2RTC records the API calls; status can be changed while a test runs.
type fakeGo2RTC struct {
	server *httptest.Server
	mu     sync.Mutex
	calls  []recorded
	status int
}

func newFakeGo2RTC(t *testing.T) *fakeGo2RTC {
	t.Helper()
	f := &fakeGo2RTC{status: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls = append(f.calls, recorded{r.Method, r.URL.RawQuery, r.URL.Path})
		status := f.status
		f.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte("rtsp://admin:geheim-passwort@192.168.1.20 failed")) // go2rtc may echo addresses
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGo2RTC) setStatus(s int) {
	f.mu.Lock()
	f.status = s
	f.mu.Unlock()
}

func (f *fakeGo2RTC) got() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.calls...)
}

type logBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func newService(t *testing.T, base string, cfg func() Config, logs io.Writer) *Service {
	t.Helper()
	s, err := New(Options{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		BaseURL:    base,
		Config:     cfg,
		Logger:     slog.New(slog.NewTextHandler(logs, nil)),
		Backoff:    func(int) time.Duration { return time.Millisecond },
		Resync:     time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func fixed(c Config) func() Config { return func() Config { return c } }

func TestNewRejectsUnusableBaseURL(t *testing.T) {
	for _, base := range []string{"", "camera:1984", "ftp://camera/", "http://", "://x"} {
		if _, err := New(Options{BaseURL: base}); err == nil {
			t.Errorf("New(%q) succeeded", base)
		}
	}
}

func TestSyncRegistersWithExactQuery(t *testing.T) {
	f := newFakeGo2RTC(t)
	s := newService(t, f.server.URL+"/", fixed(Config{Enabled: true, StreamURL: secretURL}), io.Discard)
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := f.got()
	if len(calls) != 1 {
		t.Fatalf("calls = %+v", calls)
	}
	// Name first, then the percent-encoded address; the address must arrive as
	// one single, intact src value.
	wantQuery := "name=mirror-camera&src=rtsp%3A%2F%2Fadmin%3Ageheim-passwort%40192.168.1.20%3A554%2Fstream1%3Ftoken%3Dt0k3n"
	if calls[0].method != http.MethodPut || calls[0].path != "/api/streams" || calls[0].query != wantQuery {
		t.Errorf("call = %+v, want PUT /api/streams?%s", calls[0], wantQuery)
	}
}

func TestSyncDeletesWhenNotConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"disabled", Config{Enabled: false, StreamURL: secretURL}},
		{"no address", Config{Enabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeGo2RTC(t)
			s := newService(t, f.server.URL, fixed(tt.cfg), io.Discard)
			if err := s.Sync(context.Background()); err != nil {
				t.Fatal(err)
			}
			calls := f.got()
			if len(calls) != 1 || calls[0].method != http.MethodDelete || calls[0].query != "src=mirror-camera" || calls[0].path != "/api/streams" {
				t.Errorf("calls = %+v", calls)
			}
		})
	}
}

func TestSyncNeverRegistersOtherSchemes(t *testing.T) {
	for _, raw := range []string{"exec:ffmpeg -i x", "http://cam/live", "ffmpeg:rtsp://cam/x", "rtsp-fake", ""} {
		f := newFakeGo2RTC(t)
		s := newService(t, f.server.URL, fixed(Config{Enabled: true, StreamURL: raw}), io.Discard)
		err := s.Sync(context.Background())
		if raw == "" {
			continue // not configured: a DELETE, covered above
		}
		if !errors.Is(err, ErrUnsupportedURL) {
			t.Errorf("Sync(%q) = %v, want ErrUnsupportedURL", raw, err)
		}
		if n := len(f.got()); n != 0 {
			t.Errorf("Sync(%q) reached go2rtc %d times", raw, n)
		}
	}
}

func TestErrorsNeverContainTheStreamAddress(t *testing.T) {
	f := newFakeGo2RTC(t)
	f.setStatus(http.StatusBadRequest)
	s := newService(t, f.server.URL, fixed(Config{Enabled: true, StreamURL: secretURL}), io.Discard)
	err := s.Sync(context.Background())
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v, want a status error", err)
	}
	// Unreachable service: url.Error would print the whole request URL.
	dead := httptest.NewServer(http.NotFoundHandler())
	base := dead.URL
	dead.Close()
	s = newService(t, base, fixed(Config{Enabled: true, StreamURL: secretURL}), io.Discard)
	err = s.Sync(context.Background())
	if err == nil {
		t.Fatal("expected a connection error")
	}
	for _, part := range []string{"geheim", "admin", "t0k3n", "192.168.1.20", "src="} {
		if strings.Contains(err.Error(), part) {
			t.Errorf("error leaks %q: %v", part, err)
		}
	}
}

func TestRunRetriesWithoutCrashAndNeverLogsTheAddress(t *testing.T) {
	f := newFakeGo2RTC(t)
	f.setStatus(http.StatusServiceUnavailable)
	logs := &logBuf{}
	s := newService(t, f.server.URL, fixed(Config{Enabled: true, StreamURL: secretURL}), logs)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	waitFor(t, func() bool { return len(f.got()) >= 3 }, "three failed attempts")
	f.setStatus(http.StatusOK)
	waitFor(t, func() bool { return strings.Count(logs.String(), "will retry") >= 3 }, "retry warnings")
	// After the first success nothing more is sent (resync is an hour away).
	time.Sleep(50 * time.Millisecond)
	n := len(f.got())
	time.Sleep(50 * time.Millisecond)
	if after := len(f.got()); after != n {
		t.Errorf("calls kept coming after success: %d -> %d", n, after)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not end with its context")
	}
	for _, part := range []string{"geheim", "admin", "t0k3n", "192.168.1.20"} {
		if strings.Contains(logs.String(), part) {
			t.Errorf("log leaks %q:\n%s", part, logs)
		}
	}
}

func TestApplyRegistersTheNewSettings(t *testing.T) {
	f := newFakeGo2RTC(t)
	var mu sync.Mutex
	cfg := Config{Enabled: true, StreamURL: "rtsp://one/s"}
	s := newService(t, f.server.URL, func() Config { mu.Lock(); defer mu.Unlock(); return cfg }, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	waitFor(t, func() bool { return len(f.got()) == 1 }, "initial registration")

	mu.Lock()
	cfg = Config{Enabled: false, StreamURL: "rtsp://one/s"}
	mu.Unlock()
	s.Apply()
	waitFor(t, func() bool { return len(f.got()) == 2 }, "delete after Apply")
	if last := f.got()[1]; last.method != http.MethodDelete {
		t.Errorf("second call = %+v, want DELETE", last)
	}
	s.Apply()
	s.Apply() // never blocks, however often it is called
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// --- proxy ---------------------------------------------------------------

type upstreamRequest struct {
	uri    string
	host   string
	header http.Header
}

// fakeWSUpstream accepts an upgrade like go2rtc: it answers 101 and then
// sends "video-bytes". It records what the proxy sent.
func fakeWSUpstream(t *testing.T) (*httptest.Server, func() []upstreamRequest) {
	t.Helper()
	var mu sync.Mutex
	var reqs []upstreamRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, upstreamRequest{uri: r.URL.RequestURI(), host: r.Host, header: r.Header.Clone()})
		mu.Unlock()
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "no upgrade", http.StatusBadRequest)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\nvideo-bytes")
		_ = rw.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv, func() []upstreamRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]upstreamRequest(nil), reqs...)
	}
}

func testProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(detail))
}

// upgradeRaw sends a WebSocket handshake by hand and returns the response and
// the bytes that follow it.
func upgradeRaw(t *testing.T, base, target string, extra string) (*http.Response, string) {
	t.Helper()
	addr := strings.TrimPrefix(base, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	req := "GET " + target + " HTTP/1.1\r\nHost: mirror:8080\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n" + extra + "\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return resp, "" // a plain answer keeps the connection open
	}
	rest, _ := io.ReadAll(br)
	return resp, string(rest)
}

func proxyServer(t *testing.T, s *Service) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/camera/stream", s.Handler(testProblem))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestProxyForwardsUpgradeToTheFixedStreamWhateverTheClientAsks(t *testing.T) {
	up, upReqs := fakeWSUpstream(t)
	s := newService(t, up.URL, fixed(Config{Enabled: true, StreamURL: secretURL}), io.Discard)
	srv := proxyServer(t, s)

	// The client tries to choose another source and to smuggle credentials.
	resp, rest := upgradeRaw(t, srv.URL, "/api/v1/camera/stream?src=exec:evil&src=mirror-camera&name=x",
		"Origin: http://mirror:8080\r\nCookie: mirror_session=abc\r\nAuthorization: Bearer x\r\n")
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}
	if rest != "video-bytes" {
		t.Errorf("bytes after the handshake = %q", rest)
	}
	reqs := upReqs()
	if len(reqs) != 1 {
		t.Fatalf("upstream requests = %d", len(reqs))
	}
	// go2rtc must see its own host, not the mirror's (the client sent
	// "Host: mirror:8080").
	if want := strings.TrimPrefix(up.URL, "http://"); reqs[0].host != want {
		t.Errorf("upstream Host = %q, want %q", reqs[0].host, want)
	}
	if got := reqs[0].uri; got != "/api/ws?src=mirror-camera" {
		t.Errorf("upstream URI = %q, want /api/ws?src=mirror-camera", got)
	}
	for _, h := range []string{"Cookie", "Authorization", "Origin"} {
		if v := reqs[0].header.Get(h); v != "" {
			t.Errorf("%s reached go2rtc: %q", h, v)
		}
	}
	if reqs[0].header.Get("Sec-WebSocket-Key") == "" {
		t.Error("the handshake headers were not forwarded")
	}
}

func TestProxyAnswersNonUpgradeRequestsWith426AndForwardsNothing(t *testing.T) {
	up, upReqs := fakeWSUpstream(t)
	s := newService(t, up.URL, fixed(Config{Enabled: true, StreamURL: secretURL}), io.Discard)
	srv := proxyServer(t, s)

	resp, err := http.Get(srv.URL + "/api/v1/camera/stream?src=mirror-camera")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUpgradeRequired || resp.Header.Get("Upgrade") != "websocket" {
		t.Errorf("status = %d, Upgrade = %q", resp.StatusCode, resp.Header.Get("Upgrade"))
	}
	if n := len(upReqs()); n != 0 {
		t.Errorf("%d requests reached go2rtc", n)
	}
}

func TestProxyIs404WhileNotConfigured(t *testing.T) {
	up, upReqs := fakeWSUpstream(t)
	for name, cfg := range map[string]Config{"disabled": {StreamURL: secretURL}, "no address": {Enabled: true}} {
		t.Run(name, func(t *testing.T) {
			s := newService(t, up.URL, fixed(cfg), io.Discard)
			srv := proxyServer(t, s)
			resp, _ := upgradeRaw(t, srv.URL, "/api/v1/camera/stream", "")
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("status = %d, want 404", resp.StatusCode)
			}
		})
	}
	if n := len(upReqs()); n != 0 {
		t.Errorf("%d requests reached go2rtc", n)
	}
}

func TestEndpointIsAbsoluteForEveryBaseURLShape(t *testing.T) {
	tests := []struct{ base, want string }{
		{"http://camera:1984", "http://camera:1984/api/ws?src=mirror-camera"},
		{"http://camera:1984/", "http://camera:1984/api/ws?src=mirror-camera"},
		{"http://camera:1984/prefix", "http://camera:1984/prefix/api/ws?src=mirror-camera"},
		{"http://camera:1984/prefix/", "http://camera:1984/prefix/api/ws?src=mirror-camera"},
	}
	for _, tt := range tests {
		s, err := New(Options{BaseURL: tt.base})
		if err != nil {
			t.Fatal(err)
		}
		if got := s.endpoint("api/ws", url.Values{"src": {StreamName}}).String(); got != tt.want {
			t.Errorf("base %q: %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestProxyAnswers502WhenGo2RTCIsDown(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	base := dead.URL
	dead.Close()
	logs := &logBuf{}
	s := newService(t, base, fixed(Config{Enabled: true, StreamURL: secretURL}), logs)
	srv := proxyServer(t, s)
	resp, _ := upgradeRaw(t, srv.URL, "/api/v1/camera/stream", "")
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
	if strings.Contains(logs.String(), "geheim") {
		t.Errorf("log leaks the address: %s", logs)
	}
}

func TestSupportedSchemeIsCaseInsensitive(t *testing.T) {
	for _, raw := range []string{"RTSP://host/x", "Rtsps://host/x", "rtsp://host/x"} {
		f := newFakeGo2RTC(t)
		s := newService(t, f.server.URL, fixed(Config{Enabled: true, StreamURL: raw}), io.Discard)
		if err := s.Sync(context.Background()); err != nil {
			t.Errorf("Sync(%q) = %v", raw, err)
		}
		if calls := f.got(); len(calls) != 1 || calls[0].method != http.MethodPut {
			t.Errorf("Sync(%q) calls = %+v, want one PUT", raw, calls)
		}
	}
}

func TestDefaultBackoff(t *testing.T) {
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30, 30, 30, 30, 30}
	for attempt, w := range want {
		if got := defaultBackoff(attempt); got != w*time.Second {
			t.Errorf("defaultBackoff(%d) = %v, want %v", attempt, got, w*time.Second)
		}
	}
}

func TestNewDefaultsResyncToFiveMinutes(t *testing.T) {
	s, err := New(Options{BaseURL: "http://camera:1984"})
	if err != nil {
		t.Fatal(err)
	}
	if s.resync != 5*time.Minute {
		t.Errorf("resync = %v, want 5m", s.resync)
	}
	s, _ = New(Options{BaseURL: "http://camera:1984", Resync: time.Minute})
	if s.resync != time.Minute {
		t.Errorf("explicit resync = %v, want 1m", s.resync)
	}
}

// scriptedGo2RTC answers call n with statuses[n] (the last one repeats).
func scriptedGo2RTC(t *testing.T, statuses ...int) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		st := statuses[min(n, len(statuses)-1)]
		n++
		mu.Unlock()
		w.WriteHeader(st)
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return n }
}

type attempts struct {
	mu   sync.Mutex
	list []int
	wait time.Duration
}

func (a *attempts) backoff(attempt int) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = append(a.list, attempt)
	return a.wait
}

func (a *attempts) got() []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]int(nil), a.list...)
}

func runService(t *testing.T, base string, resync time.Duration, a *attempts) *Service {
	t.Helper()
	s, err := New(Options{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		BaseURL:    base,
		Config:     fixed(Config{Enabled: true, StreamURL: "rtsp://cam/x"}),
		Backoff:    a.backoff,
		Resync:     resync,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return s
}

func TestRunResetsTheBackoffAfterASuccess(t *testing.T) {
	// fail, succeed, fail, then succeed for good.
	srv, calls := scriptedGo2RTC(t, 503, 200, 503, 200)
	a := &attempts{wait: time.Millisecond}
	runService(t, srv.URL, 5*time.Millisecond, a)
	waitFor(t, func() bool { return calls() >= 4 }, "four calls")
	if got := a.got(); len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Errorf("backoff attempts = %v, want [0 0] (reset after the success)", got)
	}
}

func TestApplyResetsTheBackoff(t *testing.T) {
	// fail, then (after Apply) fail again, then succeed.
	srv, calls := scriptedGo2RTC(t, 503, 503, 200)
	a := &attempts{wait: time.Hour}
	s := runService(t, srv.URL, time.Hour, a)
	waitFor(t, func() bool { return calls() == 1 && len(a.got()) == 1 }, "first failure")
	s.Apply()
	waitFor(t, func() bool { return calls() == 2 && len(a.got()) == 2 }, "second failure")
	if got := a.got(); got[0] != 0 || got[1] != 0 {
		t.Errorf("backoff attempts = %v, want [0 0] (reset by Apply)", got)
	}
}

func TestRunWithoutCameraDoesNotBackOffOrWarn(t *testing.T) {
	srv, calls := scriptedGo2RTC(t, 503)
	a := &attempts{wait: time.Hour}
	logs := &logBuf{}
	s, err := New(Options{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		BaseURL:    srv.URL,
		Config:     fixed(Config{}),
		Logger:     slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Backoff:    a.backoff,
		Resync:     5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	// With a one-hour backoff a second call could only come from the resync wait.
	waitFor(t, func() bool { return calls() >= 3 }, "three resyncs")
	if got := a.got(); len(got) != 0 {
		t.Errorf("backoff was used while not configured: %v", got)
	}
	if strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("warning logged while not configured:\n%s", logs)
	}
	if !strings.Contains(logs.String(), "level=DEBUG") {
		t.Errorf("expected a debug record:\n%s", logs)
	}
}

func TestRunWhileConfiguredWarnsAndBacksOff(t *testing.T) {
	srv, calls := scriptedGo2RTC(t, 503)
	a := &attempts{wait: time.Millisecond}
	logs := &logBuf{}
	s, err := New(Options{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		BaseURL:    srv.URL,
		Config:     fixed(Config{Enabled: true, StreamURL: "rtsp://cam/x"}),
		Logger:     slog.New(slog.NewTextHandler(logs, nil)),
		Backoff:    a.backoff,
		Resync:     time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	waitFor(t, func() bool { return calls() >= 3 }, "three attempts")
	if got := a.got(); len(got) < 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("backoff attempts = %v, want to start [0 1]", got)
	}
	if !strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("no warning while configured:\n%s", logs)
	}
}
