package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	commandTimeout      = 3 * time.Minute
	maxRuntimeArgs      = 32
	maxRuntimeArgBytes  = 16 << 10
	maxShellCommand     = 16 << 10
	maxShellRunsPerTurn = 3
)

func runGoCheck(ctx context.Context, root, check, secretEnvName, secret string, output io.Writer) (string, error) {
	return runProjectCommand(ctx, root, check, "./...", secretEnvName, secret, output)
}

func runProjectCommand(parent context.Context, root, operation, target, secretEnvName, secret string, output io.Writer, runtimeArgs ...string) (string, error) {
	args, err := goCommandArgs(operation, target, runtimeArgs...)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = root
	command.Env = safeGoCheckEnvironment(secretEnvName)
	if operation == "run" {
		command.Env = safeGoRunEnvironment(secretEnvName)
	}
	activity := &commandOutput{progress: output, remaining: maxToolOutput - 512, secret: secret}
	stdout, stderr := activity.stream("STDOUT"), activity.stream("STDERR")
	command.Stdout, command.Stderr = stdout, stderr
	err = command.Run()
	stdout.Flush()
	stderr.Flush()
	result := activity.result()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("go %s timed out after %s", operation, commandTimeout)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return result, fmt.Errorf("go %s was cancelled", operation)
	}
	if err != nil {
		return result, fmt.Errorf("go %s failed: %w", operation, err)
	}
	if result == "" {
		result = goCommandLabel(operation, target) + " completed"
	}
	return result, nil
}

func runShellCommand(parent context.Context, root, commandText, secretEnvName, secret string, output io.Writer) (string, error) {
	if err := validateShellCommand(commandText); err != nil {
		return "", err
	}
	commandText = strings.TrimSpace(commandText)
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", commandText)
	} else {
		command = exec.CommandContext(ctx, "/bin/sh", "-c", commandText)
	}
	command.Dir = root
	command.Env = safeCommandEnvironment(secretEnvName)
	activity := &commandOutput{progress: output, remaining: maxToolOutput - 512, secret: secret}
	stdout, stderr := activity.stream("STDOUT"), activity.stream("STDERR")
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	stdout.Flush()
	stderr.Flush()
	result := activity.result()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("shell command timed out after %s", commandTimeout)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return result, errors.New("shell command was cancelled")
	}
	if err != nil {
		return result, fmt.Errorf("shell command failed: %w", err)
	}
	if result == "" {
		result = "shell command completed"
	}
	return result, nil
}

func goCommandArgs(operation, target string, runtimeArgs ...string) ([]string, error) {
	if operation == "run" && target == "" {
		return nil, errors.New("Go run requires a package target inside the project (for example ./cmd/job)")
	}
	if operation != "run" && target == "" {
		target = "./..."
	}
	if !validGoTarget(target) {
		return nil, errors.New("Go target must be a relative package path inside the project (for example ./... or ./internal/agent)")
	}
	if operation == "run" && target == "./..." {
		return nil, errors.New("Go run requires one package target, not ./...")
	}
	if operation != "run" && len(runtimeArgs) > 0 {
		return nil, errors.New("runtime arguments are only supported for Go run")
	}
	if err := validateRuntimeArgs(runtimeArgs); err != nil {
		return nil, err
	}
	switch operation {
	case "run":
		return append([]string{"run", filepath.ToSlash(target)}, runtimeArgs...), nil
	case "test":
		return []string{"test", "-v", filepath.ToSlash(target)}, nil
	case "fmt", "list", "build", "vet":
		return []string{operation, filepath.ToSlash(target)}, nil
	default:
		return nil, errors.New("operation must be run, fmt, list, test, build, or vet")
	}
}

func validateRuntimeArgs(args []string) error {
	if len(args) > maxRuntimeArgs {
		return fmt.Errorf("Go run supports at most %d runtime arguments", maxRuntimeArgs)
	}
	total := 0
	for _, arg := range args {
		if strings.ContainsRune(arg, '\x00') {
			return errors.New("Go run arguments cannot contain a NUL byte")
		}
		total += len(arg)
		if total > maxRuntimeArgBytes {
			return fmt.Errorf("Go run arguments exceed %d bytes total", maxRuntimeArgBytes)
		}
	}
	return nil
}

func validateShellCommand(command string) error {
	if strings.TrimSpace(command) == "" {
		return errors.New("shell command is required")
	}
	if len(command) > maxShellCommand {
		return fmt.Errorf("shell command exceeds %d bytes", maxShellCommand)
	}
	if strings.ContainsRune(command, '\x00') {
		return errors.New("shell command cannot contain a NUL byte")
	}
	return nil
}

func validGoTarget(target string) bool {
	if target == "./..." || target == "." {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(target), "/") {
		if part == ".." {
			return false
		}
	}
	target = filepath.ToSlash(target)
	if !strings.HasPrefix(target, "./") || strings.ContainsAny(target, "*?[]:") {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(target, "./"), "/") {
		if part == "" || part == "." || part == ".." || part == "..." {
			return false
		}
	}
	return true
}

func goCommandLabel(operation, target string) string {
	if target == "" {
		target = "./..."
	}
	if operation == "test" {
		return "go test -v " + filepath.ToSlash(target)
	}
	return "go " + operation + " " + filepath.ToSlash(target)
}

func goCommandSummary(operation, target string, runtimeArgs []string) string {
	label := goCommandLabel(operation, target)
	if len(runtimeArgs) == 1 {
		return label + " (1 runtime argument)"
	}
	if len(runtimeArgs) > 1 {
		return fmt.Sprintf("%s (%d runtime arguments)", label, len(runtimeArgs))
	}
	return label
}

type commandOutput struct {
	mu        sync.Mutex
	progress  io.Writer
	captured  bytes.Buffer
	remaining int
	truncated bool
	notified  bool
	secret    string
}

type commandOutputStream struct {
	output *commandOutput
	label  string
	line   []byte
}

func (output *commandOutput) stream(label string) *commandOutputStream {
	return &commandOutputStream{output: output, label: label}
}

func (stream *commandOutputStream) Write(data []byte) (int, error) {
	output := stream.output
	output.mu.Lock()
	defer output.mu.Unlock()

	accepted := len(data)
	if accepted > output.remaining {
		accepted = output.remaining
		output.truncated = true
	}
	if accepted > 0 {
		output.remaining -= accepted
		_, _ = output.captured.Write(data[:accepted])
		for _, char := range data[:accepted] {
			if char == '\r' || char == '\n' {
				stream.flushLineLocked()
				continue
			}
			stream.line = append(stream.line, char)
		}
	}
	if output.truncated && !output.notified {
		stream.flushLineLocked()
		output.emitLocked("STDOUT", "[command output truncated]")
		output.notified = true
	}
	return len(data), nil
}

func (stream *commandOutputStream) Flush() {
	stream.output.mu.Lock()
	defer stream.output.mu.Unlock()
	stream.flushLineLocked()
}

func (stream *commandOutputStream) flushLineLocked() {
	if len(stream.line) == 0 {
		return
	}
	line := strings.ToValidUTF8(string(stream.line), "�")
	if stream.output.secret != "" {
		line = strings.ReplaceAll(line, stream.output.secret, "[REDACTED]")
	}
	stream.output.emitLocked(stream.label, line)
	stream.line = stream.line[:0]
}

func (output *commandOutput) emitLocked(label, line string) {
	if output.progress != nil {
		_, _ = fmt.Fprintf(output.progress, "%s %s\n", label, line)
	}
}

func (output *commandOutput) result() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	result := output.captured.String()
	if output.truncated {
		result += "\n[command output truncated]"
	}
	return result
}
