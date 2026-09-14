package mcp

// ToolDefinition is an MCP tool schema.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolDefinitions lists all tools.
func ToolDefinitions() []ToolDefinition {
	obj := func(props map[string]any, required []string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	strArr := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return []ToolDefinition{
		{
			Name:        "get_project_context",
			Description: "Get full or section-filtered project context",
			InputSchema: obj(map[string]any{
				"sections": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"architecture", "apis", "database", "dependencies", "environment", "structure", "state", "patterns"}}},
			}, nil),
		},
		{
			Name:        "get_context_for_task",
			Description: "Get smart filtered context for a task description",
			InputSchema: obj(map[string]any{
				"task_description": map[string]any{"type": "string"},
				"affected_files":   strArr,
			}, []string{"task_description"}),
		},
		{
			Name:        "get_context_for_file",
			Description: "Get context relevant to a file path",
			InputSchema: obj(map[string]any{
				"file_path": map[string]any{"type": "string"},
			}, []string{"file_path"}),
		},
		{
			Name:        "get_api_endpoints",
			Description: "List API endpoints with optional filters",
			InputSchema: obj(map[string]any{
				"filter_path":   map[string]any{"type": "string"},
				"filter_method": map[string]any{"type": "string"},
			}, nil),
		},
		{
			Name:        "get_database_schema",
			Description: "Get database schema, optionally one model",
			InputSchema: obj(map[string]any{
				"model_name":      map[string]any{"type": "string"},
				"include_diagram": map[string]any{"type": "boolean"},
			}, nil),
		},
		{
			Name:        "get_project_conventions",
			Description: "Get file structure, key files and naming conventions",
			InputSchema: obj(map[string]any{}, nil),
		},
		{
			Name:        "get_env_requirements",
			Description: "Get required environment variables",
			InputSchema: obj(map[string]any{}, nil),
		},
	}
}
