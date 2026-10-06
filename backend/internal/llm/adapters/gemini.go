package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"projemble/internal/llm"
)

type Gemini struct{ adapter }

func NewGemini(config llm.Config) *Gemini {
	return &Gemini{adapter{config: config}}
}

func (provider *Gemini) Complete(ctx context.Context, input llm.Request) (llm.Response, error) {
	var system *geminiContent
	contents := make([]geminiContent, 0, len(input.Messages))
	for _, message := range input.Messages {
		switch message.Role {
		case "system":
			system = &geminiContent{Parts: []geminiPart{{Text: message.Content}}}
		case "tool":
			contents = append(contents, geminiContent{Role: "user", Parts: []geminiPart{{
				FunctionResponse: &geminiFunctionResponse{Name: message.ToolName, ID: message.ToolCallID, Response: map[string]any{"content": message.Content}},
			}}})
		default:
			role := "user"
			if message.Role == "assistant" {
				role = "model"
			}
			content := geminiContent{Role: role}
			if message.Content != "" {
				content.Parts = append(content.Parts, geminiPart{Text: message.Content})
			}
			for _, call := range message.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
					return llm.Response{}, fmt.Errorf("provider supplied invalid tool arguments: %w", err)
				}
				content.Parts = append(content.Parts, geminiPart{FunctionCall: &geminiFunctionCall{Name: call.Name, ID: call.ID, Args: args}})
			}
			contents = append(contents, content)
		}
	}
	declarations := make([]geminiFunctionDeclaration, 0, len(input.Tools))
	for _, tool := range input.Tools {
		declarations = append(declarations, geminiFunctionDeclaration{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters})
	}
	request := geminiRequest{SystemInstruction: system, Contents: contents}
	if len(declarations) != 0 {
		request.Tools = []geminiToolSet{{FunctionDeclarations: declarations}}
	}
	var response geminiResponse
	path := "models/" + url.PathEscape(input.Model) + ":generateContent"
	headers := map[string]string{"x-goog-api-key": provider.config.APIKey}
	if err := provider.post(ctx, endpoint(provider.config.BaseURL, path), headers, request, &response); err != nil {
		return llm.Response{}, err
	}
	if len(response.Candidates) == 0 {
		return llm.Response{}, fmt.Errorf("Gemini returned no candidates")
	}
	out := llm.Message{Role: "assistant"}
	for _, part := range response.Candidates[0].Content.Parts {
		if part.Text != "" {
			out.Content += part.Text
		}
		if part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return llm.Response{}, fmt.Errorf("encode Gemini function arguments: %w", err)
			}
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: part.FunctionCall.ID, Name: part.FunctionCall.Name, Arguments: string(args)})
		}
	}
	return llm.Response{Message: out}, nil
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiToolSet `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	ID   string         `json:"id,omitempty"`
	Args map[string]any `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	ID       string         `json:"id,omitempty"`
	Response map[string]any `json:"response"`
}

type geminiToolSet struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
