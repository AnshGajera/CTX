package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	projctx "github.com/ctxdev/ctx/internal/context"
	"github.com/ctxdev/ctx/internal/extractors"
)

// ExtractionEngine orchestrates extractors.
type ExtractionEngine struct {
	root       string
	profile    *projctx.ProjectProfile
	extractors []extractors.Extractor
	onProgress func(name string)
}

// NewExtractionEngine registers extractors based on profile.
func NewExtractionEngine(root string, profile *projctx.ProjectProfile) *ExtractionEngine {
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
	hash, err := contentHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("hash context: %w", err)
	}
	ctx.ContentHash = hash
	_ = errs
	return ctx, nil
}

func contentHash(ctx *projctx.ProjectContext) (string, error) {
	clone := *ctx
	clone.ContentHash = ""
	data, err := json.Marshal(clone)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16]), nil
}
