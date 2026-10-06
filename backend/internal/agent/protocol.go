package agent

import "projemble/internal/llm"

func llmTools() []llm.Tool {
	return []llm.Tool{
		{Name: "list_files", Description: "List project files by a relative directory path (use . for the workspace).", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative directory path")}, "path")},
		{Name: "read_file", Description: "Read a UTF-8 project file by a relative path.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path")}, "path")},
		{Name: "write_file", Description: "Create or replace a UTF-8 project file by a relative path. Keep changes inside the workspace.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path"), "content": stringSchema("Complete file contents")}, "path", "content")},
		{Name: "run_go_check", Description: "Run one of the fixed Go checks: test, build, or vet. No shell commands or arguments are accepted.", Parameters: objectSchema(map[string]any{"check": map[string]any{"type": "string", "enum": []string{"test", "build", "vet"}}}, "check")},
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
