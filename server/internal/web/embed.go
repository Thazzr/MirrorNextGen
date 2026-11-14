package web

import (
	"embed"
	"io/fs"
)

// The Vite build writes into dist/; the directory itself is kept in git by
// dist/.gitignore, which is why the pattern needs "all:".
//
//go:embed all:dist
var distFS embed.FS

// Dist returns the built single-page app, rooted at dist/.
func Dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Unreachable: "dist" is embedded by the directive above.
		panic(err)
	}
	return sub
}
