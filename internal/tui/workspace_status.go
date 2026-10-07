package tui

import (
	"fmt"
)

const chatGPTUsageURL = "https://chatgpt.com/settings/usage"

func (w *agentWorkspace) sidebarText(provider, model, path string, running bool, queued int) string {
	state := "Ready"
	if running {
		state = fmt.Sprintf("%c Agent is thinking · queued %d", agentSpinnerFrames[w.spinner%len(agentSpinnerFrames)], queued)
	}
	usage := "Waiting for token counts"
	if w.session.Available {
		usage = fmt.Sprintf("%s input\n%s output\n%s total", formatTokens(w.session.InputTokens), formatTokens(w.session.OutputTokens), formatTokens(w.session.TotalTokens))
	}
	context := "Not reported"
	if w.last.Available {
		context = fmt.Sprintf("Latest: %s input", formatTokens(w.last.InputTokens))
	}
	text := fmt.Sprintf("%s %s\n%s %s\n%s %s\n\n%s\n%s\n\n%s\n%s\nWindow limit not reported\n\n%s\n%s", styleLabel("Provider:"), provider, styleLabel("Model:"), model, styleLabel("Status:"), state, styleLabel("Session tokens"), usage, styleLabel("Context"), context, styleLabel("Workspace"), path)
	if provider == "ChatGPT plan" {
		text += "\n\n" + styleLabel("Plan limits") + "\nManage usage:\nchatgpt.com/settings/usage"
	}
	return text
}

func (workspace *agentWorkspace) statusText(provider, path string, running bool, queued int) string {
	tokenLabel := "Usage:"
	tokens := "waiting for provider token counts"
	context := "token count and model limit not reported yet"
	state := "ready for instructions"
	if running {
		state = fmt.Sprintf("[%c](fg:%s,mod:bold) Agent is thinking · %d queued", agentSpinnerFrames[workspace.spinner%len(agentSpinnerFrames)], colorThinking, queued)
	}
	if workspace.session.Available {
		tokenLabel = "Session tokens:"
		tokens = fmt.Sprintf("%s input  ·  %s output  ·  %s total", formatTokens(workspace.session.InputTokens), formatTokens(workspace.session.OutputTokens), formatTokens(workspace.session.TotalTokens))
	}
	if workspace.last.Available {
		context = fmt.Sprintf("latest request used %s input tokens  ·  model window limit unavailable", formatTokens(workspace.last.InputTokens))
	}
	text := fmt.Sprintf("%s %s\n%s %s\n%s %s\n%s %s", styleLabel("Status:"), state, styleLabel("Workspace:"), path, styleLabel(tokenLabel), tokens, styleLabel("Context:"), context)
	if provider == "ChatGPT plan" {
		text += "\n" + styleLabel("Manage usage:") + " " + chatGPTUsageURL
	}
	return text
}

func formatTokens(tokens int64) string {
	if tokens <= 0 {
		return "0"
	}
	value := fmt.Sprintf("%d", tokens)
	for i := len(value) - 3; i > 0; i -= 3 {
		value = value[:i] + "," + value[i:]
	}
	return value
}
