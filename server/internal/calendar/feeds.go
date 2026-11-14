package calendar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// MaxFeedBytes is the largest calendar file the server reads.
	MaxFeedBytes = 5 << 20
	// maxConcurrentFetches bounds simultaneous downloads.
	maxConcurrentFetches = 4
	// failureRetry keeps a failing feed from being fetched on every poll.
	failureRetry = time.Minute
	// staleLimit is how old cached data may be to cover for a failing refresh.
	staleLimit = 3 * time.Hour
)

// ErrFeedTooLarge is returned for a feed above MaxFeedBytes.
var ErrFeedTooLarge = errors.New("calendar feed too large")

// FeedsOptions are the collaborators of Feeds.
type FeedsOptions struct {
	// HTTPClient must have a timeout; nil uses a client with 15 seconds.
	HTTPClient *http.Client
	// Now defaults to time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

type feedEntry struct {
	data      []byte
	fetchedAt time.Time
	failedAt  time.Time
	err       error
	inflight  chan struct{}
}

// Feeds downloads iCal files and caches them per address. The address of a
// feed may carry a secret token, so it appears neither in logs nor in errors.
type Feeds struct {
	http *http.Client
	now  func() time.Time
	log  *slog.Logger
	sem  chan struct{}

	mu         sync.Mutex
	entries    map[string]*feedEntry
	generation uint64
}

// NewFeeds returns a Feeds.
func NewFeeds(o FeedsOptions) *Feeds {
	f := &Feeds{http: o.HTTPClient, now: o.Now, log: o.Logger, sem: make(chan struct{}, maxConcurrentFetches), entries: map[string]*feedEntry{}}
	if f.http == nil {
		f.http = &http.Client{Timeout: 15 * time.Second}
	}
	if f.now == nil {
		f.now = time.Now
	}
	if f.log == nil {
		f.log = slog.Default()
	}
	return f
}

// Invalidate forgets all cached feeds; the next Fetch downloads again.
func (f *Feeds) Invalidate() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.generation++
	f.entries = map[string]*feedEntry{}
}

// Fetch returns the content of the feed, from the cache while it is younger
// than interval. Concurrent callers of one address share a download. A failed
// refresh falls back to cached data up to three hours old.
func (f *Feeds) Fetch(ctx context.Context, rawURL string, interval time.Duration) ([]byte, error) {
	f.mu.Lock()
	for {
		e := f.entries[rawURL]
		if e == nil {
			e = &feedEntry{}
			f.entries[rawURL] = e
		}
		now := f.now()
		if e.data != nil && now.Sub(e.fetchedAt) < interval {
			defer f.mu.Unlock()
			return e.data, nil
		}
		if e.err != nil && now.Sub(e.failedAt) < failureRetry {
			defer f.mu.Unlock()
			return f.fallbackLocked(e, now)
		}
		if e.inflight != nil {
			wait := e.inflight
			f.mu.Unlock()
			select {
			case <-wait:
				f.mu.Lock()
				continue
			case <-ctx.Done():
				return nil, fmt.Errorf("wait for feed: %w", ctx.Err())
			}
		}
		e.inflight = make(chan struct{})
		gen := f.generation
		f.mu.Unlock()

		data, err := f.download(ctx, rawURL)

		f.mu.Lock()
		close(e.inflight)
		e.inflight = nil
		if gen == f.generation { // a settings change dropped this entry meanwhile
			if err != nil {
				e.err, e.failedAt = err, f.now()
			} else {
				e.data, e.fetchedAt, e.err = data, f.now(), nil
			}
		}
		defer f.mu.Unlock()
		if err != nil {
			if e.data != nil && f.now().Sub(e.fetchedAt) < staleLimit {
				return e.data, nil
			}
			return nil, err
		}
		return data, nil
	}
}

func (f *Feeds) fallbackLocked(e *feedEntry, now time.Time) ([]byte, error) {
	if e.data != nil && now.Sub(e.fetchedAt) < staleLimit {
		return e.data, nil
	}
	return nil, e.err
}

// download fetches one address. Errors never contain the address.
func (f *Feeds) download(ctx context.Context, rawURL string) ([]byte, error) {
	select {
	case f.sem <- struct{}{}:
		defer func() { <-f.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for download slot: %w", ctx.Err())
	}
	target := rawURL
	if i := strings.Index(target, "://"); i > 0 {
		if s := strings.ToLower(target[:i]); s == "webcal" || s == "webcals" {
			target = "https" + target[i:]
		}
	}
	host := hostOf(target)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		f.log.Warn("calendar feed request invalid", "host", host)
		return nil, errors.New("invalid feed address")
	}
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	req.Header.Set("User-Agent", "MirrorNextGen")
	resp, err := f.http.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // url.Error would repeat the address
		}
		f.log.Warn("calendar feed request failed", "host", host, "error", err)
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		f.log.Warn("calendar feed answered with an error", "host", host, "status", resp.StatusCode)
		return nil, fmt.Errorf("feed answered status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxFeedBytes+1))
	if err != nil {
		f.log.Warn("calendar feed read failed", "host", host, "error", err)
		return nil, fmt.Errorf("read feed: %w", err)
	}
	if len(data) > MaxFeedBytes {
		f.log.Warn("calendar feed too large", "host", host, "limit", MaxFeedBytes)
		return nil, ErrFeedTooLarge
	}
	return data, nil
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}
