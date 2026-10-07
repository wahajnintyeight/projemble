package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projemble/internal/llm"
)

type permissionApproverFunc func(context.Context, PermissionRequest) (bool, error)

func (approver permissionApproverFunc) RequestPermission(ctx context.Context, request PermissionRequest) (bool, error) {
	return approver(ctx, request)
}

func TestRunUsesWorkspaceToolsAndEngineeringPrompt(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if got := r.Header.Get("Authorization"); got != "Bearer secret-test-key" {
			t.Errorf("authorization = %q", got)
		}
		var request struct {
			ToolChoice string `json:"tool_choice"`
			Messages   []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if request.ToolChoice != "auto" || len(request.Tools) != 6 {
			t.Errorf("tools not enabled: mode=%q count=%d", request.ToolChoice, len(request.Tools))
		}
		if requestCount == 1 {
			if !strings.Contains(request.Messages[0].Content, "Never overwrite user data outside the workspace") {
				t.Errorf("engineering prompt was not included")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"note.txt\",\"content\":\"generated\"}"}}]}}]}`))
			return
		}
		if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "Created note.txt") {
			t.Errorf("tool result was not sent back to provider")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Created note.txt and checked the workspace."}}]}`))
	}))
	defer server.Close()

	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "secret-test-key", AccessMode: AccessFull, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := client.Run(context.Background(), workspace, "Create a note", &output); err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 || !strings.Contains(output.String(), "checked the workspace") {
		t.Fatalf("provider loop incomplete: requests=%d output=%q", requestCount, output.String())
	}
	for _, activity := range []string{"Waiting on openai model test-model", "Writing note.txt", "Created note.txt", "Agent summary:"} {
		if !strings.Contains(output.String(), activity) {
			t.Errorf("activity output missing %q:\n%s", activity, output.String())
		}
	}
	if strings.Contains(output.String(), "secret-test-key") {
		t.Fatal("agent activity output exposed the provider API key")
	}
	got, err := os.ReadFile(filepath.Join(workspace, "note.txt"))
	if err != nil || string(got) != "generated" {
		t.Fatalf("tool did not write file: content=%q err=%v", got, err)
	}
}

func TestWriteFileDistinguishesCreatedFromUpdated(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if result, err := writeFile(root, "new.txt", "first"); err != nil || result != "Created new.txt" {
		t.Fatalf("new file result = %q, err=%v", result, err)
	}
	if result, err := writeFile(root, "new.txt", "second"); err != nil || result != "Updated new.txt" {
		t.Fatalf("updated file result = %q, err=%v", result, err)
	}
}

func TestRunReportsGoCheckProgress(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/progress\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"check-1","type":"function","function":{"name":"run_command","arguments":"{\"operation\":\"build\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Build passed."}}]}`))
	}))
	defer server.Close()
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessFull, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := client.Run(context.Background(), workspace, "Build the project", &output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Action: Running go build ./...", "Command completed: go build ./...", "Agent summary: Build passed."} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("activity output missing %q:\n%s", want, output.String())
		}
	}
}

func TestAgentRunsGoAppWithSeparateRuntimeArguments(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/job\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobDir := filepath.Join(workspace, "cmd", "job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package main\nimport (\"fmt\"; \"os\")\nfunc main() { fmt.Println(os.Args[1]) }\n"
	if err := os.WriteFile(filepath.Join(jobDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"run-1","type":"function","function":{"name":"run_command","arguments":"{\"operation\":\"run\",\"target\":\"./cmd/job\",\"args\":[\"sample-input\"]}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"The job ran successfully."}}]}`))
	}))
	defer server.Close()
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessFull, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := client.Run(context.Background(), workspace, "Run the job with sample-input", &output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Action: Running go run ./cmd/job (1 runtime argument)",
		"STDOUT sample-input",
		"Command completed: go run ./cmd/job (1 runtime argument)",
		"Agent summary: The job ran successfully.",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("activity output missing %q:\n%s", want, output.String())
		}
	}
}

func TestTurnKeepsConversationForFollowupPrompts(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if requestCount == 2 {
			foundPreviousReply := false
			for _, message := range request.Messages {
				if message.Role == "assistant" && message.Content == "The starter is ready." {
					foundPreviousReply = true
				}
			}
			if !foundPreviousReply || request.Messages[len(request.Messages)-1].Content != "Now add a health endpoint." {
				t.Errorf("follow-up did not include conversation history: %+v", request.Messages)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"The starter is ready."}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Added the health endpoint."}}]}`))
	}))
	defer server.Close()
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessFull, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Run(context.Background(), workspace, "Create the starter", nil); err != nil {
		t.Fatal(err)
	}
	if err := client.Turn(context.Background(), workspace, "Now add a health endpoint.", nil); err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 {
		t.Fatalf("provider calls = %d, want 2", requestCount)
	}
}

func TestRunAllowsMoreThanSixteenToolRoundsAndFinalResponse(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount <= 18 {
			_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-%d","type":"function","function":{"name":"list_files","arguments":"{\"path\":\".\"}"}}]}}]}`, requestCount)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Finished after inspecting the project."}}]}`))
	}))
	defer server.Close()

	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessFull, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := client.Run(context.Background(), workspace, "Inspect the project", &output); err != nil {
		t.Fatal(err)
	}
	if requestCount != 19 || !strings.Contains(output.String(), "Finished after inspecting") {
		t.Fatalf("agent did not complete after tool rounds: requests=%d output=%q", requestCount, output.String())
	}
}

func TestSafePathRejectsEscapes(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	for _, name := range []string{"../outside", filepath.Join(root, "outside"), ""} {
		if _, err := safePath(root, name); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
}

func TestRunGoCheckOnlyAcceptsFixedCommands(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := runGoCheck(context.Background(), t.TempDir(), "test; rm -rf /", "", "", nil); err == nil {
		t.Fatal("accepted arbitrary command")
	}
}

func TestAgentToolsBlockCredentialFilesAndScrubCheckEnvironment(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	for _, name := range []string{".env", ".env.local", ".aws/credentials", "private.pem"} {
		if _, err := safePath(root, name); err == nil {
			t.Errorf("credential path accepted: %s", name)
		}
	}
	t.Setenv("PROJEMBLE_TEST_API_KEY", "secret")
	t.Setenv("PROJEMBLE_TEST_PASSWORD", "secret")
	t.Setenv("PROJEMBLE_TEST_VALUE", "available")
	env := strings.Join(safeCommandEnvironment("PROJEMBLE_TEST_API_KEY"), "\n")
	if strings.Contains(env, "PROJEMBLE_TEST_API_KEY=") || strings.Contains(env, "PROJEMBLE_TEST_PASSWORD=") {
		t.Fatal("credential environment reached project checks")
	}
	if !strings.Contains(env, "PROJEMBLE_TEST_VALUE=available") {
		t.Fatal("non-secret environment was unexpectedly removed")
	}
}

func TestLocalProviderMayOmitAPIKey(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: "http://127.0.0.1:11434/v1", Model: "local-model"})
	if err != nil {
		t.Fatalf("local provider should not require key: %v", err)
	}
	if client.config.AccessMode != AccessAskAlways {
		t.Fatalf("default access mode = %q, want ask-always", client.config.AccessMode)
	}
	if _, err := New(Config{ProviderID: llm.OpenAI, BaseURL: "https://api.example.com/v1", Model: "model"}); err == nil {
		t.Fatal("remote provider without a key was accepted")
	}
	if _, err := New(Config{ProviderID: llm.OpenAI, BaseURL: "http://api.example.com/v1", Model: "model", APIKey: "key"}); err == nil {
		t.Fatal("remote provider over plaintext HTTP was accepted")
	}
}

func TestReadOnlyModeHidesAndRejectsWriteTools(t *testing.T) {
	root := t.TempDir()
	tools := llmToolsForAccess(AccessReadOnly)
	if len(tools) != 2 || tools[0].Name != "list_files" || tools[1].Name != "read_file" {
		t.Fatalf("read-only tools = %+v", tools)
	}
	write := []llm.ToolCall{{ID: "write", Name: "write_file", Arguments: `{"path":"note.txt","content":"no"}`}}
	if err := validateToolCallsForAccess(root, write, false, AccessReadOnly); err == nil {
		t.Fatal("read-only mode accepted a write")
	}
	if err := validateToolCallsForAccess(root, write, false, AccessFull); err != nil {
		t.Fatalf("full access rejected a project write: %v", err)
	}
	delegate := []llm.ToolCall{{ID: "delegate", Name: "delegate_checks", Arguments: `{"tasks":["test APIs"]}`}}
	if err := validateToolCallsForAccess(root, delegate, false, AccessReadOnly); err == nil {
		t.Fatal("read-only mode accepted worker execution")
	}
	if got := len(llmToolsForAccess(AccessAskAlways)); got != 6 {
		t.Fatalf("ask-always tools = %d, want read/write/Go/shell/swarm operations", got)
	}
}

func TestFullAccessStillApprovesUnconfinedShell(t *testing.T) {
	for _, test := range []struct {
		mode AccessMode
		tool string
		want bool
	}{
		{AccessFull, "run_shell", true},
		{AccessAskAlways, "delegate_checks", true},
		{AccessFull, "delegate_checks", false},
		{AccessFull, "run_command", false},
		{AccessAskAlways, "read_file", true},
		{AccessReadOnly, "read_file", false},
	} {
		if got := needsApproval(test.mode, test.tool); got != test.want {
			t.Errorf("needsApproval(%q, %q) = %v, want %v", test.mode, test.tool, got, test.want)
		}
	}
}

func TestAskAlwaysPromptsBeforeReadingProjectFile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "note.txt"), []byte("project note"), 0o600); err != nil {
		t.Fatal(err)
	}
	requestCount, approvals := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"read-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"note.txt\"}"}}]}}]}`))
			return
		}
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode follow-up request: %v", err)
		}
		if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "project note") {
			t.Errorf("approved read result was not returned to model: %+v", request.Messages)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Read the note."}}]}`))
	}))
	defer server.Close()
	approver := permissionApproverFunc(func(_ context.Context, request PermissionRequest) (bool, error) {
		approvals++
		if request.Action != "Read project file" || request.Target != "note.txt" {
			t.Errorf("permission prompt = %+v", request)
		}
		return true, nil
	})
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessAskAlways, Approver: approver, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Run(context.Background(), workspace, "Summarize note.txt", nil); err != nil {
		t.Fatal(err)
	}
	if approvals != 1 || requestCount != 2 {
		t.Fatalf("approval count=%d provider requests=%d, want one approval and two requests", approvals, requestCount)
	}
}

func TestFullAccessPromptsBeforeUnconfinedShell(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	requests, approvals := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"shell-1","type":"function","function":{"name":"run_shell","arguments":"{\"command\":\"echo safe-check\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Command ran after approval."}}]}`))
	}))
	defer server.Close()
	approver := permissionApproverFunc(func(_ context.Context, request PermissionRequest) (bool, error) {
		approvals++
		if request.Action != "Run shell command" || request.Target != "echo safe-check" {
			t.Errorf("shell approval = %+v", request)
		}
		return true, nil
	})
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessFull, Approver: approver, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := client.Run(context.Background(), t.TempDir(), "run an approved check", &output); err != nil {
		t.Fatal(err)
	}
	if approvals != 1 || requests != 2 || !strings.Contains(output.String(), "safe-check") {
		t.Fatalf("shell approvals=%d requests=%d output=%q", approvals, requests, output.String())
	}
}

func TestPermissionTargetStripsControlsAndBoundsLength(t *testing.T) {
	got := cleanPermissionTarget("run\n\x1b[31m this " + strings.Repeat("x", 250))
	if strings.ContainsAny(got, "\n\r\x1b") || len([]rune(got)) > 243 {
		t.Fatalf("unsafe permission target = %q", got)
	}
}
