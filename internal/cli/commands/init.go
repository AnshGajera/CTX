package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AnshGajera/CTX/internal/cache"
	"github.com/AnshGajera/CTX/internal/config"
	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/detector"
	"github.com/AnshGajera/CTX/internal/engine"
	"github.com/AnshGajera/CTX/internal/extractors"
	"github.com/AnshGajera/CTX/internal/privacy"
	"github.com/AnshGajera/CTX/internal/setup"
	"github.com/AnshGajera/CTX/internal/tui"
	"github.com/AnshGajera/CTX/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/AnshGajera/CTX/internal/version"
)

var defaultIgnore = `# ctx ignores
*.env
.env
*.pem
*.key
secrets/
node_modules/
vendor/
dist/
build/
.next/
__pycache__/
.venv/
.idea/
.vscode/
coverage/
.ctx/
.turbo/
.cache/
target/
ctx
ctx.exe
*.exe
*.mp4
*.mov
*.png
*.jpg
*.jpeg
*.pdf
`

// InitCommand implements `ctx init`.
func InitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize ctx in current project",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "project name"},
			&cli.BoolFlag{Name: "auto-extract", Value: true, Usage: "run extraction after init"},
			&cli.BoolFlag{Name: "install-hooks", Value: true, Usage: "install git hooks"},
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "non-interactive: accept defaults, skip wizard"},
			&cli.StringFlag{Name: "editor", Usage: "MCP editor setup: cursor|claude-desktop|vscode|none"},
			&cli.StringSliceFlag{Name: "sections", Usage: "limit sections (repeatable)"},
			jsonFlag(),
		},
		Action: func(c *cli.Context) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			jsonMode := c.Bool("json")
			interactive := !jsonMode && !c.Bool("yes") && tui.IsTerminal(os.Stdin)
			reader := bufio.NewReader(os.Stdin)
			if !jsonMode {
				tui.PrintBanner(version.Version)
				color.Cyan("Detecting project...")
			}
			profile, err := detector.New(cwd).Detect()
			if err != nil {
				return err
			}
			name := c.String("name")
			if name == "" {
				name = filepath.Base(cwd)
			}
			sections := c.StringSlice("sections")
			editor := c.String("editor")
			if interactive {
				if c.String("name") == "" {
					name = tui.PromptLine(reader, os.Stdout, "Project name", name)
				}
				if len(sections) == 0 {
					sections = tui.PromptSections(reader, os.Stdout, tui.DefaultSections())
				}
				if editor == "" {
					editor = tui.PromptEditor(reader, os.Stdout)
				}
			}
			if editor != "" && editor != "none" {
				valid := false
				for _, e := range setup.SupportedEditors() {
					if editor == e {
						valid = true
					}
				}
				if !valid {
					return fmt.Errorf("unknown editor %q (choose from: cursor, claude-desktop, vscode, none)", editor)
				}
			}
			if !jsonMode {
				fmt.Printf("Languages: ")
				for _, l := range profile.Languages {
					fmt.Printf("%s (%.1f%%) ", l.Name, l.Percentage)
				}
				fmt.Println()
				fmt.Printf("Frameworks: ")
				for _, f := range profile.Frameworks {
					fmt.Printf("%s ", f.Name)
				}
				fmt.Println()
				fmt.Printf("Package manager: %s | Type: %s | Monorepo: %v\n",
					profile.PackageManager, profile.ProjectType, profile.IsMonorepo)
			}

			ctxDir := filepath.Join(cwd, ".ctx")
			for _, d := range []string{"snapshots", "cache", "hooks"} {
				if err := os.MkdirAll(filepath.Join(ctxDir, d), 0o755); err != nil {
					return err
				}
			}
			manifest := projctx.NewManifest(name, cwd, *profile)
			// Preserve identity on re-init: never rotate ContextID or
			// silently drop history.
			if existing, err := projctx.LoadManifest(filepath.Join(ctxDir, "manifest.json")); err == nil && existing.ContextID != "" {
				manifest.ContextID = existing.ContextID
				manifest.CreatedAt = existing.CreatedAt
				if !jsonMode {
					color.Cyan("Existing project detected — kept context ID %s", shortHash(existing.ContextID))
				}
			}
			if err := manifest.Save(filepath.Join(ctxDir, "manifest.json")); err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(cwd, ".ctxignore")); os.IsNotExist(err) {
				_ = os.WriteFile(filepath.Join(cwd, ".ctxignore"), []byte(defaultIgnore), 0o644)
			}
			// project config
			cfgPath := config.ProjectConfigPath(cwd)
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				_ = config.WriteDefault(cfgPath)
			}
			if c.Bool("install-hooks") {
				installPostCommitHook(cwd, !jsonMode)
			}
			appendGitignore(cwd)
			if editor != "" && editor != "none" {
				path, err := setup.Install(editor, cwd)
				if err != nil {
					color.Yellow("Editor setup skipped: %v", err)
				} else if !jsonMode {
					color.Green("MCP configured for %s → %s", editor, path)
					fmt.Println("Restart your editor, then ask it to use the ctx tools (try: search_context).")
				}
			}
			if c.Bool("auto-extract") {
				_, err := runExtract(cwd, c.Bool("json"), false, sections)
				if err != nil {
					return err
				}
				if c.Bool("json") {
					manifest, _ := projctx.LoadManifest(filepath.Join(cwd, ".ctx", "manifest.json"))
					return json.NewEncoder(os.Stdout).Encode(manifest)
				}
				return nil
			}
			if c.Bool("json") {
				return json.NewEncoder(os.Stdout).Encode(manifest)
			}
			color.Green("Initialized ctx in %s", ctxDir)
			return nil
		},
	}
}

func installPostCommitHook(root string, verbose bool) {
	hookDir := filepath.Join(root, ".git", "hooks")
	if _, err := os.Stat(hookDir); err != nil {
		return
	}
	hook := filepath.Join(hookDir, "post-commit")
	hookLine := "ctx extract --quiet --on-commit &"
	if data, err := os.ReadFile(hook); err == nil {
		if strings.Contains(string(data), "ctx extract") {
			return
		}
		out := string(data)
		if len(out) > 0 && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += hookLine + "\n"
		_ = os.WriteFile(hook, []byte(out), 0o755)
		if verbose {
			fmt.Println("Installed post-commit hook")
		}
		return
	}
	content := "#!/bin/sh\n" + hookLine + "\n"
	_ = os.WriteFile(hook, []byte(content), 0o755)
	if verbose {
		fmt.Println("Installed post-commit hook")
	}
}

func appendGitignore(root string) {
	p := filepath.Join(root, ".gitignore")
	data, _ := os.ReadFile(p)
	if len(data) > 0 && containsLine(string(data), ".ctx/cache/") {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString("\n.ctx/cache/\n")
}

func containsLine(s, sub string) bool {
	for _, line := range splitLines(s) {
		if line == sub {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
		} else if r != '\r' {
			cur += string(r)
		}
	}
	out = append(out, cur)
	return out
}

// resolveSections prefers the explicit --sections flag, else config toggles.
// Empty return = all sections.
func resolveSections(flag []string, cfg config.Config) []string {
	if len(flag) > 0 {
		return flag
	}
	var out []string
	if cfg.Extraction.Architecture {
		out = append(out, engine.SectionArchitecture)
	}
	if cfg.Extraction.APIEndpoints {
		out = append(out, engine.SectionAPIEndpoints)
	}
	if cfg.Extraction.DatabaseSchema {
		out = append(out, engine.SectionDatabase)
	}
	if cfg.Extraction.Dependencies {
		out = append(out, engine.SectionDependencies)
	}
	if cfg.Extraction.BusinessRules {
		out = append(out, engine.SectionBusiness)
	}
	if cfg.Extraction.EnvVars {
		out = append(out, engine.SectionEnvVars)
	}
	if len(out) == 6 {
		return nil // all on = no filter
	}
	return out
}

// runExtract shared by init/extract. Returns deduped=true when the
// extraction produced no new snapshot.
func runExtract(root string, quiet, onCommit bool, sections []string) (bool, error) {
	manifest, err := projctx.LoadManifest(filepath.Join(root, ".ctx", "manifest.json"))
	if err != nil {
		return false, fmt.Errorf("not initialized (run ctx init): %w", err)
	}
	cfg, cfgErr := config.Load(config.ProjectConfigPath(root))
	if cfgErr != nil {
		color.Yellow("Warning: could not load config, using defaults: %v", cfgErr)
		cfg = config.Default()
	}
	// Incremental fast path: tree unchanged → refresh git state only.
	if !onCommit {
		if deduped, handled, herr := tryFastRefresh(root, cfg); herr == nil && handled {
			if quiet {
				return deduped, nil
			}
			if deduped {
				head, _ := versioning.NewContextStore(filepath.Join(root, ".ctx")).GetHead()
				color.Cyan("No file changes — HEAD unchanged (%s)", shortHash(head))
			} else {
				color.Cyan("No file changes — refreshed git state")
			}
			return deduped, nil
		}
	}
	eng := engine.NewExtractionEngine(root, &manifest.Profile, cfg.Core.MLURL, resolveSections(sections, cfg))
	if !quiet {
		eng.SetProgress(func(name string) { fmt.Printf("  extracting %s...\n", name) })
	}
	ctx, err := eng.Extract()
	if err != nil {
		return false, err
	}
	if manifest.ContextID != "" {
		ctx.ContextID = manifest.ContextID
	}
	// sanitize
	san := privacy.NewSanitizer(privacy.SanitizeConfig{
		RedactValues: cfg.Privacy.RedactValues, HashIdentifiers: cfg.Privacy.HashIdentifiers,
		ExcludePatterns: cfg.Privacy.ExcludePatterns,
	})
	san.SanitizeContext(ctx)
	ctxDir := filepath.Join(root, ".ctx")
	backfillMissingSections(ctx, root)
	if err := ctx.Save(ctxDir); err != nil {
		return false, err
	}
	store := versioning.NewContextStore(ctxDir)
	msg := "extract"
	if onCommit {
		msg = "extract (on-commit)"
	}
	snap, deduped, err := store.Commit(ctx, msg)
	if err != nil {
		return false, err
	}
	// Record tree state so the next run can take the fast path.
	if th, terr := cache.ComputeTreeHash(root); terr == nil {
		_ = cache.Save(ctxDir, th)
	}
	if !quiet {
		nEP := 0
		if ctx.APIs != nil {
			nEP = len(ctx.APIs.Endpoints)
		}
		nModels := 0
		if ctx.Database != nil {
			nModels = len(ctx.Database.Models)
		}
		if deduped {
			color.Cyan("No changes since %s — HEAD unchanged", snap.Hash)
		} else {
			color.Green("Extracted %d endpoints, %d models → %s", nEP, nModels, snap.Hash)
		}
	}
	return deduped, nil
}

// tryFastRefresh skips the full scan when the file tree is unchanged,
// refreshing only git state. Returns handled=true when the fast path
// applied (deduped reports whether HEAD moved).
func tryFastRefresh(root string, cfg config.Config) (deduped, handled bool, err error) {
	ctxDir := filepath.Join(root, ".ctx")
	th, err := cache.ComputeTreeHash(root)
	if err != nil || th == "" {
		return false, false, err
	}
	if cached := cache.Load(ctxDir); cached.Hash == "" || cached.Hash != th {
		return false, false, nil
	}
	ctx, err := projctx.LoadContext(root)
	if err != nil {
		return false, false, nil
	}
	manifest, err := projctx.LoadManifest(filepath.Join(ctxDir, "manifest.json"))
	if err != nil {
		return false, false, nil
	}
	ctx.Profile = manifest.Profile
	if manifest.ContextID != "" {
		ctx.ContextID = manifest.ContextID
	}
	ctx.ExtractedAt = time.Now().UTC()
	// Git state is cheap (no tree walk) and always worth refreshing.
	if gerr := extractors.NewGitState(root).Extract(ctx); gerr != nil {
		return false, false, nil
	}
	san := privacy.NewSanitizer(privacy.SanitizeConfig{
		RedactValues: cfg.Privacy.RedactValues, HashIdentifiers: cfg.Privacy.HashIdentifiers,
		ExcludePatterns: cfg.Privacy.ExcludePatterns,
	})
	san.SanitizeContext(ctx)
	if err := ctx.Save(ctxDir); err != nil {
		return false, false, err
	}
	store := versioning.NewContextStore(ctxDir)
	_, deduped, err = store.Commit(ctx, "extract")
	if err != nil {
		return false, false, err
	}
	return deduped, true, nil
}

// backfillMissingSections copies sections a filtered extraction skipped
// from the previous context, so partial extracts never wipe saved data.
func backfillMissingSections(ctx *projctx.ProjectContext, root string) {
	old, err := projctx.LoadContext(root)
	if err != nil {
		return
	}
	if ctx.Architecture == nil {
		ctx.Architecture = old.Architecture
	}
	if ctx.APIs == nil {
		ctx.APIs = old.APIs
	}
	if ctx.Database == nil {
		ctx.Database = old.Database
	}
	if ctx.Dependencies == nil {
		ctx.Dependencies = old.Dependencies
	}
	if ctx.Environment == nil {
		ctx.Environment = old.Environment
	}
	if ctx.FileStructure == nil {
		ctx.FileStructure = old.FileStructure
	}
	if ctx.BusinessRules == nil {
		ctx.BusinessRules = old.BusinessRules
	}
	if ctx.Decisions == nil {
		ctx.Decisions = old.Decisions
	}
	if ctx.Patterns == nil {
		ctx.Patterns = old.Patterns
	}
}

// extractSummary is the machine-readable extract result.
func extractSummary(root string, deduped bool) (map[string]any, error) {
	ctx, err := projctx.LoadContext(root)
	if err != nil {
		return nil, err
	}
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
	return map[string]any{
		"project": ctx.ProjectName, "hash": ctx.ContentHash,
		"deduped": deduped, "endpoints": nEP, "models": nModels,
		"env_vars": nEnv, "dependencies": nDeps,
	}, nil
}
