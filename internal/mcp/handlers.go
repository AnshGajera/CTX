package mcp

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/AnshGajera/CTX/internal/ai"
	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/health"
	"github.com/AnshGajera/CTX/internal/versioning"
)

// DispatchTool executes a tool by name (legacy signature).
func DispatchTool(ctx *projctx.ProjectContext, name string, args map[string]any) (any, error) {
	return DispatchToolWithStore(ctx, nil, "", name, args)
}

// DispatchToolWithStore executes a tool with access to the context versioning store.
func DispatchToolWithStore(ctx *projctx.ProjectContext, store *versioning.ContextStore, root string, name string, args map[string]any) (any, error) {
	if args == nil {
		args = map[string]any{}
	}

	if store == nil && root != "" {
		store = versioning.NewContextStore(filepath.Join(root, ".ctx"))
	}

	switch name {
	case "get_project_context":
		sections := toStringSlice(args["sections"])
		return BudgetedContext(ctx, sections, toInt(args["max_tokens"], 0)), nil
	case "get_context_for_task":
		desc, _ := args["task_description"].(string)
		files := toStringSlice(args["affected_files"])
		return ContextForTask(ctx, desc, files), nil
	case "get_context_for_file":
		fp, _ := args["file_path"].(string)
		return ContextForFile(ctx, fp), nil
	case "get_api_endpoints":
		fp, _ := args["filter_path"].(string)
		fm, _ := args["filter_method"].(string)
		return FilterEndpoints(ctx, fp, fm), nil
	case "get_database_schema":
		model, _ := args["model_name"].(string)
		includeDiagram := true
		if v, ok := args["include_diagram"].(bool); ok {
			includeDiagram = v
		}
		return FilterDatabase(ctx, model, includeDiagram), nil
	case "get_project_conventions":
		return map[string]any{"file_structure": ctx.FileStructure, "patterns": ctx.Patterns}, nil
	case "get_env_requirements":
		return ctx.Environment, nil
	case "search_context":
		query, _ := args["query"].(string)
		if strings.TrimSpace(query) == "" {
			return nil, fmt.Errorf("query is required")
		}
		return SearchChunks(ctx, query, toInt(args["top_k"], 5)), nil

	// --- Git Context Control (GCC) AI Agent Tools ---
	case "branch_context":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		action, _ := args["action"].(string)
		branch, _ := args["branch"].(string)
		startPoint, _ := args["start_point"].(string)
		if startPoint == "" {
			startPoint = "HEAD"
		}
		switch action {
		case "list":
			branches, err := store.ListBranches()
			if err != nil {
				return nil, err
			}
			return map[string]any{"branches": branches}, nil
		case "create":
			if branch == "" {
				return nil, fmt.Errorf("branch name is required for create")
			}
			if err := store.CreateBranch(branch, startPoint); err != nil {
				return nil, err
			}
			return map[string]any{"created_branch": branch, "start_point": startPoint}, nil
		case "delete":
			if branch == "" {
				return nil, fmt.Errorf("branch name is required for delete")
			}
			if err := store.DeleteBranch(branch); err != nil {
				return nil, err
			}
			return map[string]any{"deleted_branch": branch}, nil
		default:
			return nil, fmt.Errorf("unknown branch action: %s (must be list, create, delete)", action)
		}

	case "checkout_context":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		target, _ := args["target"].(string)
		if strings.TrimSpace(target) == "" {
			return nil, fmt.Errorf("target is required for checkout")
		}
		createBranch, _ := args["create_branch"].(bool)
		var snap *versioning.ContextSnapshot
		var err error
		if createBranch {
			snap, err = store.CheckoutNewBranch(target, "HEAD")
		} else {
			snap, err = store.Checkout(target)
		}
		if err != nil {
			return nil, err
		}
		curBranch, isDetached, _ := store.CurrentBranch()
		return map[string]any{
			"checked_out": target,
			"branch":      curBranch,
			"is_detached": isDetached,
			"snapshot":    snap.Hash,
			"message":     snap.Message,
		}, nil

	case "commit_context":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		msg, _ := args["message"].(string)
		if strings.TrimSpace(msg) == "" {
			return nil, fmt.Errorf("message is required for commit_context")
		}
		snap, err := store.CommitCheckpoint(ctx, msg)
		if err != nil {
			return nil, err
		}
		curBranch, detached, _ := store.CurrentBranch()
		return map[string]any{
			"snapshot":    snap.Hash,
			"branch":      curBranch,
			"is_detached": detached,
			"message":     snap.Message,
		}, nil

	case "merge_context":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		sourceBranch, _ := args["source_branch"].(string)
		if strings.TrimSpace(sourceBranch) == "" {
			return nil, fmt.Errorf("source_branch is required for merge")
		}
		msg, _ := args["message"].(string)
		snap, report, err := store.Merge(sourceBranch, msg)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"merged_snapshot": snap.Hash,
			"report":          report,
		}, nil

	case "tag_context":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		action, _ := args["action"].(string)
		tag, _ := args["tag"].(string)
		target, _ := args["target"].(string)
		if target == "" {
			target = "HEAD"
		}
		switch action {
		case "list":
			tags, err := store.ListTags()
			if err != nil {
				return nil, err
			}
			return map[string]any{"tags": tags}, nil
		case "create":
			if tag == "" {
				return nil, fmt.Errorf("tag name is required for create")
			}
			if err := store.CreateTag(tag, target); err != nil {
				return nil, err
			}
			return map[string]any{"created_tag": tag, "target": target}, nil
		case "delete":
			if tag == "" {
				return nil, fmt.Errorf("tag name is required for delete")
			}
			if err := store.DeleteTag(tag); err != nil {
				return nil, err
			}
			return map[string]any{"deleted_tag": tag}, nil
		default:
			return nil, fmt.Errorf("unknown tag action: %s", action)
		}

	case "get_context_diff":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		from, _ := args["from"].(string)
		to, _ := args["to"].(string)
		if from == "" {
			from = "HEAD~1"
		}
		if to == "" {
			to = "HEAD"
		}
		s1, err := store.LoadSnapshot(from)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", from, err)
		}
		s2, err := store.LoadSnapshot(to)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", to, err)
		}
		c1, err := versioning.SnapshotToContext(s1)
		if err != nil {
			return nil, err
		}
		c2, err := versioning.SnapshotToContext(s2)
		if err != nil {
			return nil, err
		}
		diff := versioning.ComputeDiff(c1, c2)
		return diff, nil

	case "get_context_history":
		if store == nil {
			return nil, fmt.Errorf("version store not available")
		}
		limit := toInt(args["limit"], 10)
		snaps, err := store.Log(limit)
		if err != nil {
			return nil, err
		}
		curBranch, isDetached, _ := store.CurrentBranch()
		branches, _ := store.ListBranches()
		tags, _ := store.ListTags()
		return map[string]any{
			"current_branch": curBranch,
			"is_detached":    isDetached,
			"snapshots":      snaps,
			"branches":       branches,
			"tags":           tags,
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// FilteredContext returns sections subset.
func FilteredContext(ctx *projctx.ProjectContext, sections []string) any {
	if len(sections) == 0 {
		return ctx
	}
	want := map[string]bool{}
	for _, s := range sections {
		want[strings.ToLower(s)] = true
	}
	out := map[string]any{
		"project_name": ctx.ProjectName,
		"profile":      ctx.Profile,
	}
	if want["architecture"] {
		out["architecture"] = ctx.Architecture
	}
	if want["apis"] {
		out["apis"] = ctx.APIs
	}
	if want["database"] {
		out["database"] = ctx.Database
	}
	if want["dependencies"] {
		out["dependencies"] = ctx.Dependencies
	}
	if want["environment"] {
		out["environment"] = ctx.Environment
	}
	if want["structure"] {
		out["file_structure"] = ctx.FileStructure
	}
	if want["state"] {
		out["current_state"] = ctx.CurrentState
	}
	if want["patterns"] {
		out["patterns"] = ctx.Patterns
	}
	return out
}

// ContextForTask keyword-routes sections.
func ContextForTask(ctx *projctx.ProjectContext, desc string, files []string) any {
	d := strings.ToLower(desc)
	includeAPIs := containsAny(d, []string{"api", "endpoint", "route", "handler", "rest", "http"})
	includeDB := containsAny(d, []string{"database", "model", "schema", "migration", "query", "sql", "prisma"})
	includeAuth := containsAny(d, []string{"auth", "login", "signup", "permission", "session", "jwt", "oauth"})
	includeFE := containsAny(d, []string{"component", "page", "layout", "ui", "frontend", "style", "react"})
	includeDeploy := containsAny(d, []string{"deploy", "docker", "ci", "pipeline", "env", "config"})
	out := map[string]any{
		"architecture": ctx.Architecture,
		"conventions":  ctx.FileStructure,
		"patterns":     ctx.Patterns,
	}
	if includeAPIs || includeAuth || len(files) > 0 {
		out["apis"] = ctx.APIs
	}
	if includeDB {
		out["database"] = ctx.Database
	}
	if includeAuth {
		out["business_rules"] = ctx.BusinessRules
		out["environment"] = ctx.Environment
	}
	if includeFE {
		out["file_structure"] = ctx.FileStructure
	}
	if includeDeploy {
		out["environment"] = ctx.Environment
	}
	if len(files) > 0 {
		var related []any
		for _, f := range files {
			related = append(related, ContextForFile(ctx, f))
		}
		out["file_contexts"] = related
	}
	if len(out) == 3 {
		// no keywords matched: include compact summary + apis + env names
		out["apis"] = ctx.APIs
		out["environment"] = ctx.Environment
	}
	out["task"] = desc
	return out
}

// ContextForFile finds references to a file path.
func ContextForFile(ctx *projctx.ProjectContext, filePath string) any {
	fp := strings.TrimSpace(filePath)
	out := map[string]any{"file": fp}
	if ctx.APIs != nil {
		var eps []projctx.APIEndpoint
		for _, e := range ctx.APIs.Endpoints {
			if matchFilePath(e.File, fp) {
				eps = append(eps, e)
			}
		}
		if len(eps) > 0 {
			out["endpoints"] = eps
		}
	}
	if ctx.Database != nil {
		var models []projctx.DatabaseModel
		for _, m := range ctx.Database.Models {
			if matchFilePath(m.File, fp) {
				models = append(models, m)
			}
		}
		if len(models) > 0 {
			out["models"] = models
		}
	}
	if ctx.FileStructure != nil {
		for _, k := range ctx.FileStructure.KeyFiles {
			if k.Path == fp {
				out["key_file"] = k
			}
		}
	}
	if len(out) == 1 {
		out["note"] = "no direct references found; see architecture for orientation"
		out["architecture"] = ctx.Architecture
	}
	return out
}

// FilterEndpoints filters by path/method substring.
func FilterEndpoints(ctx *projctx.ProjectContext, filterPath, filterMethod string) any {
	if ctx.APIs == nil {
		return []projctx.APIEndpoint{}
	}
	var out []projctx.APIEndpoint
	for _, e := range ctx.APIs.Endpoints {
		if filterPath != "" && !strings.Contains(strings.ToLower(e.Path), strings.ToLower(filterPath)) {
			continue
		}
		if filterMethod != "" && !strings.EqualFold(e.Method, filterMethod) && e.Method != "ALL" {
			continue
		}
		out = append(out, e)
	}
	if out == nil {
		return []projctx.APIEndpoint{}
	}
	return out
}

// FilterDatabase returns schema or single model.
func FilterDatabase(ctx *projctx.ProjectContext, model string, includeDiagram bool) any {
	if ctx.Database == nil {
		return map[string]any{}
	}
	if model == "" {
		if !includeDiagram {
			clone := *ctx.Database
			clone.Diagram = ""
			return &clone
		}
		return ctx.Database
	}
	for _, m := range ctx.Database.Models {
		if strings.EqualFold(m.Name, model) {
			return map[string]any{"model": m, "diagram": ifThen(includeDiagram, ctx.Database.Diagram, "")}
		}
	}
	return map[string]any{"error": "model not found: " + model}
}

// Summarize returns a brief project summary.
func Summarize(ctx *projctx.ProjectContext) any {
	nEP, nModels, nEnv, nDeps := 0, 0, 0, 0
	if ctx.APIs != nil {
		nEP = len(ctx.APIs.Endpoints)
	}
	if ctx.Database != nil {
		nModels = len(ctx.Database.Models)
	}
	if ctx.Environment != nil {
		nEnv = len(ctx.Environment.Variables)
	}
	if ctx.Dependencies != nil {
		nDeps = len(ctx.Dependencies.Direct) + len(ctx.Dependencies.Dev)
	}
	rep := health.Score(ctx)
	return map[string]any{
		"project": ctx.ProjectName, "type": ctx.Profile.ProjectType,
		"languages": ctx.Profile.Languages, "frameworks": ctx.Profile.Frameworks,
		"counts": map[string]int{"endpoints": nEP, "models": nModels, "env_vars": nEnv, "dependencies": nDeps},
		"branch": ctx.CurrentStateBranch(), "hash": ctx.ContentHash,
		"extracted_at": ctx.ExtractedAt.UTC().Format(time.RFC3339),
		"staleness":    ctx.Staleness(time.Now()),
		"health":       map[string]any{"score": rep.Score, "grade": rep.Grade},
	}
}

func containsAny(hay string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

// matchFilePath matches exact case-insensitive paths or path-segment suffixes.
func matchFilePath(candidate, query string) bool {
	lc, lq := strings.ToLower(candidate), strings.ToLower(query)
	if lq == "" {
		return false
	}
	return lc == lq || strings.HasSuffix(lc, "/"+lq)
}

func ifThen(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func toInt(v any, def int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case nil:
		return def
	default:
		return def
	}
}

// SearchChunks ranks chunked context against a query (local TF-IDF).
func SearchChunks(ctx *projctx.ProjectContext, query string, topK int) any {
	if topK <= 0 {
		topK = 5
	}
	chunks := ai.NewChunker().Chunk(ctx)
	ranked := ai.RankTFIDF(chunks, query, topK)
	out := make([]map[string]any, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, map[string]any{
			"kind":   r.Chunk.Kind,
			"id":     r.Chunk.ID,
			"source": r.Chunk.Source,
			"score":  r.Score,
			"text":   r.Chunk.Text,
		})
	}
	return map[string]any{"query": query, "results": out}
}
