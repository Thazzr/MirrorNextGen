package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler(t *testing.T) http.Handler {
	return newTestEnv(t).handler
}

func TestEndpoints(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"health", "/api/v1/health", `{"status":"ok"}`},
		{"info", "/api/v1/info", `{"buildTime":"2026-01-02T03:04:05Z","commit":"abc123","version":"1.2.3"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestWrongMethodIsRejected(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestWriteProblem(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		detail     string
		wantDetail bool
	}{
		{"without detail", http.StatusInternalServerError, "", false},
		{"with detail", http.StatusBadRequest, "bad input", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteProblem(rec, tt.status, tt.detail)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type = %q", ct)
			}
			var p Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Status != tt.status || p.Title != http.StatusText(tt.status) {
				t.Errorf("problem = %+v", p)
			}
			if (p.Detail != nil) != tt.wantDetail {
				t.Errorf("detail = %v, want present=%v", p.Detail, tt.wantDetail)
			}
		})
	}
}
