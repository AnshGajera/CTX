package sync

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginUsesVersionedServerRoute(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"jwt-token"}`))
	}))
	defer server.Close()

	token, err := NewSyncClient(server.URL, "").Login("user@example.com", "password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("token = %q, want %q", token, "jwt-token")
	}
	if gotPath != "/api/v1/auth/login" {
		t.Fatalf("path = %q, want %q", gotPath, "/api/v1/auth/login")
	}
}
