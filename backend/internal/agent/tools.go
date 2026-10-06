package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"projemble/internal/llm"
)

const (
	maxFileBytes         = 1 << 20
	maxToolArgumentBytes = 8 << 20
	maxDirectoryEntries  = 10_000
)

func runTool(ctx context.Context, root, name, raw, secretEnvName string) (string, error) {
	switch name {
	case "list_files":
		args, err := decodeToolArgs(raw, []string{"path"}, []string{"path"})
		if err != nil {
			return "", err
		}
		return listFiles(root, args["path"])
	case "read_file":
		args, err := decodeToolArgs(raw, []string{"path"}, []string{"path"})
		if err != nil {
			return "", err
		}
		return readFile(root, args["path"])
	case "write_file":
		args, err := decodeToolArgs(raw, []string{"path", "content"}, []string{"path", "content"})
		if err != nil {
			return "", err
		}
		return writeFile(root, args["path"], args["content"])
	case "run_go_check":
		args, err := decodeToolArgs(raw, []string{"check"}, []string{"check"})
		if err != nil {
			return "", err
		}
		return runGoCheck(ctx, root, args["check"], secretEnvName)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

// decodeToolArgs enforces the same required fields and closed object shape
// advertised in the provider schema. Provider schemas are hints; this is the
// actual execution boundary.
func decodeToolArgs(raw string, required, allowed []string) (map[string]string, error) {
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
	args := make(map[string]string, len(values))
	for field, value := range values {
		if _, ok := allowedFields[field]; !ok {
			return nil, fmt.Errorf("unexpected tool argument %q", field)
		}
		if string(value) == "null" {
			return nil, fmt.Errorf("tool argument %q must be a string", field)
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return nil, fmt.Errorf("tool argument %q must be a string", field)
		}
		args[field] = text
	}
	for _, field := range required {
		value, ok := args[field]
		if !ok || (field != "content" && strings.TrimSpace(value) == "") {
			return nil, fmt.Errorf("required tool argument %q is missing or empty", field)
		}
	}
	return args, nil
}

func validateToolCalls(root string, calls []llm.ToolCall, idOptional bool) error {
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
		if call.Name == "list_files" || call.Name == "read_file" || call.Name == "write_file" {
			if _, err := safePath(root, args["path"]); err != nil {
				return fmt.Errorf("%s path rejected: %w", call.Name, err)
			}
		}
		if call.Name == "write_file" {
			if isProtectedWritePath(args["path"]) {
				return errors.New("project instructions and agent control files are protected from automatic edits")
			}
			if len(args["content"]) > maxFileBytes {
				return fmt.Errorf("write_file content exceeds %d bytes", maxFileBytes)
			}
		}
	}
	return nil
}

func validateArgumentsOnly(name, raw string) (map[string]string, error) {
	switch name {
	case "list_files", "read_file":
		return decodeToolArgs(raw, []string{"path"}, []string{"path"})
	case "write_file":
		return decodeToolArgs(raw, []string{"path", "content"}, []string{"path", "content"})
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
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

func toolAction(name, raw string) string {
	var args map[string]string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return "Calling " + name
	}
	switch name {
	case "list_files":
		return "Listing files in " + args["path"]
	case "read_file":
		return "Reading " + args["path"]
	case "write_file":
		return "Writing " + args["path"]
	case "run_go_check":
		return "Running go " + args["check"] + " ./..."
	default:
		return "Calling " + name
	}
}

func toolOutcome(name, raw, result string) string {
	if name == "run_go_check" || strings.HasPrefix(result, "tool error:") {
		return result
	}
	var args map[string]string
	_ = json.Unmarshal([]byte(raw), &args)
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
	case "write_file":
		return fmt.Sprintf("%s (%d bytes)", result, len(args["content"]))
	default:
		return result
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
		if entry.IsDir() && current != path && (entry.Name() == ".git" || entry.Name() == ".projemble" || entry.Name() == ".codex" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			count++
			if count > 1000 {
				return filepath.SkipAll
			}
			if rel, err := filepath.Rel(root, current); err == nil {
				fmt.Fprintln(&result, filepath.ToSlash(rel))
			}
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

func readFile(root, name string) (string, error) {
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("only regular project files can be read")
	}
	if info.Size() > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	contents, err := os.ReadFile(path)
	return string(contents), err
}

func writeFile(root, name, content string) (string, error) {
	if len(content) > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	if isProtectedWritePath(name) {
		return "", errors.New("project instructions and agent control files are protected from automatic edits")
	}
	_, statErr := os.Stat(path)
	created := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !created {
		return "", statErr
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	// Recheck after creating parents so a raced symlink cannot redirect the write.
	path, err = safePath(root, name)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	verb := "Updated "
	if created {
		verb = "Created "
	}
	return verb + filepath.ToSlash(name), nil
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

func runGoCheck(parent context.Context, root, check, secretEnvName string) (string, error) {
	var args []string
	switch check {
	case "test":
		args = []string{"test", "./..."}
	case "build":
		args = []string{"build", "./..."}
	case "vet":
		args = []string{"vet", "./..."}
	default:
		return "", errors.New("check must be test, build, or vet")
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = root
	command.Env = safeGoCheckEnvironment(secretEnvName)
	var stdout, stderr cappedBuffer
	stdout.limit = maxToolOutput
	stderr.limit = maxToolOutput
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	text := stdout.String() + stderr.String()
	if stdout.truncated || stderr.truncated {
		text += "\n[check output truncated]"
	}
	text = truncate(text, maxToolOutput)
	if ctx.Err() != nil {
		return text, fmt.Errorf("go %s timed out: %w", check, ctx.Err())
	}
	if err != nil {
		return text, fmt.Errorf("go %s failed: %w", check, err)
	}
	if text == "" {
		text = "go " + check + " ./... passed"
	}
	return text, nil
}

type cappedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	remaining := buffer.limit - buffer.Len()
	if len(data) > remaining {
		if remaining > 0 {
			_, _ = buffer.Buffer.Write(data[:remaining])
		}
		buffer.truncated = true
	} else {
		_, _ = buffer.Buffer.Write(data)
	}
	return len(data), nil
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
