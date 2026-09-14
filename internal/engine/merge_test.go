package engine

import (
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestMergeDedupesEndpoints(t *testing.T) {
	dst := &projctx.ProjectContext{}
	mergePartials(dst, []*projctx.ProjectContext{
		{APIs: &projctx.APIContext{Endpoints: []projctx.APIEndpoint{
			{Method: "GET", Path: "/a", Handler: "regex"},
			{Method: "POST", Path: "/b", Handler: "regex"},
		}}},
		{APIs: &projctx.APIContext{Endpoints: []projctx.APIEndpoint{
			{Method: "GET", Path: "/a", Handler: "ast"}, // duplicate
			{Method: "DELETE", Path: "/c", Handler: "ast"},
		}}},
		nil, // nil partials must be tolerated
	})
	if len(dst.APIs.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(dst.APIs.Endpoints))
	}
	// First writer wins.
	for _, ep := range dst.APIs.Endpoints {
		if ep.Path == "/a" && ep.Handler != "regex" {
			t.Fatalf("first writer should win: %+v", ep)
		}
	}
}

func TestMergeModelsAndState(t *testing.T) {
	dst := &projctx.ProjectContext{}
	mergePartials(dst, []*projctx.ProjectContext{
		{Database: &projctx.DatabaseContext{
			Models: []projctx.DatabaseModel{{Name: "User"}},
			ORM:    "prisma",
		}},
		{Database: &projctx.DatabaseContext{
			Models: []projctx.DatabaseModel{{Name: "User"}, {Name: "Order"}},
		}},
		{CurrentState: &projctx.ProjectStateContext{
			GitBranch: "main", TODOs: []projctx.TODO{{Text: "x", File: "a.go", Line: 1}},
		}},
		{CurrentState: &projctx.ProjectStateContext{
			TODOs: []projctx.TODO{{Text: "x", File: "a.go", Line: 1}, {Text: "y", File: "b.go", Line: 2}},
		}},
	})
	if len(dst.Database.Models) != 2 || dst.Database.ORM != "prisma" {
		t.Fatalf("models merge: %+v", dst.Database)
	}
	if dst.CurrentState.GitBranch != "main" || len(dst.CurrentState.TODOs) != 2 {
		t.Fatalf("state merge: %+v", dst.CurrentState)
	}
}

func TestSetSectionsNormalization(t *testing.T) {
	e := &ExtractionEngine{}
	e.SetSections([]string{"API-Endpoints", "database_schema"})
	if !e.sectionOn("api_endpoints") || !e.sectionOn("DATABASE-SCHEMA") {
		t.Fatal("section normalization failed")
	}
	if e.sectionOn("architecture") {
		t.Fatal("unlisted section must be off")
	}
	e.SetSections(nil)
	if !e.sectionOn("architecture") {
		t.Fatal("nil sections must enable all")
	}
}
