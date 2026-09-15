package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	projctx "github.com/AnshGajera/CTX/internal/context"
	"github.com/AnshGajera/CTX/internal/versioning"
)

// PushPull helpers bridge local store and remote.

// PushSnapshot pushes the HEAD snapshot.
func PushSnapshot(client *SyncClient, store *versioning.ContextStore, projectID, message string) error {
	head, err := store.GetHead()
	if err != nil {
		return fmt.Errorf("no local snapshots: %w", err)
	}
	snap, err := store.LoadSnapshot(head)
	if err != nil {
		return fmt.Errorf("load HEAD: %w", err)
	}
	if message != "" {
		snap.Message = message
	}
	return client.Push(projectID, snap)
}

// PullSnapshot pulls remote and stores locally.
func PullSnapshot(client *SyncClient, store *versioning.ContextStore, ctxDir, projectID string) (*versioning.ContextSnapshot, error) {
	snap, err := client.Pull(projectID)
	if err != nil {
		return nil, err
	}
	// Validate server-provided snapshot before writing anything.
	if ok, _ := regexp.MatchString(`^[0-9a-f]{32}$`, snap.Hash); !ok {
		return nil, fmt.Errorf("invalid snapshot hash %q", snap.Hash)
	}
	var decoded projctx.ProjectContext
	if err := json.Unmarshal(snap.Context, &decoded); err != nil {
		return nil, fmt.Errorf("invalid snapshot context: %w", err)
	}
	expected, err := versioning.CanonicalHash(&decoded)
	if err != nil {
		return nil, fmt.Errorf("hash snapshot context: %w", err)
	}
	if expected != snap.Hash {
		return nil, fmt.Errorf("snapshot hash mismatch: got %q want %q", snap.Hash, expected)
	}
	// persist
	if err := os.MkdirAll(filepath.Join(ctxDir, "snapshots"), 0o755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "snapshots", snap.Hash+".json"), data, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "HEAD"), []byte(snap.Hash), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "context.json"), snap.Context, 0o644); err != nil {
		return nil, err
	}
	return snap, nil
}
