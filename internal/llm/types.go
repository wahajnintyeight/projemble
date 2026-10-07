package llm

import (
	"context"
	"net/http"
)

type ProviderID string

type ReasoningEffort string

const (
	ReasoningDefault ReasoningEffort = ""
	ReasoningLow     ReasoningEffort = "low"
	ReasoningMedium  ReasoningEffort = "medium"
	ReasoningHigh    ReasoningEffort = "high"
)

const (
	OpenAI      ProviderID = "openai"
	Claude      ProviderID = "claude"
	DeepSeek    ProviderID = "deepseek"
	Mistral     ProviderID = "mistral"
	Qwen        ProviderID = "qwen"
	OpenRouter  ProviderID = "openrouter"
	HuggingFace ProviderID = "huggingface"
	Gemini      ProviderID = "gemini"
	OpenAIWeb   ProviderID = "openai-web"
)

type Config struct {
	Provider    ProviderID
	Model       string
	APIKey      string
	BaseURL     string
	Client      *http.Client
	Credentials TokenSource
}

type TokenSource interface {
	AccessToken(context.Context) (string, error)
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	ToolName   string
}

type Request struct {
	Model           string
	ReasoningEffort ReasoningEffort
	Messages        []Message
	Tools           []Tool
}

type Response struct {
	Message Message
	Usage   Usage
}

// Usage contains token counts reported by a provider response.
type Usage struct {
	Available         bool
	InputTokens       int64
	OutputTokens      int64
	TotalTokens       int64
	CachedInputTokens int64
}

type Provider interface {
	Complete(context.Context, Request) (Response, error)
}
