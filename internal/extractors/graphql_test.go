package extractors

import (
	"os"
	"path/filepath"
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestGraphQLExtractor(t *testing.T) {
	root := t.TempDir()
	schema := `
type Query {
  user(id: ID!): User
  users: [User!]!
}

type Mutation {
  createUser(name: String!): User
}

type Subscription {
  userCreated: User
}
`
	if err := os.WriteFile(filepath.Join(root, "schema.graphql"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := &projctx.ProjectContext{}
	ex := NewGraphQL(root)
	if err := ex.Extract(ctx); err != nil {
		t.Fatal(err)
	}

	if ctx.APIs == nil || len(ctx.APIs.Endpoints) != 4 {
		t.Fatalf("expected 4 endpoints, got %+v", ctx.APIs)
	}

	seen := map[string]bool{}
	for _, ep := range ctx.APIs.Endpoints {
		seen[ep.Method+" "+ep.Path] = true
	}

	for _, want := range []string{"QUERY user", "QUERY users", "MUTATION createUser", "SUBSCRIPTION userCreated"} {
		if !seen[want] {
			t.Fatalf("missing %s in %v", want, seen)
		}
	}
}
