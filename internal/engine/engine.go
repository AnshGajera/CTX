package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
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
	sections   map[string]bool // nil = all sections enabled
}

// Section names for --sections / config gating.
const (
	SectionArchitecture = "architecture"
	SectionAPIEndpoints = "api_endpoints"
	SectionDatabase     = "database_schema"
	SectionDependencies = "dependencies"
	SectionBusiness     = "business_rules"
	SectionEnvVars      = "env_vars"
)

// SetSections limits extraction to named sections (empty = all).
// Names are normalized: case-insensitive, "-" and "_" equivalent.
func (e *ExtractionEngine) SetSections(sections []string) {
	if len(sections) == 0 {
		e.sections = nil
		return
	}
	e.sections = map[string]bool{}
	for _, s := range sections {
		e.sections[normalizeSectionName(s)] = true
	}
}

func normalizeSectionName(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", "_")
}

func (e *ExtractionEngine) sectionOn(name string) bool {
	if e.sections == nil {
		return true
	}
	return e.sections[normalizeSectionName(name)]
}

// NewExtractionEngine registers extractors based on profile.
// mlURL enables the sidecar AST extractor ("" disables it).
func NewExtractionEngine(root string, profile *projctx.ProjectProfile, mlURL string, sections []string) *ExtractionEngine {
	e := &ExtractionEngine{root: root, profile: profile}
	e.SetSections(sections)
	// Always-on foundation + state
	e.extractors = append(e.extractors,
		extractors.NewFileStructure(root),
		extractors.NewGitState(root),
		extractors.NewTODO(root),
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
	if e.sectionOn(SectionArchitecture) {
		e.extractors = append(e.extractors, extractors.NewArchitecture(root))
		e.extractors = append(e.extractors, extractors.NewPatterns(root))
	}
	if e.sectionOn(SectionDependencies) {
		e.extractors = append(e.extractors, extractors.NewDependencies(root))
	}
	if e.sectionOn(SectionEnvVars) {
		e.extractors = append(e.extractors, extractors.NewEnvironment(root))
	}
	if e.sectionOn(SectionAPIEndpoints) {
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
		// AST precision pass: merges only endpoints regex missed.
		e.extractors = append(e.extractors, extractors.NewSidecarAST(root, mlURL))
	}
	if e.sectionOn(SectionDatabase) {
		e.extractors = append(e.extractors, extractors.NewPrisma(root))
		e.extractors = append(e.extractors, extractors.NewGenericModels(root))
	}
	// Business rules ride on patterns for now (enriched later).
	return e
}

// SetProgress sets an optional progress callback.
func (e *ExtractionEngine) SetProgress(fn func(name string)) { e.onProgress = fn }

// Extract runs all extractors truly in parallel — each gets a private
// partial context — then merges in registration order with global
// dedupe, sorts deterministically, and hashes.
func (e *ExtractionEngine) Extract() (*projctx.ProjectContext, error) {
	name := filepath.Base(e.root)
	profile := projctx.ProjectProfile{}
	if e.profile != nil {
		profile = *e.profile
	}
	partials := make([]*projctx.ProjectContext, len(e.extractors))
	errs := make([]string, len(e.extractors))
	var wg sync.WaitGroup
	for i, ex := range e.extractors {
		wg.Add(1)
		go func(i int, ex extractors.Extractor) {
			defer wg.Done()
			if e.onProgress != nil {
				e.onProgress(ex.Name())
			}
			partial := &projctx.ProjectContext{Profile: profile}
			if err := ex.Extract(partial); err != nil {
				errs[i] = ex.Name() + ": " + err.Error()
			}
			partials[i] = partial
		}(i, ex)
	}
	wg.Wait()
	ctx := &projctx.ProjectContext{
		Version:     1,
		ProjectName: name,
		ExtractedAt: time.Now().UTC(),
		Profile:     profile,
	}
	mergePartials(ctx, partials)
	// Deterministic ordering: extractor output order and Go map
	// iteration are random, so sort everything before hashing. Without
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

// mergePartials combines per-extractor partials in order with global
// dedupe (first writer wins), so overlapping extractors — TS regex vs
// Next.js vs sidecar AST — never duplicate endpoints or models.
func mergePartials(dst *projctx.ProjectContext, partials []*projctx.ProjectContext) {
	epSeen := map[string]bool{}
	modelSeen := map[string]bool{}
	relSeen := map[string]bool{}
	migSeen := map[string]bool{}
	depSeen := map[string]bool{}
	envSeen := map[string]bool{}
	ruleSeen := map[string]bool{}
	patSeen := map[string]*projctx.CodePattern{}
	todoSeen := map[string]bool{}
	reqSeen := map[string]bool{}
	profSeen := map[string]bool{}
	for _, p := range partials {
		if p == nil {
			continue
		}
		if p.APIs != nil {
			if dst.APIs == nil {
				dst.APIs = &projctx.APIContext{}
			}
			for _, ep := range p.APIs.Endpoints {
				key := strings.ToUpper(ep.Method) + " " + ep.Path
				if epSeen[key] {
					continue
				}
				epSeen[key] = true
				dst.APIs.Endpoints = append(dst.APIs.Endpoints, ep)
			}
			if dst.APIs.AuthType == "" {
				dst.APIs.AuthType = p.APIs.AuthType
			}
			if dst.APIs.BaseURL == "" {
				dst.APIs.BaseURL = p.APIs.BaseURL
			}
			if dst.APIs.Version == "" {
				dst.APIs.Version = p.APIs.Version
			}
		}
		if p.Database != nil {
			if dst.Database == nil {
				dst.Database = &projctx.DatabaseContext{}
			}
			d, s := dst.Database, p.Database
			for _, m := range s.Models {
				if modelSeen[m.Name] {
					continue
				}
				modelSeen[m.Name] = true
				d.Models = append(d.Models, m)
			}
			for _, r := range s.Relations {
				key := r.From + ">" + r.To + "#" + r.FieldName
				if relSeen[key] {
					continue
				}
				relSeen[key] = true
				d.Relations = append(d.Relations, r)
			}
			for _, m := range s.Migrations {
				if migSeen[m.File] {
					continue
				}
				migSeen[m.File] = true
				d.Migrations = append(d.Migrations, m)
			}
			if d.ORM == "" {
				d.ORM = s.ORM
			}
			if d.Type == "" {
				d.Type = s.Type
			}
			if d.Diagram == "" {
				d.Diagram = s.Diagram
			}
		}
		if p.Dependencies != nil {
			if dst.Dependencies == nil {
				dst.Dependencies = &projctx.DependencyContext{}
			}
			for _, dep := range append(append([]projctx.Dependency{}, p.Dependencies.Direct...), p.Dependencies.Dev...) {
				if depSeen[dep.Name] {
					continue
				}
				depSeen[dep.Name] = true
				dst.Dependencies.Direct = append(dst.Dependencies.Direct, dep)
			}
			dst.Dependencies.Internal = append(dst.Dependencies.Internal, p.Dependencies.Internal...)
		}
		if p.Environment != nil {
			if dst.Environment == nil {
				dst.Environment = &projctx.EnvironmentContext{}
			}
			for _, v := range p.Environment.Variables {
				if envSeen[v.Name] {
					continue
				}
				envSeen[v.Name] = true
				dst.Environment.Variables = append(dst.Environment.Variables, v)
			}
			for _, r := range p.Environment.Required {
				if !reqSeen[r] {
					reqSeen[r] = true
					dst.Environment.Required = append(dst.Environment.Required, r)
				}
			}
			for _, pr := range p.Environment.Profiles {
				if !profSeen[pr] {
					profSeen[pr] = true
					dst.Environment.Profiles = append(dst.Environment.Profiles, pr)
				}
			}
		}
		if p.FileStructure != nil && dst.FileStructure == nil {
			dst.FileStructure = p.FileStructure
		}
		if p.Architecture != nil && dst.Architecture == nil {
			dst.Architecture = p.Architecture
		}
		if p.BusinessRules != nil {
			if dst.BusinessRules == nil {
				dst.BusinessRules = &projctx.BusinessRuleContext{}
			}
			for _, r := range p.BusinessRules.Rules {
				if ruleSeen[r.ID] {
					continue
				}
				ruleSeen[r.ID] = true
				dst.BusinessRules.Rules = append(dst.BusinessRules.Rules, r)
			}
		}
		if p.Decisions != nil {
			if dst.Decisions == nil {
				dst.Decisions = &projctx.DecisionContext{}
			}
			dst.Decisions.Decisions = append(dst.Decisions.Decisions, p.Decisions.Decisions...)
		}
		if p.Patterns != nil {
			if dst.Patterns == nil {
				dst.Patterns = &projctx.PatternContext{}
			}
			for _, cp := range p.Patterns.Patterns {
				if existing, ok := patSeen[cp.Name]; ok {
					for _, ex := range cp.Examples {
						if len(existing.Examples) >= 5 {
							break
						}
						dup := false
						for _, o := range existing.Examples {
							if o == ex {
								dup = true
								break
							}
						}
						if !dup {
							existing.Examples = append(existing.Examples, ex)
						}
					}
					existing.Frequency += cp.Frequency
					continue
				}
				cpy := cp
				dst.Patterns.Patterns = append(dst.Patterns.Patterns, cpy)
				patSeen[cp.Name] = &dst.Patterns.Patterns[len(dst.Patterns.Patterns)-1]
			}
		}
		if p.CurrentState != nil {
			if dst.CurrentState == nil {
				dst.CurrentState = &projctx.ProjectStateContext{}
			}
			d, s := dst.CurrentState, p.CurrentState
			if d.GitBranch == "" {
				d.GitBranch = s.GitBranch
			}
			if d.LastCommit == "" {
				d.LastCommit = s.LastCommit
			}
			if d.LastCommitMsg == "" {
				d.LastCommitMsg = s.LastCommitMsg
			}
			if d.DirtyFiles == 0 {
				d.DirtyFiles = s.DirtyFiles
			}
			d.RecentlyChanged = append(d.RecentlyChanged, s.RecentlyChanged...)
			d.ActiveAreas = append(d.ActiveAreas, s.ActiveAreas...)
			for _, td := range s.TODOs {
				key := td.File + ":" + strconv.Itoa(td.Line) + ":" + td.Text
				if todoSeen[key] {
					continue
				}
				todoSeen[key] = true
				d.TODOs = append(d.TODOs, td)
			}
		}
	}
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
