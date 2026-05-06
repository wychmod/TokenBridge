package embed

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed admin/*
var adminFS embed.FS

// AdminFS returns the embedded admin frontend files as an http.FileSystem.
func AdminFS() http.FileSystem {
	sub, _ := fs.Sub(adminFS, "admin")
	return http.FS(sub)
}

// AdminAssetsFS returns the embedded admin frontend files as an fs.FS.
func AdminAssetsFS() fs.FS {
	sub, _ := fs.Sub(adminFS, "admin")
	return sub
}

// AdminFileServer returns an http.Handler that serves the embedded admin frontend.
func AdminFileServer() http.Handler {
	sub, _ := fs.Sub(adminFS, "admin")
	return http.FileServer(http.FS(sub))
}
