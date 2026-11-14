// Package web serves the embedded single-page app.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
)

const (
	notBuiltHint = "Die Oberfläche ist nicht gebaut. Führe im Ordner web \"npm run build\" aus."
	indexFile    = "index.html"
)

// hashedAsset matches Vite's content-hashed file names, e.g. index-AbCd1234.js.
var hashedAsset = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.[A-Za-z0-9]+$`)

// Handler serves fsys as a single-page app: existing files as they are,
// unknown extension-less paths as index.html so client-side routes survive a
// reload, unknown file-like paths as 404.
func Handler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.Trim(path.Clean("/"+r.URL.Path), "/")
		if hasDotSegment(name) {
			notFound(w)
			return
		}

		if !isFile(fsys, indexFile) {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			if r.Method != http.MethodHead {
				_, _ = w.Write([]byte(notBuiltHint))
			}
			return
		}

		if name == "" {
			serve(w, r, fsys, indexFile)
			return
		}
		if isFile(fsys, name) {
			serve(w, r, fsys, name)
			return
		}
		if strings.Contains(path.Base(name), ".") {
			notFound(w)
			return
		}
		serve(w, r, fsys, indexFile)
	})
}

func serve(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	if isHashedAsset(name) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	// ServeFileFS redirects a request path ending in "/index.html" to "./";
	// the path is only used for that check, so a neutral one keeps the SPA
	// fallback and a direct /index.html request from redirecting.
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/"
	http.ServeFileFS(w, r2, fsys, name)
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache")
	http.Error(w, "404 page not found", http.StatusNotFound)
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}

func hasDotSegment(name string) bool {
	for _, seg := range strings.Split(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func isHashedAsset(name string) bool {
	return strings.HasPrefix(name, "assets/") && hashedAsset.MatchString(path.Base(name))
}
