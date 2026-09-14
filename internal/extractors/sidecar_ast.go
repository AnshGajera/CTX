package extractors

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// SidecarASTExtractor merges precise AST-based routes from the ctx-ml
// sidecar. It is a silent no-op when the sidecar is unreachable; the
// regex extractors remain the offline fallback.
type SidecarASTExtractor struct {
	Base
	mlURL  string
	client *http.Client
}

// NewSidecarAST creates the extractor; mlURL "" disables it.
func NewSidecarAST(root, mlURL string) *SidecarASTExtractor {
	return &SidecarASTExtractor{
		Base:   NewBase(root),
		mlURL:  strings.TrimRight(mlURL, "/"),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (e *SidecarASTExtractor) Name() string { return "sidecar-ast" }

type sidecarEndpoint struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Handler   string `json:"handler"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Framework string `json:"framework"`
}

func (e *SidecarASTExtractor) Extract(ctx *projctx.ProjectContext) error {
	if e.mlURL == "" {
		return nil
	}
	// Fast health check; connection-refused locally returns instantly.
	health, err := e.client.Get(e.mlURL + "/health")
	if err != nil {
		return nil
	}
	health.Body.Close()

	body, _ := json.Marshal(map[string]string{"root": e.Root})
	resp, err := e.client.Post(e.mlURL+"/extract/routes", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var out struct {
		Endpoints []sidecarEndpoint `json:"endpoints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil
	}
	if len(out.Endpoints) == 0 {
		return nil
	}
	existing := map[string]bool{}
	if ctx.APIs != nil {
		for _, ep := range ctx.APIs.Endpoints {
			existing[ep.Method+" "+ep.Path] = true
		}
	} else {
		ctx.APIs = &projctx.APIContext{}
	}
	for _, se := range out.Endpoints {
		if se.Path == "" {
			continue
		}
		method := strings.ToUpper(se.Method)
		// Normalize multi-method "GET,POST" into separate endpoints.
		methods := strings.Split(method, ",")
		for _, m := range methods {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			key := m + " " + se.Path
			if existing[key] {
				continue
			}
			existing[key] = true
			ctx.APIs.Endpoints = append(ctx.APIs.Endpoints, projctx.APIEndpoint{
				Method: m, Path: se.Path, Handler: se.Handler,
				File: se.File, Line: se.Line,
				Description:  "via AST (" + se.Framework + ")",
				Parameters:   pathParams(se.Path),
				AuthRequired: false,
			})
		}
	}
	return nil
}
