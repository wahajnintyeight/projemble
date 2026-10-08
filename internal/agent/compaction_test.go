package agent

import (
	"context"
	"strings"
	"testing"

	"projemble/internal/llm"
)

type compactionProvider struct {
	request  llm.Request
	response llm.Response
}

func (provider *compactionProvider) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	provider.request = request
	return provider.response, nil
}

func TestCompactReplacesMemoryWithoutToolsAndSavesUsage(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	provider := &compactionProvider{response: llm.Response{
		Message: llm.Message{Role: "assistant", Content: "Goal: finish the API\nNext steps: add auth"},
		Usage:   llm.Usage{Available: true, InputTokens: 40, OutputTokens: 12, TotalTokens: 52},
	}}
	workspace := t.TempDir()
	client := &Agent{provider: provider, workspace: workspace, config: Config{Model: "test-model"}, summary: "User request: build the API\nTool result: tests passed", usage: llm.Usage{Available: true, TotalTokens: 3}}
	var output strings.Builder
	if err := client.Compact(context.Background(), "focus on remaining work", &output); err != nil {
		t.Fatal(err)
	}
	if len(provider.request.Tools) != 0 || !strings.Contains(provider.request.Messages[1].Content, "tests passed") || !strings.Contains(provider.request.Messages[0].Content, "focus on remaining work") {
		t.Fatalf("summary request was not scoped correctly: %+v", provider.request)
	}
	if client.summary != provider.response.Message.Content || client.Usage().TotalTokens != 55 || !strings.Contains(output.String(), "Context compacted") {
		t.Fatalf("compact state: summary=%q usage=%+v output=%q", client.summary, client.Usage(), output.String())
	}
}

func TestAutomaticCompactionKeepsCurrentRequest(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	provider := &compactionProvider{response: llm.Response{Message: llm.Message{Role: "assistant", Content: "Current task: continue the API"}}}
	client := &Agent{provider: provider, workspace: t.TempDir(), config: Config{Model: "test-model"}, summary: strings.Repeat("context ", 2_000), messages: []llm.Message{
		{Role: "system", Content: "old system"}, {Role: "user", Content: "Continue the API without changing its routes"},
	}}
	client.compactIfNeeded(context.Background(), nil)
	if client.summary != "Current task: continue the API" || len(client.messages) != 2 || client.messages[1].Content != "Continue the API without changing its routes" {
		t.Fatalf("automatic compaction lost active request: summary=%q messages=%+v", client.summary, client.messages)
	}
}
