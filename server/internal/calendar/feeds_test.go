package calendar

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type feedClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *feedClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *feedClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type logSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logSink) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logSink) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func newFeeds(t *testing.T, h http.Handler) (*Feeds, *httptest.Server, *feedClock, *logSink) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	clock := &feedClock{t: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	logs := &logSink{}
	f := NewFeeds(FeedsOptions{HTTPClient: srv.Client(), Now: clock.now, Logger: slog.New(slog.NewTextHandler(logs, nil))})
	return f, srv, clock, logs
}

func TestFeedsRewriteWebcalToHTTPS(t *testing.T) {
	var scheme atomic.Value
	f, srv, _, _ := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme.Store(r.TLS != nil)
		_, _ = io.WriteString(w, "BEGIN:VCALENDAR")
	}))
	for _, prefix := range []string{"webcal", "webcals"} {
		scheme.Store(false)
		u := strings.Replace(srv.URL, "https", prefix, 1) + "/a.ics"
		if _, err := f.Fetch(context.Background(), u, time.Hour); err != nil {
			t.Fatalf("%s: %v", prefix, err)
		}
		if scheme.Load() != true {
			t.Errorf("%s was not fetched over https", prefix)
		}
	}
}

func TestFeedsSizeLimit(t *testing.T) {
	var size atomic.Int64
	f, srv, _, logs := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), int(size.Load())))
	}))
	size.Store(MaxFeedBytes)
	if data, err := f.Fetch(context.Background(), srv.URL+"/ok?token=SECRET", time.Hour); err != nil || len(data) != MaxFeedBytes {
		t.Fatalf("exact limit: %d bytes, %v", len(data), err)
	}
	size.Store(MaxFeedBytes + 1)
	_, err := f.Fetch(context.Background(), srv.URL+"/big?token=SECRET", time.Hour)
	if err != ErrFeedTooLarge {
		t.Fatalf("err = %v, want ErrFeedTooLarge", err)
	}
	if strings.Contains(logs.String(), "SECRET") {
		t.Errorf("log leaks the address: %s", logs.String())
	}
}

func TestFeedsCacheHonoursInterval(t *testing.T) {
	var calls atomic.Int32
	f, srv, clock, _ := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "v")
	}))
	ctx := context.Background()
	u := srv.URL + "/a.ics"
	for range 3 {
		if _, err := f.Fetch(ctx, u, 15*time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 within the interval", calls.Load())
	}
	clock.add(14 * time.Minute)
	_, _ = f.Fetch(ctx, u, 15*time.Minute)
	if calls.Load() != 1 {
		t.Errorf("refetched after 14 min: %d", calls.Load())
	}
	clock.add(2 * time.Minute)
	_, _ = f.Fetch(ctx, u, 15*time.Minute)
	if calls.Load() != 2 {
		t.Errorf("not refetched after 16 min: %d", calls.Load())
	}
	// A shorter interval makes the cached copy stale at once.
	_, _ = f.Fetch(ctx, u, time.Second)
	_, _ = f.Fetch(ctx, srv.URL+"/other.ics", time.Hour)
	if calls.Load() != 3 {
		t.Errorf("other address shares the cache or short interval ignored: %d", calls.Load())
	}
	f.Invalidate()
	_, _ = f.Fetch(ctx, u, time.Hour)
	if calls.Load() != 4 {
		t.Errorf("Invalidate did not drop the cache: %d", calls.Load())
	}
}

func TestFeedsSingleFlight(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	f, srv, _, _ := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release
		_, _ = io.WriteString(w, "v")
	}))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.Fetch(context.Background(), srv.URL+"/a.ics", time.Hour); err != nil {
				t.Error(err)
			}
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // let the others queue behind the first
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Errorf("upstream calls = %d, want 1 shared download", calls.Load())
	}
}

func TestFeedsAtMostFourConcurrentDownloads(t *testing.T) {
	var cur, peak atomic.Int32
	f, srv, _, _ := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		cur.Add(-1)
		_, _ = io.WriteString(w, "v")
	}))
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.Fetch(context.Background(), srv.URL+"/"+string(rune('a'+i))+".ics", time.Hour)
		}()
	}
	wg.Wait()
	if peak.Load() > 4 || peak.Load() < 2 {
		t.Errorf("peak concurrency = %d, want 2..4", peak.Load())
	}
}

func TestFeedsFailureIsCachedBrieflyAndNeverLeaksTheAddress(t *testing.T) {
	var calls atomic.Int32
	f, srv, clock, logs := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	u := srv.URL + "/secret/path/a.ics?token=SECRET"
	_, err := f.Fetch(context.Background(), u, time.Hour)
	if err == nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "/secret/path") {
		t.Fatalf("err = %v", err)
	}
	_, _ = f.Fetch(context.Background(), u, time.Hour)
	if calls.Load() != 1 {
		t.Errorf("failure not cached: %d calls", calls.Load())
	}
	clock.add(2 * time.Minute)
	_, _ = f.Fetch(context.Background(), u, time.Hour)
	if calls.Load() != 2 {
		t.Errorf("no retry after a minute: %d calls", calls.Load())
	}
	out := logs.String()
	if strings.Contains(out, "SECRET") || strings.Contains(out, "/secret/path") {
		t.Errorf("log leaks the address: %s", out)
	}
	if !strings.Contains(out, "host=127.0.0.1") {
		t.Errorf("log lacks the host: %s", out)
	}
}

func TestFeedsConnectionErrorDoesNotLeakTheAddress(t *testing.T) {
	f := NewFeeds(FeedsOptions{HTTPClient: &http.Client{Timeout: time.Second}})
	var logs logSink
	f.log = slog.New(slog.NewTextHandler(&logs, nil))
	_, err := f.Fetch(context.Background(), "http://127.0.0.1:1/private/a.ics?token=SECRET", time.Hour)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, s := range []string{err.Error(), logs.String()} {
		if strings.Contains(s, "SECRET") || strings.Contains(s, "/private/") {
			t.Errorf("leak: %s", s)
		}
	}
}

func TestFeedsServeStaleDataWhenRefreshFails(t *testing.T) {
	var fail atomic.Bool
	f, srv, clock, _ := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, "good")
	}))
	u := srv.URL + "/a.ics"
	if _, err := f.Fetch(context.Background(), u, 15*time.Minute); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	clock.add(30 * time.Minute)
	data, err := f.Fetch(context.Background(), u, 15*time.Minute)
	if err != nil || string(data) != "good" {
		t.Errorf("stale fallback: %q %v", data, err)
	}
	clock.add(4 * time.Hour)
	if _, err := f.Fetch(context.Background(), u, 15*time.Minute); err == nil {
		t.Error("data older than three hours still served")
	}
}

func TestMaxFeedBytesIs5MiB(t *testing.T) {
	if MaxFeedBytes != 5*1024*1024 {
		t.Errorf("MaxFeedBytes = %d, want 5242880", MaxFeedBytes)
	}
}

func TestFeedsInvalidateDuringDownloadDropsTheStaleResult(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	f, srv, _, _ := newFeeds(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		started <- struct{}{}
		if n == 1 {
			<-release
		}
		_, _ = io.WriteString(w, "v"+strconv.Itoa(int(n)))
	}))
	u := srv.URL + "/a.ics"
	done := make(chan []byte, 1)
	go func() {
		data, err := f.Fetch(context.Background(), u, time.Hour)
		if err != nil {
			t.Error(err)
		}
		done <- data
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not start")
	}
	f.Invalidate() // the settings changed while the download was in flight
	close(release)
	if got := string(<-done); got != "v1" {
		t.Fatalf("in-flight caller got %q, want its own download v1", got)
	}
	data, err := f.Fetch(context.Background(), u, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || string(data) != "v2" {
		t.Errorf("calls = %d, data = %q; the stale download was cached across Invalidate", calls.Load(), data)
	}
}
