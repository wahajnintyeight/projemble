package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"projemble/internal/llm"
)

type Anthropic struct{ adapter }

func NewAnthropic(config llm.Config) *Anthropic {
	return &Anthropic{adapter{config: config}}
}

func (provider *Anthropic) Complete(ctx context.Context, input llm.Request) (llm.Response, error) {
	var system string
	messages := make([]anthropicMessage, 0, len(input.Messages))
	for i := 0; i < len(input.Messages); i++ {
		message := input.Messages[i]
		if message.Role == "system" {
			if system != "" {
				system += "\n\n"
			}
			system += message.Content
			continue
		}
		if message.Role == "tool" {
			blocks := []anthropicBlock{}
			for i < len(input.Messages) && input.Messages[i].Role == "tool" {
				result := input.Messages[i]
				blocks = append(blocks, anthropicBlock{Type: "tool_result", ToolUseID: result.ToolCallID, Content: result.Content})
				i++
			}
			i--
			messages = append(messages, anthropicMessage{Role: "user", Content: blocks})
			continue
		}
		role := message.Role
		if role != "assistant" {
			role = "user"
		}
		blocks := []anthropicBlock{}
		if message.Content != "" {
			blocks = append(blocks, anthropicBlock{Type: "text", Text: message.Content})
		}
		for _, call := range message.ToolCalls {
			var args any
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				return llm.Response{}, fmt.Errorf("provider supplied invalid tool arguments: %w", err)
			}
			blocks = append(blocks, anthropicBlock{Type: "tool_use", ID: call.ID, Name: call.Name, Input: args})
		}
		messages = append(messages, anthropicMessage{Role: role, Content: blocks})
	}
	tools := make([]anthropicTool, 0, len(input.Tools))
	for _, tool := range input.Tools {
		tools = append(tools, anthropicTool{Name: tool.Name, Description: tool.Description, InputSchema: tool.Parameters})
	}
	request := anthropicRequest{Model: input.Model, MaxTokens: 8192, System: system, Messages: messages, Tools: tools}
	headers := map[string]string{"x-api-key": provider.config.APIKey, "anthropic-version": "2023-06-01"}
	var response anthropicResponse
	if err := provider.post(ctx, endpoint(provider.config.BaseURL, "messages"), headers, request, &response); err != nil {
		return llm.Response{}, err
	}
	out := llm.Message{Role: "assistant"}
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			out.Content += block.Text
		case "tool_use":
			args, err := json.Marshal(block.Input)
			if err != nil {
				return llm.Response{}, fmt.Errorf("encode Anthropic tool arguments: %w", err)
			}
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: block.ID, Name: block.Name, Arguments: string(args)})
		}
	}
	cachedInput := response.Usage.CacheReadInputTokens + response.Usage.CacheCreationInputTokens
	usage := llm.Usage{
		InputTokens:       int64(response.Usage.InputTokens + cachedInput),
		OutputTokens:      int64(response.Usage.OutputTokens),
		CachedInputTokens: int64(cachedInput),
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	usage.Available = usage.InputTokens > 0 || usage.OutputTokens > 0
	return llm.Response{Message: out, Usage: usage}, nil
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicResponse struct {
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	Content []struct {
		Type  string `json:"type"`
		Text  string `json:"text"`
		ID    string `json:"id"`
		Name  string `json:"name"`
		Input any    `json:"input"`
	} `json:"content"`
}
