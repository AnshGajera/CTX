// Package cache provides a tree-state fast path: a cheap hash over the
// file tree (paths + sizes + mtimes) that lets extract skip the full
// scan when nothing changed, refreshing only git state.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/AnshGajera/CTX/internal/extractors"
)

// TreeState is the persisted tree hash.
type TreeState struct {
	Hash    string    `json:"hash"`
	SavedAt time.Time `json:"saved_at"`
}

// ComputeTreeHash hashes sorted "rel:size:mtime" entries using the same
// skip/ignore rules as the extractors.
func ComputeTreeHash(root string) (string, error) {
	base := extractors.NewBase(root)
	var entries []string
	err := base.WalkFiles(func(path, rel string, info os.FileInfo) error {
		entries = append(entries, fmt.Sprintf("%s:%d:%d", rel, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func statePath(ctxDir string) string {
	return filepath.Join(ctxDir, "cache", "treehash.json")
}

// Load reads the cached tree state (empty Hash when absent).
func Load(ctxDir string) TreeState {
	var st TreeState
	data, err := os.ReadFile(statePath(ctxDir))
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, &st)
	return st
}

// Save persists the tree hash.
func Save(ctxDir, hash string) error {
	if err := os.MkdirAll(filepath.Join(ctxDir, "cache"), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(TreeState{Hash: hash, SavedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(ctxDir), data, 0o644)
}
