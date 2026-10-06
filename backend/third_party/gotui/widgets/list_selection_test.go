package widgets

import (
	ui "github.com/metaspartan/gotui/v5"
	"testing"
)

func TestSelectedListRowUsesOneHighlightStyle(t *testing.T) {
	base := ui.NewStyle(ui.ColorWhite)
	selected := ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierReverse|ui.ModifierBold)
	list := &List{Rows: []string{"[dev](fg:cyan,mod:bold) - saved | E:\\work\\dev"}, TextStyle: base, SelectedStyle: selected, SelectedRow: 0}
	cells := list.getRowCells(0)
	if len(cells) == 0 {
		t.Fatal("selected row rendered no cells")
	}
	for i, cell := range cells {
		if cell.Style != selected {
			t.Fatalf("cell %d style = %+v, want uniform selected style %+v", i, cell.Style, selected)
		}
	}
}

func TestUnselectedListRowKeepsInlineColors(t *testing.T) {
	base := ui.NewStyle(ui.ColorWhite)
	list := &List{Rows: []string{"[dev](fg:cyan,mod:bold) - saved"}, TextStyle: base, SelectedStyle: ui.NewStyle(ui.ColorWhite, ui.ColorBlack), SelectedRow: -1}
	cells := list.getRowCells(0)
	if cells[0].Style.Fg != ui.ColorLightCyan {
		t.Fatalf("inline label color = %v, want cyan", cells[0].Style.Fg)
	}
	if cells[len(cells)-1].Style != base {
		t.Fatal("unstyled row suffix lost its default style")
	}
}
