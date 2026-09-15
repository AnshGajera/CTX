package commands

import (
	"strings"
	"testing"

	"github.com/AnshGajera/CTX/internal/ai"
	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
)

func TestFormatStatus(t *testing.T) {
	ctx := &projctx.ProjectContext{
		ProjectName:  "demo",
		ContentHash:  "abc123",
		Profile:      projctx.ProjectProfile{ProjectType: "api_service"},
		Architecture: &projctx.ArchitectureContext{Pattern: "mvc"},
		APIs: &projctx.APIContext{Endpoints: []projctx.APIEndpoint{
			{Method: "GET", Path: "/a"},
		}},
		CurrentState: &projctx.ProjectStateContext{GitBranch: "main"},
	}
	out := formatStatus(ctx)
	for _, want := range []string{"demo", "api_service", "main", "mvc", "abc123", "Endpoints:    1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatHits(t *testing.T) {
	ranked := []ai.RankedChunk{
		{Chunk: ai.Chunk{Kind: "api", ID: "GET /a", Snippet: "snip"}, Score: 0.9},
	}
	out := formatHits("query", ranked)
	if !strings.Contains(out, "GET /a") || !strings.Contains(out, "0.900") {
		t.Fatalf("bad hits output:\n%s", out)
	}
}

func TestFormatDiff(t *testing.T) {
	d := &versioning.ContextDiff{
		AddedEndpoints: []projctx.APIEndpoint{{Method: "GET", Path: "/new"}},
		RemovedEnvVars: []projctx.EnvVariable{{Name: "OLD"}},
		Summary:        "test summary",
	}
	out := formatDiff(d)
	for _, want := range []string{"+ GET /new", "- env OLD", "test summary"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatHistory(t *testing.T) {
	out := formatHistory([]versioning.ContextSnapshot{
		{Hash: "abcdef1234567890", Author: "a", Message: "msg"},
	})
	if !strings.Contains(out, "abcdef123456") || !strings.Contains(out, "msg") {
		t.Fatalf("bad history output:\n%s", out)
	}
}
