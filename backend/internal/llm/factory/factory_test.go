package factory

import (
	"context"
	"testing"

	"projemble/internal/llm"
)

func TestNewSelectsEverySupportedProvider(t *testing.T) {
	providers := []llm.ProviderID{llm.OpenAI, llm.Claude, llm.DeepSeek, llm.Mistral, llm.Qwen, llm.OpenRouter, llm.HuggingFace, llm.Gemini}
	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			created, err := New(llm.Config{Provider: provider, Model: "model", APIKey: "test-key"})
			if err != nil {
				t.Fatal(err)
			}
			if created == nil {
				t.Fatal("factory returned nil provider")
			}
		})
	}
	created, err := New(llm.Config{Provider: llm.OpenAIWeb, Model: "model", Credentials: testToken{}})
	if err != nil || created == nil {
		t.Fatalf("OpenAI web adapter: provider=%v err=%v", created, err)
	}
}

func TestNewValidatesProviderCredentialsAndEndpoints(t *testing.T) {
	if _, err := New(llm.Config{Provider: llm.Claude, Model: "model"}); err == nil {
		t.Fatal("remote provider without key was accepted")
	}
	if _, err := New(llm.Config{Provider: llm.OpenAIWeb, Model: "model"}); err == nil {
		t.Fatal("web auth without credentials was accepted")
	}
	if _, err := New(llm.Config{Provider: "unknown", Model: "model", APIKey: "key"}); err == nil {
		t.Fatal("unknown provider was accepted")
	}
	if _, err := New(llm.Config{Provider: llm.OpenAI, Model: "model", BaseURL: "http://api.example.com/v1", APIKey: "key"}); err == nil {
		t.Fatal("remote plaintext endpoint was accepted")
	}
	if _, err := New(llm.Config{Provider: llm.OpenAI, Model: "model", BaseURL: "http://127.0.0.1:11434/v1"}); err != nil {
		t.Fatalf("local endpoint was rejected: %v", err)
	}
}

type testToken struct{}

func (testToken) AccessToken(context.Context) (string, error) { return "token", nil }
