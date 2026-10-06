package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projemble/internal/llm"
)

func TestRunUsesWorkspaceToolsAndEngineeringPrompt(t *testing.T) {
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
		if request.ToolChoice != "auto" || len(request.Tools) != 4 {
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
		if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "Wrote note.txt") {
			t.Errorf("tool result was not sent back to provider")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Created note.txt and checked the workspace."}}]}`))
	}))
	defer server.Close()

	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "secret-test-key", Client: server.Client()})
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
	got, err := os.ReadFile(filepath.Join(workspace, "note.txt"))
	if err != nil || string(got) != "generated" {
		t.Fatalf("tool did not write file: content=%q err=%v", got, err)
	}
}

func TestSafePathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../outside", filepath.Join(root, "outside"), ""} {
		if _, err := safePath(root, name); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
}

func TestRunGoCheckOnlyAcceptsFixedCommands(t *testing.T) {
	if _, err := runGoCheck(context.Background(), t.TempDir(), "test; rm -rf /", ""); err == nil {
		t.Fatal("accepted arbitrary command")
	}
}

func TestAgentToolsBlockCredentialFilesAndScrubCheckEnvironment(t *testing.T) {
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

func TestCappedBufferBoundsCapturedOutput(t *testing.T) {
	var buffer cappedBuffer
	buffer.limit = 5
	if _, err := buffer.Write([]byte("12345678")); err != nil {
		t.Fatal(err)
	}
	if buffer.String() != "12345" || !buffer.truncated {
		t.Fatalf("output not capped: %q truncated=%v", buffer.String(), buffer.truncated)
	}
}

func TestLocalProviderMayOmitAPIKey(t *testing.T) {
	_, err := New(Config{ProviderID: llm.OpenAI, BaseURL: "http://127.0.0.1:11434/v1", Model: "local-model"})
	if err != nil {
		t.Fatalf("local provider should not require key: %v", err)
	}
	if _, err := New(Config{ProviderID: llm.OpenAI, BaseURL: "https://api.example.com/v1", Model: "model"}); err == nil {
		t.Fatal("remote provider without a key was accepted")
	}
	if _, err := New(Config{ProviderID: llm.OpenAI, BaseURL: "http://api.example.com/v1", Model: "model", APIKey: "key"}); err == nil {
		t.Fatal("remote provider over plaintext HTTP was accepted")
	}
}
