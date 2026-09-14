package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctxdev/ctx/internal/config"
	projctx "github.com/ctxdev/ctx/internal/context"
	"github.com/ctxdev/ctx/internal/detector"
	"github.com/ctxdev/ctx/internal/engine"
	"github.com/ctxdev/ctx/internal/privacy"
	"github.com/ctxdev/ctx/internal/versioning"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
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
		},
		Action: func(c *cli.Context) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			name := c.String("name")
			if name == "" {
				name = filepath.Base(cwd)
			}
			color.Cyan("Detecting project...")
			profile, err := detector.New(cwd).Detect()
			if err != nil {
				return err
			}
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

			ctxDir := filepath.Join(cwd, ".ctx")
			for _, d := range []string{"snapshots", "cache", "hooks"} {
				if err := os.MkdirAll(filepath.Join(ctxDir, d), 0o755); err != nil {
					return err
				}
			}
			manifest := projctx.NewManifest(name, cwd, *profile)
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
				installPostCommitHook(cwd)
			}
			appendGitignore(cwd)
			if c.Bool("auto-extract") {
				return runExtract(cwd, false, false, nil)
			}
			color.Green("Initialized ctx in %s", ctxDir)
			return nil
		},
	}
}

func installPostCommitHook(root string) {
	hookDir := filepath.Join(root, ".git", "hooks")
	if _, err := os.Stat(hookDir); err != nil {
		return
	}
	hook := filepath.Join(hookDir, "post-commit")
	content := "#!/bin/sh\nctx extract --quiet --on-commit &\n"
	_ = os.WriteFile(hook, []byte(content), 0o755)
	fmt.Println("Installed post-commit hook")
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

// runExtract shared by init/extract.
func runExtract(root string, quiet, onCommit bool, sections []string) error {
	_ = sections
	manifest, err := projctx.LoadManifest(filepath.Join(root, ".ctx", "manifest.json"))
	if err != nil {
		return fmt.Errorf("not initialized (run ctx init): %w", err)
	}
	_ = manifest
	eng := engine.NewExtractionEngine(root, &manifest.Profile)
	if !quiet {
		eng.SetProgress(func(name string) { fmt.Printf("  extracting %s...\n", name) })
	}
	ctx, err := eng.Extract()
	if err != nil {
		return err
	}
	if manifest.ContextID != "" {
		ctx.ContextID = manifest.ContextID
	}
	// sanitize
	cfg, _ := config.Load(config.ProjectConfigPath(root))
	san := privacy.NewSanitizer(privacy.SanitizeConfig{
		RedactValues: cfg.Privacy.RedactValues, HashIdentifiers: cfg.Privacy.HashIdentifiers,
		ExcludePatterns: cfg.Privacy.ExcludePatterns,
	})
	san.SanitizeContext(ctx)
	ctxDir := filepath.Join(root, ".ctx")
	if err := ctx.Save(ctxDir); err != nil {
		return err
	}
	store := versioning.NewContextStore(ctxDir)
	msg := "extract"
	if onCommit {
		msg = "extract (on-commit)"
	}
	snap, err := store.Commit(ctx, msg)
	if err != nil {
		return err
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
		color.Green("Extracted %d endpoints, %d models → %s", nEP, nModels, snap.Hash)
	}
	return nil
}
