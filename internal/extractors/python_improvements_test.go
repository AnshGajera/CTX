package extractors

import (
	"os"
	"path/filepath"
	"testing"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestNestedDependenciesAndModernSQLAlchemy(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "backend", "requirements.txt"), "fastapi==0.115.0\nsqlalchemy>=2.0\n")
	mustWrite(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":{"next":"14.2.0"},"devDependencies":{"typescript":"5.0.0"}}`)
	mustWrite(t, filepath.Join(root, "backend", "models.py"), "class User(Base):\n    id: Mapped[int] = mapped_column(primary_key=True)\n    email: Mapped[str] = mapped_column(unique=True)\n")

	ctx := &projctx.ProjectContext{}
	if err := NewDependencies(root).Extract(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.Dependencies.Direct) != 3 || len(ctx.Dependencies.Dev) != 1 {
		t.Fatalf("unexpected dependencies: direct=%d dev=%d", len(ctx.Dependencies.Direct), len(ctx.Dependencies.Dev))
	}
	if err := NewGenericModels(root).Extract(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.Database.Models) != 1 || len(ctx.Database.Models[0].Fields) != 2 {
		t.Fatalf("modern SQLAlchemy model was not extracted: %+v", ctx.Database)
	}
}

func TestPythonAPIExtractsDocstringAndAuthDependency(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "api.py"), `from fastapi import Depends
@router.get("/documents/{document_id}")
async def get_document(document_id: int, user=Depends(get_current_user)):
    """Return a document for the authenticated user."""
    return {}
`)
	ctx := &projctx.ProjectContext{}
	if err := NewPythonAPI(root).Extract(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.APIs.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %d", len(ctx.APIs.Endpoints))
	}
	endpoint := ctx.APIs.Endpoints[0]
	if !endpoint.AuthRequired || endpoint.Description == "" || endpoint.Handler != "get_document" {
		t.Fatalf("missing endpoint metadata: %+v", endpoint)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
