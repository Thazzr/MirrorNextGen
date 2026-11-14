// Package config reads the server configuration from the environment.
package config

import (
	"fmt"
	"net"
	"strings"
)

const (
	defaultAddr    = ":8080"
	defaultDataDir = "/data"

	defaultOpenWeatherURL = "https://api.openweathermap.org/"
	defaultBringURL       = "https://api.getbring.com/rest/v2/"
	defaultGo2RTCURL      = "http://camera:1984/"
)

// Config holds the runtime configuration of the server.
type Config struct {
	// Addr is the listen address (MIRROR_ADDR).
	Addr string
	// DataDir is the directory for persistent state (MIRROR_DATA_DIR).
	DataDir string
	// OpenWeatherURL is the base URL of the OpenWeather API
	// (MIRROR_OPENWEATHER_URL); only tests and smoke runs change it.
	OpenWeatherURL string
	// BringURL is the base URL of the Bring! API (MIRROR_BRING_URL); only
	// tests and smoke runs change it.
	BringURL string
	// Go2RTCURL is the base URL of the internal go2rtc service
	// (MIRROR_GO2RTC_URL). It must never be reachable from the LAN.
	Go2RTCURL string
}

// Load reads the configuration through getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:    valueOr(getenv("MIRROR_ADDR"), defaultAddr),
		DataDir: valueOr(getenv("MIRROR_DATA_DIR"), defaultDataDir),

		OpenWeatherURL: valueOr(getenv("MIRROR_OPENWEATHER_URL"), defaultOpenWeatherURL),
		BringURL:       valueOr(getenv("MIRROR_BRING_URL"), defaultBringURL),
		Go2RTCURL:      valueOr(getenv("MIRROR_GO2RTC_URL"), defaultGo2RTCURL),
	}
	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		return Config{}, fmt.Errorf("MIRROR_ADDR %q is not a valid host:port address: %w", cfg.Addr, err)
	}
	return cfg, nil
}

func valueOr(v, def string) string {
	if v = strings.TrimSpace(v); v == "" {
		return def
	}
	return v
}
