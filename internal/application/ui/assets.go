package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/app.css assets/htmx.min.js assets/htmx-ext-sse.js assets/alpine.min.js
var staticFiles embed.FS

// staticFS returns the embedded assets as an http.FileSystem rooted at "assets/".
func staticFS() http.FileSystem {
	sub, err := fs.Sub(staticFiles, "assets")
	if err != nil {
		panic("ui: failed to sub assets FS: " + err.Error())
	}
	return http.FS(sub)
}
