package versioning

import (
	"testing"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

func TestCanonicalHashStableAcrossTimestamps(t *testing.T) {
	a := &projctx.ProjectContext{Version: 1, ProjectName: "x", ExtractedAt: time.Now().UTC()}
	b := &projctx.ProjectContext{Version: 1, ProjectName: "x", ExtractedAt: time.Now().UTC().Add(time.Hour)}
	ha, err := CanonicalHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := CanonicalHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hashes differ for identical content: %s vs %s", ha, hb)
	}
}

func TestCommitDedupesUnchangedAutoExtract(t *testing.T) {
	dir := t.TempDir()
	store := NewContextStore(dir)
	ctx := &projctx.ProjectContext{Version: 1, ProjectName: "x"}
	s1, deduped, err := store.Commit(ctx, "extract")
	if err != nil || deduped {
		t.Fatalf("first commit: %v deduped=%v", err, deduped)
	}
	// Simulate re-extraction later with a new timestamp but same content.
	ctx2 := &projctx.ProjectContext{Version: 1, ProjectName: "x", ExtractedAt: time.Now().UTC().Add(time.Minute)}
	s2, deduped, err := store.Commit(ctx2, "extract")
	if err != nil || !deduped {
		t.Fatalf("expected dedupe, got %v deduped=%v", err, deduped)
	}
	if s1.Hash != s2.Hash {
		t.Fatalf("dedupe must return HEAD hash: %s vs %s", s1.Hash, s2.Hash)
	}
	snaps, err := store.Log(10)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d (%v)", len(snaps), err)
	}
}

func TestCommitCustomMessageAlwaysSnapshots(t *testing.T) {
	dir := t.TempDir()
	store := NewContextStore(dir)
	ctx := &projctx.ProjectContext{Version: 1, ProjectName: "x"}
	if _, _, err := store.Commit(ctx, "extract"); err != nil {
		t.Fatal(err)
	}
	// Identical content never forks history, whatever the message.
	if _, deduped, err := store.Commit(ctx, "release checkpoint"); err != nil || !deduped {
		t.Fatalf("identical content must dedupe: %v deduped=%v", err, deduped)
	}
	// Changed content with a custom message snapshots.
	ctx2 := &projctx.ProjectContext{Version: 1, ProjectName: "y"}
	if _, deduped, err := store.Commit(ctx2, "release checkpoint"); err != nil || deduped {
		t.Fatalf("changed content must snapshot: %v deduped=%v", err, deduped)
	}
}
