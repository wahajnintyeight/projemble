package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"projemble/internal/llm"
)

func TestOpenAICompatibleMapsMessagesAndTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var request struct {
			Model      string            `json:"model"`
			ToolChoice string            `json:"tool_choice"`
			Tools      []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Model != "model-x" || request.ToolChoice != "auto" || len(request.Tools) != 1 {
			t.Errorf("request mapping = %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"a.txt\"}"}}]}}]}`)
	}))
	defer server.Close()
	provider := NewOpenAICompatible(llm.Config{Provider: llm.OpenAI, Model: "model-x", APIKey: "test-key", BaseURL: server.URL + "/v1", Client: server.Client()})
	response, err := provider.Complete(context.Background(), llm.Request{Model: "model-x", Messages: []llm.Message{{Role: "user", Content: "hello"}}, Tools: []llm.Tool{{Name: "write_file", Parameters: map[string]any{"type": "object"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].Name != "write_file" {
		t.Fatalf("tool calls = %+v", response.Message.ToolCalls)
	}
}

func TestAnthropicMapsToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "anthropic-key" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("request headers/path: %s %+v", r.URL.Path, r.Header)
		}
		var request struct {
			System    string `json:"system"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.System != "system prompt" || request.MaxTokens == 0 {
			t.Errorf("request mapping = %+v", request)
		}
		_, _ = fmt.Fprint(w, `{"content":[{"type":"tool_use","id":"tool-1","name":"list_files","input":{"path":"."}}]}`)
	}))
	defer server.Close()
	provider := NewAnthropic(llm.Config{Provider: llm.Claude, Model: "claude-model", APIKey: "anthropic-key", BaseURL: server.URL + "/v1", Client: server.Client()})
	response, err := provider.Complete(context.Background(), llm.Request{Model: "claude-model", Messages: []llm.Message{{Role: "system", Content: "system prompt"}}, Tools: []llm.Tool{{Name: "list_files", Parameters: map[string]any{"type": "object"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].ID != "tool-1" {
		t.Fatalf("tool calls = %+v", response.Message.ToolCalls)
	}
}

func TestGeminiMapsFunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" || r.Header.Get("x-goog-api-key") != "gemini-key" {
			t.Errorf("path/header = %s %q", r.URL.Path, r.Header.Get("x-goog-api-key"))
		}
		_, _ = fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"read_file","id":"fc-1","args":{"path":"main.go"}}}]}}]}`)
	}))
	defer server.Close()
	provider := NewGemini(llm.Config{Provider: llm.Gemini, Model: "gemini-test", APIKey: "gemini-key", BaseURL: server.URL + "/v1beta", Client: server.Client()})
	response, err := provider.Complete(context.Background(), llm.Request{Model: "gemini-test", Messages: []llm.Message{{Role: "user", Content: "inspect"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Message.ToolCalls) != 1 || !strings.Contains(response.Message.ToolCalls[0].Arguments, "main.go") {
		t.Fatalf("tool calls = %+v", response.Message.ToolCalls)
	}
}

func TestOpenAIResponsesUsesOAuthAndAssemblesStreamedToolArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer chatgpt-token" {
			t.Errorf("path/auth = %s %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request["store"] != false || request["stream"] != true {
			t.Errorf("privacy/stream flags = %+v", request)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		events := []map[string]any{
			{"type": "response.output_item.added", "item": map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "write_file", "arguments": ""}},
			{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "delta": `{"path":"`},
			{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "delta": `a.txt"}`},
		}
		for _, event := range events {
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		}
	}))
	defer server.Close()
	provider := NewOpenAIResponses(llm.Config{Provider: llm.OpenAIWeb, Model: "gpt-test", BaseURL: server.URL + "/v1", Credentials: staticToken("chatgpt-token"), Client: server.Client()})
	response, err := provider.Complete(context.Background(), llm.Request{Model: "gpt-test", Messages: []llm.Message{{Role: "system", Content: "rules"}, {Role: "user", Content: "write"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].ID != "call_1" || response.Message.ToolCalls[0].Arguments != `{"path":"a.txt"}` {
		t.Fatalf("streamed tool call = %+v", response.Message.ToolCalls)
	}
}

type staticToken string

func (token staticToken) AccessToken(context.Context) (string, error) { return string(token), nil }
