package extractors

import (
	"os/exec"
	"strconv"
	"strings"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// GitStateExtractor captures git working state.
type GitStateExtractor struct{ Base }

func NewGitState(root string) *GitStateExtractor {
	return &GitStateExtractor{Base: NewBase(root)}
}

func (e *GitStateExtractor) Name() string { return "git-state" }

func (e *GitStateExtractor) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = e.Root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (e *GitStateExtractor) Extract(ctx *projctx.ProjectContext) error {
	st := &projctx.ProjectStateContext{}
	if ctx.CurrentState != nil {
		// preserve TODOs collected by other extractor ordering
		st.TODOs = ctx.CurrentState.TODOs
	}
	branch, err := e.runGit("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		ctx.CurrentState = st
		return nil // not a git repo; graceful
	}
	st.GitBranch = branch
	if h, err := e.runGit("rev-parse", "--short", "HEAD"); err == nil {
		st.LastCommit = h
	}
	if m, err := e.runGit("log", "-1", "--pretty=%s"); err == nil {
		st.LastCommitMsg = m
	}
	if p, err := e.runGit("status", "--porcelain"); err == nil {
		if strings.TrimSpace(p) == "" {
			st.DirtyFiles = 0
		} else {
			st.DirtyFiles = len(strings.Split(strings.TrimSpace(p), "\n"))
		}
	}
	if names, err := e.runGit("log", "--name-only", "--pretty=format:", "-5"); err == nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(names, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				st.RecentlyChanged = append(st.RecentlyChanged, line)
			}
			if len(st.RecentlyChanged) >= 30 {
				break
			}
		}
	}
	st.ActiveAreas = activeAreas(st.RecentlyChanged)
	// dirty count fallback
	_ = strconv.Itoa(st.DirtyFiles)
	ctx.CurrentState = st
	return nil
}

func activeAreas(files []string) []string {
	counts := map[string]int{}
	for _, f := range files {
		dir := f
		if idx := strings.LastIndex(f, "/"); idx > 0 {
			dir = f[:idx]
		} else if idx := strings.LastIndex(f, "\\"); idx > 0 {
			dir = f[:idx]
		} else {
			dir = "."
		}
		counts[dir]++
	}
	type kv struct {
		k string
		v int
	}
	var kvs []kv
	for k, v := range counts {
		kvs = append(kvs, kv{k, v})
	}
	for i := 0; i < len(kvs); i++ {
		for j := i + 1; j < len(kvs); j++ {
			if kvs[j].v > kvs[i].v {
				kvs[i], kvs[j] = kvs[j], kvs[i]
			}
		}
	}
	var out []string
	for i, kv := range kvs {
		if i >= 5 {
			break
		}
		out = append(out, kv.k)
	}
	return out
}
