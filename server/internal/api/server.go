package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/AnshGajera/CTX/server/internal/auth"
	"github.com/AnshGajera/CTX/server/internal/config"
	"github.com/AnshGajera/CTX/server/internal/store"
)

// contextKey is a private type for context keys.
type contextKey string

const userKey contextKey = "user"

// maxBodySize caps request bodies at 10 MiB.
const maxBodySize = 10 << 20

// Server is the CTX HTTP API server.
type Server struct {
	cfg   *config.Config
	store *store.SQLiteStore
	mux   *http.ServeMux
}

// NewServer creates a server with all routes registered.
func NewServer(cfg *config.Config, db *store.SQLiteStore) *Server {
	s := &Server{cfg: cfg, store: db, mux: http.NewServeMux()}
	s.registerRoutes()
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.corsMiddleware(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) registerRoutes() {
	// Public endpoints
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)

	// Authenticated endpoints
	s.mux.HandleFunc("GET /api/v1/orgs", s.authed(s.handleListOrgs))
	s.mux.HandleFunc("POST /api/v1/orgs", s.authed(s.handleCreateOrg))
	s.mux.HandleFunc("GET /api/v1/orgs/{orgID}/projects", s.authed(s.handleListProjects))
	s.mux.HandleFunc("POST /api/v1/orgs/{orgID}/projects", s.authed(s.handleCreateProject))

	s.mux.HandleFunc("POST /api/v1/projects/{projectID}/push", s.authed(s.handlePush))
	s.mux.HandleFunc("GET /api/v1/projects/{projectID}/pull", s.authed(s.handlePull))
	s.mux.HandleFunc("GET /api/v1/projects/{projectID}/history", s.authed(s.handleHistory))
	s.mux.HandleFunc("POST /api/v1/projects/{projectID}/share", s.authed(s.handleShare))

	s.mux.HandleFunc("POST /api/v1/tokens", s.authed(s.handleCreateToken))
}

// --- Middleware ---

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization")
			return
		}

		// Try JWT first
		claims, err := auth.ValidateJWT(s.cfg.JWTSecret, token)
		if err == nil {
			ctx := context.WithValue(r.Context(), userKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try API token
		hash := auth.HashAPIToken(token)
		apiToken, err := s.store.GetAPITokenByHash(hash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
			writeError(w, http.StatusUnauthorized, "token expired")
			return
		}

		ctx := context.WithValue(r.Context(), userKey, apiToken.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func getUserID(r *http.Request) string {
	v, _ := r.Context().Value(userKey).(string)
	return v
}

func extractToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": "0.1.0",
		"time":    time.Now().UTC(),
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := s.store.CreateUser(req.Email, req.Name, hash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "create user failed")
		return
	}

	token, err := auth.GenerateJWT(s.cfg.JWTSecret, user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate token failed")
		return
	}

	_ = s.store.LogAudit(user.ID, "register", "user", user.ID, "{}", r.RemoteAddr)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	user, err := s.store.GetUserByEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.CheckPassword(req.Password, user.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.GenerateJWT(s.cfg.JWTSecret, user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate token failed")
		return
	}

	_ = s.store.LogAudit(user.ID, "login", "user", user.ID, "{}", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	orgs, err := s.store.ListOrgs(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list orgs failed")
		return
	}
	if orgs == nil {
		orgs = []store.Organization{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizations": orgs})
}

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug required")
		return
	}

	userID := getUserID(r)
	org, err := s.store.CreateOrg(req.Name, req.Slug, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "slug already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "create org failed")
		return
	}

	_ = s.store.LogAudit(userID, "create_org", "organization", org.ID, "{}", r.RemoteAddr)
	writeJSON(w, http.StatusCreated, org)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	userID := getUserID(r)
	projects, err := s.store.ListProjects(userID, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list projects failed")
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ContextID   string `json:"context_id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" || req.ContextID == "" {
		writeError(w, http.StatusBadRequest, "name and context_id required")
		return
	}

	userID := getUserID(r)
	project, err := s.store.CreateProject(orgID, req.Name, req.Description, req.ContextID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "context_id already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "create project failed")
		return
	}

	_ = s.store.LogAudit(userID, "create_project", "project", project.ID, "{}", r.RemoteAddr)
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	userID := getUserID(r)

	// Verify access
	canAccess, access, err := s.store.UserCanAccessProject(userID, projectID)
	if err != nil || !canAccess {
		// Also try by context_id
		project, perr := s.store.GetProjectByContextID(projectID)
		if perr != nil {
			writeError(w, http.StatusForbidden, "no access to project")
			return
		}
		projectID = project.ID
		canAccess, access, err = s.store.UserCanAccessProject(userID, projectID)
		if err != nil || !canAccess {
			writeError(w, http.StatusForbidden, "no access to project")
			return
		}
	}
	if access == "read" {
		writeError(w, http.StatusForbidden, "read-only access")
		return
	}

	var snap store.Snapshot
	if !decodeBody(w, r, &snap) {
		return
	}
	snap.ProjectID = projectID

	if err := s.store.PushSnapshot(&snap); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("push failed: %v", err))
		return
	}

	_ = s.store.LogAudit(userID, "push", "project", projectID, fmt.Sprintf(`{"hash":"%s"}`, snap.Hash), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"hash": snap.Hash, "status": "pushed"})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	userID := getUserID(r)

	canAccess, _, err := s.store.UserCanAccessProject(userID, projectID)
	if err != nil || !canAccess {
		project, perr := s.store.GetProjectByContextID(projectID)
		if perr != nil {
			writeError(w, http.StatusForbidden, "no access to project")
			return
		}
		projectID = project.ID
		canAccess, _, err = s.store.UserCanAccessProject(userID, projectID)
		if err != nil || !canAccess {
			writeError(w, http.StatusForbidden, "no access to project")
			return
		}
	}

	snap, err := s.store.PullSnapshot(projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "no snapshots found")
			return
		}
		writeError(w, http.StatusInternalServerError, "pull failed")
		return
	}

	_ = s.store.LogAudit(userID, "pull", "project", projectID, fmt.Sprintf(`{"hash":"%s"}`, snap.Hash), r.RemoteAddr)
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	userID := getUserID(r)

	canAccess, _, err := s.store.UserCanAccessProject(userID, projectID)
	if err != nil || !canAccess {
		writeError(w, http.StatusForbidden, "no access to project")
		return
	}

	snaps, err := s.store.ListSnapshots(projectID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history failed")
		return
	}
	if snaps == nil {
		snaps = []store.Snapshot{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
}

func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	userID := getUserID(r)

	canAccess, access, err := s.store.UserCanAccessProject(userID, projectID)
	if err != nil || !canAccess || access == "read" {
		writeError(w, http.StatusForbidden, "admin or write access required to share")
		return
	}

	var req struct {
		TargetTeam string   `json:"target_team"`
		Sections   []string `json:"sections"`
		Level      string   `json:"level"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.TargetTeam == "" {
		writeError(w, http.StatusBadRequest, "target_team required")
		return
	}
	if req.Level == "" {
		req.Level = "L1"
	}

	policy, err := s.store.CreateSharePolicy(projectID, req.TargetTeam, req.Sections, req.Level, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share failed")
		return
	}

	_ = s.store.LogAudit(userID, "share", "project", projectID,
		fmt.Sprintf(`{"target_team":"%s","level":"%s"}`, req.TargetTeam, req.Level), r.RemoteAddr)
	writeJSON(w, http.StatusCreated, policy)
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		ReadOnly bool   `json:"read_only"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	userID := getUserID(r)
	raw, hash, err := auth.GenerateAPIToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate token failed")
		return
	}

	apiToken, err := s.store.CreateAPIToken(userID, req.Name, hash, req.ReadOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save token failed")
		return
	}

	_ = s.store.LogAudit(userID, "create_token", "api_token", apiToken.ID, "{}", r.RemoteAddr)
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":     raw,
		"id":        apiToken.ID,
		"name":      apiToken.Name,
		"read_only": apiToken.ReadOnly,
		"note":      "Save this token — it will not be shown again",
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "read body failed")
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}
