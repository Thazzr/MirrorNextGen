package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/thazzr/mirrornextgen/server/internal/auth"
	"github.com/thazzr/mirrornextgen/server/internal/bring"
	"github.com/thazzr/mirrornextgen/server/internal/calendar"
	"github.com/thazzr/mirrornextgen/server/internal/camera"
	"github.com/thazzr/mirrornextgen/server/internal/display"
	"github.com/thazzr/mirrornextgen/server/internal/icons"
	"github.com/thazzr/mirrornextgen/server/internal/logbook"
	"github.com/thazzr/mirrornextgen/server/internal/settings"
	"github.com/thazzr/mirrornextgen/server/internal/update"
	"github.com/thazzr/mirrornextgen/server/internal/weather"
)

// BuildInfo describes the running binary; main fills it from -ldflags.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// Deps are the collaborators of the API server.
type Deps struct {
	Build    BuildInfo
	Logger   *slog.Logger
	Admin    *auth.Store
	Sessions *auth.Sessions
	Throttle *auth.Throttle
	Settings *settings.Store
	Weather  *weather.Service
	Bring    *bring.Service
	Calendar *calendar.Service
	Icons    *icons.Store
	Display  *display.Controller
	// Camera registers the stream with go2rtc and proxies the video; nil
	// switches the camera routes off (tests).
	Camera *camera.Service
	// Logbook holds the recent log records; nil serves an empty logbook.
	Logbook *logbook.Logbook
	// Update talks to the host's updater; nil means updates by button are
	// unavailable.
	Update *update.Service
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Server implements the generated StrictServerInterface.
type Server struct {
	build    BuildInfo
	admin    *auth.Store
	sessions *auth.Sessions
	throttle *auth.Throttle
	settings *settings.Store
	weather  *weather.Service
	bring    *bring.Service
	calendar *calendar.Service
	icons    *icons.Store
	display  *display.Controller
	camera   *camera.Service
	logbook  *logbook.Logbook
	update   *update.Service
	now      func() time.Time
	logger   *slog.Logger
	// iconMu serialises icon deletes with calendar updates, so an icon cannot
	// be deleted while a settings change that references it is being stored.
	iconMu sync.Mutex
}

var _ StrictServerInterface = (*Server)(nil)

// NewServer returns a Server wired to deps.
func NewServer(deps Deps) *Server {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	upd := deps.Update
	if upd == nil {
		upd = update.New("")
	}
	return &Server{logbook: deps.Logbook, update: upd, now: now, logger: deps.Logger, build: deps.Build, admin: deps.Admin, sessions: deps.Sessions, throttle: deps.Throttle, settings: deps.Settings, weather: deps.Weather, bring: deps.Bring, calendar: deps.Calendar, icons: deps.Icons, display: deps.Display, camera: deps.Camera}
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
func Register(mux *http.ServeMux, deps Deps) {
	register(mux, deps)
	if deps.Camera != nil {
		// Not a strict-server operation (it upgrades to a WebSocket and is
		// proxied), so the auth middleware does not see it. It is public on
		// purpose: the dashboard has no session. The handler only forwards to
		// the one fixed stream and answers 404 while no camera is configured.
		mux.Handle("GET /api/v1/camera/stream", deps.Camera.Handler(WriteProblem))
	}
}

// register is Register for any router; extra middlewares wrap the auth
// middleware (tests use that to observe operation IDs).
func register(router ServeMux, deps Deps, extra ...StrictMiddlewareFunc) {
	requestError := func(w http.ResponseWriter, _ *http.Request, err error) {
		WriteProblem(w, http.StatusBadRequest, err.Error())
	}
	responseError := func(w http.ResponseWriter, r *http.Request, err error) {
		deps.Logger.Error("api request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		WriteProblem(w, http.StatusInternalServerError, "")
	}
	srv := NewServer(deps)
	middlewares := append([]StrictMiddlewareFunc{srv.authMiddleware}, extra...)
	strict := NewStrictHandlerWithOptions(srv, middlewares, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestError,
		ResponseErrorHandlerFunc: responseError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: requestError,
	})
}

// NewHandler returns a handler serving only the API routes.
func NewHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	Register(mux, deps)
	return mux
}
