package mcp

import (
	"encoding/json"
	"testing"
)

func TestToolDefinitionsUseArrayForRequired(t *testing.T) {
	data, err := json.Marshal(ToolDefinitions())
	if err != nil {
		t.Fatalf("marshal tool definitions: %v", err)
	}

	var tools []struct {
		InputSchema struct {
			Required []string `json:"required"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(data, &tools); err != nil {
		t.Fatalf("unmarshal tool definitions: %v", err)
	}
	for _, tool := range tools {
		if tool.InputSchema.Required == nil {
			t.Fatal("tool schema required must be an array, not null")
		}
	}
}
