package tui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// PromptLine reads one line with a default.
func PromptLine(r *bufio.Reader, w io.Writer, label, def string) string {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// PromptConfirm reads a y/n answer with a default.
func PromptConfirm(r *bufio.Reader, w io.Writer, label string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	fmt.Fprintf(w, "%s [%s]: ", label, hint)
	line, err := r.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.ToLower(strings.TrimSpace(line))
	switch line {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return def
	}
}

// SectionOption is one toggleable extraction section.
type SectionOption struct {
	Key   string
	Label string
	On    bool
}

// DefaultSections mirrors the default config toggles.
func DefaultSections() []SectionOption {
	return []SectionOption{
		{Key: "architecture", Label: "Architecture (layers, services, diagrams)", On: true},
		{Key: "api_endpoints", Label: "API endpoints", On: true},
		{Key: "database_schema", Label: "Database schema", On: true},
		{Key: "dependencies", Label: "Dependencies", On: true},
		{Key: "business_rules", Label: "Business rules / patterns", On: true},
		{Key: "env_vars", Label: "Environment variables", On: true},
	}
}

// PromptSections lets the user toggle sections. Returns nil for "all on"
// (no filter), otherwise the enabled keys. Empty/invalid input keeps defaults.
func PromptSections(r *bufio.Reader, w io.Writer, opts []SectionOption) []string {
	cyan := color.New(color.FgCyan)
	cyan.Fprintln(w, "Extraction sections (all on by default):")
	for i, o := range opts {
		mark := " "
		if o.On {
			mark = "x"
		}
		fmt.Fprintf(w, "  %d) [%s] %s\n", i+1, mark, o.Label)
	}
	fmt.Fprint(w, "Toggle numbers (e.g. 2,5), 'all', or Enter to keep: ")
	line, err := r.ReadString('\n')
	if err != nil {
		return nil
	}
	picks, all, none := parseSelection(line, len(opts))
	_ = none
	if all {
		return nil
	}
	if len(picks) == 0 {
		return nil // keep defaults = all on
	}
	// Start from all-on, toggle picked.
	on := map[string]bool{}
	for _, o := range opts {
		on[o.Key] = true
	}
	for _, idx := range picks {
		k := opts[idx].Key
		on[k] = !on[k]
	}
	var out []string
	for _, o := range opts {
		if on[o.Key] {
			out = append(out, o.Key)
		}
	}
	if len(out) == len(opts) {
		return nil
	}
	return out
}

// parseSelection parses "1,3" / "all" / "" into 0-based indices.
func parseSelection(line string, n int) (picks []int, all bool, none bool) {
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" || line == "all" {
		return nil, true, false
	}
	if line == "none" {
		return nil, false, true
	}
	seen := map[int]bool{}
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		i, err := strconv.Atoi(part)
		if err != nil || i < 1 || i > n {
			continue
		}
		if !seen[i-1] {
			seen[i-1] = true
			picks = append(picks, i-1)
		}
	}
	return picks, false, false
}

// PromptEditor offers MCP editor setup. Returns cursor|claude-desktop|vscode|none.
func PromptEditor(r *bufio.Reader, w io.Writer) string {
	options := []string{"cursor", "claude-desktop", "vscode", "none"}
	cyan := color.New(color.FgCyan)
	cyan.Fprintln(w, "Connect an AI editor via MCP?")
	for i, o := range options {
		fmt.Fprintf(w, "  %d) %s\n", i+1, o)
	}
	fmt.Fprint(w, "Choice [4]: ")
	line, err := r.ReadString('\n')
	if err != nil {
		return "none"
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return "none"
	}
	for _, o := range options {
		if line == o {
			return o
		}
	}
	if i, err := strconv.Atoi(line); err == nil && i >= 1 && i <= len(options) {
		return options[i-1]
	}
	return "none"
}
