package ai

import (
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestRankTFIDFPrefersRelevantChunk(t *testing.T) {
	chunks := []Chunk{
		{ID: "a", Kind: "api", Text: "GET /api/users handler=listUsers file=users.ts"},
		{ID: "b", Kind: "model", Text: "model Invoice fields: total, currency"},
		{ID: "c", Kind: "env", Text: "env STRIPE_KEY category=payment"},
	}
	ranked := RankTFIDF(chunks, "list users endpoint", 3)
	if len(ranked) != 3 {
		t.Fatalf("expected 3, got %d", len(ranked))
	}
	if ranked[0].Chunk.ID != "a" {
		t.Fatalf("expected chunk a first, got %s (%.3f)", ranked[0].Chunk.ID, ranked[0].Score)
	}
}

func TestRankTFIDFEmpty(t *testing.T) {
	if out := RankTFIDF(nil, "query", 5); len(out) != 0 {
		t.Fatal("expected no results")
	}
	if out := RankTFIDF([]Chunk{{ID: "a", Text: "x"}}, "", 5); len(out) != 0 {
		t.Fatal("expected no results for empty query")
	}
}

func TestChunkerCoversSections(t *testing.T) {
	ctx := &projctx.ProjectContext{
		APIs: &projctx.APIContext{Endpoints: []projctx.APIEndpoint{
			{Method: "GET", Path: "/health", Handler: "h", File: "main.go"},
		}},
		Database: &projctx.DatabaseContext{Models: []projctx.DatabaseModel{
			{Name: "User", Fields: []projctx.ModelField{{Name: "email", Type: "String"}}},
		}},
		Environment: &projctx.EnvironmentContext{Variables: []projctx.EnvVariable{
			{Name: "DB_URL", Category: "database"},
		}},
	}
	chunks := NewChunker().Chunk(ctx)
	kinds := map[string]bool{}
	for _, c := range chunks {
		kinds[c.Kind] = true
		if c.Snippet == "" || c.Text == "" {
			t.Fatal("chunk must have text and snippet")
		}
	}
	for _, k := range []string{"api", "model", "env"} {
		if !kinds[k] {
			t.Fatalf("missing chunk kind %s", k)
		}
	}
}
