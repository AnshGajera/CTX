package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCursorInstallMerges(t *testing.T) {
	root := t.TempDir()
	// Pre-existing config with another server must survive.
	pre := map[string]any{"mcpServers": map[string]any{"other": map[string]any{"command": "other"}}}
	raw, _ := json.Marshal(pre)
	if err := os.MkdirAll(filepath.Join(root, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursor", "mcp.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := Install("cursor", root)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("existing server lost")
	}
	ctx, ok := servers["ctx"].(map[string]any)
	if !ok || ctx["command"] != "ctx" {
		t.Fatalf("ctx server wrong: %v", servers["ctx"])
	}
}

func TestVSCodeInstallType(t *testing.T) {
	root := t.TempDir()
	path, err := Install("vscode", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join(".vscode", "mcp.json")) {
		t.Fatalf("path = %s", path)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"type": "stdio"`) {
		t.Fatalf("vscode entry needs stdio type:\n%s", data)
	}
}

func TestClaudeInstallAndSnippet(t *testing.T) {
	home := t.TempDir()
	path, err := InstallClaude(home, "/proj")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "claude_desktop_config.json") {
		t.Fatalf("path = %s", path)
	}
	snip, err := Snippet("claude-desktop", "/proj")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snip, "--stdio") || !strings.Contains(snip, "/proj") {
		t.Fatalf("snippet:\n%s", snip)
	}
	if _, err := Install("vim", "/proj"); err == nil {
		t.Fatal("unknown editor must error")
	}
}
