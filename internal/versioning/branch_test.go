package versioning

import (
	"testing"
	"os"
	"path/filepath"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestBranchAndCheckout(t *testing.T) {
	dir := t.TempDir()
	ctxDir := filepath.Join(dir, ".ctx")
	if err := os.MkdirAll(ctxDir, 0755); err != nil {
		t.Fatalf("failed to create .ctx dir: %v", err)
	}
	if err := os.MkdirAll(ctxDir, 0755); err != nil {
		t.Fatalf("failed to create .ctx dir: %v", err)
	}
	store := NewContextStore(dir)
	ctx1 := &projctx.ProjectContext{
		Version:     1,
		ProjectName: "app",
		APIs: &projctx.APIContext{
			Endpoints: []projctx.APIEndpoint{
				{Method: "GET", Path: "/api/health"},
			},
		},
	}

	snap1, deduped, err := store.Commit(ctx1, "initial extract")
	if err != nil || deduped {
		t.Fatalf("commit 1 failed: %v", err)
	}

	// Verify default branch is main
	cur, detached, err := store.CurrentBranch()
	if err != nil || detached || cur != "main" {
		t.Fatalf("expected current branch 'main', got cur=%q detached=%v err=%v", cur, detached, err)
	}

	// Create and checkout new branch 'feature-auth'
	_, err = store.CheckoutNewBranch("feature-auth", "HEAD")
	if err != nil {
		t.Fatalf("failed to checkout new branch: %v", err)
	}

	cur, detached, _ = store.CurrentBranch()
	if cur != "feature-auth" || detached {
		t.Fatalf("expected current branch 'feature-auth', got %q", cur)
	}

	// On feature-auth, add an endpoint and commit checkpoint
	ctx2 := &projctx.ProjectContext{
		Version:     1,
		ProjectName: "app",
		APIs: &projctx.APIContext{
			Endpoints: []projctx.APIEndpoint{
				{Method: "GET", Path: "/api/health"},
				{Method: "POST", Path: "/api/login"},
			},
		},
	}
	snap2, err := store.CommitCheckpoint(ctx2, "added login endpoint")
	if err != nil {
		t.Fatalf("commit checkpoint failed: %v", err)
	}
	if snap2.ParentHash != snap1.Hash {
		t.Fatalf("expected snap2 parent to be snap1: got %s vs %s", snap2.ParentHash, snap1.Hash)
	}

	// Switch back to main
	snapMain, err := store.Checkout("main")
	if err != nil {
		t.Fatalf("checkout main failed: %v", err)
	}
	if snapMain.Hash != snap1.Hash {
		t.Fatalf("expected main to be at snap1 %s, got %s", snap1.Hash, snapMain.Hash)
	}

	// Verify working context restored to snap1
	loaded, err := projctx.LoadContext(dir)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if len(loaded.APIs.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint on main, got %d", len(loaded.APIs.Endpoints))
	}

	// Now merge 'feature-auth' into 'main'
	mergeSnap, report, err := store.Merge("feature-auth", "merge auth feature")
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if report.AddedEndpoints != 1 {
		t.Fatalf("expected 1 added endpoint in report, got %d", report.AddedEndpoints)
	}

	mergedCtx, err := SnapshotToContext(mergeSnap)
	if err != nil {
		t.Fatal(err)
	}
	if len(mergedCtx.APIs.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints in merged context, got %d", len(mergedCtx.APIs.Endpoints))
	}

	// Test Tagging
	if err := store.CreateTag("v1.0.0", "HEAD"); err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	tags, err := store.ListTags()
	if err != nil || len(tags) != 1 || tags[0].Name != "v1.0.0" {
		t.Fatalf("unexpected tags: %+v", tags)
	}
}

func TestDeleteBranch(t *testing.T) {
	dir := t.TempDir()
	store := NewContextStore(dir)

	ctx := &projctx.ProjectContext{Version: 1, ProjectName: "demo"}
	_, _, err := store.Commit(ctx, "init")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CreateBranch("temp", "HEAD"); err != nil {
		t.Fatal(err)
	}

	// Cannot delete checked out branch
	if err := store.DeleteBranch("main"); err == nil {
		t.Fatal("expected error deleting current branch")
	}

	// Can delete other branch
	if err := store.DeleteBranch("temp"); err != nil {
		t.Fatalf("delete branch: %v", err)
	}

	branches, _ := store.ListBranches()
	for _, b := range branches {
		if b.Name == "temp" {
			t.Fatal("temp branch was not deleted")
		}
	}
}

func TestResolveRef(t *testing.T) {
	dir := t.TempDir()
	store := NewContextStore(dir)

	ctx := &projctx.ProjectContext{Version: 1, ProjectName: "test"}
	snap, _, err := store.Commit(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}

	// By branch
	h, err := store.ResolveRef("main")
	if err != nil || h != snap.Hash {
		t.Fatalf("resolve main: got %s, want %s (err: %v)", h, snap.Hash, err)
	}

	// By HEAD
	h, err = store.ResolveRef("HEAD")
	if err != nil || h != snap.Hash {
		t.Fatalf("resolve HEAD: got %s, want %s", h, snap.Hash)
	}

	// By tag
	_ = store.CreateTag("v0.1", snap.Hash)
	h, err = store.ResolveRef("v0.1")
	if err != nil || h != snap.Hash {
		t.Fatalf("resolve tag v0.1: got %s, want %s", h, snap.Hash)
	}
}
