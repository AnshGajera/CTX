package extractors

import (
	"os"
	"path/filepath"
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestTypeScriptMultiRoutePerLine(t *testing.T) {
	root := t.TempDir()
	src := `const express = require('express');
const app = express();
app.get('/hello', (req, res) => res.send('hi')); app.post('/echo', (req, res) => res.send('e'));
r.delete('/old', handler);
`
	if err := os.WriteFile(filepath.Join(root, "app.js"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &projctx.ProjectContext{}
	if err := NewTypeScriptAPI(root).Extract(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.APIs == nil || len(ctx.APIs.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %+v", ctx.APIs)
	}
	seen := map[string]bool{}
	for _, e := range ctx.APIs.Endpoints {
		seen[e.Method+" "+e.Path] = true
	}
	for _, want := range []string{"GET /hello", "POST /echo", "DELETE /old"} {
		if !seen[want] {
			t.Fatalf("missing %s in %v", want, seen)
		}
	}
}
