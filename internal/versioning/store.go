package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// ContextSnapshot is one committed context version.
type ContextSnapshot struct {
	Hash       string          `json:"hash"`
	ParentHash string          `json:"parent_hash,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
	Author     string          `json:"author"`
	Message    string          `json:"message"`
	GitCommit  string          `json:"git_commit,omitempty"`
	GitBranch  string          `json:"git_branch,omitempty"`
	Context    json.RawMessage `json:"context"`
}

// ContextStore persists snapshots under ctxDir/snapshots.
type ContextStore struct {
	ctxDir string
}

// NewContextStore creates a store.
func NewContextStore(ctxDir string) *ContextStore {
	return &ContextStore{ctxDir: ctxDir}
}

func (s *ContextStore) snapshotsDir() string {
	return filepath.Join(s.ctxDir, "snapshots")
}

func (s *ContextStore) headPath() string {
	return filepath.Join(s.ctxDir, "HEAD")
}

// CanonicalHash hashes context excluding volatile fields (timestamps), so
// identical code produces identical hashes across extractions.
func CanonicalHash(ctx *projctx.ProjectContext) (string, error) {
	clone := *ctx
	clone.ExtractedAt = time.Time{}
	clone.ContentHash = ""
	data, err := json.Marshal(clone)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16]), nil
}

// autoMessages are messages for which identical content does not create a new snapshot.
var autoMessages = map[string]bool{
	"extract": true, "extract (on-commit)": true, "watch: auto-update": true,
}

// Commit serializes ctx, hashes, saves snapshot, updates HEAD.
// Returns deduped=true when content is unchanged and the message is automatic;
// in that case the existing HEAD snapshot is returned and nothing is written.
func (s *ContextStore) Commit(ctx *projctx.ProjectContext, message string) (snap *ContextSnapshot, deduped bool, err error) {
	if err := os.MkdirAll(s.snapshotsDir(), 0o755); err != nil {
		return nil, false, fmt.Errorf("create snapshots dir: %w", err)
	}
	hash, err := CanonicalHash(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("hash context: %w", err)
	}
	ctx.ContentHash = hash
	parent, _ := s.GetHead()
	if parent != "" && autoMessages[message] {
		if head, lerr := s.LoadSnapshot(parent); lerr == nil {
			if oldCtx, derr := SnapshotToContext(head); derr == nil {
				if oldCtx.ContentHash == "" {
					// Legacy snapshot: compare canonical hashes directly.
					if oldHash, herr := CanonicalHash(oldCtx); herr == nil && oldHash == hash {
						return head, true, nil
					}
				} else if oldCtx.ContentHash == hash {
					return head, true, nil
				}
			}
		}
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("marshal context: %w", err)
	}
	snap = &ContextSnapshot{
		Hash:       hash,
		ParentHash: parent,
		Timestamp:  time.Now().UTC(),
		Author:     gitAuthor(s.ctxDir),
		Message:    message,
		Context:    data,
	}
	if ctx.CurrentState != nil {
		snap.GitCommit = ctx.CurrentState.LastCommit
		snap.GitBranch = ctx.CurrentState.GitBranch
	}
	out, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.snapshotsDir(), hash+".json"), out, 0o644); err != nil {
		return nil, false, fmt.Errorf("write snapshot: %w", err)
	}
	if err := os.WriteFile(s.headPath(), []byte(hash), 0o644); err != nil {
		return nil, false, fmt.Errorf("write HEAD: %w", err)
	}
	return snap, false, nil
}

// GetHead reads the HEAD file.
func (s *ContextStore) GetHead() (string, error) {
	data, err := os.ReadFile(s.headPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// LoadSnapshot loads one snapshot by hash (supports HEAD and HEAD~N).
func (s *ContextStore) LoadSnapshot(hash string) (*ContextSnapshot, error) {
	if hash == "HEAD" {
		h, err := s.GetHead()
		if err != nil {
			return nil, err
		}
		hash = h
	}
	if strings.HasPrefix(hash, "HEAD~") {
		nStr := strings.TrimPrefix(hash, "HEAD~")
		var n int
		if _, err := fmt.Sscanf(nStr, "%d", &n); err != nil {
			return nil, fmt.Errorf("bad revision %s", hash)
		}
		return s.resolveHeadN(n)
	}
	// short-hash prefix match
	if len(hash) < 32 {
		matches, _ := filepath.Glob(filepath.Join(s.snapshotsDir(), hash+"*.json"))
		if len(matches) == 1 {
			hash = strings.TrimSuffix(filepath.Base(matches[0]), ".json")
		}
	}
	data, err := os.ReadFile(filepath.Join(s.snapshotsDir(), hash+".json"))
	if err != nil {
		return nil, fmt.Errorf("load snapshot %s: %w", hash, err)
	}
	var snap ContextSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	return &snap, nil
}

func (s *ContextStore) resolveHeadN(n int) (*ContextSnapshot, error) {
	head, err := s.GetHead()
	if err != nil {
		return nil, err
	}
	cur := head
	for i := 0; i < n; i++ {
		snap, err := s.LoadSnapshot(cur)
		if err != nil {
			return nil, err
		}
		if snap.ParentHash == "" {
			return nil, fmt.Errorf("no parent at HEAD~%d", i+1)
		}
		cur = snap.ParentHash
	}
	return s.LoadSnapshot(cur)
}

// Log lists snapshots newest-first.
func (s *ContextStore) Log(limit int) ([]ContextSnapshot, error) {
	entries, err := os.ReadDir(s.snapshotsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read snapshots: %w", err)
	}
	var out []ContextSnapshot
	for _, en := range entries {
		if !strings.HasSuffix(en.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.snapshotsDir(), en.Name()))
		if err != nil {
			continue
		}
		var snap ContextSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func gitAuthor(dir string) string {
	root := dir
	// snapshots live in <root>/.ctx; git repo is parent
	if strings.HasSuffix(filepath.ToSlash(dir), "/.ctx") || filepath.Base(dir) == ".ctx" {
		root = filepath.Dir(dir)
	}
	nameOut, _ := exec.Command("git", "-C", root, "config", "user.name").Output()
	emailOut, _ := exec.Command("git", "-C", root, "config", "user.email").Output()
	name := strings.TrimSpace(string(nameOut))
	email := strings.TrimSpace(string(emailOut))
	if name == "" && email == "" {
		return "unknown"
	}
	if email != "" {
		return name + " <" + email + ">"
	}
	return name
}

// SnapshotToContext decodes a snapshot's context.
func SnapshotToContext(snap *ContextSnapshot) (*projctx.ProjectContext, error) {
	var ctx projctx.ProjectContext
	if err := json.Unmarshal(snap.Context, &ctx); err != nil {
		return nil, fmt.Errorf("decode snapshot context: %w", err)
	}
	return &ctx, nil
}
