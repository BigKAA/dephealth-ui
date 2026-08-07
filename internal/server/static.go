package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
)

//go:embed all:static
var staticFS embed.FS

// hashedAssetRe matches Vite-generated hashed filenames like index-AbCdEf12.js
var hashedAssetRe = regexp.MustCompile(`-[a-zA-Z0-9]{8,}\.\w+$`)

// newStaticHandler returns an http.Handler that serves embedded SPA files.
// Files with extensions are served directly; all other paths fall back to index.html.
// Hashed assets get immutable cache headers; non-hashed files get no-cache.
func newStaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean path
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}

		// Check if file exists in embedded FS
		if hasExtension(p) {
			if _, err := fs.Stat(sub, p); err == nil {
				setCacheHeaders(w, p)
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for all non-file routes
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// setCacheHeaders sets appropriate Cache-Control headers based on the file path.
func setCacheHeaders(w http.ResponseWriter, p string) {
	if isHashedAsset(p) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
}

// isHashedAsset returns true if the path is a Vite-generated hashed asset.
func isHashedAsset(p string) bool {
	if !strings.HasPrefix(p, "assets/") {
		return false
	}
	return hashedAssetRe.MatchString(p)
}

// hasExtension returns true if the path has a file extension.
func hasExtension(p string) bool {
	return path.Ext(p) != ""
}
