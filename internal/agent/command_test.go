package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestCommandOutputStreamsLinesAndCapsResult(t *testing.T) {
	var progress strings.Builder
	output := &commandOutput{progress: &progress, remaining: 7}
	stdout, stderr := output.stream("STDOUT"), output.stream("STDERR")
	if _, err := stdout.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(progress.String(), "STDOUT hello") {
		t.Fatalf("complete line was not streamed immediately: %q", progress.String())
	}
	if _, err := stderr.Write([]byte("error")); err != nil {
		t.Fatal(err)
	}
	stderr.Flush()
	if got := output.result(); !strings.Contains(got, "[command output truncated]") || len(strings.TrimSuffix(got, "\n[command output truncated]")) > 7 {
		t.Fatalf("command output was not capped: %q", got)
	}
	if !strings.Contains(progress.String(), "STDERR") || !strings.Contains(progress.String(), "command output truncated") {
		t.Fatalf("stderr or truncation status was not streamed: %q", progress.String())
	}
}

func TestCommandOutputHandlesConcurrentStreams(t *testing.T) {
	var progress strings.Builder
	output := &commandOutput{progress: &progress, remaining: 16 << 10}
	stdout, stderr := output.stream("STDOUT"), output.stream("STDERR")
	var writers sync.WaitGroup
	for _, stream := range []*commandOutputStream{stdout, stderr} {
		writers.Add(1)
		go func(stream *commandOutputStream) {
			defer writers.Done()
			for index := range 50 {
				_, _ = fmt.Fprintf(stream, "line-%d\n", index)
			}
		}(stream)
	}
	writers.Wait()
	stdout.Flush()
	stderr.Flush()
	if got := strings.Count(progress.String(), "STDOUT line-"); got != 50 {
		t.Errorf("streamed %d stdout lines, want 50", got)
	}
	if got := strings.Count(progress.String(), "STDERR line-"); got != 50 {
		t.Errorf("streamed %d stderr lines, want 50", got)
	}
}

func TestGoCommandArgumentsStayWithinWorkspace(t *testing.T) {
	for _, target := range []string{"../outside", "./internal/../../outside", `C:\outside`, "/outside", "./../outside"} {
		if _, err := goCommandArgs("test", target); err == nil {
			t.Errorf("accepted outside target %q", target)
		}
	}
	for _, target := range []string{"./...", "./internal/agent", "."} {
		if _, err := goCommandArgs("test", target); err != nil {
			t.Errorf("rejected project target %q: %v", target, err)
		}
	}
	if _, err := goCommandArgs("test; rm -rf /", "./..."); err == nil {
		t.Fatal("accepted a non-allowlisted command")
	}
}

func TestGoRunUsesValidatedPackageAndSeparateRuntimeArguments(t *testing.T) {
	args, err := goCommandArgs("run", "./cmd/job", "https://example.com", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(args, " "), "run ./cmd/job https://example.com --json"; got != want {
		t.Fatalf("Go run args = %q, want %q", got, want)
	}
	for _, target := range []string{"", "./...", "../outside", `C:\outside`} {
		if _, err := goCommandArgs("run", target); err == nil {
			t.Errorf("accepted unsafe or missing Go run target %q", target)
		}
	}
	if _, err := goCommandArgs("test", "./...", "runtime input"); err == nil {
		t.Fatal("accepted runtime arguments for a check operation")
	}
	if _, err := goCommandArgs("run", "./cmd/job", "bad\x00arg"); err == nil {
		t.Fatal("accepted a NUL byte in a runtime argument")
	}
}

func TestRunCommandArgumentsAreTypedAndClosed(t *testing.T) {
	raw := `{"operation":"run","target":"./cmd/job","args":["https://example.com","--json"]}`
	args, runtimeArgs, err := decodeRunCommandArguments(raw)
	if err != nil {
		t.Fatal(err)
	}
	if args["operation"] != "run" || args["target"] != "./cmd/job" || strings.Join(runtimeArgs, " ") != "https://example.com --json" {
		t.Fatalf("decoded command = %#v with args %#v", args, runtimeArgs)
	}
	for _, invalid := range []string{
		`{"operation":"run","target":"./cmd/job","args":"https://example.com"}`,
		`{"operation":"run","target":"./cmd/job","args":[1]}`,
		`{"operation":"run","target":"./cmd/job","args":["bad\u0000arg"]}`,
		`{"operation":"run","target":"./..."}`,
		`{"operation":"run","target":"../outside"}`,
		`{"operation":"run","target":"./cmd/job","shell":"go run"}`,
	} {
		if _, err := validateArgumentsOnly("run_command", invalid); err == nil {
			t.Errorf("accepted invalid run_command arguments %s", invalid)
		}
	}
}

func TestRunCommandSchemaExposesRunAndArgumentArray(t *testing.T) {
	for _, tool := range llmTools() {
		if tool.Name != "run_command" {
			continue
		}
		properties := tool.Parameters["properties"].(map[string]any)
		args := properties["args"].(map[string]any)
		items := args["items"].(map[string]any)
		if args["type"] != "array" || items["type"] != "string" {
			t.Fatalf("runtime args schema = %#v", args)
		}
		operations := properties["operation"].(map[string]any)["enum"].([]string)
		for _, operation := range operations {
			if operation == "run" {
				return
			}
		}
		t.Fatal("run operation is missing from tool schema")
	}
	t.Fatal("run_command tool is missing")
}

func TestRunProjectCommandUsesWorkspaceAndStreamsProcessOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/streamed\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress strings.Builder
	result, err := runProjectCommand(context.Background(), root, "list", "./...", "", "", &progress)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "example.com/streamed") || !strings.Contains(progress.String(), "STDOUT example.com/streamed") {
		t.Fatalf("workspace output was not returned and streamed: result=%q progress=%q", result, progress.String())
	}
}

func TestRunProjectCommandRunsAppWithArgumentsAndStreamsOutput(t *testing.T) {
	t.Setenv("SCRAPER_API_KEY", "must-not-reach-project-app")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/job\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobDir := filepath.Join(root, "cmd", "job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package main\nimport (\"fmt\"; \"os\")\nfunc main() { fmt.Printf(\"%s|%s\\n\", os.Args[1], os.Getenv(\"SCRAPER_API_KEY\")) }\n"
	if err := os.WriteFile(filepath.Join(jobDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress strings.Builder
	result, err := runProjectCommand(context.Background(), root, "run", "./cmd/job", "", "", &progress, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "https://example.com|") || strings.Contains(result, "must-not-reach-project-app") ||
		!strings.Contains(progress.String(), "STDOUT https://example.com|") {
		t.Fatalf("runtime output was not returned and streamed: result=%q progress=%q", result, progress.String())
	}
	raw := `{"operation":"run","target":"./cmd/job","args":["https://example.com"]}`
	if got, want := toolAction("run_command", raw), "Running go run ./cmd/job (1 runtime argument)"; got != want {
		t.Fatalf("run activity = %q, want %q", got, want)
	}
	if got, want := toolOutcome("run_command", raw, result), "Command completed: go run ./cmd/job (1 runtime argument)"; got != want {
		t.Fatalf("run outcome = %q, want %q", got, want)
	}
}

func TestRunShellStartsInProjectAndStreamsOutput(t *testing.T) {
	root := t.TempDir()
	command := "printf 'shell-ok\\n'; pwd"
	if runtime.GOOS == "windows" {
		command = "Write-Output 'shell-ok'; Get-Location"
	}
	var progress strings.Builder
	result, err := runShellCommand(context.Background(), root, command, "", "", &progress)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "shell-ok") || !strings.Contains(result, root) || !strings.Contains(progress.String(), "STDOUT shell-ok") {
		t.Fatalf("shell did not run from the project or stream output: result=%q progress=%q", result, progress.String())
	}
}

func TestCommandOutputRedactsSecretsBeforeStreaming(t *testing.T) {
	var progress strings.Builder
	output := &commandOutput{progress: &progress, remaining: maxToolOutput, secret: "test-secret"}
	stdout := output.stream("STDOUT")
	_, _ = stdout.Write([]byte("token=test-secret\n"))
	if strings.Contains(progress.String(), "test-secret") || !strings.Contains(progress.String(), "[REDACTED]") {
		t.Fatalf("secret reached live output: %q", progress.String())
	}
}
