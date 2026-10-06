package tui

import (
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/llm"
)

const (
	maxPromptRunes     = 32 << 10
	maxPendingPrompts  = 32
	agentSpinnerFrames = "|/-\\"
)

type agentWorkspace struct {
	header     *sessionPanel
	transcript *transcriptView
	composer   *widgets.TextArea
	session    llm.Usage
	last       llm.Usage
	spinner    int
	details    bool
	navigation *sessionPanel
}

type workspaceAction struct {
	prompt   string
	queued   bool
	back     bool
	quit     bool
	scroll   bool
	follow   bool
	provider bool
	model    bool
}

func newAgentWorkspace() *agentWorkspace {
	header := newSessionPanel()
	header.Border = true
	header.Title = "Projemble agent"
	transcript := newTranscriptView()
	transcript.Border = true
	transcript.Title = "Conversation and live activity"
	composer := widgets.NewTextArea()
	composer.Border = true
	composer.Title = "Message"
	composer.Text = ""
	composer.CursorStyle = focusedStyle()
	navigation := newSessionPanel()
	navigation.Border = true
	navigation.Title = "Navigate"
	return &agentWorkspace{header: header, transcript: transcript, composer: composer, navigation: navigation}
}

func (workspace *agentWorkspace) Tick() {
	workspace.spinner = (workspace.spinner + 1) % len(agentSpinnerFrames)
}

func (workspace *agentWorkspace) AddUsage(usage llm.Usage) {
	if !usage.Available {
		return
	}
	workspace.last = usage
	workspace.session.Available = true
	workspace.session.InputTokens += usage.InputTokens
	workspace.session.OutputTokens += usage.OutputTokens
	workspace.session.CachedInputTokens += usage.CachedInputTokens
	if usage.TotalTokens != 0 {
		workspace.session.TotalTokens += usage.TotalTokens
	} else {
		workspace.session.TotalTokens += usage.InputTokens + usage.OutputTokens
	}
}
