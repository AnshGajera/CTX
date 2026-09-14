package openapi

import (
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestBuildParamConversionAndTags(t *testing.T) {
	doc := Build("demo", []projctx.APIEndpoint{
		{Method: "GET", Path: "/api/users/:id", Handler: "getUser",
			Parameters: []projctx.APIParam{{Name: "id", In: "path", Type: "string", Required: true}}},
		{Method: "ALL", Path: "/health", Handler: "health"},
	})
	item, ok := doc.Paths["/api/users/{id}"]
	if !ok {
		t.Fatalf("paths: %v", doc.Paths)
	}
	op := item["get"]
	if len(op.Parameters) != 1 || op.Parameters[0].Name != "id" {
		t.Fatalf("params: %+v", op.Parameters)
	}
	if len(op.Tags) != 1 || op.Tags[0] != "users" {
		t.Fatalf("tags: %+v", op.Tags)
	}
	health, ok := doc.Paths["/health"]
	if !ok {
		t.Fatal("missing /health")
	}
	for _, m := range []string{"get", "post", "put", "patch", "delete"} {
		if _, ok := health[m]; !ok {
			t.Fatalf("ALL should expand to %s", m)
		}
	}
	if doc.OpenAPI != "3.0.0" || doc.Info.Title != "demo" {
		t.Fatalf("doc meta: %+v", doc)
	}
}
