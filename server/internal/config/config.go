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
)

// Config holds the runtime configuration of the server.
type Config struct {
	// Addr is the listen address (MIRROR_ADDR).
	Addr string
	// DataDir is the directory for persistent state (MIRROR_DATA_DIR).
	DataDir string
}

// Load reads the configuration through getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:    valueOr(getenv("MIRROR_ADDR"), defaultAddr),
		DataDir: valueOr(getenv("MIRROR_DATA_DIR"), defaultDataDir),
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
