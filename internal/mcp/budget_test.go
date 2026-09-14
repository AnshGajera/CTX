package mcp

import (
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func budgetFixture() *projctx.ProjectContext {
	return &projctx.ProjectContext{
		ProjectName:   "demo",
		Architecture:  &projctx.ArchitectureContext{Pattern: "layered"},
		APIs:          &projctx.APIContext{Endpoints: []projctx.APIEndpoint{{Method: "GET", Path: "/a"}}},
		Database:      &projctx.DatabaseContext{Models: []projctx.DatabaseModel{{Name: "User"}}},
		Environment:   &projctx.EnvironmentContext{Variables: []projctx.EnvVariable{{Name: "DB_URL"}}},
		Dependencies:  &projctx.DependencyContext{Direct: []projctx.Dependency{{Name: "react"}}},
		Patterns:      &projctx.PatternContext{Patterns: []projctx.CodePattern{{Name: "MVC"}}},
		CurrentState:  &projctx.ProjectStateContext{GitBranch: "main"},
		FileStructure: &projctx.FileStructureContext{KeyFiles: []projctx.KeyFile{{Path: "go.mod"}}},
	}
}

func TestEstimateTokensPositive(t *testing.T) {
	if n := EstimateTokens(map[string]string{"a": "hello world"}); n <= 0 {
		t.Fatalf("expected positive estimate, got %d", n)
	}
}

func TestBudgetedContextDropsUnderTinyBudget(t *testing.T) {
	out := BudgetedContext(budgetFixture(), nil, 40)
	if out["_truncated"] != true {
		t.Fatal("expected truncation flag")
	}
	dropped, ok := out["_dropped"].([]string)
	if !ok || len(dropped) == 0 {
		t.Fatalf("expected dropped sections, got %v", out["_dropped"])
	}
	// Highest-priority content must survive.
	if _, ok := out["profile"]; !ok {
		t.Fatal("profile should survive budgeting")
	}
}

func TestBudgetedContextUnlimitedKeepsAll(t *testing.T) {
	out := BudgetedContext(budgetFixture(), nil, 0)
	if _, bad := out["_truncated"]; bad {
		t.Fatal("unlimited budget must not truncate")
	}
	for _, k := range []string{"apis", "database", "environment", "dependencies"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("missing section %s", k)
		}
	}
}

func TestSearchChunksFindsRelevant(t *testing.T) {
	res, ok := SearchChunks(budgetFixture(), "api endpoint", 5).(map[string]any)
	if !ok {
		t.Fatal("unexpected result shape")
	}
	results, ok := res["results"].([]map[string]any)
	if !ok || len(results) == 0 {
		t.Fatalf("expected results, got %v", res)
	}
	if results[0]["kind"] != "api" {
		t.Fatalf("expected api first, got %v", results[0]["kind"])
	}
}
