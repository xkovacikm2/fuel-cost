package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var files embed.FS

func Handler() http.Handler {
	assets, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := path.Clean("/" + r.URL.Path)
		if strings.HasPrefix(requestPath, "/api/") || requestPath == "/api" {
			http.NotFound(w, r)
			return
		}
		if requestPath == "/manifest" || requestPath == "/manifest.json" {
			serveFile(w, r, assets, "manifest.webmanifest", "application/manifest+json; charset=utf-8")
			return
		}
		if requestPath == "/service-worker" {
			w.Header().Set("Service-Worker-Allowed", "/")
			w.Header().Set("Cache-Control", "no-cache")
			serveFile(w, r, assets, "service-worker.js", "text/javascript; charset=utf-8")
			return
		}
		name := strings.TrimPrefix(requestPath, "/")
		if name == "" {
			name = "index.html"
		} else if strings.Contains(path.Base(name), ".") {
			if _, err := fs.Stat(assets, name); err != nil {
				http.NotFound(w, r)
				return
			}
		} else {
			name = "index.html"
		}
		serveFile(w, r, assets, name, "")
	})
}

func serveFile(w http.ResponseWriter, r *http.Request, assets fs.FS, name, explicitType string) {
	contents, err := fs.ReadFile(assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := explicitType
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(name))
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contents)
}
