package versioning

import (
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func testCtx(endpoints []projctx.APIEndpoint, models []string, env []string, deps []string) *projctx.ProjectContext {
	ctx := &projctx.ProjectContext{
		APIs:         &projctx.APIContext{},
		Database:     &projctx.DatabaseContext{},
		Environment:  &projctx.EnvironmentContext{},
		Dependencies: &projctx.DependencyContext{},
	}
	ctx.APIs.Endpoints = endpoints
	for _, m := range models {
		ctx.Database.Models = append(ctx.Database.Models, projctx.DatabaseModel{Name: m})
	}
	for _, v := range env {
		ctx.Environment.Variables = append(ctx.Environment.Variables, projctx.EnvVariable{Name: v})
	}
	for _, d := range deps {
		ctx.Dependencies.Direct = append(ctx.Dependencies.Direct, projctx.Dependency{Name: d})
	}
	return ctx
}

func TestComputeDiffEndpoints(t *testing.T) {
	old := testCtx([]projctx.APIEndpoint{{Method: "GET", Path: "/a"}, {Method: "POST", Path: "/b"}}, nil, nil, nil)
	new := testCtx([]projctx.APIEndpoint{{Method: "GET", Path: "/a"}, {Method: "DELETE", Path: "/c"}}, nil, nil, nil)
	d := ComputeDiff(old, new)
	if len(d.AddedEndpoints) != 1 || d.AddedEndpoints[0].Path != "/c" {
		t.Fatalf("added = %+v", d.AddedEndpoints)
	}
	if len(d.RemovedEndpoints) != 1 || d.RemovedEndpoints[0].Path != "/b" {
		t.Fatalf("removed = %+v", d.RemovedEndpoints)
	}
	if d.IsEmpty() {
		t.Fatal("diff should not be empty")
	}
}

func TestComputeDiffEmpty(t *testing.T) {
	old := testCtx(nil, []string{"User"}, []string{"DB_URL"}, []string{"react"})
	new := testCtx(nil, []string{"User"}, []string{"DB_URL"}, []string{"react"})
	if d := ComputeDiff(old, new); !d.IsEmpty() {
		t.Fatalf("expected empty diff, got %s", d.Summary)
	}
}

func TestComputeDiffModelsEnvDeps(t *testing.T) {
	old := testCtx(nil, []string{"User"}, []string{"OLD_VAR"}, []string{"lodash"})
	new := testCtx(nil, []string{"User", "Order"}, []string{"NEW_VAR"}, []string{"lodash", "zod"})
	d := ComputeDiff(old, new)
	if len(d.AddedModels) != 1 || len(d.AddedEnvVars) != 1 || len(d.RemovedEnvVars) != 1 {
		t.Fatalf("unexpected diff: %+v", d)
	}
	if len(d.AddedDeps) != 1 || d.AddedDeps[0].Name != "zod" {
		t.Fatalf("deps diff: %+v", d.AddedDeps)
	}
	if d.Summary == "" {
		t.Fatal("summary must not be empty")
	}
}

func TestComputeDiffNil(t *testing.T) {
	if d := ComputeDiff(nil, testCtx(nil, nil, nil, nil)); d.Summary == "" {
		t.Fatal("nil-safe summary expected")
	}
}
