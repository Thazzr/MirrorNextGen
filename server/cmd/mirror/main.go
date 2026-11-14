// Command mirror is the MirrorNextGen server: API plus embedded web app.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // the distroless image has no zoneinfo

	"github.com/thazzr/mirrornextgen/server/internal/api"
	"github.com/thazzr/mirrornextgen/server/internal/auth"
	"github.com/thazzr/mirrornextgen/server/internal/bring"
	"github.com/thazzr/mirrornextgen/server/internal/calendar"
	"github.com/thazzr/mirrornextgen/server/internal/camera"
	"github.com/thazzr/mirrornextgen/server/internal/config"
	"github.com/thazzr/mirrornextgen/server/internal/display"
	"github.com/thazzr/mirrornextgen/server/internal/httpserver"
	"github.com/thazzr/mirrornextgen/server/internal/icons"
	"github.com/thazzr/mirrornextgen/server/internal/logbook"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
	"github.com/thazzr/mirrornextgen/server/internal/update"
	"github.com/thazzr/mirrornextgen/server/internal/weather"
	"github.com/thazzr/mirrornextgen/server/internal/web"
)

// Set at build time: -ldflags "-X main.version=… -X main.commit=… -X main.buildTime=…".
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = ""
)

const (
	shutdownTimeout    = 10 * time.Second
	healthcheckTimeout = 3 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is main without process globals; it returns the exit code.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	// Everything still goes to stdout; the logbook keeps the last records for
	// the admin page.
	book := logbook.New()
	logger := slog.New(book.Handler(slog.NewJSONHandler(stdout, nil)))

	cfg, err := config.Load(getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}

	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck(ctx, cfg.Addr, stderr)
	}

	// A corrupt admin.json must stop the server: starting as "not set up"
	// would let anyone run the setup and take over the mirror.
	admin, err := auth.Load(cfg.DataDir)
	if err != nil {
		logger.Error("cannot load admin credentials", "dataDir", cfg.DataDir, "error", err)
		return 1
	}
	// Bad settings content never stops the server (it is moved aside); only
	// an I/O failure does.
	settingsStore, err := settings.Load(cfg.DataDir, logger)
	if err != nil {
		logger.Error("cannot load settings", "dataDir", cfg.DataDir, "error", err)
		return 1
	}
	weatherSvc := weather.New(weather.Options{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		BaseURL:    cfg.OpenWeatherURL,
		Config: func() weather.Config {
			w := settingsStore.Get().Weather
			return weather.Config{Location: w.Location, APIKey: w.APIKey, ShowIcons: w.ShowIcons, RefreshMinutes: w.RefreshMinutes}
		},
		Now:      time.Now,
		Location: time.Local,
		Logger:   logger,
	})
	bringSvc := bring.New(bring.Options{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		BaseURL:    cfg.BringURL,
		Config: func() bring.Config {
			b := settingsStore.Get().Bring
			return bring.Config{Email: b.Email, Password: b.Password, ListUUID: b.ListUUID, MaxItems: b.MaxItems, RefreshMinutes: b.RefreshMinutes}
		},
		Now:    time.Now,
		Logger: logger,
	})
	iconStore, err := icons.Load(cfg.DataDir)
	if err != nil {
		logger.Error("cannot load icons", "dataDir", cfg.DataDir, "error", err)
		return 1
	}
	settingsStore.SetIconChecker(iconStore.Exists)
	calendarSvc := calendar.NewService(calendar.ServiceOptions{
		Feeds:    calendar.NewFeeds(calendar.FeedsOptions{HTTPClient: &http.Client{Timeout: 15 * time.Second}, Now: time.Now, Logger: logger}),
		Now:      time.Now,
		Location: time.Local,
		Logger:   logger,
	})
	displayCtl := display.NewController(display.Options{
		Schedule: func() display.Schedule { return settingsStore.Get().Display.Schedule },
		Now:      time.Now,
		Location: mirrorLocation(getenv("TZ")),
	})
	cameraSvc, err := camera.New(camera.Options{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		BaseURL:    cfg.Go2RTCURL,
		Config: func() camera.Config {
			c := settingsStore.Get().Camera
			return camera.Config{Enabled: c.Enabled, StreamURL: c.StreamURL}
		},
		Logger: logger,
	})
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	deps := api.Deps{
		Build:    api.BuildInfo{Version: version, Commit: commit, BuildTime: buildTime},
		Logger:   logger,
		Admin:    admin,
		Sessions: auth.NewSessions(time.Now),
		Throttle: auth.NewThrottle(time.Now),
		Settings: settingsStore,
		Weather:  weatherSvc,
		Bring:    bringSvc,
		Calendar: calendarSvc,
		Icons:    iconStore,
		Display:  displayCtl,
		Camera:   cameraSvc,
		Logbook:  book,
		Update:   update.New(cfg.ControlDir),
		Now:      time.Now,
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpserver.New(deps, web.Dist()),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Error("cannot listen", "addr", cfg.Addr, "error", err)
		return 1
	}
	logger.Info("listening", "addr", listener.Addr().String(), "version", version, "commit", commit, "dataDir", cfg.DataDir)

	// Owned by this function: ends with ctx (a failing start returns first and
	// cancels it through the deferred stop).
	cameraCtx, stopCamera := context.WithCancel(ctx)
	defer stopCamera()
	go cameraSvc.Run(cameraCtx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	select {
	case err := <-serveErr:
		logger.Error("server stopped", "error", err)
		return 1
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return 1
	}
	return 0
}

// mirrorLocation is the mirror's time zone: TZ by name when it loads (so the
// admin can show the name; time.Local reports only "Local"), else the process
// default.
func mirrorLocation(tz string) *time.Location {
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.Local
}

func healthcheck(ctx context.Context, addr string, stderr io.Writer) int {
	url, err := healthcheckURL(addr)
	if err == nil {
		err = runHealthcheck(ctx, &http.Client{Timeout: healthcheckTimeout}, url)
	}
	if err != nil {
		fmt.Fprintln(stderr, "healthcheck failed:", err)
		return 1
	}
	return 0
}

// healthcheckURL turns the listen address into the URL of the health endpoint
// on the local machine; wildcard hosts are probed through loopback.
func healthcheckURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid address %q: %w", addr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/api/v1/health", nil
}

// runHealthcheck succeeds only for a 200 answer whose JSON status is "ok".
func runHealthcheck(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var body api.Health
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	if body.Status != api.Ok {
		return errors.New("health status is " + string(body.Status))
	}
	return nil
}
