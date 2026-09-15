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

// ComputeTreeHash hashes sorted "rel:size:content" entries using the same
// skip/ignore rules as the extractors. Content (not mtime) is hashed so
// same-size edits with preserved timestamps cannot fool the fast path.
// Files larger than maxContentHashBytes hash size + mtime + first/last
// sample instead of full content. Extractor inputs outside the walk
// (.ctxignore, .ctx/config.toml) are folded in so config changes invalidate.
func ComputeTreeHash(root string) (string, error) {
	base := extractors.NewBase(root)
	var entries []string
	err := base.WalkFiles(func(path, rel string, info os.FileInfo) error {
		sum, herr := hashFileContent(path, info)
		if herr != nil {
			// Unreadable file: fall back to size+mtime so one bad file
			// never aborts the fast-path decision.
			sum = fmt.Sprintf("unreadable:%d:%d", info.Size(), info.ModTime().UnixNano())
		}
		entries = append(entries, rel+"\x00"+sum)
		return nil
	})
	if err != nil {
		return "", err
	}
	// Fold extractor inputs that live outside the walked set.
	for _, extra := range []string{".ctxignore", filepath.Join(".ctx", "config.toml")} {
		if data, rerr := os.ReadFile(filepath.Join(root, extra)); rerr == nil {
			sum := sha256.Sum256(data)
			entries = append(entries, extra+"\x00"+hex.EncodeToString(sum[:]))
		}
	}
	sort.Strings(entries)
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// maxContentHashBytes bounds full-content hashing per file.
const maxContentHashBytes = 2 << 20

// sampleBytes bounds head/tail sampling for oversized files.
const sampleBytes = 64 << 10

func hashFileContent(path string, info os.FileInfo) (string, error) {
	if info.Size() > maxContentHashBytes {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		h := sha256.New()
		head := make([]byte, sampleBytes)
		n, _ := f.Read(head)
		h.Write(head[:n])
		if info.Size() > sampleBytes {
			off := info.Size() - sampleBytes
			if off < int64(n) {
				off = int64(n)
			}
			tail := make([]byte, sampleBytes)
			if _, serr := f.ReadAt(tail, off); serr == nil {
				h.Write(tail)
			}
		}
		fmt.Fprintf(h, ":%d:%d", info.Size(), info.ModTime().UnixNano())
		return "sample:" + hex.EncodeToString(h.Sum(nil)), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%d:%s", info.Size(), hex.EncodeToString(sum[:])), nil
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
