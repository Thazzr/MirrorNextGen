// Package httpserver composes the API routes and the embedded web app.
package httpserver

import (
	"io/fs"
	"net/http"

	"github.com/thazzr/mirrornextgen/server/internal/api"
	"github.com/thazzr/mirrornextgen/server/internal/web"
)

// New returns the complete handler of the server.
func New(deps api.Deps, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	api.Register(mux, deps)
	// Unknown API paths must answer a JSON 404, never the SPA's index.html.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteProblem(w, http.StatusNotFound, "")
	})
	mux.Handle("/", web.Handler(webFS))
	return noSniff(mux)
}

func noSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
