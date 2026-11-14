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
	"github.com/thazzr/mirrornextgen/server/internal/config"
	"github.com/thazzr/mirrornextgen/server/internal/httpserver"
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
	logger := slog.New(slog.NewJSONHandler(stdout, nil))

	cfg, err := config.Load(getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}

	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck(ctx, cfg.Addr, stderr)
	}

	build := api.BuildInfo{Version: version, Commit: commit, BuildTime: buildTime}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpserver.New(build, web.Dist(), logger),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Error("cannot listen", "addr", cfg.Addr, "error", err)
		return 1
	}
	logger.Info("listening", "addr", listener.Addr().String(), "version", version, "commit", commit, "dataDir", cfg.DataDir)

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
