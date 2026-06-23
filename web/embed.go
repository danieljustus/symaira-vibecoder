// Package web embeds the built Baukasten board so symvibe ships as a single
// binary with no Node toolchain required at runtime. The committed
// dist/index.html is a dependency-free board that works immediately; running
// `make web` (Vite) overwrites dist/ with the richer React build.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded board filesystem, rooted at dist/.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// dist/index.html is committed, so this cannot fail in a built binary.
		panic("web: embedded dist missing: " + err.Error())
	}
	return sub
}
