package tui

import (
	"projemble/internal/llm"
)

const (
	maxPromptRunes     = 32 << 10
	maxPendingPrompts  = 32
	agentSpinnerFrames = "|/-\\"
)

type agentWorkspace struct {
	header      *sessionPanel
	transcript  *transcriptView
	composer    *messageComposer
	session     llm.Usage
	last        llm.Usage
	spinner     int
	details     bool
	showSidebar bool
	navigation  *sessionPanel
	mentions    *fileMention
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
	thinking bool
	access   bool
}

func newAgentWorkspace() *agentWorkspace {
	header := newSessionPanel()
	header.Border = true
	header.Title = "Projemble agent"
	transcript := newTranscriptView()
	transcript.Border = true
	transcript.Title = "Conversation and live activity"
	composer := newMessageComposer()
	composer.Text = ""
	navigation := newSessionPanel()
	navigation.Border = true
	navigation.Title = "Navigate"
	return &agentWorkspace{header: header, transcript: transcript, composer: composer, navigation: navigation, mentions: newFileMention(), showSidebar: true}
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
