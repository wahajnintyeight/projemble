package tui

import (
	"image"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
)

func TestTranscriptScrollsVisualLinesAndPreservesStyles(t *testing.T) {
	view := newTranscriptView()
	view.SetRect(0, 0, 45, 12)
	rows := []string{styledMarkdown(strings.Repeat("reading 世界 ", 70), "fg:cyan")}
	view.content(rows, true, true)
	if len(view.lines) < 20 || !view.AtBottom() {
		t.Fatal("long message did not wrap/follow")
	}
	bottom := view.offset
	view.ScrollPageUp()
	if view.offset >= bottom || view.AtBottom() {
		t.Fatal("PageUp did not review visible lines")
	}
	for _, line := range view.lines {
		if runewidth.StringWidth(ui.CellsToString(line)) > view.width {
			t.Fatal("wrapped line exceeds viewport")
		}
		for _, cell := range line {
			if cell.Style.Fg != ui.ColorLightCyan {
				t.Fatal("wrapping changed text color")
			}
		}
	}
	buf := ui.NewBuffer(image.Rect(0, 0, 45, 12))
	view.Draw(buf)
	if buf.GetCell(image.Pt(3, 1)).Style.Fg != ui.ColorLightCyan {
		t.Fatal("drawing overwrote message styling")
	}
	for !view.AtBottom() {
		view.ScrollPageDown()
	}
	if view.offset != bottom {
		t.Fatal("PageDown did not return to latest activity")
	}
}

func TestCompactActivityRetainsChangesErrorsAndExpandableOutput(t *testing.T) {
	var rows []string
	for _, event := range []string{"You: fix it", "Waiting on mistral", "Action: Reading main.go", "Read main.go (200 bytes)", "Action: Writing main.go", "Updated main.go", "Action: Running go test ./...", "verbose output", "tool error: build failed", "Agent summary: ### Done\nFixed **main.go**."} {
		rows = appendActivity(rows, event)
	}
	compact := visibleStyledText(strings.Join(conversationRows(rows, false), "\n"))
	for _, want := range []string{"YOU", "fix it", "updated main.go", "failed", "build", "Done", "Fixed main.go", "Agent activity", "Ctrl+O expand"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("missing %q: %s", want, compact)
		}
	}
	if strings.Contains(compact, "verbose output") {
		t.Fatal("command payload not folded")
	}
	if strings.Contains(compact, "Action:") || strings.Contains(compact, "Command failed:") {
		t.Fatalf("agent event labels still look like raw messages: %s", compact)
	}
	full := visibleStyledText(strings.Join(conversationRows(rows, true), "\n"))
	if !strings.Contains(full, "verbose output") {
		t.Fatal("details lost original output")
	}
	workspace := newAgentWorkspace()
	workspace.composer.Text = "keep draft"
	workspace.Handle(ui.Event{ID: "<C-o>", Type: ui.KeyboardEvent}, true)
	if !workspace.details || workspace.composer.Text != "keep draft" {
		t.Fatal("detail toggle consumed draft")
	}
}

func TestMessageFramesDistinguishSpeakersAndPreserveFormatting(t *testing.T) {
	rows := appendActivity(nil, "You: change the greeting")
	rows = appendAssistantMarkdown(rows, "### Updated\nThe greeting is **ready**.")
	formatted := formatTranscriptRows(borderedMessages(rows, 32), 32)
	var framed []string
	for _, line := range formatted {
		framed = append(framed, ui.CellsToString(line))
	}
	text := strings.Join(framed, "\n")
	for _, want := range []string{"╭─ YOU", "│ change the greeting", "╰", "╭─ AGENT", "│ Updated", "greeting is ready"} {
		if !strings.Contains(text, want) {
			t.Errorf("message border missing %q:\n%s", want, text)
		}
	}
	userStart := -1
	agentStart := -1
	for i, line := range framed {
		if strings.HasPrefix(line, "╭─ YOU ") {
			userStart = i
		}
		if strings.HasPrefix(line, "╭─ AGENT ") {
			agentStart = i
			break
		}
	}
	if agentStart < 0 {
		t.Fatal("agent border missing")
	}
	if formatted[userStart][0].Style.Fg == formatted[agentStart][0].Style.Fg {
		t.Fatal("speaker frames do not use distinct colors")
	}
}

func TestAgentActionsStayOutsideSpeakerMessageFrames(t *testing.T) {
	rows := appendActivity(nil, "Action: Writing internal/app.go")
	formatted := borderedMessages(rows, 32)
	if len(formatted) != 1 || !strings.HasPrefix(formatted[0], "[> write]") {
		t.Fatalf("agent action should render as a standalone event row: %#v", formatted)
	}
}

func TestMarkdownPreservesCodeAndHeadingStructure(t *testing.T) {
	rows := appendAssistantMarkdown(nil, "### Security Considerations\nUse **bold**, *italic*, and `x[y]`.\n\n```go\nfunc main() {\n    // ### literal\n}\n```\n\n> quoted\n\n1. First\n2. Second")
	text := visibleStyledText(strings.Join(rows, "\n"))
	for _, want := range []string{"Security Considerations", "bold", "italic", "x[y]", "    // ### literal", "quoted", "1. First", "2. Second"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "\n#\n") || strings.Contains(text, "**bold**") {
		t.Fatal("raw formatting markers leaked")
	}
}
