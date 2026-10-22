package agent

import (
	"context"
	"fmt"
)

type Tool interface {
	Name() string
	Run(context.Context, string) (string, error)
}

type Runner struct {
	tools map[string]Tool
}

func New(tools ...Tool) Runner {
	registry := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		if tool != nil && tool.Name() != "" {
			registry[tool.Name()] = tool
		}
	}
	return Runner{tools: registry}
}

func (r Runner) Call(ctx context.Context, name, input string) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return tool.Run(ctx, input)
}
