package adapters

import (
	"context"
	"fmt"

	"projemble/internal/llm"
)

type OpenAICompatible struct{ adapter }

func NewOpenAICompatible(config llm.Config) *OpenAICompatible {
	return &OpenAICompatible{adapter{config: config}}
}

func (provider *OpenAICompatible) Complete(ctx context.Context, input llm.Request) (llm.Response, error) {
	messages := make([]chatMessage, 0, len(input.Messages))
	for _, message := range input.Messages {
		if provider.config.Provider == llm.DeepSeek && message.Role == "assistant" && len(message.ToolCalls) != 0 {
			continue
		}
		role := message.Role
		if provider.config.Provider == llm.DeepSeek && role == "tool" {
			role = "system"
		}
		item := chatMessage{Role: role, Content: message.Content, ToolCallID: message.ToolCallID, Name: message.ToolName}
		for _, call := range message.ToolCalls {
			item.ToolCalls = append(item.ToolCalls, chatToolCall{ID: call.ID, Type: "function", Function: chatFunction{Name: call.Name, Arguments: call.Arguments}})
		}
		if provider.config.Provider == llm.DeepSeek && message.Role == "tool" {
			item.Content = fmt.Sprintf("Tool result from %s: %s", message.ToolName, message.Content)
			item.ToolCallID = ""
			item.Name = ""
		}
		messages = append(messages, item)
	}
	tools := make([]chatTool, 0, len(input.Tools))
	for _, tool := range input.Tools {
		tools = append(tools, chatTool{Type: "function", Function: chatFunctionDefinition{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters}})
	}
	request := chatRequest{Model: input.Model, Messages: messages, Tools: tools, ToolChoice: "auto"}
	var response chatResponse
	if err := provider.post(ctx, endpoint(provider.config.BaseURL, "chat/completions"), bearer(provider.config.APIKey), request, &response); err != nil {
		return llm.Response{}, err
	}
	if len(response.Choices) == 0 {
		return llm.Response{}, fmt.Errorf("%s returned no choices", provider.config.Provider)
	}
	message := response.Choices[0].Message
	out := llm.Message{Role: message.Role, Content: message.Content}
	for _, call := range message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return llm.Response{Message: out}, nil
}

type chatRequest struct {
	Model      string        `json:"model"`
	Messages   []chatMessage `json:"messages"`
	Tools      []chatTool    `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
}

type chatTool struct {
	Type     string                 `json:"type"`
	Function chatFunctionDefinition `json:"function"`
}

type chatFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Role      string         `json:"role"`
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}
