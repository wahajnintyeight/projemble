package tui

import (
	ui "github.com/metaspartan/gotui/v5"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMessageLayoutWrapsLongTextKeepsCursorAndUnicode(t *testing.T) {
	text := "Long sentence " + strings.Repeat("overlapping words ", 12) + "東京 final"
	cursor := image.Pt(len([]rune(text)), 0)
	rows, position := layoutMessage(text, cursor, 18)
	if len(rows) < 5 {
		t.Fatalf("text was not wrapped: %d rows", len(rows))
	}
	if strings.Join(rows, "") != text {
		t.Fatalf("wrapping changed draft: %q", strings.Join(rows, ""))
	}
	if position.Y != len(rows)-1 || position.X == 0 {
		t.Fatalf("cursor at (%d,%d), rows=%d", position.X, position.Y, len(rows))
	}
	composer := newMessageComposer()
	composer.Text = text
	composer.Cursor = cursor
	composer.SetRect(0, 0, 24, 8)
	widerRows, widerCursor := layoutMessage(text, cursor, 22)
	if got := composer.visualLines(22); got != len(widerRows) {
		t.Fatalf("visual lines %d != %d", got, len(rows))
	}
	buffer := ui.NewBuffer(image.Rect(0, 0, 24, 8))
	composer.Draw(buffer)
	last := buffer.GetCell(image.Pt(composer.Inner.Min.X+widerCursor.X, composer.Inner.Min.Y+widerCursor.Y-composer.top))
	if last.Style != composer.CursorStyle {
		t.Fatal("wrapped composer cursor is not visible")
	}
}

func TestFileMentionListsFoldersFiltersAndCompletesPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src code"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.go", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("sample"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "src code", "server.go"), []byte("package src"), 0600); err != nil {
		t.Fatal(err)
	}
	c := newMessageComposer()
	c.Text = "Please inspect @src code"
	c.Cursor = image.Pt(len([]rune(c.Text)), 0)
	m := newFileMention()
	if !m.active(root, c) || len(m.paths) != 1 || m.paths[0] != "src code/" {
		t.Fatalf("directory suggestions: %v", m.paths)
	}
	m.list.SelectedRow = 0
	if !m.choose(c) || !strings.HasSuffix(c.Text, "@src code/") {
		t.Fatalf("directory completion: %q", c.Text)
	}
	if !m.active(root, c) || len(m.paths) != 1 || m.paths[0] != "src code/server.go" {
		t.Fatalf("nested suggestions: %v", m.paths)
	}
	m.list.SelectedRow = 0
	if !m.choose(c) || !strings.HasSuffix(c.Text, "@src code/server.go ") {
		t.Fatalf("file completion: %q", c.Text)
	}
}

func TestMessageComposerGrowsWithinTerminalBudget(t *testing.T) {
	w := newAgentWorkspace()
	options := generationOptions{Provider: "mistral", Model: "model"}
	w.Render(100, 30, nil, options, "", false, true, 0)
	short := w.composer.Max.Y - w.composer.Min.Y
	w.composer.Text = strings.Repeat("long words ", 30)
	w.composer.Cursor = image.Pt(len([]rune(w.composer.Text)), 0)
	w.Render(100, 30, nil, options, "", false, true, 0)
	long := w.composer.Max.Y - w.composer.Min.Y
	if long <= short || long > 15 {
		t.Fatalf("composer heights short=%d long=%d", short, long)
	}
}
