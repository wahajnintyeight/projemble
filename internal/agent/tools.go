package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"projemble/internal/llm"
)

const (
	maxFileBytes         = 1 << 20
	maxToolArgumentBytes = 8 << 20
	maxDirectoryEntries  = 10_000
)

func runTool(ctx context.Context, root, name, raw, secretEnvName, secret string, output io.Writer, history *[]editSnapshot) (string, error) {
	switch name {
	case "list_files":
		args, err := decodeToolArgs(raw, []string{"path"}, []string{"path"})
		if err != nil {
			return "", err
		}
		return listFiles(root, args["path"])
	case "read_file":
		args, err := decodeReadFileArguments(raw)
		if err != nil {
			return "", err
		}
		return readFileRange(root, args["path"], parseOptionalLine(args["start_line"]), parseOptionalLine(args["end_line"]))
	case "search_files":
		args, err := decodeSearchArguments(raw)
		if err != nil {
			return "", err
		}
		return searchFiles(root, args["path"], args["query"])
	case "write_file":
		args, err := decodeToolArgs(raw, []string{"path", "content"}, []string{"path", "content"})
		if err != nil {
			return "", err
		}
		return createFile(root, args["path"], args["content"], history)
	case "edit_file":
		args, err := decodeToolArgs(raw, []string{"path", "old_text", "new_text"}, []string{"path", "old_text", "new_text"})
		if err != nil {
			return "", err
		}
		return editFile(root, args["path"], args["old_text"], args["new_text"], history)
	case "undo_last_edit":
		if _, err := decodeToolObject(raw, nil); err != nil {
			return "", err
		}
		return undoLastEdit(root, history)
	case "run_go_check":
		args, err := decodeToolArgs(raw, []string{"check"}, []string{"check"})
		if err != nil {
			return "", err
		}
		return runGoCheck(ctx, root, args["check"], secretEnvName, secret, output)
	case "run_command":
		args, runtimeArgs, err := decodeRunCommandArguments(raw)
		if err != nil {
			return "", err
		}
		return runProjectCommand(ctx, root, args["operation"], args["target"], secretEnvName, secret, output, runtimeArgs...)
	case "run_shell":
		args, err := decodeToolArgs(raw, []string{"command"}, []string{"command"})
		if err != nil {
			return "", err
		}
		return runShellCommand(ctx, root, args["command"], secretEnvName, secret, output)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

// decodeToolArgs enforces the same required fields and closed object shape
// advertised in the provider schema. Provider schemas are hints; this is the
// actual execution boundary.
func decodeToolArgs(raw string, required, allowed []string) (map[string]string, error) {
	values, err := decodeToolObject(raw, allowed)
	if err != nil {
		return nil, err
	}
	args := make(map[string]string, len(values))
	for field, value := range values {
		text, err := decodeToolString(value, field)
		if err != nil {
			return nil, err
		}
		args[field] = text
	}
	for _, field := range required {
		value, ok := args[field]
		if !ok || (field != "content" && field != "new_text" && strings.TrimSpace(value) == "") {
			return nil, fmt.Errorf("required tool argument %q is missing or empty", field)
		}
	}
	return args, nil
}

func decodeToolObject(raw string, allowed []string) (map[string]json.RawMessage, error) {
	if len(raw) > maxToolArgumentBytes {
		return nil, fmt.Errorf("tool arguments exceed %d bytes", maxToolArgumentBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	var values map[string]json.RawMessage
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if values == nil {
		return nil, errors.New("tool arguments must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("tool arguments contain trailing JSON data")
		}
		return nil, fmt.Errorf("invalid trailing tool arguments: %w", err)
	}
	allowedFields := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedFields[field] = struct{}{}
	}
	for field := range values {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("unexpected tool argument %q", field)
		}
	}
	return values, nil
}

func decodeToolString(value json.RawMessage, field string) (string, error) {
	if string(value) == "null" {
		return "", fmt.Errorf("tool argument %q must be a string", field)
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return "", fmt.Errorf("tool argument %q must be a string", field)
	}
	return text, nil
}

func decodeRunCommandArguments(raw string) (map[string]string, []string, error) {
	values, err := decodeToolObject(raw, []string{"operation", "target", "args"})
	if err != nil {
		return nil, nil, err
	}
	args := make(map[string]string, 2)
	for _, field := range []string{"operation", "target"} {
		if value, ok := values[field]; ok {
			text, err := decodeToolString(value, field)
			if err != nil {
				return nil, nil, err
			}
			args[field] = text
		}
	}
	if strings.TrimSpace(args["operation"]) == "" {
		return nil, nil, errors.New("required tool argument \"operation\" is missing or empty")
	}

	var runtimeArgs []string
	if value, ok := values["args"]; ok {
		if string(value) == "null" || json.Unmarshal(value, &runtimeArgs) != nil || runtimeArgs == nil {
			return nil, nil, errors.New("tool argument \"args\" must be an array of strings")
		}
	}
	if err := validateRuntimeArgs(runtimeArgs); err != nil {
		return nil, nil, err
	}
	return args, runtimeArgs, nil
}

func validateToolCalls(root string, calls []llm.ToolCall, idOptional bool) error {
	return validateToolCallsForAccess(root, calls, idOptional, AccessFull)
}

func validateToolCallsForAccess(root string, calls []llm.ToolCall, idOptional bool, mode AccessMode) error {
	seenIDs := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.ID) == "" && !idOptional {
			return errors.New("tool call is missing its call ID")
		}
		if call.ID != "" {
			if _, duplicate := seenIDs[call.ID]; duplicate {
				return fmt.Errorf("duplicate tool call ID %q", call.ID)
			}
			seenIDs[call.ID] = struct{}{}
		}
		args, err := validateArgumentsOnly(call.Name, call.Arguments)
		if err != nil {
			return err
		}
		if mode == AccessReadOnly && call.Name != "list_files" && call.Name != "read_file" && call.Name != "search_files" {
			return fmt.Errorf("read-only mode blocks %s", call.Name)
		}
		if call.Name == "list_files" || call.Name == "read_file" || call.Name == "write_file" || call.Name == "edit_file" || call.Name == "search_files" {
			name := args["path"]
			if name == "" {
				name = "."
			}
			if _, err := safePath(root, name); err != nil {
				return fmt.Errorf("%s path rejected: %w", call.Name, err)
			}
		}
		if call.Name == "run_command" && args["target"] != "" && args["target"] != "./..." {
			target := filepath.FromSlash(strings.TrimPrefix(args["target"], "./"))
			if _, err := safePath(root, target); err != nil {
				return fmt.Errorf("run_command target rejected: %w", err)
			}
		}
		if call.Name == "run_shell" && strings.TrimSpace(args["command"]) == "" {
			return errors.New("shell command is required")
		}
		if call.Name == "write_file" {
			if isProtectedWritePath(args["path"]) {
				return errors.New("project instructions and agent control files are protected from automatic edits")
			}
			if len(args["content"]) > maxFileBytes {
				return fmt.Errorf("write_file content exceeds %d bytes", maxFileBytes)
			}
		}
		if call.Name == "edit_file" {
			if isProtectedWritePath(args["path"]) {
				return errors.New("project instructions and agent control files are protected from automatic edits")
			}
			if args["old_text"] == "" || len(args["old_text"]) > maxFileBytes || len(args["new_text"]) > maxFileBytes {
				return fmt.Errorf("edit_file requires non-empty old_text and text fields no larger than %d bytes", maxFileBytes)
			}
		}
	}
	return nil
}

func validateArgumentsOnly(name, raw string) (map[string]string, error) {
	switch name {
	case "list_files":
		return decodeToolArgs(raw, []string{"path"}, []string{"path"})
	case "read_file":
		return decodeReadFileArguments(raw)
	case "search_files":
		return decodeSearchArguments(raw)
	case "write_file":
		return decodeToolArgs(raw, []string{"path", "content"}, []string{"path", "content"})
	case "edit_file":
		return decodeToolArgs(raw, []string{"path", "old_text", "new_text"}, []string{"path", "old_text", "new_text"})
	case "undo_last_edit":
		if _, err := decodeToolObject(raw, nil); err != nil {
			return nil, err
		}
		return map[string]string{}, nil
	case "run_go_check":
		args, err := decodeToolArgs(raw, []string{"check"}, []string{"check"})
		if err != nil {
			return nil, err
		}
		switch args["check"] {
		case "test", "build", "vet":
			return args, nil
		default:
			return nil, errors.New("run_go_check check must be test, build, or vet")
		}
	case "run_command":
		args, runtimeArgs, err := decodeRunCommandArguments(raw)
		if err != nil {
			return nil, err
		}
		if _, err := goCommandArgs(args["operation"], args["target"], runtimeArgs...); err != nil {
			return nil, err
		}
		return args, nil
	case "run_shell":
		args, err := decodeToolArgs(raw, []string{"command"}, []string{"command"})
		if err != nil {
			return nil, err
		}
		if err := validateShellCommand(args["command"]); err != nil {
			return nil, err
		}
		return args, nil
	case "delegate_checks":
		if _, err := decodeSwarmTasks(raw); err != nil {
			return nil, err
		}
		return map[string]string{}, nil
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

func toolAction(name, raw string) string {
	if name == "delegate_checks" {
		tasks, err := decodeSwarmTasks(raw)
		if err != nil {
			return "Starting verification swarm"
		}
		return fmt.Sprintf("Spawning %d verification workers", len(tasks))
	}
	if name == "run_command" {
		args, runtimeArgs, err := decodeRunCommandArguments(raw)
		if err != nil {
			return "Calling run_command"
		}
		return "Running " + goCommandSummary(args["operation"], args["target"], runtimeArgs)
	}
	if name == "run_shell" {
		args, err := decodeToolArgs(raw, []string{"command"}, []string{"command"})
		if err != nil {
			return "Calling run_shell"
		}
		command := strings.Join(strings.Fields(args["command"]), " ")
		if len(command) > 120 {
			command = command[:120] + "..."
		}
		return "Running shell in project: " + command
	}
	if name == "read_file" {
		args, err := decodeReadFileArguments(raw)
		if err != nil {
			return "Reading project file"
		}
		if args["start_line"] != "" || args["end_line"] != "" {
			start, end := args["start_line"], args["end_line"]
			if start == "" {
				start = "1"
			}
			if end == "" {
				end = "next 200 lines"
			}
			return "Reading " + cleanPermissionTarget(args["path"]) + " lines " + start + "-" + end
		}
		return "Reading " + cleanPermissionTarget(args["path"])
	}
	if name == "search_files" {
		args, err := decodeSearchArguments(raw)
		if err != nil {
			return "Searching project files"
		}
		return "Searching " + cleanPermissionTarget(args["path"]) + " for " + cleanPermissionTarget(args["query"])
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return "Calling " + name
	}
	switch name {
	case "list_files":
		return "Listing files in " + cleanPermissionTarget(args["path"])
	case "read_file":
		return "Reading " + cleanPermissionTarget(args["path"])
	case "search_files":
		return "Searching project files"
	case "write_file":
		return "Creating " + cleanPermissionTarget(args["path"])
	case "edit_file":
		return "Editing " + cleanPermissionTarget(args["path"])
	case "undo_last_edit":
		return "Undoing the last agent edit"
	case "run_go_check":
		return "Running go " + args["check"] + " ./..."
	default:
		return "Calling " + name
	}
}

func toolOutcome(name, raw, result string) string {
	if strings.HasPrefix(result, "permission denied") {
		return "Permission denied:" + strings.TrimPrefix(result, "permission denied")
	}
	if name == "run_command" {
		if strings.HasPrefix(result, "guardrail:") {
			return "Command skipped: " + strings.TrimPrefix(result, "guardrail:")
		}
		if index := strings.LastIndex(result, "tool error:"); index >= 0 {
			return "Command failed: " + strings.TrimSpace(result[index+len("tool error:"):])
		}
		args, runtimeArgs, err := decodeRunCommandArguments(raw)
		if err != nil {
			return "Command completed"
		}
		return "Command completed: " + goCommandSummary(args["operation"], args["target"], runtimeArgs)
	}
	if name == "run_shell" {
		if index := strings.LastIndex(result, "tool error:"); index >= 0 {
			return "Command failed: " + strings.TrimSpace(result[index+len("tool error:"):])
		}
		return "Command completed: shell"
	}
	if name == "delegate_checks" {
		if strings.HasPrefix(result, "tool error:") {
			return "Verification swarm failed: " + strings.TrimPrefix(result, "tool error:")
		}
		tasks, _ := decodeSwarmTasks(raw)
		failed := strings.Count(result, "\nFAILED:")
		return fmt.Sprintf("Verification swarm finished · %d workers · %d failed", len(tasks), failed)
	}
	if name == "run_go_check" || strings.HasPrefix(result, "tool error:") {
		return result
	}
	var args map[string]string
	if name == "read_file" {
		args, _ = decodeReadFileArguments(raw)
	} else {
		_ = json.Unmarshal([]byte(raw), &args)
	}
	switch name {
	case "list_files":
		count := 0
		for _, line := range strings.Split(result, "\n") {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "[limited to first") {
				count++
			}
		}
		return fmt.Sprintf("Listed %d project files", count)
	case "read_file":
		return fmt.Sprintf("Read %s (%d bytes)", args["path"], len(result))
	case "search_files":
		count := 0
		for _, line := range strings.Split(result, "\n") {
			if strings.Contains(line, ":") && !strings.HasPrefix(line, "[") {
				count++
			}
		}
		return fmt.Sprintf("Found %d code matches", count)
	case "write_file":
		return fmt.Sprintf("%s (%d bytes)", result, len(args["content"]))
	case "edit_file", "undo_last_edit":
		return result
	default:
		return result
	}
}

func permissionForTool(name, raw string) (PermissionRequest, error) {
	switch name {
	case "list_files", "read_file", "search_files":
		var args map[string]string
		var err error
		if name == "read_file" {
			args, err = decodeReadFileArguments(raw)
		} else if name == "search_files" {
			args, err = decodeSearchArguments(raw)
		} else {
			args, err = decodeToolArgs(raw, []string{"path"}, []string{"path"})
		}
		if err != nil {
			return PermissionRequest{}, err
		}
		action, target := "Read project directory", args["path"]
		if name == "read_file" {
			action = "Read project file"
		} else if name == "search_files" {
			action = "Search project files"
			target += " for " + args["query"]
		}
		return PermissionRequest{Action: action, Target: cleanPermissionTarget(target)}, nil
	case "write_file":
		args, err := decodeToolArgs(raw, []string{"path", "content"}, []string{"path", "content"})
		if err != nil {
			return PermissionRequest{}, err
		}
		return PermissionRequest{Action: "Create project file", Target: cleanPermissionTarget(args["path"])}, nil
	case "edit_file":
		args, err := decodeToolArgs(raw, []string{"path", "old_text", "new_text"}, []string{"path", "old_text", "new_text"})
		if err != nil {
			return PermissionRequest{}, err
		}
		target := filepath.ToSlash(args["path"]) + " | - " + truncateUTF8(args["old_text"], 72) + " + " + truncateUTF8(args["new_text"], 72)
		return PermissionRequest{Action: "Review and apply file patch", Target: cleanPermissionTarget(target)}, nil
	case "undo_last_edit":
		if _, err := decodeToolObject(raw, nil); err != nil {
			return PermissionRequest{}, err
		}
		return PermissionRequest{Action: "Undo last agent edit", Target: "Restore the file only if it has not changed since the edit"}, nil
	case "run_command":
		args, runtimeArgs, err := decodeRunCommandArguments(raw)
		if err != nil {
			return PermissionRequest{}, err
		}
		return PermissionRequest{Action: "Run Go command", Target: cleanPermissionTarget(goCommandSummary(args["operation"], args["target"], runtimeArgs))}, nil
	case "run_shell":
		args, err := decodeToolArgs(raw, []string{"command"}, []string{"command"})
		if err != nil {
			return PermissionRequest{}, err
		}
		command := strings.Join(strings.Fields(args["command"]), " ")
		if len(command) > 160 {
			command = command[:160] + "..."
		}
		return PermissionRequest{Action: "Run shell command (not OS-confined to project)", Target: cleanPermissionTarget(command + " | starts in project; may access other paths")}, nil
	case "delegate_checks":
		tasks, err := decodeSwarmTasks(raw)
		if err != nil {
			return PermissionRequest{}, err
		}
		return PermissionRequest{Action: "Spawn verification agents", Target: fmt.Sprintf("%d workers; runs Go checks and localhost HTTP probes: %s", len(tasks), cleanPermissionTarget(strings.Join(tasks, " | ")))}, nil
	default:
		return PermissionRequest{}, fmt.Errorf("unknown tool %q", name)
	}
}

func safePath(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", errors.New("a non-empty relative path is required")
	}
	path := filepath.Join(root, filepath.Clean(name))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path must stay inside the project workspace")
	}
	if isCredentialPath(rel) {
		return "", errors.New("credential files are unavailable to agent tools")
	}
	if isPrivateControlPath(rel) {
		return "", errors.New("private project and tool configuration is unavailable to agent tools")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic links are not allowed in agent tool paths")
		}
	}
	return path, nil
}

func listFiles(root, name string) (string, error) {
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	var result strings.Builder
	count := 0
	entries := 0
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxDirectoryEntries {
			return filepath.SkipAll
		}
		if entry.IsDir() && current != path {
			rel, relErr := filepath.Rel(root, current)
			if skipProjectDirectory(entry.Name()) || (relErr == nil && (isCredentialPath(rel) || isPrivateControlPath(rel))) {
				return filepath.SkipDir
			}
		}
		if !entry.IsDir() {
			rel, err := filepath.Rel(root, current)
			if err != nil || isCredentialPath(rel) || isPrivateControlPath(rel) {
				return nil
			}
			count++
			if count > 1000 {
				return filepath.SkipAll
			}
			fmt.Fprintln(&result, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if count > 1000 {
		result.WriteString("[limited to first 1,000 files]\n")
	}
	if entries > maxDirectoryEntries {
		result.WriteString("[directory listing stopped after 10,000 entries]\n")
	}
	return result.String(), nil
}

func writeFile(root, name, content string) (string, error) {
	return createFile(root, name, content, nil)
}

func isPrivateControlPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch strings.ToLower(part) {
		case ".git", ".projemble":
			return true
		}
	}
	return false
}

func isProtectedWritePath(path string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(path))), "/")
	for index, part := range parts {
		if part == ".codex" || part == ".claude-plugin" || part == ".projemble" || part == ".git" {
			return true
		}
		if (part == ".claude" || part == ".cursor") && index+1 < len(parts) &&
			(strings.HasPrefix(parts[index+1], "settings") || part == ".cursor") {
			return true
		}
		if part == ".github" && index+1 < len(parts) &&
			(parts[index+1] == "workflows" || parts[index+1] == "actions") {
			return true
		}
		if part == "agents.md" || (strings.HasPrefix(part, "agents.") && strings.HasSuffix(part, ".md")) ||
			part == "claude.md" || part == "codex.md" || part == "gemini.md" ||
			part == "copilot-instructions.md" || part == ".windsurfrules" || part == ".cursorrules" || part == "codeowners" {
			return true
		}
		if index+1 < len(parts) && part == ".github" && parts[index+1] == "copilot-instructions.md" {
			return true
		}
		if index+1 < len(parts) && part == "internal" && parts[index+1] == "agent" &&
			strings.HasSuffix(parts[len(parts)-1], ".go") {
			return true
		}
	}
	return false
}

func isCredentialPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for index, part := range parts {
		lower := strings.ToLower(part)
		if lower == ".aws" || lower == ".ssh" || lower == ".gnupg" || lower == ".azure" || lower == ".kube" ||
			strings.HasPrefix(lower, ".env") || lower == ".netrc" || lower == ".git-credentials" ||
			lower == "credentials" || lower == "credentials.json" || lower == "id_rsa" || lower == "id_ed25519" ||
			lower == "id_ecdsa" || lower == "id_dsa" || lower == "authorized_keys" || lower == "known_hosts" ||
			strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") {
			return true
		}
		if lower == ".config" && index+1 < len(parts) && strings.EqualFold(parts[index+1], "gcloud") {
			return true
		}
	}
	return false
}

func safeCommandEnvironment(secretEnvName string) []string {
	var safe []string
	blocked := strings.ToUpper(secretEnvName)
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(name)
		if upper == blocked || isSensitiveEnvironmentName(upper) {
			continue
		}
		safe = append(safe, entry)
	}
	return safe
}

func isSensitiveEnvironmentName(name string) bool {
	if strings.Contains(name, "APIKEY") {
		return true
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' }) {
		switch part {
		case "KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH", "OAUTH", "PAT", "COOKIE", "JWT":
			return true
		}
	}
	return false
}

func safeGoCheckEnvironment(secretEnvName string) []string {
	var safe []string
	for _, entry := range safeCommandEnvironment(secretEnvName) {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOFLAGS":
			continue
		}
		safe = append(safe, entry)
	}
	return append(safe, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly")
}

func safeGoRunEnvironment(secretEnvName string) []string {
	var safe []string
	for _, entry := range safeCommandEnvironment(secretEnvName) {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOFLAGS":
			continue
		}
		safe = append(safe, entry)
	}
	return append(safe, "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly")
}
