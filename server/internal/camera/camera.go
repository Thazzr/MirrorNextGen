// Package camera connects the mirror to its internal go2rtc service: it
// registers the configured RTSP stream there and hands the video to the
// browser through a WebSocket reverse proxy.
//
// go2rtc's API has no authentication and can run arbitrary commands
// (exec: sources), so it must stay inside the Docker network and this package
// only ever registers rtsp://, rtsps:// and rtspx:// addresses under one fixed name. The
// stream address usually contains credentials: it is sent to go2rtc and
// nowhere else, and never appears in a log line or an error.
package camera

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"
)

// StreamName is the one stream name the mirror registers and proxies.
const StreamName = "mirror-camera"

// ErrUnsupportedURL is returned for a stream address that is not rtsp://,
// rtsps:// or rtspx://. The settings validate this already; the client checks again
// because the camera service would treat other schemes as commands.
var ErrUnsupportedURL = errors.New("stream address is not an rtsp, rtsps or rtspx address")

// Config is the part of the camera settings the service needs.
type Config struct {
	Enabled   bool
	StreamURL string
}

func (c Config) configured() bool { return c.Enabled && c.StreamURL != "" }

// Options configure a Service.
type Options struct {
	// HTTPClient talks to go2rtc; it needs a timeout.
	HTTPClient *http.Client
	// BaseURL is the address of go2rtc's API, e.g. http://camera:1984/.
	BaseURL string
	// Config returns the current settings.
	Config func() Config
	Logger *slog.Logger
	// Backoff is the wait before retry number attempt (0-based) while go2rtc
	// is not reachable; nil uses 1 s doubling up to 30 s.
	Backoff func(attempt int) time.Duration
	// Resync is how often a successful registration is checked (Ensure, a
	// read-only GET), so a camera service that lost its state gets the
	// stream again; 0 means 5 minutes. It is not re-registered while go2rtc
	// still knows it: every PUT puts a new stream object
	// in go2rtc's map (a running consumer stays on the orphaned old one) and
	// rewrites its config file on the SD card.
	Resync time.Duration
}

// Service registers the stream with go2rtc and proxies the video.
type Service struct {
	client  *http.Client
	base    *url.URL
	config  func() Config
	log     *slog.Logger
	backoff func(int) time.Duration
	resync  time.Duration
	notify  chan struct{}
}

// New returns a Service; it fails on an unusable BaseURL.
func New(o Options) (*Service, error) {
	base, err := url.Parse(o.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, errors.New("camera service address must be an http(s) URL with a host")
	}
	s := &Service{client: o.HTTPClient, base: base, config: o.Config, log: o.Logger, backoff: o.Backoff, resync: o.Resync, notify: make(chan struct{}, 1)}
	if s.client == nil {
		s.client = &http.Client{Timeout: 10 * time.Second}
	}
	if s.log == nil {
		s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if s.backoff == nil {
		s.backoff = defaultBackoff
	}
	if s.resync <= 0 {
		s.resync = 5 * time.Minute
	}
	return s, nil
}

func defaultBackoff(attempt int) time.Duration {
	d := time.Second << min(attempt, 5)
	return min(d, 30*time.Second)
}

// Apply asks Run to register the current settings again; it never blocks.
func (s *Service) Apply() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// Run keeps go2rtc in step with the settings until ctx ends. It registers on
// start and after every Apply (Sync), retries with backoff while go2rtc is
// down (Sync again) and, once a pass succeeded and Resync has passed, only
// checks that go2rtc still knows the stream (Ensure). A failed check is
// followed by backoff and a full Sync.
func (s *Service) Run(ctx context.Context) {
	attempt := 0
	checkOnly := false // the next pass is the periodic check, not a registration
	for {
		wait := s.resync
		var err error
		if checkOnly {
			err = s.Ensure(ctx)
		} else {
			err = s.Sync(ctx)
		}
		checkOnly = false
		if err != nil && !s.config().configured() {
			// Nothing to register and go2rtc is not running: that is the normal
			// state for a mirror without the camera service, so no backoff and no warning.
			s.log.Debug("camera service not reachable, camera is not configured", "error", err)
		} else if err != nil {
			wait = s.backoff(attempt)
			attempt++
			// err never contains the stream address (see request).
			s.log.Warn("camera service not updated, will retry", "error", err, "retryIn", wait.String())
		} else {
			attempt = 0
			checkOnly = true
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.notify:
			timer.Stop()
			attempt = 0
			checkOnly = false
		case <-timer.C:
		}
	}
}

// Sync makes go2rtc match the current settings once: register the stream
// while the camera is configured, remove it otherwise.
func (s *Service) Sync(ctx context.Context) error {
	cfg := s.config()
	if !cfg.configured() {
		return s.request(ctx, http.MethodDelete, url.Values{"src": {StreamName}})
	}
	if !supportedScheme(cfg.StreamURL) {
		return ErrUnsupportedURL
	}
	return s.request(ctx, http.MethodPut, url.Values{"name": {StreamName}, "src": {go2rtcSource(cfg.StreamURL)}})
}

// Ensure is the read-only counterpart of Sync: it asks go2rtc whether it
// knows the stream and only acts when that disagrees with the settings,
// registering a missing stream or removing one that is no longer wanted.
// Unlike a repeated PUT it leaves a running stream and go2rtc's config file
// alone.
func (s *Service) Ensure(ctx context.Context) error {
	cfg := s.config()
	if cfg.configured() && !supportedScheme(cfg.StreamURL) {
		return ErrUnsupportedURL
	}
	status, err := s.do(ctx, http.MethodGet, url.Values{"src": {StreamName}})
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		if cfg.configured() {
			return nil
		}
		return s.request(ctx, http.MethodDelete, url.Values{"src": {StreamName}})
	case http.StatusNotFound:
		if cfg.configured() {
			return s.Sync(ctx)
		}
		return nil
	default:
		return fmt.Errorf("GET camera stream: camera service answered %d", status)
	}
}

func supportedScheme(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.HasPrefix(lower, "rtsp://") || strings.HasPrefix(lower, "rtsps://") || strings.HasPrefix(lower, "rtspx://")
}

// go2rtcSource is the stream address as go2rtc gets it: without the
// enableSrtp query parameter, which UniFi Protect adds to its links but which
// go2rtc's docs say not to use. Everything else stays byte for byte (the
// token in the path is case-sensitive); the stored settings are not touched.
func go2rtcSource(raw string) string {
	base, query, found := strings.Cut(raw, "?")
	if !found {
		return raw
	}
	var kept []string
	for _, pair := range strings.Split(query, "&") {
		key, _, _ := strings.Cut(pair, "=")
		if k, err := url.QueryUnescape(key); err == nil {
			key = k
		}
		if strings.EqualFold(key, "enableSrtp") {
			continue
		}
		kept = append(kept, pair)
	}
	if len(kept) == 0 {
		return base
	}
	return base + "?" + strings.Join(kept, "&")
}

// endpoint is the go2rtc URL of an API path below the base URL, with the
// path always absolute (a base without a trailing slash must still work).
func (s *Service) endpoint(apiPath string, query url.Values) *url.URL {
	u := *s.base
	u.Path = path.Join("/", s.base.Path, apiPath)
	u.RawPath = ""
	u.RawQuery = query.Encode()
	return &u
}

// do calls go2rtc's /api/streams and returns the status code. Errors carry the
// transport cause only: the request URL holds the stream address in its query
// and url.Error would print it. The body (which may echo the address) is read
// in a bounded way and dropped.
func (s *Service) do(ctx context.Context, method string, query url.Values) (int, error) {
	u := s.endpoint("api/streams", query)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return 0, errors.New("build camera service request")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return 0, fmt.Errorf("%s camera stream: %w", method, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, nil
}

// request calls go2rtc and treats anything but 2xx as an error carrying the
// method and status code only.
func (s *Service) request(ctx context.Context, method string, query url.Values) error {
	status, err := s.do(ctx, method, query)
	if err != nil {
		return err
	}
	if status/100 != 2 {
		return fmt.Errorf("%s camera stream: camera service answered %d", method, status)
	}
	return nil
}

// Handler returns the WebSocket proxy of the live video. It forwards to
// go2rtc's /api/ws for the fixed StreamName; nothing of the client's query
// reaches go2rtc, so a client cannot pick another source. writeProblem
// answers the cases that are not proxied.
func (s *Service) Handler(writeProblem func(w http.ResponseWriter, status int, detail string)) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL = s.endpoint("api/ws", url.Values{"src": {StreamName}})
			pr.Out.Host = pr.Out.URL.Host
			// The mirror's session and any other credentials are not for
			// go2rtc, and its origin check would reject the browser's origin.
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Del("Origin")
		},
		ErrorLog: log.New(io.Discard, "", 0),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.log.Warn("camera proxy failed", "error", err)
			writeProblem(w, http.StatusBadGateway, "Die Kamera ist nicht erreichbar.")
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config().configured() {
			writeProblem(w, http.StatusNotFound, "Die Kamera ist nicht eingerichtet.")
			return
		}
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			w.Header().Set("Upgrade", "websocket")
			writeProblem(w, http.StatusUpgradeRequired, "Dieser Endpunkt spricht nur WebSocket.")
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
