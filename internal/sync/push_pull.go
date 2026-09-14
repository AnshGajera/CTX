package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
