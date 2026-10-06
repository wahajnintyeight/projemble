package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxFileBytes = 1 << 20

func runTool(ctx context.Context, root, name, raw, secretEnvName string) (string, error) {
	var args map[string]string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	switch name {
	case "list_files":
		return listFiles(root, args["path"])
	case "read_file":
		return readFile(root, args["path"])
	case "write_file":
		return writeFile(root, args["path"], args["content"])
	case "run_go_check":
		return runGoCheck(ctx, root, args["check"], secretEnvName)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
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
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && current != path && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
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
	return "Wrote " + filepath.ToSlash(name), nil
}

func isCredentialPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		lower := strings.ToLower(part)
		if lower == ".aws" || lower == ".ssh" || strings.HasPrefix(lower, ".env") ||
			lower == "credentials" || lower == "id_rsa" || lower == "id_ed25519" ||
			strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") {
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
	command.Env = safeCommandEnvironment(secretEnvName)
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
		if upper == blocked || strings.Contains(upper, "API_KEY") || strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "CREDENTIAL") {
			continue
		}
		safe = append(safe, entry)
	}
	return safe
}
