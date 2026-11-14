package kiosk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const fetchTimeout = 5 * time.Second

// Commander runs an external program with a fixed argument list (never a
// shell) and returns its standard output.
type Commander interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type kioskStateBody struct {
	Rotation int    `json:"rotation"`
	Output   string `json:"output"`
}

// FetchState reads the target state from the mirror server. baseURL ends in "/".
func FetchState(ctx context.Context, client *http.Client, baseURL string) (State, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"api/v1/kiosk/state", nil)
	if err != nil {
		return State{}, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return State{}, fmt.Errorf("fetch kiosk state: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return State{}, fmt.Errorf("fetch kiosk state: unexpected status %d", resp.StatusCode)
	}
	var body kioskStateBody
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return State{}, fmt.Errorf("decode kiosk state: %w", err)
	}
	return State{Rotation: body.Rotation, Output: body.Output}, nil
}

// WaitHealthy blocks until GET <baseURL>api/v1/health answers 200. It retries
// every interval, logs a waiting message every logEvery, and has no timeout of
// its own; only ctx ends it early.
func WaitHealthy(ctx context.Context, client *http.Client, baseURL string, interval, logEvery time.Duration, log *slog.Logger) error {
	lastLog := time.Now()
	for {
		err := checkHealth(ctx, client, baseURL+"api/v1/health")
		if err == nil {
			return nil
		}
		if time.Since(lastLog) >= logEvery {
			log.Info("waiting for the mirror server", "url", baseURL, "error", err)
			lastLog = time.Now()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func checkHealth(ctx context.Context, client *http.Client, url string) error {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Agent applies the server's target state to the compositor.
type Agent struct {
	Fetch     func(ctx context.Context) (State, error)
	Commander Commander
	Log       *slog.Logger

	applied *State
}

// Tick runs one reconcile pass. A failure leaves the applied state untouched,
// so the next tick retries.
func (a *Agent) Tick(ctx context.Context) error {
	desired, err := a.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("get target state: %w", err)
	}
	if _, err := Transform(desired.Rotation); err != nil {
		return fmt.Errorf("target state: %w", err)
	}
	out, err := a.Commander.Run(ctx, "wlr-randr", "--json")
	if err != nil {
		return fmt.Errorf("list outputs: %w", err)
	}
	name, err := PickOutput(out, desired.Output)
	if err != nil {
		return err
	}
	plan := Plan(desired, a.applied, name)
	for _, args := range plan {
		if _, err := a.Commander.Run(ctx, "wlr-randr", args...); err != nil {
			return fmt.Errorf("wlr-randr %v: %w", args, err)
		}
	}
	if len(plan) > 0 {
		a.Log.Info("applied screen state", "rotation", desired.Rotation, "output", name)
	}
	a.applied = &State{Rotation: desired.Rotation, Output: name}
	return nil
}

// Run ticks once immediately and then on every value of ticks until ctx ends.
// Errors are logged and never stop the loop.
func (a *Agent) Run(ctx context.Context, ticks <-chan time.Time) {
	for {
		if err := a.Tick(ctx); err != nil && ctx.Err() == nil {
			a.Log.Warn("cannot apply screen state, will retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}
