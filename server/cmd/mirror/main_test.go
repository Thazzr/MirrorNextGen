package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthcheckURL(t *testing.T) {
	tests := []struct {
		addr    string
		want    string
		wantErr bool
	}{
		{":8080", "http://127.0.0.1:8080/api/v1/health", false},
		{"0.0.0.0:9000", "http://127.0.0.1:9000/api/v1/health", false},
		{"[::]:8080", "http://127.0.0.1:8080/api/v1/health", false},
		{"192.168.1.5:8080", "http://192.168.1.5:8080/api/v1/health", false},
		{"[fe80::1]:8080", "http://[fe80::1]:8080/api/v1/health", false},
		{"8080", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got, err := healthcheckURL(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("url = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunHealthcheck(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"ok", 200, `{"status":"ok"}`, false},
		{"server error", 500, `{"status":"ok"}`, true},
		{"other status value", 200, `{"status":"degraded"}`, true},
		{"missing status", 200, `{}`, true},
		{"invalid json", 200, `not json`, true},
		{"empty body", 200, ``, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			err := runHealthcheck(context.Background(), srv.Client(), srv.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunHealthcheckConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	if err := runHealthcheck(context.Background(), &http.Client{Timeout: time.Second}, url); err == nil {
		t.Fatal("expected an error for a closed server")
	}
}

func TestRunHealthcheckSubcommandExitCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	addr := strings.TrimPrefix(srv.URL, "http://")
	getenv := func(k string) string {
		if k == "MIRROR_ADDR" {
			return addr
		}
		return ""
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"healthcheck"}, getenv, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d (stderr %q), want 0", code, stderr.String())
	}

	srv.Close()
	stderr.Reset()
	if code := run(context.Background(), []string{"healthcheck"}, getenv, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected the reason on stderr")
	}
}

func TestRunRejectsInvalidConfig(t *testing.T) {
	getenv := func(k string) string {
		if k == "MIRROR_ADDR" {
			return "nonsense"
		}
		return ""
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, getenv, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "MIRROR_ADDR") {
		t.Errorf("log %q does not name MIRROR_ADDR", stdout.String())
	}
}
