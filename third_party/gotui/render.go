package gotui

import (
	"image"
	"os"

	"github.com/gdamore/tcell/v3"
	"github.com/mattn/go-runewidth"
)

// Render renders the given drawables to the screen.
func Render(items ...Drawable) {
	DefaultBackend.Render(items...)
}

func (b *Backend) Render(items ...Drawable) {
	if b.Screen == nil || len(items) == 0 {
		return
	}

	minX, minY, maxX, maxY := calculateBounds(items)

	// Ensure minimum buffer dimensions to prevent rendering issues
	if maxX <= minX || maxY <= minY {
		b.Screen.Show()
		return
	}

	buf := NewBuffer(image.Rect(minX, minY, maxX, maxY))

	for _, item := range items {
		item.Lock()
		item.Draw(buf)
		item.Unlock()
	}

	if b.ScreenshotMode {
		width, height := 120, 60
		if err := SaveImage("screenshot.png", width, height, items...); err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	b.renderBuffer(buf)
}

func calculateBounds(items []Drawable) (minX, minY, maxX, maxY int) {
	minX, minY = items[0].GetRect().Min.X, items[0].GetRect().Min.Y
	maxX, maxY = items[0].GetRect().Max.X, items[0].GetRect().Max.Y

	for _, item := range items {
		r := item.GetRect()
		if r.Min.X < minX {
			minX = r.Min.X
		}
		if r.Min.Y < minY {
			minY = r.Min.Y
		}
		if r.Max.X > maxX {
			maxX = r.Max.X
		}
		if r.Max.Y > maxY {
			maxY = r.Max.Y
		}
	}
	return
}

func (b *Backend) renderBuffer(buf *Buffer) {
	bufWidth := buf.Dx()
	if bufWidth <= 0 {
		b.Screen.Show()
		return
	}

	// Get screen dimensions for clipping
	screenW, screenH := b.Screen.Size()

	for i, cell := range buf.Cells {
		x := (i % bufWidth) + buf.Min.X
		y := (i / bufWidth) + buf.Min.Y

		// Skip cells outside visible screen area
		if x < 0 || y < 0 || x >= screenW || y >= screenH {
			continue
		}
		if cell.Rune == 0 {
			// A zero cell after a wide rune is its continuation; leave that cell
			// to tcell, but clear every other untouched position to remove stale
			// text when widgets shrink or move between frames.
			if x > buf.Min.X && runewidth.RuneWidth(buf.Cells[i-1].Rune) == 2 {
				continue
			}
			b.Screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
			continue
		}

		style := tcell.StyleDefault.
			Foreground(cell.Style.Fg).
			Background(cell.Style.Bg).
			Bold(cell.Style.Modifier&tcell.AttrBold != 0).
			Reverse(cell.Style.Modifier&tcell.AttrReverse != 0).
			Dim(cell.Style.Modifier&tcell.AttrDim != 0).
			Blink(cell.Style.Modifier&tcell.AttrBlink != 0).
			Italic(cell.Style.Modifier&tcell.AttrItalic != 0).
			StrikeThrough(cell.Style.Modifier&tcell.AttrStrikeThrough != 0)

		b.Screen.SetContent(x, y, cell.Rune, nil, style)
	}
	b.Screen.Show()
}
