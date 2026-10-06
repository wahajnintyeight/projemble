package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"projemble/internal/llm"
)

// OpenAIResponses adapts Responses API streaming turns, used by ChatGPT-plan OAuth.
type OpenAIResponses struct{ adapter }

func NewOpenAIResponses(config llm.Config) *OpenAIResponses {
	return &OpenAIResponses{adapter{config: config}}
}

func (provider *OpenAIResponses) Complete(ctx context.Context, input llm.Request) (llm.Response, error) {
	var instructions string
	items := make([]map[string]any, 0, len(input.Messages))
	for _, message := range input.Messages {
		switch message.Role {
		case "system":
			instructions += message.Content + "\n\n"
		case "tool":
			items = append(items, map[string]any{"type": "function_call_output", "call_id": message.ToolCallID, "output": message.Content})
		case "assistant":
			if message.Content != "" {
				items = append(items, map[string]any{"type": "message", "role": "assistant", "content": message.Content})
			}
			for _, call := range message.ToolCalls {
				items = append(items, map[string]any{"type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": call.Arguments})
			}
		default:
			items = append(items, map[string]any{"type": "message", "role": "user", "content": message.Content})
		}
	}
	tools := make([]map[string]any, 0, len(input.Tools))
	for _, tool := range input.Tools {
		tools = append(tools, map[string]any{"type": "function", "name": tool.Name, "description": tool.Description, "parameters": tool.Parameters, "strict": false})
	}
	payload := map[string]any{"model": input.Model, "instructions": instructions, "input": items, "tools": tools, "store": false, "stream": true}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.Response{}, fmt.Errorf("encode Responses request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(provider.config.BaseURL, "responses"), bytes.NewReader(body))
	if err != nil {
		return llm.Response{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	token := provider.config.APIKey
	if provider.config.Credentials != nil {
		token, err = provider.config.Credentials.AccessToken(ctx)
		if err != nil {
			return llm.Response{}, fmt.Errorf("get OpenAI ChatGPT access token: %w", err)
		}
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := provider.config.Client.Do(request)
	if err != nil {
		return llm.Response{}, fmt.Errorf("call OpenAI Responses API: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxProviderResponse+1))
		if readErr != nil {
			return llm.Response{}, fmt.Errorf("read OpenAI response error: %w", readErr)
		}
		return llm.Response{}, fmt.Errorf("OpenAI returned HTTP %d: %s", response.StatusCode, clipped(string(data), 2048))
	}
	return readResponseEvents(response.Body)
}

func readResponseEvents(body io.Reader) (llm.Response, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, maxProviderResponse+1))
	scanner.Buffer(make([]byte, 4096), maxProviderResponse)
	toolCalls := make([]llm.ToolCall, 0)
	itemCalls := make(map[string]int)
	var message llm.Message
	var responseUsage llm.Usage
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := bytes.TrimPrefix(line, []byte("data: "))
		if bytes.Equal(data, []byte("[DONE]")) {
			break
		}
		var event struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				Usage struct {
					InputTokens        int `json:"input_tokens"`
					OutputTokens       int `json:"output_tokens"`
					TotalTokens        int `json:"total_tokens"`
					InputTokensDetails struct {
						CachedTokens int `json:"cached_tokens"`
					} `json:"input_tokens_details"`
				} `json:"usage"`
			} `json:"response"`
			Item struct {
				Type      string `json:"type"`
				ID        string `json:"id"`
				CallID    string `json:"call_id"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"item"`
			ItemID    string `json:"item_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Error     any    `json:"error"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return llm.Response{}, fmt.Errorf("decode OpenAI stream event: %w", err)
		}
		switch event.Type {
		case "response.output_text.delta":
			message.Content += event.Delta
		case "response.output_item.added":
			if event.Item.Type == "function_call" {
				itemCalls[event.Item.ID] = len(toolCalls)
				toolCalls = append(toolCalls, llm.ToolCall{ID: event.Item.CallID, Name: event.Item.Name, Arguments: event.Item.Arguments})
			}
		case "response.function_call_arguments.delta":
			if index, ok := itemCalls[event.ItemID]; ok {
				toolCalls[index].Arguments += event.Delta
			}
		case "response.function_call_arguments.done":
			if index, ok := itemCalls[event.ItemID]; ok && event.Arguments != "" {
				toolCalls[index].Arguments = event.Arguments
			}
		case "response.completed", "response.done":
			usage := event.Response.Usage
			responseUsage = llm.Usage{
				Available:         usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.TotalTokens > 0,
				InputTokens:       int64(usage.InputTokens),
				OutputTokens:      int64(usage.OutputTokens),
				TotalTokens:       int64(usage.TotalTokens),
				CachedInputTokens: int64(usage.InputTokensDetails.CachedTokens),
			}
		case "response.failed", "error":
			return llm.Response{}, fmt.Errorf("OpenAI Responses request failed: %v", event.Error)
		}
	}
	if err := scanner.Err(); err != nil {
		return llm.Response{}, fmt.Errorf("read OpenAI stream: %w", err)
	}
	message.ToolCalls = toolCalls
	if len(message.ToolCalls) == 0 && message.Content == "" {
		return llm.Response{}, errors.New("OpenAI Responses stream completed without a message")
	}
	message.Role = "assistant"
	return llm.Response{Message: message, Usage: responseUsage}, nil
}
