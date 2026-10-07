package agent

import "projemble/internal/llm"

func llmTools() []llm.Tool {
	return llmToolsForAccess(AccessFull)
}

func llmToolsForAccess(mode AccessMode) []llm.Tool {
	definitions := map[string]llm.Tool{
		"list_files":  {Name: "list_files", Description: "List a bounded set of project files using a relative directory path (use . for the workspace).", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative directory path")}, "path")},
		"read_file":   {Name: "read_file", Description: "Read a UTF-8 project file by a relative path. Credential and private state paths are blocked.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path")}, "path")},
		"write_file":  {Name: "write_file", Description: "Create or replace a UTF-8 project file by a relative path. Workspace boundaries, credentials, and protected instruction/control files are enforced by the tool dispatcher.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path"), "content": stringSchema("Complete file contents")}, "path", "content")},
		"run_command": {Name: "run_command", Description: "Run fixed Go checks or a one-shot Go app from the project root. run requires one relative package target and accepts up to 32 separate runtime args (16 KB total). Output streams live; each run times out after 3 minutes; an agent turn can start at most 3 runs. No long-running server sessions.", Parameters: objectSchema(map[string]any{"operation": map[string]any{"type": "string", "enum": []string{"run", "fmt", "list", "test", "build", "vet"}}, "target": stringSchema("Relative Go package target; run requires one target such as ./cmd/job"), "args": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "operation")},
		"run_shell":   {Name: "run_shell", Description: "Run a shell command with the project directory as its working directory. Output streams live and the command times out after 3 minutes. Use for project tasks that need the platform shell; do not access paths outside this project.", Parameters: objectSchema(map[string]any{"command": stringSchema("Shell command to run")}, "command")},
	}
	result := make([]llm.Tool, 0, len(mode.tools()))
	for _, name := range mode.tools() {
		if tool, ok := definitions[name]; ok {
			result = append(result, tool)
		}
	}
	return result
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
