package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
)

// MCPServer serves context over HTTP.
type MCPServer struct {
	store *versioning.ContextStore
	root  string
	port  int
	bind  string
}

// maxBodyBytes caps JSON request bodies (5 MiB).
const maxBodyBytes = 5 << 20

// snapshotRefRe allowlists handleDiff refs (HEAD, HEAD~N, or hex hashes).
var snapshotRefRe = regexp.MustCompile(`^(HEAD(~\d+)?|[0-9a-fA-F]{7,64})$`)

// NewMCPServer creates a server.
func NewMCPServer(root string, store *versioning.ContextStore, port int) *MCPServer {
	return &MCPServer{root: root, store: store, port: port, bind: "127.0.0.1"}
}

// SetBind overrides the listen address (default "127.0.0.1").
func (s *MCPServer) SetBind(bind string) {
	if strings.TrimSpace(bind) != "" {
		s.bind = bind
	}
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
	// Local web dashboard (single-file, offline, no build step).
	mux.HandleFunc("/ui", serveUI)
	mux.HandleFunc("/ui/", serveUI)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ui", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/diff", s.handleDiff)
	mux.HandleFunc("/api/search", s.handleSearch)
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
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
		}
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
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
		}
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
	bind := s.bind
	if strings.TrimSpace(bind) == "" {
		bind = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", bind, s.port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *MCPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(data, &req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	// compat: some clients send {"tool": "...", "params": {...}}
	if req.Name == "" {
		var alt struct {
			Tool   string         `json:"tool"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(data, &alt); err == nil && alt.Tool != "" {
			req.Name = alt.Tool
			if req.Arguments == nil {
				req.Arguments = alt.Params
			}
		}
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

func (s *MCPServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	snaps, err := s.store.Log(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if snaps == nil {
		snaps = []versioning.ContextSnapshot{}
	}
	writeJSON(w, map[string]any{"snapshots": snaps})
}

func (s *MCPServer) handleDiff(w http.ResponseWriter, r *http.Request) {
	head, err := s.store.GetHead()
	if err != nil {
		http.Error(w, "no snapshots (run ctx extract first)", http.StatusNotFound)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if to == "" {
		to = head
	}
	if from == "" {
		cur, err := s.store.LoadSnapshot(to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if cur.ParentHash == "" {
			writeJSON(w, map[string]any{"summary": "only one snapshot — extract again after changing code"})
			return
		}
		from = cur.ParentHash
	}
	if !snapshotRefRe.MatchString(from) || !snapshotRefRe.MatchString(to) {
		http.Error(w, "invalid ref: must be HEAD, HEAD~N, or a hex hash", http.StatusBadRequest)
		return
	}
	s1, err := s.store.LoadSnapshot(from)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s2, err := s.store.LoadSnapshot(to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	c1, _ := versioning.SnapshotToContext(s1)
	c2, _ := versioning.SnapshotToContext(s2)
	writeJSON(w, versioning.ComputeDiff(c1, c2))
}

func (s *MCPServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}
	if strings.TrimSpace(q) == "" {
		http.Error(w, "missing ?q=", http.StatusBadRequest)
		return
	}
	topK := 5
	if v := r.URL.Query().Get("top_k"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 && n <= 50 {
			topK = n
		}
	}
	ctx, err := s.loadHead()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, SearchChunks(ctx, q, topK))
}
