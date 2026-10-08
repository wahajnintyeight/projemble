package agent

import "projemble/internal/llm"

func llmTools() []llm.Tool {
	return llmToolsForAccess(AccessFull)
}

func llmToolsForAccess(mode AccessMode) []llm.Tool {
	definitions := map[string]llm.Tool{
		"list_files":      {Name: "list_files", Description: "List a bounded set of project files using a relative directory path (use . for the workspace).", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative directory path")}, "path")},
		"search_files":    {Name: "search_files", Description: "Case-insensitive literal search through bounded UTF-8 project files; returns path, line, and a short excerpt. The optional relative directory defaults to the workspace.", Parameters: objectSchema(map[string]any{"query": stringSchema("Literal text to search for; one line, up to 1,024 bytes"), "path": stringSchema("Optional relative directory path; defaults to .")}, "query")},
		"read_file":       {Name: "read_file", Description: "Read a UTF-8 project file as numbered lines. Defaults to the first 200 lines; use start_line and end_line to fetch a smaller range. Output is bounded and gives a continuation line when needed.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path"), "start_line": map[string]any{"type": "integer", "minimum": 1}, "end_line": map[string]any{"type": "integer", "minimum": 1}}, "path")},
		"write_file":      {Name: "write_file", Description: "Create a new UTF-8 project file. It refuses to replace an existing file; use edit_file for an exact targeted change. Workspace boundaries and protected instruction/control files are enforced by the tool dispatcher.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path"), "content": stringSchema("Complete file contents for a new file")}, "path", "content")},
		"edit_file":       {Name: "edit_file", Description: "Replace one exact, unique text span in an existing UTF-8 project file. Returns a compact diff and records the prior contents for undo_last_edit. Refuses ambiguous matches and protected files.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path"), "old_text": stringSchema("Non-empty exact text that appears exactly once"), "new_text": stringSchema("Replacement text; may be empty to remove the matched span")}, "path", "old_text", "new_text")},
		"undo_last_edit":  {Name: "undo_last_edit", Description: "Undo the most recent agent-created or agent-edited file if it has not changed since that action.", Parameters: objectSchema(map[string]any{})},
		"run_command":     {Name: "run_command", Description: "Run fixed Go checks or a one-shot Go app from the project root. run requires one relative package target and accepts up to 32 separate runtime args (16 KB total). Output streams live; each run times out after 3 minutes; an agent turn can start at most 3 runs. No long-running server sessions.", Parameters: objectSchema(map[string]any{"operation": map[string]any{"type": "string", "enum": []string{"run", "fmt", "list", "test", "build", "vet"}}, "target": stringSchema("Relative Go package target; run requires one target such as ./cmd/job"), "args": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "operation")},
		"run_shell":       {Name: "run_shell", Description: "Run a shell command with the project directory as its working directory. The operating system does not confine it to that directory, so every command requires user approval and can access other paths. Output streams live; timeout is 3 minutes.", Parameters: objectSchema(map[string]any{"command": stringSchema("Shell command to run")}, "command")},
		"delegate_checks": {Name: "delegate_checks", Description: "When independent checks can run concurrently, spawn up to 3 verification agents with distinct scopes and concrete targets. They can inspect the project, run targeted Go test/build/vet checks, or probe a running localhost HTTP API. Workers have no file-editing or shell tools, cannot access remote hosts, and cannot spawn workers.", Parameters: objectSchema(map[string]any{"tasks": map[string]any{"type": "array", "items": stringSchema("One independent verification task with a clear target and expected evidence")}}, "tasks")},
	}
	result := make([]llm.Tool, 0, len(mode.tools()))
	for _, name := range mode.tools() {
		if tool, ok := definitions[name]; ok {
			result = append(result, tool)
		}
	}
	return result
}

func workerTools() []llm.Tool {
	return []llm.Tool{
		{Name: "list_files", Description: "List project files using a relative directory path. Read-only and bounded.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative directory path; use . for the workspace")}, "path")},
		{Name: "search_files", Description: "Search UTF-8 project files for literal text. Read-only, bounded, and returns path:line excerpts.", Parameters: objectSchema(map[string]any{"query": stringSchema("Literal text to search for"), "path": stringSchema("Optional relative directory path; defaults to .")}, "query")},
		{Name: "read_file", Description: "Read a project file as numbered lines. Use start_line and end_line for a focused range; output is bounded.", Parameters: objectSchema(map[string]any{"path": stringSchema("Relative file path"), "start_line": map[string]any{"type": "integer", "minimum": 1}, "end_line": map[string]any{"type": "integer", "minimum": 1}}, "path")},
		{Name: "run_check", Description: "Run one bounded Go test, build, or vet check from the project directory. No shell or file editing.", Parameters: objectSchema(map[string]any{"operation": map[string]any{"type": "string", "enum": []string{"test", "build", "vet"}}, "target": stringSchema("Relative Go package target, such as ./internal/service")}, "operation", "target")},
		{Name: "probe_http", Description: "Send a read-only GET, HEAD, or OPTIONS request to a loopback HTTP endpoint. Redirects are not followed; response bodies are bounded.", Parameters: objectSchema(map[string]any{"method": map[string]any{"type": "string", "enum": []string{"GET", "HEAD", "OPTIONS"}}, "url": stringSchema("HTTP(S) URL on localhost, 127.0.0.1, or ::1")}, "method", "url")},
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
