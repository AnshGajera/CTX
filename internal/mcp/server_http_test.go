package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AnshGajera/CTX/internal/versioning"
)

func TestHandlerRejectsInvalidJSONForTaskContext(t *testing.T) {
	server := NewMCPServer(t.TempDir(), versioning.NewContextStore(t.TempDir()), 0)
	req := httptest.NewRequest(http.MethodPost, "/context/for-task", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerRejectsInvalidJSONForFileContext(t *testing.T) {
	server := NewMCPServer(t.TempDir(), versioning.NewContextStore(t.TempDir()), 0)
	req := httptest.NewRequest(http.MethodPost, "/context/for-file", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
