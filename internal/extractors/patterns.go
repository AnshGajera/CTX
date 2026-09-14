package extractors

import (
	"os"
	"path/filepath"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// PatternExtractor detects architectural/code patterns.
type PatternExtractor struct{ Base }

func NewPatterns(root string) *PatternExtractor {
	return &PatternExtractor{Base: NewBase(root)}
}

func (e *PatternExtractor) Name() string { return "patterns" }

func (e *PatternExtractor) Extract(ctx *projctx.ProjectContext) error {
	counts := map[string]int{}
	examples := map[string][]string{}
	add := func(name, ex string) {
		counts[name]++
		if len(examples[name]) < 5 {
			examples[name] = append(examples[name], filepath.ToSlash(ex))
		}
	}
	hasDir := func(names ...string) bool {
		for _, n := range names {
			if st, err := os.Stat(filepath.Join(e.Root, n)); err == nil && st.IsDir() {
				return true
			}
		}
		return false
	}
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		base := strings.ToLower(filepath.Base(rel))
		lower := strings.ToLower(rel)
		if strings.Contains(base, "repository") || strings.Contains(base, "repo") {
			add("Repository Pattern", rel)
		}
		if strings.Contains(base, "service") {
			add("Service Layer", rel)
		}
		if strings.Contains(base, ".action.ts") {
			add("Server Actions", rel)
		}
		if strings.Contains(lower, "middleware") {
			add("Middleware Chain", rel)
		}
		if strings.Contains(lower, "events/") || strings.Contains(lower, "listeners/") || strings.Contains(lower, "subscribers/") {
			add("Event-Driven", rel)
		}
		if strings.Contains(base, "getinstance") {
			add("Singleton", rel)
		}
		// singleton via content scan (cheap: filename hint + content)
		return nil
	})
	// content scan for singleton getInstance
	_ = e.WalkFiles(func(path, rel string, info os.FileInfo) error {
		if !IsSourceFile(filepath.Base(path)) || info.Size() > 500_000 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), "getInstance") {
			add("Singleton", rel)
		}
		return nil
	})
	if hasDir("models", "views", "controllers") {
		add("MVC", "models/+views/+controllers")
	}
	if st, err := os.Stat(filepath.Join(e.Root, "components")); err == nil && st.IsDir() {
		n := 0
		_ = filepath.Walk(filepath.Join(e.Root, "components"), func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				n++
			}
			return nil
		})
		if n > 3 {
			counts["Component-Based"] = n
			examples["Component-Based"] = []string{"components/"}
		}
	}
	if hasDir("features", "modules") {
		add("Feature Modules", "features/ or modules/")
	}
	descs := map[string]string{
		"Repository Pattern": "data access abstraction", "Service Layer": "business logic layer",
		"Server Actions": "next.js server actions", "Middleware Chain": "request middleware pipeline",
		"Event-Driven": "events/listeners/subscribers", "Singleton": "single shared instance",
		"MVC": "models/views/controllers separation", "Component-Based": "reusable ui components",
		"Feature Modules": "domain feature folders",
	}
	var patterns []projctx.CodePattern
	for name, c := range counts {
		patterns = append(patterns, projctx.CodePattern{
			Name: name, Type: "architectural", Description: descs[name],
			Examples: examples[name], Frequency: c,
		})
	}
	ctx.Patterns = &projctx.PatternContext{Patterns: patterns}
	// Business rules stub: derive from patterns dir presence (real impl: rules engine later)
	if ctx.BusinessRules == nil {
		ctx.BusinessRules = &projctx.BusinessRuleContext{}
	}
	return nil
}
