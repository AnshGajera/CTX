package versioning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// BranchInfo holds metadata about a context branch.
type BranchInfo struct {
	Name      string    `json:"name"`
	Hash      string    `json:"hash"`
	IsCurrent bool      `json:"is_current"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// TagInfo holds metadata about a context tag/milestone.
type TagInfo struct {
	Name      string    `json:"name"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Message   string    `json:"message,omitempty"`
}

func (s *ContextStore) refsHeadsDir() string {
	return filepath.Join(s.ctxDir, "refs", "heads")
}

func (s *ContextStore) refsTagsDir() string {
	return filepath.Join(s.ctxDir, "refs", "tags")
}

// GetHeadRaw returns the exact string contents of the HEAD file.
func (s *ContextStore) GetHeadRaw() (string, error) {
	data, err := os.ReadFile(s.headPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// CurrentBranch returns the name of the current branch, or the detached commit hash.
func (s *ContextStore) CurrentBranch() (string, bool, error) {
	raw, err := s.GetHeadRaw()
	if err != nil {
		if os.IsNotExist(err) {
			return "main", false, nil
		}
		return "", false, err
	}
	if strings.HasPrefix(raw, "ref: refs/heads/") {
		branch := strings.TrimSpace(strings.TrimPrefix(raw, "ref: refs/heads/"))
		return branch, false, nil
	}
	return raw, true, nil
}

// updateHead writes the commit hash to the current branch ref or detached HEAD.
func (s *ContextStore) updateHead(hash string) error {
	raw, err := s.GetHeadRaw()
	if err != nil && os.IsNotExist(err) {
		// Initialize default HEAD as ref: refs/heads/main
		if err := os.MkdirAll(s.refsHeadsDir(), 0o755); err != nil {
			return fmt.Errorf("create refs/heads: %w", err)
		}
		branchFile := filepath.Join(s.refsHeadsDir(), "main")
		if err := os.WriteFile(branchFile, []byte(hash+"\n"), 0o644); err != nil {
			return fmt.Errorf("write main branch ref: %w", err)
		}
		if err := os.WriteFile(s.headPath(), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			return fmt.Errorf("write HEAD: %w", err)
		}
		return nil
	}

	if strings.HasPrefix(raw, "ref: ") {
		relRef := strings.TrimSpace(strings.TrimPrefix(raw, "ref: "))
		targetPath := filepath.Join(s.ctxDir, filepath.FromSlash(relRef))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create ref dir: %w", err)
		}
		return os.WriteFile(targetPath, []byte(hash+"\n"), 0o644)
	}

	// Detached HEAD or legacy direct hash:
	return os.WriteFile(s.headPath(), []byte(hash+"\n"), 0o644)
}

// ResolveRef resolves a ref (HEAD, HEAD~N, branch name, tag name, or short hash) into a full snapshot hash.
func (s *ContextStore) ResolveRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || ref == "HEAD" {
		return s.GetHead()
	}
	if strings.HasPrefix(ref, "HEAD~") {
		snap, err := s.LoadSnapshot(ref)
		if err != nil {
			return "", err
		}
		return snap.Hash, nil
	}

	// 1. Check branch
	branchPath := filepath.Join(s.refsHeadsDir(), ref)
	if data, err := os.ReadFile(branchPath); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// 2. Check tag
	tagPath := filepath.Join(s.refsTagsDir(), ref)
	if data, err := os.ReadFile(tagPath); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// 3. Short-hash or direct snapshot match
	snap, err := s.LoadSnapshot(ref)
	if err == nil {
		return snap.Hash, nil
	}

	return "", fmt.Errorf("ref %q not found", ref)
}

// ListBranches returns all local context branches with current branch flagged.
func (s *ContextStore) ListBranches() ([]BranchInfo, error) {
	curBranch, isDetached, _ := s.CurrentBranch()
	entries, err := os.ReadDir(s.refsHeadsDir())
	if err != nil {
		if os.IsNotExist(err) {
			// If no branches folder but HEAD exists, offer synthetic main
			head, err := s.GetHead()
			if err == nil && head != "" {
				return []BranchInfo{{
					Name:      "main",
					Hash:      head,
					IsCurrent: !isDetached,
				}}, nil
			}
			return nil, nil
		}
		return nil, fmt.Errorf("read refs/heads: %w", err)
	}

	var branches []BranchInfo
	for _, en := range entries {
		if en.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.refsHeadsDir(), en.Name()))
		if err != nil {
			continue
		}
		hash := strings.TrimSpace(string(data))
		b := BranchInfo{
			Name:      en.Name(),
			Hash:      hash,
			IsCurrent: (!isDetached && en.Name() == curBranch),
		}
		if snap, err := s.LoadSnapshot(hash); err == nil {
			b.Message = snap.Message
			b.Timestamp = snap.Timestamp
		}
		branches = append(branches, b)
	}

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

// CreateBranch creates a new branch pointing to startRef (default HEAD).
func (s *ContextStore) CreateBranch(name string, startRef string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if name == "HEAD" || strings.ContainsAny(name, " \t\r\n~^:?*[]\\") {
		return fmt.Errorf("invalid branch name %q", name)
	}

	branchPath := filepath.Join(s.refsHeadsDir(), name)
	if _, err := os.Stat(branchPath); err == nil {
		return fmt.Errorf("branch %q already exists", name)
	}

	hash, err := s.ResolveRef(startRef)
	if err != nil {
		return fmt.Errorf("cannot resolve start point %q: %w", startRef, err)
	}
	if hash == "" {
		return fmt.Errorf("cannot create branch from empty context (run ctx extract first)")
	}

	if err := os.MkdirAll(s.refsHeadsDir(), 0o755); err != nil {
		return fmt.Errorf("create refs/heads dir: %w", err)
	}

	return os.WriteFile(branchPath, []byte(hash+"\n"), 0o644)
}

// DeleteBranch deletes a branch reference. Cannot delete the active branch.
func (s *ContextStore) DeleteBranch(name string) error {
	name = strings.TrimSpace(name)
	curBranch, isDetached, _ := s.CurrentBranch()
	if !isDetached && curBranch == name {
		return fmt.Errorf("cannot delete checked-out branch %q", name)
	}

	branchPath := filepath.Join(s.refsHeadsDir(), name)
	if _, err := os.Stat(branchPath); os.IsNotExist(err) {
		return fmt.Errorf("branch %q not found", name)
	}

	return os.Remove(branchPath)
}

// Checkout switches to a branch or snapshot, and restores .ctx/context.json.
func (s *ContextStore) Checkout(target string) (*ContextSnapshot, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("checkout target cannot be empty")
	}

	branchPath := filepath.Join(s.refsHeadsDir(), target)
	var snapHash string
	var headContent string

	if _, err := os.Stat(branchPath); err == nil {
		// Target is a branch
		data, err := os.ReadFile(branchPath)
		if err != nil {
			return nil, fmt.Errorf("read branch %s: %w", target, err)
		}
		snapHash = strings.TrimSpace(string(data))
		headContent = fmt.Sprintf("ref: refs/heads/%s\n", target)
	} else if tagPath := filepath.Join(s.refsTagsDir(), target); isFile(tagPath) {
		// Target is a tag
		data, err := os.ReadFile(tagPath)
		if err != nil {
			return nil, fmt.Errorf("read tag %s: %w", target, err)
		}
		snapHash = strings.TrimSpace(string(data))
		headContent = snapHash + "\n"
	} else {
		// Target is a snapshot hash or HEAD~N
		snap, err := s.LoadSnapshot(target)
		if err != nil {
			return nil, fmt.Errorf("cannot checkout %q: neither branch, tag, nor snapshot exists", target)
		}
		snapHash = snap.Hash
		headContent = snapHash + "\n"
	}

	snap, err := s.LoadSnapshot(snapHash)
	if err != nil {
		return nil, fmt.Errorf("load snapshot %s: %w", snapHash, err)
	}

	// Update HEAD
	if err := os.WriteFile(s.headPath(), []byte(headContent), 0o644); err != nil {
		return nil, fmt.Errorf("update HEAD: %w", err)
	}

	// Restore .ctx/context.json
	ctxObj, err := SnapshotToContext(snap)
	if err == nil {
		_ = ctxObj.Save(s.ctxDir)
	}

	return snap, nil
}

// CheckoutNewBranch creates a branch and checks it out immediately.
func (s *ContextStore) CheckoutNewBranch(name string, startRef string) (*ContextSnapshot, error) {
	if err := s.CreateBranch(name, startRef); err != nil {
		return nil, err
	}
	return s.Checkout(name)
}

// ListTags lists all context tags.
func (s *ContextStore) ListTags() ([]TagInfo, error) {
	entries, err := os.ReadDir(s.refsTagsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read refs/tags: %w", err)
	}

	var tags []TagInfo
	for _, en := range entries {
		if en.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.refsTagsDir(), en.Name()))
		if err != nil {
			continue
		}
		hash := strings.TrimSpace(string(data))
		t := TagInfo{
			Name: en.Name(),
			Hash: hash,
		}
		if snap, err := s.LoadSnapshot(hash); err == nil {
			t.Message = snap.Message
			t.Timestamp = snap.Timestamp
		}
		tags = append(tags, t)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags, nil
}

// CreateTag creates a named tag reference pointing to targetRef (default HEAD).
func (s *ContextStore) CreateTag(name string, targetRef string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\r\n~^:?*[]\\") {
		return fmt.Errorf("invalid tag name %q", name)
	}

	tagPath := filepath.Join(s.refsTagsDir(), name)
	if _, err := os.Stat(tagPath); err == nil {
		return fmt.Errorf("tag %q already exists", name)
	}

	hash, err := s.ResolveRef(targetRef)
	if err != nil {
		return fmt.Errorf("cannot resolve target ref %q: %w", targetRef, err)
	}
	if hash == "" {
		return fmt.Errorf("cannot create tag from empty context")
	}

	if err := os.MkdirAll(s.refsTagsDir(), 0o755); err != nil {
		return fmt.Errorf("create refs/tags dir: %w", err)
	}

	return os.WriteFile(tagPath, []byte(hash+"\n"), 0o644)
}

// DeleteTag removes a tag.
func (s *ContextStore) DeleteTag(name string) error {
	name = strings.TrimSpace(name)
	tagPath := filepath.Join(s.refsTagsDir(), name)
	if _, err := os.Stat(tagPath); os.IsNotExist(err) {
		return fmt.Errorf("tag %q not found", name)
	}
	return os.Remove(tagPath)
}

// CommitCheckpoint creates an explicit milestone snapshot, saving agent reasoning or user notes.
func (s *ContextStore) CommitCheckpoint(ctx *projctx.ProjectContext, message string) (*ContextSnapshot, error) {
	if err := os.MkdirAll(s.snapshotsDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create snapshots dir: %w", err)
	}

	if ctx.CurrentState == nil {
		ctx.CurrentState = &projctx.ProjectStateContext{}
	}
	curBranch, _, _ := s.CurrentBranch()
	ctx.CurrentState.GitBranch = curBranch
	ctx.CurrentState.LastCommitMsg = message

	data, err := json.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("marshal context: %w", err)
	}

	// Hash canonical context plus the checkpoint message & timestamp to ensure unique checkpoint hash
	h := CanonicalOrCheckpointHash(ctx, message)
	ctx.ContentHash = h
	parent, _ := s.GetHead()

	snap := &ContextSnapshot{
		Hash:       h,
		ParentHash: parent,
		Timestamp:  time.Now().UTC(),
		Author:     gitAuthor(s.ctxDir),
		Message:    message,
		Context:    data,
		GitBranch:  curBranch,
	}

	out, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	snapFile := filepath.Join(s.snapshotsDir(), h+".json")
	if err := os.WriteFile(snapFile, out, 0o644); err != nil {
		return nil, fmt.Errorf("write snapshot: %w", err)
	}

	if err := s.updateHead(h); err != nil {
		return nil, fmt.Errorf("update HEAD: %w", err)
	}

	_ = ctx.Save(s.ctxDir)
	return snap, nil
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
