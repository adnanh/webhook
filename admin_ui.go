package main

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed adminui/index.html adminui/assets/*
var adminUIFiles embed.FS

var (
	adminIndexTemplate = template.Must(template.ParseFS(adminUIFiles, "adminui/index.html"))
	adminStaticFS      = mustAdminSub(adminUIFiles, "adminui")
)

type adminUIData struct {
	BasePath string
}

func mustAdminSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}

	return sub
}

func adminUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := adminIndexTemplate.Execute(w, adminUIData{BasePath: currentAdminAuth.basePath}); err != nil {
		http.Error(w, "admin UI render failed", http.StatusInternalServerError)
	}
}

func adminStaticHandler() http.Handler {
	return http.FileServer(http.FS(adminStaticFS))
}
