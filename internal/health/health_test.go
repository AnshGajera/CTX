package health

import (
	"testing"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestScoreEmpty(t *testing.T) {
	ctx := &projctx.ProjectContext{ProjectName: "x"}
	rep := Score(ctx)
	if rep.Staleness != "expired" {
		t.Fatalf("zero time must be expired, got %s", rep.Staleness)
	}
	if rep.Grade != "D" {
		t.Fatalf("empty context must grade D, got %s (%d)", rep.Grade, rep.Score)
	}
	if len(rep.Checks) != 11 {
		t.Fatalf("expected 11 checks, got %d", len(rep.Checks))
	}
}

func TestScoreHealthy(t *testing.T) {
	ctx := &projctx.ProjectContext{
		ProjectName: "x",
		ExtractedAt: time.Now().UTC(),
		Architecture: &projctx.ArchitectureContext{
			Pattern: "layered",
			Diagram: "flowchart TD\n  A-->B",
		},
		APIs: &projctx.APIContext{Endpoints: []projctx.APIEndpoint{
			{Method: "GET", Path: "/a", Description: "get a"},
		}},
		Database: &projctx.DatabaseContext{
			Diagram: "erDiagram",
			Models: []projctx.DatabaseModel{
				{Name: "User", Fields: []projctx.ModelField{{Name: "id", Type: "string"}}},
			},
		},
		Dependencies: &projctx.DependencyContext{
			Direct: []projctx.Dependency{{Name: "x"}},
		},
		Environment: &projctx.EnvironmentContext{
			Variables: []projctx.EnvVariable{{Name: "PORT", Description: "port", Required: true}},
		},
		FileStructure: &projctx.FileStructureContext{
			Tree:     &projctx.DirectoryNode{Name: "x"},
			KeyFiles: []projctx.KeyFile{{Path: "go.mod"}},
		},
		BusinessRules: &projctx.BusinessRuleContext{
			Rules: []projctx.BusinessRule{{ID: "r1", Name: "rule"}},
		},
		Patterns: &projctx.PatternContext{
			Patterns: []projctx.CodePattern{{Name: "Singleton"}},
		},
	}
	rep := Score(ctx)
	if rep.Score != 100 || rep.Grade != "A" {
		t.Fatalf("healthy context must be 100/A, got %d/%s: %+v", rep.Score, rep.Grade, rep.Checks)
	}
	if len(rep.Tips) != 0 {
		t.Fatalf("healthy context must have no tips, got %v", rep.Tips)
	}
}

func TestStalenessBuckets(t *testing.T) {
	now := time.Now().UTC()
	cases := map[time.Duration]string{
		-time.Hour:       "fresh",
		-25 * time.Hour:  "stale",
		-8 * 24 * time.Hour: "expired",
	}
	for d, want := range cases {
		ctx := &projctx.ProjectContext{ExtractedAt: now.Add(d)}
		if got := ctx.Staleness(now); got != want {
			t.Fatalf("offset %v: want %s got %s", d, want, got)
		}
	}
}
