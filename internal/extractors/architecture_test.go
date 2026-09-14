package extractors

import (
	"os"
	"path/filepath"
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestArchitectureGoLayout(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"cmd", "pkg", "internal"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx := &projctx.ProjectContext{}
	if err := NewArchitecture(root).Extract(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Architecture == nil {
		t.Fatal("architecture must be set")
	}
	if ctx.Architecture.Pattern != "go-standard-layout" {
		t.Fatalf("pattern = %s", ctx.Architecture.Pattern)
	}
	if len(ctx.Architecture.Layers) != 3 {
		t.Fatalf("layers = %+v", ctx.Architecture.Layers)
	}
	if ctx.Architecture.Diagram == "" {
		t.Fatal("diagram must be generated")
	}
}

func TestArchitectureComposeServices(t *testing.T) {
	root := t.TempDir()
	compose := `services:
  api:
    ports:
      - "8080:80"
    depends_on:
      - db
  db:
    ports:
      - "5432:5432"
`
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &projctx.ProjectContext{}
	if err := NewArchitecture(root).Extract(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.Architecture.Services) != 2 {
		t.Fatalf("services = %+v", ctx.Architecture.Services)
	}
	if len(ctx.Architecture.Communication) != 1 {
		t.Fatalf("edges = %+v", ctx.Architecture.Communication)
	}
	edge := ctx.Architecture.Communication[0]
	if edge.From != "api" || edge.To != "db" {
		t.Fatalf("edge = %+v", edge)
	}
}
