package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"projemble/internal/llm"
	"sort"
	"strings"
	"time"
)

type ModelOption struct {
	ID          string
	DisplayName string
}

// ListModels uses the provider's catalog, with bounded pagination and response size.
func ListModels(ctx context.Context, config llm.Config) ([]string, error) {
	if config.Provider == llm.OpenAIWeb {
		options, err := ListChatGPTModels(ctx, config)
		if err != nil {
			return nil, err
		}
		models := make([]string, 0, len(options))
		for _, option := range options {
			models = append(models, option.ID)
		}
		return models, nil
	}
	base := defaultBaseURL(config.Provider)
	if config.BaseURL != "" {
		base = strings.TrimRight(config.BaseURL, "/")
	}
	if base == "" {
		return nil, fmt.Errorf("unsupported provider")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || (parsed.Scheme == "http" && !localEndpoint(base)) {
		return nil, fmt.Errorf("invalid model catalog endpoint")
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client = &copyClient
	next := base + "/models"
	seen := map[string]bool{}
	for page := 0; page < 10; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		switch config.Provider {
		case llm.Claude:
			req.Header.Set("x-api-key", config.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		case llm.Gemini:
			req.Header.Set("x-goog-api-key", config.APIKey)
		default:
			if config.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+config.APIKey)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("model catalog request failed")
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("model catalog returned HTTP %d; enter a custom model ID", resp.StatusCode)
		}
		if readErr != nil || len(data) > 4<<20 {
			return nil, fmt.Errorf("model catalog response exceeded limits")
		}
		var result struct {
			Data []struct {
				ID           string `json:"id"`
				Type         string `json:"type"`
				Capabilities *struct {
					CompletionChat bool `json:"completion_chat"`
				} `json:"capabilities"`
			} `json:"data"`
			Models []struct {
				Name    string   `json:"name"`
				Methods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
			HasMore       bool   `json:"has_more"`
			LastID        string `json:"last_id"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("invalid model catalog response")
		}
		for _, m := range result.Data {
			if len(seen) < 2000 && m.ID != "" && (m.Capabilities == nil || m.Capabilities.CompletionChat) {
				seen[m.ID] = true
			}
		}
		for _, m := range result.Models {
			for _, method := range m.Methods {
				if method == "generateContent" && len(seen) < 2000 {
					seen[strings.TrimPrefix(m.Name, "models/")] = true
					break
				}
			}
		}
		values := url.Values{}
		if result.NextPageToken != "" {
			values.Set("pageToken", result.NextPageToken)
		} else if result.HasMore && result.LastID != "" {
			values.Set("after_id", result.LastID)
		} else {
			break
		}
		next = base + "/models?" + values.Encode()
	}
	models := make([]string, 0, len(seen))
	for id := range seen {
		models = append(models, id)
	}
	sort.Strings(models)
	if len(models) > 2000 {
		models = models[:2000]
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("no chat models returned; enter a custom model ID")
	}
	return models, nil
}

// ListChatGPTModels returns the account-specific models documented for Sign in with ChatGPT.
func ListChatGPTModels(ctx context.Context, config llm.Config) ([]ModelOption, error) {
	if config.Credentials == nil {
		return nil, fmt.Errorf("ChatGPT sign-in is required to list available models")
	}
	token, err := config.Credentials.AccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get ChatGPT access token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("ChatGPT sign-in returned an empty access token")
	}
	base := "https://api.openai.com/v1"
	if config.BaseURL != "" {
		base = strings.TrimRight(config.BaseURL, "/")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && localEndpoint(base))) {
		return nil, fmt.Errorf("invalid ChatGPT model catalog endpoint")
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := clientCopy.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request ChatGPT model catalog: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read ChatGPT model catalog: %w", err)
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("ChatGPT model catalog exceeded 4 MiB")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ChatGPT model catalog returned HTTP %d", response.StatusCode)
	}
	var catalog struct {
		Models []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Visibility  string `json:"visibility"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("decode ChatGPT model catalog: %w", err)
	}
	models := make([]ModelOption, 0, min(len(catalog.Models), 1000))
	seen := make(map[string]struct{}, min(len(catalog.Models), 1000))
	for _, model := range catalog.Models {
		if model.Visibility != "list" || model.Slug == "" {
			continue
		}
		if _, exists := seen[model.Slug]; exists {
			continue
		}
		seen[model.Slug] = struct{}{}
		name := strings.TrimSpace(model.DisplayName)
		if name == "" {
			name = model.Slug
		}
		models = append(models, ModelOption{ID: model.Slug, DisplayName: name})
		if len(models) == 1000 {
			break
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("no ChatGPT models are available to list for this account; enter a custom model ID")
	}
	return models, nil
}
