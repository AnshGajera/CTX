package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/AnshGajera/CTX/server/internal/config"
	"github.com/AnshGajera/CTX/server/internal/store"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	db, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "ctx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.JWTSecret = "test-secret"
	apiServer := NewServer(cfg, db)
	httpServer := httptest.NewServer(apiServer.corsMiddleware(apiServer.mux))
	t.Cleanup(httpServer.Close)
	return apiServer, httpServer
}

func requestJSON(t *testing.T, client *http.Client, method, url, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return resp, out
}

func TestRegisterLoginAndAuthenticatedOrganizationFlow(t *testing.T) {
	_, server := newTestServer(t)
	client := server.Client()

	register, body := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/register", "", map[string]any{
		"email":    "reviewer@example.com",
		"name":     "Reviewer",
		"password": "correct horse battery staple",
	})
	if register.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, body = %#v", register.StatusCode, body)
	}
	token, ok := body["token"].(string)
	if !ok || token == "" {
		t.Fatalf("register token missing: %#v", body)
	}

	login, loginBody := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "", map[string]any{
		"email":    "reviewer@example.com",
		"password": "correct horse battery staple",
	})
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %#v", login.StatusCode, loginBody)
	}
	if loginBody["token"] == "" {
		t.Fatalf("login token missing: %#v", loginBody)
	}

	org, orgBody := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/orgs", token, map[string]any{
		"name": "Review Org",
		"slug": "review-org",
	})
	if org.StatusCode != http.StatusCreated {
		t.Fatalf("create org status = %d, body = %#v", org.StatusCode, orgBody)
	}
	if orgBody["id"] == "" {
		t.Fatalf("organization id missing: %#v", orgBody)
	}

	orgs, orgsBody := requestJSON(t, client, http.MethodGet, server.URL+"/api/v1/orgs", token, nil)
	if orgs.StatusCode != http.StatusOK {
		t.Fatalf("list orgs status = %d, body = %#v", orgs.StatusCode, orgsBody)
	}

	orgID, ok := orgBody["id"].(string)
	if !ok || orgID == "" {
		t.Fatalf("organization id invalid: %#v", orgBody)
	}
	project, projectBody := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/orgs/"+orgID+"/projects", token, map[string]any{
		"name": "Review Project", "context_id": "review-context",
	})
	if project.StatusCode != http.StatusCreated {
		t.Fatalf("create project status = %d, body = %#v", project.StatusCode, projectBody)
	}
	projectID, ok := projectBody["id"].(string)
	if !ok || projectID == "" {
		t.Fatalf("project id invalid: %#v", projectBody)
	}

	push, pushBody := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/projects/"+projectID+"/push", token, map[string]any{
		"hash":   "0123456789abcdef0123456789abcdef",
		"author": "reviewer", "message": "initial context",
		"context": `{"project_name":"review-project"}`,
	})
	if push.StatusCode != http.StatusOK {
		t.Fatalf("push status = %d, body = %#v", push.StatusCode, pushBody)
	}

	pull, pullBody := requestJSON(t, client, http.MethodGet, server.URL+"/api/v1/projects/"+projectID+"/pull", token, nil)
	if pull.StatusCode != http.StatusOK {
		t.Fatalf("pull status = %d, body = %#v", pull.StatusCode, pullBody)
	}
	if pullBody["hash"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("pulled hash = %#v", pullBody["hash"])
	}
}

func TestAuthenticatedEndpointRejectsMissingTokenAndInvalidJSON(t *testing.T) {
	_, server := newTestServer(t)
	client := server.Client()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/orgs", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	req, err = http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/register", bytes.NewBufferString("{"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPushRejectsInvalidContext(t *testing.T) {
	_, server := newTestServer(t)
	client := server.Client()
	register, body := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/register", "", map[string]any{
		"email": "limits@example.com", "password": "correct horse battery staple",
	})
	if register.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d", register.StatusCode)
	}
	token := body["token"].(string)
	org, orgBody := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/orgs", token, map[string]any{
		"name": "Limits", "slug": "limits",
	})
	if org.StatusCode != http.StatusCreated {
		t.Fatalf("org status = %d", org.StatusCode)
	}
	orgID := orgBody["id"].(string)
	project, projectBody := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/orgs/"+orgID+"/projects", token, map[string]any{
		"name": "Limits Project", "context_id": "limits-context",
	})
	if project.StatusCode != http.StatusCreated {
		t.Fatalf("project status = %d", project.StatusCode)
	}
	projectID := projectBody["id"].(string)
	push, _ := requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/projects/"+projectID+"/push", token, map[string]any{
		"hash": "0123456789abcdef0123456789abcdef", "context": "{",
	})
	if push.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid context status = %d, want %d", push.StatusCode, http.StatusBadRequest)
	}
}
