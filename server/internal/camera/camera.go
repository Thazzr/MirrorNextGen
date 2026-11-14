// Package camera connects the mirror to its internal go2rtc service: it
// registers the configured RTSP stream there and hands the video to the
// browser through a WebSocket reverse proxy.
//
// go2rtc's API has no authentication and can run arbitrary commands
// (exec: sources), so it must stay inside the Docker network and this package
// only ever registers rtsp:// and rtsps:// addresses under one fixed name. The
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

// ErrUnsupportedURL is returned for a stream address that is not rtsp:// or
// rtsps://. The settings validate this already; the client checks again
// because the camera service would treat other schemes as commands.
var ErrUnsupportedURL = errors.New("stream address is not an rtsp or rtsps address")

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
	// Resync is how often a successful registration is repeated, so a camera
	// service that lost its state gets the stream again; 0 means 5 minutes.
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
// start and after every Apply, retries with backoff while go2rtc is down and
// repeats a successful registration now and then.
func (s *Service) Run(ctx context.Context) {
	attempt := 0
	for {
		wait := s.resync
		if err := s.Sync(ctx); err != nil {
			wait = s.backoff(attempt)
			attempt++
			// err never contains the stream address (see request).
			s.log.Warn("camera service not updated, will retry", "error", err, "retryIn", wait.String())
		} else {
			attempt = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.notify:
			timer.Stop()
			attempt = 0
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
	return s.request(ctx, http.MethodPut, url.Values{"name": {StreamName}, "src": {cfg.StreamURL}})
}

func supportedScheme(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.HasPrefix(lower, "rtsp://") || strings.HasPrefix(lower, "rtsps://")
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

// request calls go2rtc's /api/streams. Errors carry the status code or the
// transport cause only: the request URL holds the stream address in its query
// and url.Error would print it.
func (s *Service) request(ctx context.Context, method string, query url.Values) error {
	u := s.endpoint("api/streams", query)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return errors.New("build camera service request")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("%s camera stream: %w", method, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s camera stream: camera service answered %d", method, resp.StatusCode)
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
