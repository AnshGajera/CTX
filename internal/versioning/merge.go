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

// MergeReport summarizes what was integrated during a context merge.
type MergeReport struct {
	SourceRef        string   `json:"source_ref"`
	TargetRef        string   `json:"target_ref"`
	SourceBranch     string   `json:"source_branch,omitempty"`
	TargetBranch     string   `json:"target_branch,omitempty"`
	AddedEndpoints   int      `json:"added_endpoints"`
	UpdatedEndpoints int      `json:"updated_endpoints"`
	AddedModels      int      `json:"added_models"`
	UpdatedModels    int      `json:"updated_models"`
	AddedEnvVars     int      `json:"added_env_vars"`
	AddedDeps        int      `json:"added_deps"`
	MergedPatterns   int      `json:"merged_patterns"`
	Details          []string `json:"details"`
}

// Merge combines context from sourceRef into the currently checked-out branch.
// It reconciles APIs, database models, environment variables, dependencies, and patterns,
// then creates a merge snapshot and updates the active branch and .ctx/context.json.
func (s *ContextStore) Merge(sourceRef string, message string) (*ContextSnapshot, *MergeReport, error) {
	curBranch, isDetached, err := s.CurrentBranch()
	if err != nil {
		return nil, nil, fmt.Errorf("determine current branch: %w", err)
	}

	targetHash, err := s.GetHead()
	if err != nil || targetHash == "" {
		return nil, nil, fmt.Errorf("current HEAD has no commits to merge into")
	}

	sourceHash, err := s.ResolveRef(sourceRef)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve source ref %q: %w", sourceRef, err)
	}

	report := &MergeReport{
		SourceRef:    sourceHash,
		TargetRef:    targetHash,
		SourceBranch: sourceRef,
		TargetBranch: curBranch,
	}

	if sourceHash == targetHash {
		report.Details = append(report.Details, "Already up to date.")
		targetSnap, _ := s.LoadSnapshot(targetHash)
		return targetSnap, report, nil
	}

	targetSnap, err := s.LoadSnapshot(targetHash)
	if err != nil {
		return nil, nil, fmt.Errorf("load target snapshot %s: %w", targetHash, err)
	}
	sourceSnap, err := s.LoadSnapshot(sourceHash)
	if err != nil {
		return nil, nil, fmt.Errorf("load source snapshot %s: %w", sourceHash, err)
	}

	targetCtx, err := SnapshotToContext(targetSnap)
	if err != nil {
		return nil, nil, fmt.Errorf("decode target context: %w", err)
	}
	sourceCtx, err := SnapshotToContext(sourceSnap)
	if err != nil {
		return nil, nil, fmt.Errorf("decode source context: %w", err)
	}

	// Perform intelligent context reconciliation
	reconcileContexts(targetCtx, sourceCtx, report)

	// Build merge commit message
	if strings.TrimSpace(message) == "" {
		if !isDetached {
			message = fmt.Sprintf("Merge branch '%s' into %s", sourceRef, curBranch)
		} else {
			message = fmt.Sprintf("Merge %s into HEAD", sourceRef)
		}
	}

	if targetCtx.CurrentState == nil {
		targetCtx.CurrentState = &projctx.ProjectStateContext{}
	}
	targetCtx.CurrentState.GitBranch = curBranch
	targetCtx.CurrentState.LastCommitMsg = message

	data, err := json.Marshal(targetCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal merged context: %w", err)
	}

	newHash := CanonicalOrCheckpointHash(targetCtx, message)
	targetCtx.ContentHash = newHash

	mergeSnap := &ContextSnapshot{
		Hash:       newHash,
		ParentHash: targetHash,
		Timestamp:  time.Now().UTC(),
		Author:     gitAuthor(s.ctxDir),
		Message:    message,
		Context:    data,
		GitBranch:  curBranch,
	}

	out, err := json.MarshalIndent(mergeSnap, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal merge snapshot: %w", err)
	}

	snapFile := filepath.Join(s.snapshotsDir(), newHash+".json")
	if err := os.WriteFile(snapFile, out, 0o644); err != nil {
		return nil, nil, fmt.Errorf("write merge snapshot: %w", err)
	}

	if err := s.updateHead(newHash); err != nil {
		return nil, nil, fmt.Errorf("update HEAD: %w", err)
	}

	// Update working context file
	_ = targetCtx.Save(s.ctxDir)

	return mergeSnap, report, nil
}

func reconcileContexts(target, source *projctx.ProjectContext, report *MergeReport) {
	// 1. Merge APIs
	if source.APIs != nil && len(source.APIs.Endpoints) > 0 {
		if target.APIs == nil {
			target.APIs = &projctx.APIContext{}
		}
		epMap := make(map[string]int)
		for idx, ep := range target.APIs.Endpoints {
			key := strings.ToUpper(ep.Method) + " " + ep.Path
			epMap[key] = idx
		}

		for _, srcEp := range source.APIs.Endpoints {
			key := strings.ToUpper(srcEp.Method) + " " + srcEp.Path
			if existingIdx, exists := epMap[key]; exists {
				// Enrich existing endpoint with any missing details
				orig := target.APIs.Endpoints[existingIdx]
				changed := false
				if orig.Description == "" && srcEp.Description != "" {
					orig.Description = srcEp.Description
					changed = true
				}
				if !orig.AuthRequired && srcEp.AuthRequired {
					orig.AuthRequired = srcEp.AuthRequired
					changed = true
				}
				if len(orig.Parameters) == 0 && len(srcEp.Parameters) > 0 {
					orig.Parameters = srcEp.Parameters
					changed = true
				}
				if len(orig.Middleware) == 0 && len(srcEp.Middleware) > 0 {
					orig.Middleware = srcEp.Middleware
					changed = true
				}
				if changed {
					target.APIs.Endpoints[existingIdx] = orig
					report.UpdatedEndpoints++
					report.Details = append(report.Details, fmt.Sprintf("Updated endpoint %s", key))
				}
			} else {
				target.APIs.Endpoints = append(target.APIs.Endpoints, srcEp)
				epMap[key] = len(target.APIs.Endpoints) - 1
				report.AddedEndpoints++
				report.Details = append(report.Details, fmt.Sprintf("Added endpoint %s", key))
			}
		}
		sort.Slice(target.APIs.Endpoints, func(i, j int) bool {
			return target.APIs.Endpoints[i].Path < target.APIs.Endpoints[j].Path
		})
	}

	// 2. Merge Database Models
	if source.Database != nil && len(source.Database.Models) > 0 {
		if target.Database == nil {
			target.Database = &projctx.DatabaseContext{}
		}
		modelMap := make(map[string]int)
		for idx, m := range target.Database.Models {
			modelMap[m.Name] = idx
		}

		for _, srcM := range source.Database.Models {
			if existingIdx, exists := modelMap[srcM.Name]; exists {
				// Merge model fields
				curM := target.Database.Models[existingIdx]
				fieldMap := make(map[string]bool)
				for _, f := range curM.Fields {
					fieldMap[f.Name] = true
				}
				addedField := false
				for _, sf := range srcM.Fields {
					if !fieldMap[sf.Name] {
						curM.Fields = append(curM.Fields, sf)
						fieldMap[sf.Name] = true
						addedField = true
					}
				}
				if addedField {
					target.Database.Models[existingIdx] = curM
					report.UpdatedModels++
					report.Details = append(report.Details, fmt.Sprintf("Updated model %s (added fields)", srcM.Name))
				}
			} else {
				target.Database.Models = append(target.Database.Models, srcM)
				modelMap[srcM.Name] = len(target.Database.Models) - 1
				report.AddedModels++
				report.Details = append(report.Details, fmt.Sprintf("Added model %s", srcM.Name))
			}
		}
	}

	// 3. Merge Environment Variables
	if source.Environment != nil && len(source.Environment.Variables) > 0 {
		if target.Environment == nil {
			target.Environment = &projctx.EnvironmentContext{}
		}
		envMap := make(map[string]bool)
		for _, v := range target.Environment.Variables {
			envMap[v.Name] = true
		}
		for _, sv := range source.Environment.Variables {
			if !envMap[sv.Name] {
				target.Environment.Variables = append(target.Environment.Variables, sv)
				envMap[sv.Name] = true
				report.AddedEnvVars++
				report.Details = append(report.Details, fmt.Sprintf("Added env var %s", sv.Name))
			}
		}
	}

	// 4. Merge Dependencies
	if source.Dependencies != nil {
		if target.Dependencies == nil {
			target.Dependencies = &projctx.DependencyContext{}
		}
		depMap := make(map[string]bool)
		for _, d := range target.Dependencies.Direct {
			depMap[d.Name] = true
		}
		for _, sd := range source.Dependencies.Direct {
			if !depMap[sd.Name] {
				target.Dependencies.Direct = append(target.Dependencies.Direct, sd)
				depMap[sd.Name] = true
				report.AddedDeps++
				report.Details = append(report.Details, fmt.Sprintf("Added dependency %s@%s", sd.Name, sd.Version))
			}
		}
	}

	// 5. Merge Patterns & Conventions
	if source.Patterns != nil && len(source.Patterns.Patterns) > 0 {
		if target.Patterns == nil {
			target.Patterns = &projctx.PatternContext{}
		}
		patMap := make(map[string]bool)
		for _, p := range target.Patterns.Patterns {
			patMap[p.Name] = true
		}
		for _, sp := range source.Patterns.Patterns {
			if !patMap[sp.Name] {
				target.Patterns.Patterns = append(target.Patterns.Patterns, sp)
				patMap[sp.Name] = true
				report.MergedPatterns++
			}
		}
	}

	// 6. Merge AI Conventions
	if len(source.AIConventions) > 0 {
		aiMap := make(map[string]bool)
		for _, c := range target.AIConventions {
			aiMap[c.Name] = true
		}
		for _, sc := range source.AIConventions {
			if !aiMap[sc.Name] {
				target.AIConventions = append(target.AIConventions, sc)
				aiMap[sc.Name] = true
			}
		}
	}
}
