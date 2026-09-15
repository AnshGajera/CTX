package mcp

import (
	_ "embed"
	"net/http"
)

//go:embed ui.html
var uiHTML string

func serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(uiHTML))
}
