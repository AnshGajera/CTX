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
				"sections":   map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"architecture", "apis", "database", "dependencies", "environment", "structure", "state", "patterns"}}},
				"max_tokens": map[string]any{"type": "integer", "description": "token budget; least-important sections are dropped first"},
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
		{
			Name:        "search_context",
			Description: "Hybrid semantic search over chunked project context (TF-IDF ranking, no sidecar needed)",
			InputSchema: obj(map[string]any{
				"query": map[string]any{"type": "string", "description": "natural language query, e.g. 'where is auth handled'"},
				"top_k": map[string]any{"type": "integer", "description": "max results (default 5)"},
			}, []string{"query"}),
		},
		{
			Name:        "branch_context",
			Description: "Git Context Control: list, create, or delete context branches to explore alternative reasoning paths or sub-tasks without polluting the main context",
			InputSchema: obj(map[string]any{
				"action":      map[string]any{"type": "string", "enum": []string{"list", "create", "delete"}, "description": "action to perform"},
				"branch":      map[string]any{"type": "string", "description": "branch name (for create or delete)"},
				"start_point": map[string]any{"type": "string", "description": "starting ref or snapshot (default HEAD)"},
			}, []string{"action"}),
		},
		{
			Name:        "checkout_context",
			Description: "Git Context Control: switch active context branch or restore context to a snapshot/milestone",
			InputSchema: obj(map[string]any{
				"target":        map[string]any{"type": "string", "description": "branch name, tag, or snapshot hash"},
				"create_branch": map[string]any{"type": "boolean", "description": "if true, creates and switches to a new branch"},
			}, []string{"target"}),
		},
		{
			Name:        "commit_context",
			Description: "Git Context Control: save an explicit context checkpoint with a reasoning milestone description",
			InputSchema: obj(map[string]any{
				"message": map[string]any{"type": "string", "description": "milestone description or reasoning note"},
			}, []string{"message"}),
		},
		{
			Name:        "merge_context",
			Description: "Git Context Control: merge discovered context (APIs, schemas, env vars, dependencies, conventions) from another branch into the active branch",
			InputSchema: obj(map[string]any{
				"source_branch": map[string]any{"type": "string", "description": "source branch or ref to merge"},
				"message":       map[string]any{"type": "string", "description": "merge commit message"},
			}, []string{"source_branch"}),
		},
		{
			Name:        "tag_context",
			Description: "Git Context Control: tag a context milestone or release baseline",
			InputSchema: obj(map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "create", "delete"}, "description": "tag operation"},
				"tag":    map[string]any{"type": "string", "description": "tag name"},
				"target": map[string]any{"type": "string", "description": "target ref or snapshot hash (default HEAD)"},
			}, []string{"action"}),
		},
		{
			Name:        "get_context_diff",
			Description: "Git Context Control: compute structured diff between two context snapshots, branches, or tags",
			InputSchema: obj(map[string]any{
				"from": map[string]any{"type": "string", "description": "source ref (default HEAD~1)"},
				"to":   map[string]any{"type": "string", "description": "target ref (default HEAD)"},
			}, nil),
		},
		{
			Name:        "get_context_history",
			Description: "Git Context Control: get context snapshot and checkpoint timeline with branch and tag metadata",
			InputSchema: obj(map[string]any{
				"limit": map[string]any{"type": "integer", "description": "maximum snapshots to return (default 10)"},
			}, nil),
		},
	}
}
