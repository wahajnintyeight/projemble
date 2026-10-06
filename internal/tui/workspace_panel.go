package tui

import (
	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"image"
)

// sessionPanel shares the conversation's Unicode-safe, style-preserving wrapper.
type sessionPanel struct{ *widgets.Paragraph }

func newSessionPanel() *sessionPanel { return &sessionPanel{widgets.NewParagraph()} }
func (p *sessionPanel) Draw(buf *ui.Buffer) {
	p.Block.Draw(buf)
	lines := wrapTranscriptCells(ui.ParseStyles(p.Text, p.TextStyle), max(1, p.Inner.Dx()))
	for y, line := range lines {
		if y >= p.Inner.Dy() {
			break
		}
		x := p.Inner.Min.X
		for _, cell := range line {
			if x >= p.Inner.Max.X {
				break
			}
			buf.SetCell(cell, image.Pt(x, p.Inner.Min.Y+y))
			x += runewidth.RuneWidth(cell.Rune)
		}
	}
}
