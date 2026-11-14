// Package weather fetches the weather from OpenWeather One Call 3.0 and turns
// it into what the dashboard shows: current values, a day chart, a three-day
// forecast and a short alert text.
package weather

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Messages shown on the dashboard; the details of a failure go to the log.
const (
	MsgLocationNotFound = "Der Ort wurde nicht gefunden."
	MsgNotEnabled       = "OpenWeather One Call 3.0 ist für diesen API-Key nicht freigeschaltet."
	MsgLoadFailed       = "Die Wetterdaten konnten nicht geladen werden."
)

const (
	// staleLimit is how old the last good data may be to be shown when a
	// refresh fails.
	staleLimit = 3 * time.Hour
	// failureRetry keeps a failing upstream from being called on every poll
	// of the dashboard.
	failureRetry = time.Minute
	// refreshTimeout bounds one refresh including its time machine calls.
	refreshTimeout = 30 * time.Second
	minRefresh     = 10 * time.Minute
)

// Config is the part of the settings the service needs.
type Config struct {
	Location       string
	APIKey         string
	ShowIcons      bool
	RefreshMinutes int
}

// Data is one successful refresh. It is never modified after creation.
type Data struct {
	Location                 string
	UpdatedAt                time.Time
	TemperatureC             int
	FeelsLikeC               int
	Humidity                 int
	Condition                string
	IconCode                 string
	WindKph                  int
	PrecipitationProbability int
	Sunrise                  *time.Time
	Sunset                   *time.Time
	UVIndex                  int
	Alert                    string
	Hourly                   []HourlyPoint
	Forecast                 []ForecastDay
}

// Result is what Get reports to the dashboard.
type Result struct {
	// Configured is false while location or API key is missing.
	Configured bool
	ShowIcons  bool
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
	// Now defaults to time.Now; Location to time.Local.
	Now      func() time.Time
	Location *time.Location
	Logger   *slog.Logger
}

// workCache holds what a refresh reuses. A new one is made whenever the
// location or key changes, so a refresh never mixes places.
type workCache struct {
	geo    *upstreamGeo
	hourly *hourlyCache
}

type call struct{ done chan struct{} }

// Service caches the weather and refreshes it at most every refresh interval.
// Concurrent callers share one refresh.
type Service struct {
	http    *http.Client
	baseURL string
	cfg     func() Config
	now     func() time.Time
	loc     *time.Location
	log     *slog.Logger

	mu          sync.Mutex
	key         [2]string // location and API key the state belongs to
	work        *workCache
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
	s := &Service{http: o.HTTPClient, baseURL: o.BaseURL, cfg: o.Config, now: o.Now, loc: o.Location, log: o.Logger}
	if s.http == nil {
		s.http = &http.Client{Timeout: 15 * time.Second}
	}
	if s.baseURL == "" {
		s.baseURL = DefaultBaseURL
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.loc == nil {
		s.loc = time.Local
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	return s
}

// Invalidate forgets the cached data; the next Get refreshes. The settings
// handler calls it after a change.
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

func (s *Service) resetLocked() {
	s.generation++
	s.work = &workCache{hourly: newHourlyCache()}
	s.data = nil
	s.fetchedAt = time.Time{}
	s.failedAt = time.Time{}
}

// Get returns the weather, refreshing it when it is older than the refresh
// interval.
func (s *Service) Get(ctx context.Context) Result {
	cfg := s.cfg()
	res := Result{ShowIcons: cfg.ShowIcons}
	if cfg.Location == "" || cfg.APIKey == "" {
		return res
	}
	res.Configured = true
	interval := max(time.Duration(cfg.RefreshMinutes)*time.Minute, minRefresh)

	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		now := s.now()
		if key := [2]string{cfg.Location, cfg.APIKey}; !s.initialized || key != s.key {
			s.key, s.initialized = key, true
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

// failureLocked reports the outcome of a failed refresh: recent data as
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
// with s.mu held and unlocks it while it talks to OpenWeather.
func (s *Service) refreshLocked(ctx context.Context, cfg Config, now time.Time) {
	c := &call{done: make(chan struct{})}
	s.inflight = c
	gen, work := s.generation, s.work
	s.mu.Unlock()

	// The refresh serves every waiting caller, so it must not die with the
	// request that happened to start it.
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), refreshTimeout)
	data, err := s.fetch(rctx, cfg, work, now)
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
		s.log.Warn("weather refresh failed", "error", redact(err, cfg.APIKey))
		return
	}
	s.data, s.fetchedAt, s.failedAt = data, now, time.Time{}
}

// userMessage maps a refresh error to the German message of the dashboard.
func userMessage(err error) string {
	var se *statusError
	switch {
	case errors.Is(err, errLocationNotFound):
		return MsgLocationNotFound
	case errors.As(err, &se) && (se.Code == http.StatusUnauthorized || se.Code == http.StatusForbidden):
		return MsgNotEnabled
	}
	return MsgLoadFailed
}

// redact makes sure a secret never reaches the log, even if an error text
// should contain it.
func redact(err error, secret string) string {
	msg := err.Error()
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "***")
}

// fetch performs one refresh against OpenWeather.
func (s *Service) fetch(ctx context.Context, cfg Config, work *workCache, now time.Time) (*Data, error) {
	cl := &client{http: s.http, baseURL: s.baseURL, apiKey: cfg.APIKey}
	if work.geo == nil {
		g, err := cl.geocode(ctx, cfg.Location)
		if err != nil {
			return nil, fmt.Errorf("geocode location: %w", err)
		}
		work.geo = &g
	}
	geo := *work.geo
	oc, err := cl.oneCall(ctx, geo)
	if err != nil {
		return nil, fmt.Errorf("one call: %w", err)
	}

	work.hourly.prune(now, s.loc)
	for _, h := range oc.Hourly {
		work.hourly.put(hourlyFromForecast(h, s.loc))
	}
	history := func(ctx context.Context, at time.Time) (HourlyPoint, bool, error) {
		p, ok, err := cl.timeMachine(ctx, geo, at)
		if err != nil || !ok {
			if err != nil {
				s.log.Warn("weather time machine failed", "error", redact(err, cfg.APIKey))
			}
			return HourlyPoint{}, false, err
		}
		return hourlyFromHistory(p, at), true, nil
	}
	chart, err := chartPoints(ctx, now, s.loc, work.hourly, history)
	if err != nil {
		return nil, fmt.Errorf("build chart: %w", err)
	}

	cur := oc.Current
	w := firstWeather(cur.Weather)
	name := geo.Name
	if name == "" {
		name = cfg.Location
	}
	d := &Data{
		Location:                 name,
		UpdatedAt:                now,
		TemperatureC:             roundInt(cur.Temp),
		FeelsLikeC:               roundInt(cur.FeelsLike),
		Humidity:                 cur.Humidity,
		Condition:                capitalize(w.Description),
		IconCode:                 w.Icon,
		WindKph:                  kph(cur.WindSpeed),
		PrecipitationProbability: currentPrecipitation(cur.Rain.OneHour, chart, now),
		Sunrise:                  unixTime(cur.Sunrise, s.loc),
		Sunset:                   unixTime(cur.Sunset, s.loc),
		UVIndex:                  roundInt(cur.UVI),
		Alert:                    weatherAlert(chart, now),
		Hourly:                   chart,
		Forecast:                 forecastDays(oc.Daily, now, s.loc),
	}
	return d, nil
}

func unixTime(sec int64, loc *time.Location) *time.Time {
	if sec <= 0 {
		return nil
	}
	t := time.Unix(sec, 0).In(loc)
	return &t
}
