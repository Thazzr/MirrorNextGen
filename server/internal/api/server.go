package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// BuildInfo describes the running binary; main fills it from -ldflags.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// Server implements the generated StrictServerInterface.
type Server struct {
	build BuildInfo
}

var _ StrictServerInterface = (*Server)(nil)

// NewServer returns a Server that reports the given build information.
func NewServer(build BuildInfo) *Server {
	return &Server{build: build}
}

// GetHealth answers the liveness check.
func (s *Server) GetHealth(_ context.Context, _ GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{Status: Ok}, nil
}

// GetInfo returns the build information.
func (s *Server) GetInfo(_ context.Context, _ GetInfoRequestObject) (GetInfoResponseObject, error) {
	return GetInfo200JSONResponse{
		Version:   s.build.Version,
		Commit:    s.build.Commit,
		BuildTime: s.build.BuildTime,
	}, nil
}

// WriteProblem writes a Problem as application/problem+json.
func WriteProblem(w http.ResponseWriter, status int, detail string) {
	p := Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	// A failed write means the client is gone; there is nothing left to report.
	_ = json.NewEncoder(w).Encode(p)
}

// Register mounts the API routes on mux. Request errors answer 400 and
// handler errors 500, both as problem+json; the Go error text of a 500 is
// only logged, never sent to the client.
func Register(mux *http.ServeMux, build BuildInfo, logger *slog.Logger) {
	requestError := func(w http.ResponseWriter, _ *http.Request, err error) {
		WriteProblem(w, http.StatusBadRequest, err.Error())
	}
	responseError := func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("api request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		WriteProblem(w, http.StatusInternalServerError, "")
	}
	strict := NewStrictHandlerWithOptions(NewServer(build), nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestError,
		ResponseErrorHandlerFunc: responseError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: requestError,
	})
}

// NewHandler returns a handler serving only the API routes.
func NewHandler(build BuildInfo, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	Register(mux, build, logger)
	return mux
}
