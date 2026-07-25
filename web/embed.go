// Package web embeds all templates, static assets, and fonts into the binary
// so MikVoc ships as a single file with no external web/ folder required.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var assets embed.FS

// TemplatesFS returns the embedded templates/ subtree as an fs.FS.
func TemplatesFS() fs.FS {
	sub, err := fs.Sub(assets, "templates")
	if err != nil {
		panic(err)
	}
	return sub
}

// StaticFS returns the embedded static/ subtree as an fs.FS.
func StaticFS() fs.FS {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
