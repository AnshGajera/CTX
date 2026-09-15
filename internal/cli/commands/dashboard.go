package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/AnshGajera/CTX/internal/ai"
	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/mcp"
	"github.com/AnshGajera/CTX/internal/openapi"
	"github.com/AnshGajera/CTX/internal/tui"
	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/urfave/cli/v2"
)

// DashboardCommand implements `ctx dashboard`.
func DashboardCommand() *cli.Command {
	return &cli.Command{
		Name:  "dashboard",
		Usage: "Interactive terminal dashboard (task menu)",
		Action: func(c *cli.Context) error {
			if !tui.IsTerminal(os.Stdout) {
				return fmt.Errorf("dashboard needs an interactive terminal")
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if _, err := projctx.LoadManifest(filepath.Join(cwd, ".ctx", "manifest.json")); err != nil {
				return fmt.Errorf("not initialized (run ctx init): %w", err)
			}
			return tui.RunDashboard(&dashboardBackend{root: cwd})
		},
	}
}

// dashboardBackend implements tui.DashboardBackend.
type dashboardBackend struct {
	root      string
	mu        sync.Mutex
	serving   bool
	serveAddr string
}

func (b *dashboardBackend) Title() string {
	ctx, err := projctx.LoadContext(b.root)
	if err != nil {
		return b.root + " (no context yet — run Extract)"
	}
	head, _ := versioning.NewContextStore(filepath.Join(b.root, ".ctx")).GetHead()
	return fmt.Sprintf("%s · %s · %s", ctx.ProjectName, shortHash(head), gitBranchOf(ctx))
}

func gitBranchOf(ctx *projctx.ProjectContext) string {
	if ctx.CurrentState == nil || ctx.CurrentState.GitBranch == "" {
		return "no git"
	}
	return ctx.CurrentState.GitBranch
}

func (b *dashboardBackend) Status() (string, error) {
	ctx, err := projctx.LoadContext(b.root)
	if err != nil {
		return "", fmt.Errorf("no context (run Extract first): %w", err)
	}
	return formatStatus(ctx), nil
}

func (b *dashboardBackend) Extract() (string, error) {
	deduped, err := runExtract(b.root, true, false, nil)
	if err != nil {
		return "", err
	}
	s, err := b.Status()
	if err != nil {
		return "", err
	}
	if deduped {
		return "No changes — HEAD unchanged\n\n" + s, nil
	}
	return "Extracted fresh snapshot\n\n" + s, nil
}

func (b *dashboardBackend) Search(query string) (string, error) {
	ctx, err := projctx.LoadContext(b.root)
	if err != nil {
		return "", err
	}
	chunks := ai.NewChunker().Chunk(ctx)
	ranked := ai.RankTFIDF(chunks, query, 10)
	if len(ranked) == 0 {
		return "No results.", nil
	}
	return formatHits(query, ranked), nil
}

func (b *dashboardBackend) Diff() (string, error) {
	store := versioning.NewContextStore(filepath.Join(b.root, ".ctx"))
	s2, err := store.LoadSnapshot("HEAD")
	if err != nil {
		return "", fmt.Errorf("no snapshots: %w", err)
	}
	if s2.ParentHash == "" {
		return "Only one snapshot — extract again after changing code.", nil
	}
	s1, err := store.LoadSnapshot(s2.ParentHash)
	if err != nil {
		return "", err
	}
	c1, err := versioning.SnapshotToContext(s1)
	if err != nil {
		return "", err
	}
	c2, err := versioning.SnapshotToContext(s2)
	if err != nil {
		return "", err
	}
	return formatDiff(versioning.ComputeDiff(c1, c2)), nil
}

func (b *dashboardBackend) History() (string, error) {
	snaps, err := versioning.NewContextStore(filepath.Join(b.root, ".ctx")).Log(20)
	if err != nil {
		return "", err
	}
	if len(snaps) == 0 {
		return "No snapshots yet.", nil
	}
	return formatHistory(snaps), nil
}

func (b *dashboardBackend) ToggleServe(port int) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.serving {
		return "Already serving at " + b.serveAddr + "\n(It stops when you quit the dashboard.)", nil
	}
	store := versioning.NewContextStore(filepath.Join(b.root, ".ctx"))
	srv := mcp.NewMCPServer(b.root, store, port)
	go func() { _ = srv.Start() }() //nolint:errcheck
	b.serving = true
	b.serveAddr = fmt.Sprintf("http://localhost:%d", port)
	return "Serving MCP/REST at " + b.serveAddr + "\nTools: search_context, get_project_context, …\n(It stops when you quit the dashboard.)", nil
}

func (b *dashboardBackend) Export() (string, error) {
	ctx, err := projctx.LoadContext(b.root)
	if err != nil {
		return "", err
	}
	var eps []projctx.APIEndpoint
	if ctx.APIs != nil {
		eps = ctx.APIs.Endpoints
	}
	doc := openapi.Build(ctx.ProjectName, eps)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	out := filepath.Join(b.root, "openapi.json")
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %s (%d endpoints)", out, len(eps)), nil
}

// --- pure format helpers (unit-tested) ---

func formatStatus(ctx *projctx.ProjectContext) string {
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
		nDeps = len(ctx.Dependencies.Direct)
	}
	arch := "-"
	if ctx.Architecture != nil && ctx.Architecture.Pattern != "" {
		arch = ctx.Architecture.Pattern
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Project:      %s (%s)\n", ctx.ProjectName, ctx.Profile.ProjectType)
	fmt.Fprintf(&b, "Branch:       %s\n", gitBranchOf(ctx))
	fmt.Fprintf(&b, "Architecture: %s\n", arch)
	fmt.Fprintf(&b, "Endpoints:    %d\nModels:       %d\nEnv vars:     %d\nDependencies: %d\n",
		nEP, nModels, nEnv, nDeps)
	fmt.Fprintf(&b, "Hash:         %s", ctx.ContentHash)
	return b.String()
}

func formatHits(query string, ranked []ai.RankedChunk) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Top %d for %q:\n\n", len(ranked), query)
	for i, r := range ranked {
		snip := r.Chunk.Snippet
		if snip == "" {
			snip = r.Chunk.Text
		}
		if len(snip) > 160 {
			snip = snip[:160] + "…"
		}
		fmt.Fprintf(&b, "%d. [%s] %.3f %s\n   %s\n", i+1, r.Chunk.Kind, r.Score, r.Chunk.ID, snip)
	}
	return b.String()
}

func formatDiff(d *versioning.ContextDiff) string {
	var b strings.Builder
	for _, e := range d.AddedEndpoints {
		fmt.Fprintf(&b, "+ %s %s (%s)\n", e.Method, e.Path, e.File)
	}
	for _, e := range d.RemovedEndpoints {
		fmt.Fprintf(&b, "- %s %s (%s)\n", e.Method, e.Path, e.File)
	}
	for _, m := range d.AddedModels {
		fmt.Fprintf(&b, "+ model %s\n", m.Name)
	}
	for _, m := range d.RemovedModels {
		fmt.Fprintf(&b, "- model %s\n", m.Name)
	}
	for _, v := range d.AddedEnvVars {
		fmt.Fprintf(&b, "+ env %s\n", v.Name)
	}
	for _, v := range d.RemovedEnvVars {
		fmt.Fprintf(&b, "- env %s\n", v.Name)
	}
	for _, dep := range d.AddedDeps {
		fmt.Fprintf(&b, "+ dep %s@%s\n", dep.Name, dep.Version)
	}
	for _, dep := range d.RemovedDeps {
		fmt.Fprintf(&b, "- dep %s@%s\n", dep.Name, dep.Version)
	}
	fmt.Fprintf(&b, "\nSummary: %s", d.Summary)
	return b.String()
}

func formatHistory(snaps []versioning.ContextSnapshot) string {
	var b strings.Builder
	for _, s := range snaps {
		fmt.Fprintf(&b, "%s  %s  %s\n    %s\n", shortHash(s.Hash),
			s.Timestamp.Format("2006-01-02 15:04"), s.Author, s.Message)
	}
	return b.String()
}
