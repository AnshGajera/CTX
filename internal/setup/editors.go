// Package setup writes AI-editor MCP configurations for ctx.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SupportedEditors lists editor IDs accepted by Install and Snippet.
func SupportedEditors() []string {
	return []string{"cursor", "claude-desktop", "vscode"}
}

// ServerEntry is the MCP server block shared by all editors.
func serverEntry(root string) map[string]any {
	return map[string]any{
		"command": "ctx",
		"args":    []string{"serve", "--stdio"},
		"cwd":     root,
	}
}

// Install writes/merges the editor's MCP config to point at this project.
// Returns the config file path written.
func Install(editor, root string) (string, error) {
	switch editor {
	case "cursor":
		return writeMerged(filepath.Join(root, ".cursor", "mcp.json"), "mcpServers", root)
	case "vscode":
		return writeMerged(filepath.Join(root, ".vscode", "mcp.json"), "servers", root)
	case "claude-desktop":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return InstallClaude(home, root)
	default:
		return "", fmt.Errorf("unknown editor %q (choose from: cursor, claude-desktop, vscode, none)", editor)
	}
}

// InstallClaude merges the ctx server into Claude Desktop's user config
// under the given home directory (injectable for tests).
func InstallClaude(home, root string) (string, error) {
	var path string
	switch runtime.GOOS {
	case "windows":
		path = filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	case "darwin":
		path = filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	default:
		path = filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}
	return writeMerged(path, "mcpServers", root)
}

// Snippet returns a copy-pasteable config block for manual setup.
func Snippet(editor, root string) (string, error) {
	var doc map[string]any
	switch editor {
	case "cursor", "claude-desktop":
		doc = map[string]any{"mcpServers": map[string]any{"ctx": serverEntry(root)}}
	case "vscode":
		doc = map[string]any{"servers": map[string]any{"ctx": serverEntry(root)}}
	default:
		return "", fmt.Errorf("unknown editor %q", editor)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeMerged merges {"ctx": server} into doc[topKey] in the JSON file at
// path, preserving other servers. Creates parent dirs as needed.
func writeMerged(path, topKey, root string) (string, error) {
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf("parse existing %s: %w", path, err)
		}
		// Back up the original config, preserving the first version.
		if _, statErr := os.Stat(path + ".bak"); os.IsNotExist(statErr) {
			_ = os.WriteFile(path+".bak", data, 0o644)
		}
	}
	servers, ok := doc[topKey].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	// Preserve an existing ctx entry instead of overwriting it.
	if _, exists := servers["ctx"]; !exists {
		entry := serverEntry(root)
		// VS Code convention carries an explicit stdio type.
		if topKey == "servers" {
			entry["type"] = "stdio"
		}
		servers["ctx"] = entry
	}
	doc[topKey] = servers
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
