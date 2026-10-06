package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/projectstore"
)

func TestWelcomeKeepsProjectChoicesVisibleAcrossTerminalSizes(t *testing.T) {
	previous := ui.DefaultBackend.Screen
	defer func() { ui.DefaultBackend.Screen = previous }()
	for _, size := range [][2]int{{40, 12}, {60, 18}, {80, 24}, {120, 40}} {
		width, height := size[0], size[1]
		terminal := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)})
		screen, err := tcell.NewTerminfoScreenFromTty(terminal)
		if err != nil {
			t.Fatal(err)
		}
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		defer screen.Fini()
		ui.DefaultBackend.Screen = screen
		list := widgets.NewList()
		updateChoiceList(list, "Projects", homeChoices(projectstore.NewConfig()), 0, width, height)
		renderProjectHome(list, "Create your first blueprint.", width, height)
		if err := terminal.Drain(); err != nil {
			t.Fatal(err)
		}
		var visible strings.Builder
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				visible.WriteString(terminal.GetCell(vt.Coord{X: vt.Col(x), Y: vt.Row(y)}).C)
			}
			visible.WriteByte('\n')
		}
		if !strings.Contains(visible.String(), "Create a project") || list.Inner.Dy() < 1 {
			t.Fatalf("project entry unavailable at %dx%d:\n%s", width, height, visible.String())
		}
		input := widgets.NewInput()
		renderOnboardingInput(input, projectNamePage, width, height)
		if input.Min.Y < 0 || input.Max.Y > height || input.Inner.Dy() < 1 {
			t.Fatalf("project input unavailable at %dx%d: %v", width, height, input.Rectangle)
		}
	}
}

func TestProjectNameBackReturnsToProjectsWithoutProviderSetup(t *testing.T) {
	input := widgets.NewInput()
	input.Text = "keep-my-idea"
	var next page
	advance, quit, message := handleTextInput(ui.Event{ID: "<Escape>"}, projectNamePage, input, widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), &next)
	if !advance || quit || message != "" || next != homePage || input.Text != "keep-my-idea" {
		t.Fatalf("back navigation: next=%v input=%q message=%q", next, input.Text, message)
	}
}
