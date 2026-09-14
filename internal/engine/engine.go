package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/extractors"
	"github.com/AnshGajera/CTX/internal/versioning"
)

// ExtractionEngine orchestrates extractors.
type ExtractionEngine struct {
	root       string
	profile    *projctx.ProjectProfile
	extractors []extractors.Extractor
	onProgress func(name string)
}

// NewExtractionEngine registers extractors based on profile.
// mlURL enables the sidecar AST extractor ("" disables it).
func NewExtractionEngine(root string, profile *projctx.ProjectProfile, mlURL string) *ExtractionEngine {
	e := &ExtractionEngine{root: root, profile: profile}
	// Always-on
	e.extractors = append(e.extractors,
		extractors.NewFileStructure(root),
		extractors.NewDependencies(root),
		extractors.NewEnvironment(root),
		extractors.NewGitState(root),
		extractors.NewTODO(root),
		extractors.NewPatterns(root),
	)
	langs := map[string]bool{}
	fws := map[string]bool{}
	if profile != nil {
		for _, l := range profile.Languages {
			langs[strings.ToLower(l.Name)] = true
		}
		for _, f := range profile.Frameworks {
			fws[strings.ToLower(f.Name)] = true
		}
	}
	hasTS := langs["typescript"] || langs["javascript"]
	if hasTS {
		e.extractors = append(e.extractors, extractors.NewTypeScriptAPI(root))
	}
	if fws["next.js"] || fws["next"] {
		e.extractors = append(e.extractors, extractors.NewNextJS(root))
	}
	if fws["express"] || fws["fastify"] || fws["hono"] {
		e.extractors = append(e.extractors, extractors.NewExpressAPI(root))
	}
	if langs["go"] {
		e.extractors = append(e.extractors, extractors.NewGoAPI(root))
	}
	if langs["python"] {
		e.extractors = append(e.extractors, extractors.NewPythonAPI(root))
	}
	e.extractors = append(e.extractors, extractors.NewPrisma(root))
	e.extractors = append(e.extractors, extractors.NewGenericModels(root))
	// AST precision pass last: merges only endpoints regex missed.
	e.extractors = append(e.extractors, extractors.NewSidecarAST(root, mlURL))
	return e
}

// SetProgress sets an optional progress callback.
func (e *ExtractionEngine) SetProgress(fn func(name string)) { e.onProgress = fn }

// Extract runs all extractors and returns the context.
func (e *ExtractionEngine) Extract() (*projctx.ProjectContext, error) {
	name := filepath.Base(e.root)
	profile := projctx.ProjectProfile{}
	if e.profile != nil {
		profile = *e.profile
	}
	ctx := &projctx.ProjectContext{
		Version:     1,
		ProjectName: name,
		ExtractedAt: time.Now().UTC(),
		Profile:     profile,
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make([]string, 0)
	for _, ex := range e.extractors {
		wg.Add(1)
		go func(ex extractors.Extractor) {
			defer wg.Done()
			if e.onProgress != nil {
				e.onProgress(ex.Name())
			}
			mu.Lock()
			defer mu.Unlock()
			if err := ex.Extract(ctx); err != nil {
				errs = append(errs, ex.Name()+": "+err.Error())
			}
		}(ex)
	}
	wg.Wait()
	// fixup: TODO extractor may run before GitState creates CurrentState;
	// both handle nil, but ensure TODOs survive regardless of order by
	// re-running TODO last if state was overwritten — instead we merge:
	// (extractors already nil-guard, last writer wins only for CurrentState
	//  struct creation; TODOs could be lost if GitState ran after TODO).
	// Re-collect TODOs deterministically if missing.
	if ctx.CurrentState != nil && len(ctx.CurrentState.TODOs) == 0 {
		mu.Lock()
		_ = extractors.NewTODO(e.root).Extract(ctx)
		mu.Unlock()
	}
	// Deterministic ordering: extractors run concurrently and Go map
	// iteration is random, so sort everything before hashing. Without
	// this, identical code produces different hashes on every run.
	sortContext(ctx)
	hash, err := versioning.CanonicalHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("hash context: %w", err)
	}
	ctx.ContentHash = hash
	_ = errs
	return ctx, nil
}

// sortContext orders all list fields deterministically.
func sortContext(ctx *projctx.ProjectContext) {
	if ctx.APIs != nil {
		sort.Slice(ctx.APIs.Endpoints, func(i, j int) bool {
			a, b := ctx.APIs.Endpoints[i], ctx.APIs.Endpoints[j]
			if a.Method != b.Method {
				return a.Method < b.Method
			}
			if a.Path != b.Path {
				return a.Path < b.Path
			}
			return a.File < b.File
		})
	}
	if ctx.Database != nil {
		sort.Slice(ctx.Database.Models, func(i, j int) bool {
			return ctx.Database.Models[i].Name < ctx.Database.Models[j].Name
		})
		for k := range ctx.Database.Models {
			m := &ctx.Database.Models[k]
			sort.Slice(m.Fields, func(i, j int) bool { return m.Fields[i].Name < m.Fields[j].Name })
		}
		sort.Slice(ctx.Database.Relations, func(i, j int) bool {
			a, b := ctx.Database.Relations[i], ctx.Database.Relations[j]
			if a.From != b.From {
				return a.From < b.From
			}
			return a.To < b.To
		})
		sort.Slice(ctx.Database.Migrations, func(i, j int) bool {
			return ctx.Database.Migrations[i].File < ctx.Database.Migrations[j].File
		})
	}
	if ctx.Dependencies != nil {
		sort.Slice(ctx.Dependencies.Direct, func(i, j int) bool {
			return ctx.Dependencies.Direct[i].Name < ctx.Dependencies.Direct[j].Name
		})
		sort.Slice(ctx.Dependencies.Dev, func(i, j int) bool {
			return ctx.Dependencies.Dev[i].Name < ctx.Dependencies.Dev[j].Name
		})
	}
	if ctx.Environment != nil {
		sort.Slice(ctx.Environment.Variables, func(i, j int) bool {
			return ctx.Environment.Variables[i].Name < ctx.Environment.Variables[j].Name
		})
		sort.Strings(ctx.Environment.Required)
	}
	if ctx.Patterns != nil {
		sort.Slice(ctx.Patterns.Patterns, func(i, j int) bool {
			return ctx.Patterns.Patterns[i].Name < ctx.Patterns.Patterns[j].Name
		})
	}
	if ctx.CurrentState != nil {
		sort.Strings(ctx.CurrentState.RecentlyChanged)
		sort.Strings(ctx.CurrentState.ActiveAreas)
		sort.Slice(ctx.CurrentState.TODOs, func(i, j int) bool {
			a, b := ctx.CurrentState.TODOs[i], ctx.CurrentState.TODOs[j]
			if a.File != b.File {
				return a.File < b.File
			}
			return a.Line < b.Line
		})
	}
	if ctx.FileStructure != nil {
		sort.Slice(ctx.FileStructure.KeyFiles, func(i, j int) bool {
			return ctx.FileStructure.KeyFiles[i].Path < ctx.FileStructure.KeyFiles[j].Path
		})
		sort.Slice(ctx.FileStructure.Conventions, func(i, j int) bool {
			return ctx.FileStructure.Conventions[i].Pattern < ctx.FileStructure.Conventions[j].Pattern
		})
		sortDir(ctx.FileStructure.Tree)
	}
}

func sortDir(n *projctx.DirectoryNode) {
	if n == nil {
		return
	}
	sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Name < n.Children[j].Name })
	for _, c := range n.Children {
		sortDir(c)
	}
}
