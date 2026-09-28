package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
)

// StdioServer implements MCP over JSON-RPC 2.0 stdio.
type StdioServer struct {
	store *versioning.ContextStore
	root  string
}

// NewStdioServer creates a stdio server.
func NewStdioServer(root string, store *versioning.ContextStore) *StdioServer {
	return &StdioServer{root: root, store: store}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id"`
	Result  any     `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *StdioServer) loadCtx() (*projctx.ProjectContext, error) {
	head, err := s.store.GetHead()
	if err != nil {
		return projctx.LoadContext(s.root)
	}
	snap, err := s.store.LoadSnapshot(head)
	if err != nil {
		return projctx.LoadContext(s.root)
	}
	return versioning.SnapshotToContext(snap)
}

// Serve loops over stdin.
func (s *StdioServer) Serve() error {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeRPC(out, rpcResponse{JSONRPC: "2.0", Error: &rpcErr{Code: -32700, Message: "parse error"}})
			continue
		}
		// notifications have no id
		if req.Method == "notifications/initialized" {
			continue
		}
		switch req.Method {
		case "initialize":
			writeRPC(out, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "ctx", "version": "1.0.0"},
			}})
		case "tools/list":
			writeRPC(out, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": ToolDefinitions()}})
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			ctx, err := s.loadCtx()
			if err != nil {
				writeRPC(out, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32000, Message: err.Error()}})
				continue
			}
			res, err := DispatchToolWithStore(ctx, s.store, s.root, p.Name, p.Arguments)
			if err != nil {
				writeRPC(out, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32602, Message: err.Error()}})
				continue
			}
			writeRPC(out, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"content": []any{map[string]any{"type": "text", "text": toJSONString(res)}}}})
		default:
			writeRPC(out, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcErr{Code: -32601, Message: "method not found: " + req.Method}})
		}
	}
	return sc.Err()
}

func writeRPC(w *bufio.Writer, resp rpcResponse) {
	data, _ := json.Marshal(resp)
	_, _ = fmt.Fprintln(w, string(data))
	_ = w.Flush()
}

func toJSONString(v any) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}
