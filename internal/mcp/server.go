package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
)

// MCPServer serves context over HTTP.
type MCPServer struct {
	store *versioning.ContextStore
	root  string
	port  int
}

// NewMCPServer creates a server.
func NewMCPServer(root string, store *versioning.ContextStore, port int) *MCPServer {
	return &MCPServer{root: root, store: store, port: port}
}

func (s *MCPServer) loadHead() (*projctx.ProjectContext, error) {
	head, err := s.store.GetHead()
	if err != nil {
		// fallback to working file
		return projctx.LoadContext(s.root)
	}
	snap, err := s.store.LoadSnapshot(head)
	if err != nil {
		return projctx.LoadContext(s.root)
	}
	return versioning.SnapshotToContext(snap)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Handler returns the HTTP mux.
func (s *MCPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/tools/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, map[string]any{"tools": ToolDefinitions()})
	})
	mux.HandleFunc("/mcp/tools/call", s.handleToolCall)
	mux.HandleFunc("/context", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if budget := r.URL.Query().Get("max_tokens"); budget != "" {
			var n int
			if _, err := fmt.Sscanf(budget, "%d", &n); err == nil && n > 0 {
				writeJSON(w, BudgetedContext(ctx, nil, n))
				return
			}
		}
		writeJSON(w, ctx)
	})
	mux.HandleFunc("/context/architecture", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.Architecture)
	})
	mux.HandleFunc("/context/apis", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.APIs)
	})
	mux.HandleFunc("/context/database", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.Database)
	})
	mux.HandleFunc("/context/dependencies", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.Dependencies)
	})
	mux.HandleFunc("/context/env", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.Environment)
	})
	mux.HandleFunc("/context/structure", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.FileStructure)
	})
	mux.HandleFunc("/context/state", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ctx.CurrentState)
	})
	mux.HandleFunc("/context/summary", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, Summarize(ctx))
	})
	mux.HandleFunc("/context/for-task", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TaskDescription string   `json:"task_description"`
			AffectedFiles   []string `json:"affected_files"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ContextForTask(ctx, req.TaskDescription, req.AffectedFiles))
	})
	mux.HandleFunc("/context/for-file", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			FilePath string `json:"file_path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.FilePath == "" {
			req.FilePath = r.URL.Query().Get("file_path")
		}
		ctx, err := s.loadHead()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ContextForFile(ctx, req.FilePath))
	})
	return mux
}

// Start runs the HTTP server.
func (s *MCPServer) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, s.Handler()) //nolint:gosec
}

func (s *MCPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	// compat: some clients send {"tool": "...", "params": {...}}
	if req.Name == "" {
		var alt struct {
			Tool   string         `json:"tool"`
			Params map[string]any `json:"params"`
		}
		_ = alt
	}
	ctx, err := s.loadHead()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	result, err := DispatchTool(ctx, req.Name, req.Arguments)
	if err != nil {
		if strings.Contains(err.Error(), "unknown tool") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"result": result})
}
