package extractors

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// Extractor extracts one section of project context.
type Extractor interface {
	Name() string
	Extract(ctx *projctx.ProjectContext) error
}

// Base holds shared state for extractors.
type Base struct {
	Root    string
	Ignored []string
	ignore  []*ignorePattern
}

// NewBase creates a Base loading .ctxignore.
func NewBase(root string) Base {
	raw := LoadIgnore(root)
	return Base{Root: root, Ignored: raw, ignore: compileIgnorePatterns(raw)}
}

// SkipDirs are never walked.
var SkipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true,
	"build": true, ".next": true, "__pycache__": true, ".venv": true,
	"venv": true, ".idea": true, ".vscode": true, "coverage": true,
	".ctx": true, ".turbo": true, ".cache": true, "target": true,
	".nuxt": true, ".output": true, "out": true,
}

// LoadIgnore reads .ctxignore + defaults.
func LoadIgnore(root string) []string {
	patterns := []string{"*.env", ".env"}
	path := filepath.Join(root, ".ctxignore")
	f, err := os.Open(path)
	if err != nil {
		return patterns
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// ShouldSkipDir reports if a dir name should be skipped.
func ShouldSkipDir(name string) bool { return SkipDirs[name] }

// ShouldExclude reports if rel path matches ignore patterns.
func (b Base) ShouldExclude(rel string) bool {
	if len(b.ignore) > 0 {
		return matchIgnore(b.ignore, rel)
	}
	// Fallback for zero-value Base in tests: compile raw patterns on the fly.
	return matchIgnore(compileIgnorePatterns(b.Ignored), rel)
}

// WalkFiles walks source files, skipping ignored dirs.
func (b Base) WalkFiles(fn func(path, rel string, info os.FileInfo) error) error {
	return filepath.Walk(b.Root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(b.Root, p)
		if info.IsDir() {
			if p != b.Root && ShouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if b.ShouldExclude(rel) {
			return nil
		}
		return fn(p, rel, info)
	})
}

// SourceExtensions limits code scanning.
func IsSourceFile(name string) bool {
	switch {
	case strings.HasSuffix(name, ".ts"), strings.HasSuffix(name, ".tsx"),
		strings.HasSuffix(name, ".js"), strings.HasSuffix(name, ".jsx"),
		strings.HasSuffix(name, ".mjs"), strings.HasSuffix(name, ".cjs"),
		strings.HasSuffix(name, ".go"), strings.HasSuffix(name, ".py"),
		strings.HasSuffix(name, ".rs"), strings.HasSuffix(name, ".java"),
		strings.HasSuffix(name, ".kt"), strings.HasSuffix(name, ".rb"),
		strings.HasSuffix(name, ".vue"), strings.HasSuffix(name, ".svelte"):
		return true
	}
	return false
}
