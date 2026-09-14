package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeHashStableAndSensitive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, err := ComputeTreeHash(root)
	if err != nil || h1 == "" {
		t.Fatalf("hash: %s %v", h1, err)
	}
	h2, err := ComputeTreeHash(root)
	if err != nil || h1 != h2 {
		t.Fatal("hash must be stable")
	}
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("package y"), 0o644); err != nil {
		t.Fatal(err)
	}
	h3, err := ComputeTreeHash(root)
	if err != nil || h3 == h1 {
		t.Fatal("hash must change on new file")
	}
	ctxDir := t.TempDir()
	if got := Load(ctxDir); got.Hash != "" {
		t.Fatal("empty load expected")
	}
	if err := Save(ctxDir, h1); err != nil {
		t.Fatal(err)
	}
	if got := Load(ctxDir); got.Hash != h1 {
		t.Fatalf("round trip: %s", got.Hash)
	}
}
