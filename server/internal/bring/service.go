package bring

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Messages shown on the dashboard and in the admin UI; the details of a
// failure go to the log.
const (
	MsgLoginFailed = "Anmeldung bei Bring! fehlgeschlagen."
	MsgLoadFailed  = "Die Bring!-Liste konnte nicht geladen werden."
	// MsgListsFailed is the admin's text when the account's lists cannot be read.
	MsgListsFailed = "Die Bring!-Listen konnten nicht geladen werden."
)

// ErrNotConfigured is returned by Lists while e-mail or password is missing.
var ErrNotConfigured = errors.New("bring is not configured")

const (
	// staleLimit is how old the last good list may be to be shown when a
	// refresh fails.
	staleLimit = 3 * time.Hour
	// failureRetry keeps a failing upstream from being called on every poll
	// of the dashboard.
	failureRetry   = time.Minute
	refreshTimeout = 30 * time.Second
)

// Config is the part of the settings the service needs.
type Config struct {
	Email          string
	Password       string
	ListUUID       string
	MaxItems       int
	RefreshMinutes int
}

func (c Config) creds() Credentials { return Credentials{Email: c.Email, Password: c.Password} }

// DashboardItem is an item as the dashboard shows it.
type DashboardItem struct {
	Name          string
	Specification string
	IconURL       string
}

// Data is one successful refresh. It is never modified after creation.
type Data struct {
	UpdatedAt time.Time
	Items     []DashboardItem
}

// Result is what Get reports to the dashboard.
type Result struct {
	// Configured is false while e-mail or password is missing.
	Configured bool
	// Stale marks Data from an earlier refresh because the latest failed.
	Stale bool
	// Error is a German message; set only when there is no Data.
	Error string
	Data  *Data
}

// Options are the collaborators of a Service.
type Options struct {
	// HTTPClient must have a timeout; nil uses a client with 15 seconds.
	HTTPClient *http.Client
	// BaseURL defaults to DefaultBaseURL.
	BaseURL string
	// Config returns the current settings on every call.
	Config func() Config
	// Now defaults to time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

type call struct{ done chan struct{} }

// Service caches the shopping list and refreshes it at most every refresh
// interval. Concurrent callers share one refresh.
type Service struct {
	client *Client
	cfg    func() Config
	now    func() time.Time
	log    *slog.Logger

	mu          sync.Mutex
	key         Config // the settings the state belongs to
	data        *Data
	fetchedAt   time.Time
	failedAt    time.Time
	lastError   string
	inflight    *call
	generation  uint64
	initialized bool
}

// New returns a Service.
func New(o Options) *Service {
	s := &Service{cfg: o.Config, now: o.Now, log: o.Logger}
	if s.now == nil {
		s.now = time.Now
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	s.client = NewClient(o.HTTPClient, o.BaseURL, s.now)
	return s
}

// Invalidate forgets the cached list; the next Get refreshes. The settings
// handler calls it after a change.
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

func (s *Service) resetLocked() {
	s.generation++
	s.data = nil
	s.fetchedAt = time.Time{}
	s.failedAt = time.Time{}
}

func configured(c Config) bool { return c.Email != "" && c.Password != "" }

// Get returns the list, refreshing it when it is older than the refresh
// interval.
func (s *Service) Get(ctx context.Context) Result {
	cfg := s.cfg()
	var res Result
	if !configured(cfg) {
		return res
	}
	res.Configured = true
	interval := max(time.Duration(cfg.RefreshMinutes)*time.Minute, time.Minute)

	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		now := s.now()
		if !s.initialized || cfg != s.key {
			s.key, s.initialized = cfg, true
			s.resetLocked()
		}
		if s.data != nil && now.Sub(s.fetchedAt) < interval {
			res.Data = s.data
			return res
		}
		if !s.failedAt.IsZero() && now.Sub(s.failedAt) < failureRetry {
			return s.failureLocked(res, now)
		}
		if c := s.inflight; c != nil {
			s.mu.Unlock()
			select {
			case <-c.done:
				s.mu.Lock()
				continue
			case <-ctx.Done():
				s.mu.Lock()
				res.Error = MsgLoadFailed
				return res
			}
		}
		s.refreshLocked(ctx, cfg, now)
	}
}

// failureLocked reports the outcome of a failed refresh: a recent list as
// stale, otherwise the error.
func (s *Service) failureLocked(res Result, now time.Time) Result {
	if s.data != nil && now.Sub(s.data.UpdatedAt) < staleLimit {
		res.Data, res.Stale = s.data, true
		return res
	}
	res.Error = s.lastError
	return res
}

// refreshLocked runs one refresh as the single flight. It is entered and left
// with s.mu held and unlocks it while it talks to Bring!.
func (s *Service) refreshLocked(ctx context.Context, cfg Config, now time.Time) {
	c := &call{done: make(chan struct{})}
	s.inflight = c
	gen := s.generation
	s.mu.Unlock()

	// The refresh serves every waiting caller, so it must not die with the
	// request that happened to start it.
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), refreshTimeout)
	data, err := s.fetch(rctx, cfg, now)
	cancel()

	s.mu.Lock()
	s.inflight = nil
	close(c.done)
	if gen != s.generation {
		return // the settings changed meanwhile; the result belongs to the old ones
	}
	if err != nil {
		s.failedAt = now
		s.lastError = userMessage(err)
		s.log.Warn("bring refresh failed", "error", redact(err, cfg.Password))
		return
	}
	s.data, s.fetchedAt, s.failedAt = data, now, time.Time{}
}

func (s *Service) fetch(ctx context.Context, cfg Config, now time.Time) (*Data, error) {
	content, err := s.client.Content(ctx, cfg.creds(), cfg.ListUUID)
	if err != nil {
		return nil, err
	}
	items := Select(content.Items, cfg.MaxItems)
	out := &Data{UpdatedAt: now, Items: make([]DashboardItem, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, DashboardItem{Name: it.Name, Specification: it.Specification, IconURL: IconURL(it.Name, content.UserIcons)})
	}
	return out, nil
}

// Lists returns the lists of the account whose credentials are stored. It
// answers ErrNotConfigured without them and ErrLoginFailed when Bring!
// refuses them.
func (s *Service) Lists(ctx context.Context) ([]List, error) {
	cfg := s.cfg()
	if !configured(cfg) {
		return nil, ErrNotConfigured
	}
	lists, err := s.client.Lists(ctx, cfg.creds())
	if err != nil {
		if !errors.Is(err, ErrLoginFailed) {
			s.log.Warn("bring lists failed", "error", redact(err, cfg.Password))
		}
		return nil, err
	}
	return lists, nil
}

// userMessage maps a refresh error to the German message of the dashboard.
func userMessage(err error) string {
	if errors.Is(err, ErrLoginFailed) {
		return MsgLoginFailed
	}
	return MsgLoadFailed
}

// redact makes sure the password never reaches the log, even if an error text
// should contain it.
func redact(err error, secret string) string {
	msg := err.Error()
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "***")
}
