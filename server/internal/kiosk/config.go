// Package kiosk holds the pure logic of the kiosk agent (kioskd): configuration,
// browser and compositor command lines, and the decision which wlr-randr
// commands bring the screen to the state the server asks for.
package kiosk

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Rotation modes: who turns the picture.
const (
	// RotationCompositor lets kioskd rotate the output with wlr-randr.
	RotationCompositor = "compositor"
	// RotationBrowser leaves the output alone; the dashboard rotates itself.
	RotationBrowser = "browser"
)

const minPollInterval = time.Second

// Config is the agent's configuration, read from the environment.
type Config struct {
	URL          string // normalised: always ends in "/"
	PollInterval time.Duration
	DisableGPU   bool
	ShowCursor   bool // keep Cage's visible mouse arrow (debugging)
	Rotation     string
	ExtraFlags   []string
	User         string
}

// Load reads the configuration through getenv. An invalid value yields an
// error that names the variable.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		URL:          "http://mirror:8080/",
		PollInterval: 5 * time.Second,
		Rotation:     RotationCompositor,
		User:         "kiosk",
	}

	if v := getenv("KIOSK_URL"); v != "" {
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, fmt.Errorf("KIOSK_URL: %q is not an http(s) URL", v)
		}
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
		cfg.URL = u.String()
	}
	if v := getenv("KIOSK_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("KIOSK_POLL_INTERVAL: %w", err)
		}
		if d < minPollInterval {
			return Config{}, fmt.Errorf("KIOSK_POLL_INTERVAL: %s is below the minimum of %s", d, minPollInterval)
		}
		cfg.PollInterval = d
	}
	if v := getenv("KIOSK_DISABLE_GPU"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("KIOSK_DISABLE_GPU: %q is not a boolean", v)
		}
		cfg.DisableGPU = b
	}
	if v := getenv("KIOSK_SHOW_CURSOR"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("KIOSK_SHOW_CURSOR: %q is not a boolean", v)
		}
		cfg.ShowCursor = b
	}
	if v := getenv("KIOSK_ROTATION"); v != "" {
		if v != RotationCompositor && v != RotationBrowser {
			return Config{}, fmt.Errorf("KIOSK_ROTATION: %q is neither %q nor %q", v, RotationCompositor, RotationBrowser)
		}
		cfg.Rotation = v
	}
	// Split on whitespace only: the flags are never passed through a shell.
	cfg.ExtraFlags = strings.Fields(getenv("KIOSK_EXTRA_FLAGS"))
	if v := getenv("KIOSK_USER"); v != "" {
		cfg.User = v
	}
	return cfg, nil
}

// BrowserURL is the address Chromium opens. With browser rotation the
// dashboard learns about it through the query parameter.
func BrowserURL(cfg Config) string {
	if cfg.Rotation != RotationBrowser {
		return cfg.URL
	}
	u, err := url.Parse(cfg.URL)
	if err != nil { // Load validated it; keep a safe fallback for hand-built configs.
		return cfg.URL + "?rotation=browser"
	}
	q := u.Query()
	q.Set("rotation", "browser")
	u.RawQuery = q.Encode()
	return u.String()
}

// ChromiumArgs is the Chromium command line for the kiosk; the URL comes last.
func ChromiumArgs(cfg Config, pageURL string) []string {
	args := []string{
		"--kiosk",
		"--ozone-platform=wayland",
		"--noerrdialogs",
		"--disable-infobars",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-session-crashed-bubble",
		"--disable-translate",
		"--disable-features=Translate,TranslateUI",
		"--password-store=basic",
		"--user-data-dir=/tmp/kiosk-chromium",
		"--force-device-scale-factor=1",
		"--autoplay-policy=no-user-gesture-required",
		"--check-for-update-interval=31536000",
		"--overscroll-history-navigation=0",
		"--disable-pinch",
		"--hide-scrollbars",
	}
	if cfg.DisableGPU {
		args = append(args, "--disable-gpu")
	}
	args = append(args, cfg.ExtraFlags...)
	return append(args, pageURL)
}

// CageArgs is the argument list of the `cage` binary running Chromium.
func CageArgs(chromium []string) []string {
	args := []string{"-d", "-s", "-m", "last", "--", "chromium"}
	return append(args, chromium...)
}
