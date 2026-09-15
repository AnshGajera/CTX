package health

import (
	"sort"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// Check is one scored dimension of context quality.
type Check struct {
	Name   string  `json:"name"`
	Max    int     `json:"max"`
	Score  float64 `json:"score"`
	Status string  `json:"status"` // pass | warn | fail
	Tip    string  `json:"tip,omitempty"`
}

// Report is the full context health assessment (0-100).
type Report struct {
	Score     int     `json:"score"`
	Grade     string  `json:"grade"` // A >= 85, B >= 70, C >= 50, D otherwise
	Staleness string  `json:"staleness"`
	Checks    []Check `json:"checks"`
	Tips      []string `json:"tips"`
}

func ratio(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total)
}

// Score grades context quality so teams can gamify keeping it healthy.
func Score(ctx *projctx.ProjectContext) Report {
	r := Report{Staleness: ctx.Staleness(time.Now())}
	add := func(name string, max int, got float64, tip string) {
		st := "pass"
		switch {
		case got >= float64(max)-0.001:
			st = "pass"
		case got > 0:
			st = "warn"
		default:
			st = "fail"
		}
		r.Checks = append(r.Checks, Check{Name: name, Max: max, Score: got, Status: st, Tip: tip})
		if st != "pass" && tip != "" {
			r.Tips = append(r.Tips, tip)
		}
	}

	// API documentation (15)
	nEP, documented := 0, 0
	if ctx.APIs != nil {
		for _, e := range ctx.APIs.Endpoints {
			nEP++
			if e.Description != "" {
				documented++
			}
		}
	}
	add("API endpoints documented", 15, 15*ratio(documented, nEP),
		"Add descriptions to route handlers so endpoints self-document")

	// Architecture pattern (10)
	arch := ""
	if ctx.Architecture != nil {
		arch = ctx.Architecture.Pattern
	}
	archScore := 0.0
	if arch != "" {
		archScore = 10
	}
	add("Architecture pattern detected", 10, archScore,
		"Use conventional layouts (cmd/, internal/, routes/) for automatic detection")

	// Database diagram (10)
	diagram := ""
	if ctx.Database != nil {
		diagram = ctx.Database.Diagram
	}
	diaScore := 0.0
	if diagram != "" {
		diaScore = 10
	}
	add("Database diagram generated", 10, diaScore,
		"Add Prisma schema or GORM/SQLAlchemy models for ER diagrams")

	// Env descriptions (10)
	nEnv, descEnv := 0, 0
	if ctx.Environment != nil {
		for _, v := range ctx.Environment.Variables {
			nEnv++
			if v.Description != "" {
				descEnv++
			}
		}
	}
	add("Env vars described", 10, 10*ratio(descEnv, nEnv),
		"Document variables in .env.example with trailing comments")

	// Business rules (10)
	rules := 0
	if ctx.BusinessRules != nil {
		rules = len(ctx.BusinessRules.Rules)
	}
	ruleScore := 0.0
	if rules > 0 {
		ruleScore = 10
	}
	add("Business rules captured", 10, ruleScore,
		"Add docs/decisions/ ADR files for automatic extraction")

	// Freshness (15)
	freshScore := 0.0
	switch r.Staleness {
	case "fresh":
		freshScore = 15
	case "stale":
		freshScore = 7
	}
	add("Context freshness", 15, freshScore,
		"Run ctx extract — context is "+r.Staleness)

	// Patterns (5)
	pats := 0
	if ctx.Patterns != nil {
		pats = len(ctx.Patterns.Patterns)
	}
	patScore := 0.0
	if pats > 0 {
		patScore = 5
	}
	add("Code patterns detected", 5, patScore, "")

	// Key files (5)
	keys := 0
	if ctx.FileStructure != nil {
		keys = len(ctx.FileStructure.KeyFiles)
	}
	keyScore := 0.0
	if keys > 0 {
		keyScore = 5
	}
	add("Key files identified", 5, keyScore, "")

	// Dependencies (5)
	deps := 0
	if ctx.Dependencies != nil {
		deps = len(ctx.Dependencies.Direct) + len(ctx.Dependencies.Dev)
	}
	depScore := 0.0
	if deps > 0 {
		depScore = 5
	}
	add("Dependencies tracked", 5, depScore, "")

	// File structure (5)
	structScore := 0.0
	if ctx.FileStructure != nil && ctx.FileStructure.Tree != nil {
		structScore = 5
	}
	add("File structure mapped", 5, structScore, "")

	// Models with fields (10)
	nModels, withFields := 0, 0
	if ctx.Database != nil {
		for _, m := range ctx.Database.Models {
			nModels++
			if len(m.Fields) > 0 {
				withFields++
			}
		}
	}
	modelScore := 10 * ratio(withFields, nModels)
	if nModels == 0 {
		modelScore = 10 // no database is not a defect
	}
	add("Models have fields", 10, modelScore,
		"Use tagged struct fields / ORM models so columns are extracted")

	total := 0.0
	for _, c := range r.Checks {
		total += c.Score
	}
	r.Score = int(total + 0.5)
	switch {
	case r.Score >= 85:
		r.Grade = "A"
	case r.Score >= 70:
		r.Grade = "B"
	case r.Score >= 50:
		r.Grade = "C"
	default:
		r.Grade = "D"
	}
	sort.Strings(r.Tips)
	return r
}
