package ui

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed assets/app.css assets/htmx.min.js assets/htmx-ext-sse.js assets/alpine.min.js assets/mermaid.min.js
var staticFiles embed.FS

// AssetVersion is an 8-char content hash over all embedded assets, computed
// once at startup. Append as ?v=<AssetVersion> to asset URLs so browsers can
// cache them indefinitely and still get fresh files after a deploy.
var AssetVersion string

func init() {
	h := sha256.New()
	fs.WalkDir(staticFiles, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := staticFiles.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(data)
		return nil
	})
	AssetVersion = fmt.Sprintf("%x", h.Sum(nil))[:8]
}

// staticFS returns the embedded assets as an http.FileSystem rooted at "assets/".
func staticFS() http.FileSystem {
	sub, err := fs.Sub(staticFiles, "assets")
	if err != nil {
		panic("ui: failed to sub assets FS: " + err.Error())
	}
	return http.FS(sub)
}
