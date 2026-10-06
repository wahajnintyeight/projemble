package tui

import (
	"context"
	"strings"
	"testing"
)

func TestAssistantSummaryFormatsMarkdown(t *testing.T) {
	rows := appendActivity(nil, "Agent summary: ### Final ReportThe project is **complete** and ready. - **Tests**: All passed.\n\n| Check | Result |\n|---|---|\n| Build | `go build ./...` |")
	joined := strings.Join(rows, "\n")
	for _, raw := range []string{"Agent summary:", "###", "**", "|---|"} {
		if strings.Contains(joined, raw) {
			t.Errorf("formatted response still contains raw Markdown %q:\n%s", raw, joined)
		}
	}
	visible := visibleStyledText(joined)
	for _, want := range []string{"Final Report", "The project is complete and ready.", "• Tests: All passed.", "Build", "go build ./..."} {
		if !strings.Contains(visible, want) {
			t.Errorf("formatted response is missing %q:\n%s", want, joined)
		}
	}
}

func TestGenerationReporterKeepsAssistantMarkdownTogether(t *testing.T) {
	updates := make(chan generationUpdate, 1)
	writer := generationReporter{ctx: context.Background(), updates: updates, secret: "secret-key"}
	input := []byte("Agent summary: ### Summary\nThe key is secret-key and **ready**.\n")
	written, err := writer.Write(input)
	if err != nil || written != len(input) {
		t.Fatalf("write response: bytes=%d err=%v", written, err)
	}
	update := <-updates
	if update.activity != "Agent summary: ### Summary\nThe key is [REDACTED] and **ready**.\n" {
		t.Fatalf("assistant Markdown was split or not redacted: %q", update.activity)
	}
}
