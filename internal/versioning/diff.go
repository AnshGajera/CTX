package versioning

import (
	"fmt"
	"sort"

	projctx "github.com/ctxdev/ctx/internal/context"
)

// ContextDiff describes changes between two contexts.
type ContextDiff struct {
	AddedEndpoints   []projctx.APIEndpoint   `json:"added_endpoints,omitempty"`
	RemovedEndpoints []projctx.APIEndpoint   `json:"removed_endpoints,omitempty"`
	AddedModels      []projctx.DatabaseModel `json:"added_models,omitempty"`
	RemovedModels    []projctx.DatabaseModel `json:"removed_models,omitempty"`
	AddedEnvVars     []projctx.EnvVariable   `json:"added_env_vars,omitempty"`
	RemovedEnvVars   []projctx.EnvVariable   `json:"removed_env_vars,omitempty"`
	AddedDeps        []projctx.Dependency    `json:"added_dependencies,omitempty"`
	RemovedDeps      []projctx.Dependency    `json:"removed_dependencies,omitempty"`
	Summary          string                  `json:"summary"`
}

// ComputeDiff compares old vs new.
func ComputeDiff(old, new *projctx.ProjectContext) *ContextDiff {
	d := &ContextDiff{}
	if old == nil || new == nil {
		d.Summary = "insufficient history for diff"
		return d
	}
	oldEP := map[string]projctx.APIEndpoint{}
	if old.APIs != nil {
		for _, e := range old.APIs.Endpoints {
			oldEP[e.Method+" "+e.Path] = e
		}
	}
	newEP := map[string]projctx.APIEndpoint{}
	if new.APIs != nil {
		for _, e := range new.APIs.Endpoints {
			newEP[e.Method+" "+e.Path] = e
		}
	}
	for k, e := range newEP {
		if _, ok := oldEP[k]; !ok {
			d.AddedEndpoints = append(d.AddedEndpoints, e)
		}
	}
	for k, e := range oldEP {
		if _, ok := newEP[k]; !ok {
			d.RemovedEndpoints = append(d.RemovedEndpoints, e)
		}
	}

	oldM := map[string]projctx.DatabaseModel{}
	if old.Database != nil {
		for _, m := range old.Database.Models {
			oldM[m.Name] = m
		}
	}
	newM := map[string]projctx.DatabaseModel{}
	if new.Database != nil {
		for _, m := range new.Database.Models {
			newM[m.Name] = m
		}
	}
	for k, m := range newM {
		if _, ok := oldM[k]; !ok {
			d.AddedModels = append(d.AddedModels, m)
		}
	}
	for k, m := range oldM {
		if _, ok := newM[k]; !ok {
			d.RemovedModels = append(d.RemovedModels, m)
		}
	}

	oldE := map[string]projctx.EnvVariable{}
	if old.Environment != nil {
		for _, v := range old.Environment.Variables {
			oldE[v.Name] = v
		}
	}
	newE := map[string]projctx.EnvVariable{}
	if new.Environment != nil {
		for _, v := range new.Environment.Variables {
			newE[v.Name] = v
		}
	}
	for k, v := range newE {
		if _, ok := oldE[k]; !ok {
			d.AddedEnvVars = append(d.AddedEnvVars, v)
		}
	}
	for k, v := range oldE {
		if _, ok := newE[k]; !ok {
			d.RemovedEnvVars = append(d.RemovedEnvVars, v)
		}
	}

	oldD := map[string]projctx.Dependency{}
	if old.Dependencies != nil {
		for _, dep := range append(append([]projctx.Dependency{}, old.Dependencies.Direct...), old.Dependencies.Dev...) {
			oldD[dep.Name] = dep
		}
	}
	newD := map[string]projctx.Dependency{}
	if new.Dependencies != nil {
		for _, dep := range append(append([]projctx.Dependency{}, new.Dependencies.Direct...), new.Dependencies.Dev...) {
			newD[dep.Name] = dep
		}
	}
	for k, dep := range newD {
		if _, ok := oldD[k]; !ok {
			d.AddedDeps = append(d.AddedDeps, dep)
		}
	}
	for k, dep := range oldD {
		if _, ok := newD[k]; !ok {
			d.RemovedDeps = append(d.RemovedDeps, dep)
		}
	}

	sort.Slice(d.AddedEndpoints, func(i, j int) bool {
		return d.AddedEndpoints[i].Path < d.AddedEndpoints[j].Path
	})
	sort.Slice(d.RemovedEndpoints, func(i, j int) bool {
		return d.RemovedEndpoints[i].Path < d.RemovedEndpoints[j].Path
	})

	d.Summary = fmt.Sprintf("+%d endpoints, -%d endpoints, +%d models, -%d models, +%d env, -%d env, +%d deps, -%d deps",
		len(d.AddedEndpoints), len(d.RemovedEndpoints),
		len(d.AddedModels), len(d.RemovedModels),
		len(d.AddedEnvVars), len(d.RemovedEnvVars),
		len(d.AddedDeps), len(d.RemovedDeps))
	return d
}

// IsEmpty reports no changes.
func (d *ContextDiff) IsEmpty() bool {
	return len(d.AddedEndpoints) == 0 && len(d.RemovedEndpoints) == 0 &&
		len(d.AddedModels) == 0 && len(d.RemovedModels) == 0 &&
		len(d.AddedEnvVars) == 0 && len(d.RemovedEnvVars) == 0 &&
		len(d.AddedDeps) == 0 && len(d.RemovedDeps) == 0
}
