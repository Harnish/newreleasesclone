package main

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
)

// The frontend lives in web/ (index.html, app.css, app.js) and is compiled into
// the binary, so there is still no build step.
//
//go:embed web
var webFS embed.FS

// __BUILD__ in index.html becomes the build version.
var indexHTML = bytes.ReplaceAll(mustRead("web/index.html"), []byte("__BUILD__"), []byte(buildVersion()))

func mustRead(name string) []byte {
	b, err := webFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return b
}

// staticHandler serves web/ under /static/. no-cache because embedded files
// carry no modtime/ETag, so browsers must not guess how long to keep them.
func staticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}
