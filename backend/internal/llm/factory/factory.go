package factory

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"projemble/internal/llm"
	"projemble/internal/llm/adapters"
)

// New constructs the provider adapter matching the selected provider ID.
func New(config llm.Config) (llm.Provider, error) {
	config.Provider = llm.ProviderID(strings.ToLower(strings.TrimSpace(string(config.Provider))))
	if config.Provider == "" {
		config.Provider = llm.OpenAI
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("provider model is required")
	}
	if config.Client == nil {
		config.Client = &http.Client{Timeout: 90 * time.Second}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(config.Provider)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("provider %q requires an API base URL", config.Provider)
	}
	if config.Provider == llm.OpenAIWeb && config.Credentials == nil {
		return nil, errors.New("OpenAI web authentication requires a ChatGPT credential source")
	}
	if config.Provider != llm.OpenAIWeb && config.APIKey == "" && !localEndpoint(baseURL) {
		return nil, fmt.Errorf("provider %q requires an API key", config.Provider)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("provider base URL must use http or https")
	}
	if parsed.Scheme == "http" && !localEndpoint(baseURL) {
		return nil, errors.New("remote provider base URLs must use HTTPS")
	}
	config.BaseURL = baseURL
	switch config.Provider {
	case llm.OpenAI, llm.DeepSeek, llm.Mistral, llm.Qwen, llm.OpenRouter, llm.HuggingFace:
		return adapters.NewOpenAICompatible(config), nil
	case llm.Claude:
		return adapters.NewAnthropic(config), nil
	case llm.Gemini:
		return adapters.NewGemini(config), nil
	case llm.OpenAIWeb:
		return adapters.NewOpenAIResponses(config), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", config.Provider)
	}
}

func defaultBaseURL(provider llm.ProviderID) string {
	switch provider {
	case llm.OpenAI, llm.OpenAIWeb:
		return "https://api.openai.com/v1"
	case llm.Claude:
		return "https://api.anthropic.com/v1"
	case llm.DeepSeek:
		return "https://api.deepseek.com"
	case llm.Mistral:
		return "https://api.mistral.ai/v1"
	case llm.Qwen:
		return "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	case llm.OpenRouter:
		return "https://openrouter.ai/api/v1"
	case llm.HuggingFace:
		return "https://router.huggingface.co/v1"
	case llm.Gemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	default:
		return ""
	}
}

func localEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
