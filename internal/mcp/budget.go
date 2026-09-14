package mcp

import (
	"encoding/json"

	projctx "github.com/AnshGajera/CTX/internal/context"
)

// EstimateTokens approximates token count as chars/4 (rule of thumb for
// English/code; keeps MCP responses inside model context windows).
func EstimateTokens(v any) int {
	data, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(data) / 4
}

// sectionPriority orders sections from most to least valuable when a
// token budget forces truncation.
var sectionPriority = []string{
	"profile", "architecture", "apis", "database", "environment",
	"dependencies", "file_structure", "patterns", "business_rules",
	"decisions", "current_state",
}

// BudgetedContext returns sections greedily within maxTokens (<=0 = unlimited).
// Dropped sections are listed under "_dropped" and "_truncated" is set.
func BudgetedContext(ctx *projctx.ProjectContext, sections []string, maxTokens int) map[string]any {
	want := map[string]bool{}
	if len(sections) == 0 {
		for _, s := range sectionPriority {
			want[s] = true
		}
	} else {
		for _, s := range sections {
			want[normalizeSection(s)] = true
		}
	}
	out := map[string]any{"project_name": ctx.ProjectName}
	used := EstimateTokens(out)
	var dropped []string
	ordered := sectionPriority
	if len(sections) > 0 {
		// preserve requested order, then priority for the rest
		ordered = append(append([]string{}, sections...), sectionPriority...)
		seen := map[string]bool{}
		var dedup []string
		for _, s := range ordered {
			n := normalizeSection(s)
			if !seen[n] {
				seen[n] = true
				dedup = append(dedup, n)
			}
		}
		ordered = dedup
	}
	for _, name := range ordered {
		if !want[name] {
			continue
		}
		val := sectionValue(ctx, name)
		if val == nil {
			continue
		}
		if maxTokens > 0 && used+EstimateTokens(val) > maxTokens && len(out) > 1 {
			dropped = append(dropped, name)
			continue
		}
		out[jsonSectionName(name)] = val
		used += EstimateTokens(val)
	}
	if len(dropped) > 0 {
		out["_truncated"] = true
		out["_dropped"] = dropped
		out["_budget_tokens"] = maxTokens
	}
	return out
}

func normalizeSection(s string) string {
	switch s {
	case "structure", "file-structure", "files":
		return "file_structure"
	case "state", "git":
		return "current_state"
	case "env", "environment":
		return "environment"
	case "deps", "dependencies":
		return "dependencies"
	case "db", "database":
		return "database"
	case "api", "apis", "endpoints":
		return "apis"
	case "arch", "architecture":
		return "architecture"
	case "rules", "business_rules":
		return "business_rules"
	default:
		return s
	}
}

func jsonSectionName(name string) string {
	switch name {
	case "file_structure":
		return "file_structure"
	case "current_state":
		return "current_state"
	default:
		return name
	}
}

func sectionValue(ctx *projctx.ProjectContext, name string) any {
	switch name {
	case "profile":
		return ctx.Profile
	case "architecture":
		return ctx.Architecture
	case "apis":
		return ctx.APIs
	case "database":
		return ctx.Database
	case "environment":
		return ctx.Environment
	case "dependencies":
		return ctx.Dependencies
	case "file_structure":
		return ctx.FileStructure
	case "patterns":
		return ctx.Patterns
	case "business_rules":
		return ctx.BusinessRules
	case "decisions":
		return ctx.Decisions
	case "current_state":
		return ctx.CurrentState
	}
	return nil
}
