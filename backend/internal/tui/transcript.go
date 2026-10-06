package tui

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
)

// transcriptView scrolls rendered lines, rather than selecting log entries.
type transcriptView struct {
	ui.Block
	rows    []string
	lines   [][]ui.Cell
	width   int
	offset  int
	details bool
}

func newTranscriptView() *transcriptView {
	return &transcriptView{Block: *ui.NewBlock()}
}

func (v *transcriptView) content(rows []string, details, follow bool) {
	width := max(1, min(100, v.Inner.Dx()-4))
	if width != v.width || details != v.details || !slices.Equal(rows, v.rows) {
		v.rows = slices.Clone(rows)
		v.width, v.details = width, details
		v.lines = nil
		for _, row := range conversationRows(rows, details) {
			cells := ui.ParseStyles(row, ui.NewStyle(ui.ColorClear))
			if len(cells) == 0 {
				v.lines = append(v.lines, nil)
				continue
			}
			v.lines = append(v.lines, wrapTranscriptCells(cells, width)...)
		}
	}
	if follow {
		v.offset = v.bottom()
	}
	v.offset = min(max(0, v.offset), v.bottom())
	v.Title = "Conversation · compact · Ctrl+O details"
	if details {
		v.Title = "Conversation · full activity · Ctrl+O compact"
	}
}

func wrapTranscriptCells(cells []ui.Cell, width int) [][]ui.Cell {
	var lines [][]ui.Cell
	for len(cells) > 0 {
		used, end, space := 0, 0, -1
		for end < len(cells) {
			if cells[end].Rune == '\n' {
				break
			}
			w := runewidth.RuneWidth(cells[end].Rune)
			if used+w > width {
				break
			}
			used += w
			if cells[end].Rune == ' ' && used > 1 {
				space = end
			}
			end++
		}
		if end == 0 {
			end = 1
		}
		if end < len(cells) && cells[end].Rune != '\n' && space > 0 {
			end = space + 1
		}
		lines = append(lines, cells[:end])
		cells = cells[end:]
		if len(cells) > 0 && cells[0].Rune == '\n' {
			cells = cells[1:]
		}
	}
	return lines
}

func activityCounts(reads, checks, output int) string {
	return fmt.Sprintf(" · %d reads · %d checks · %d output lines · Ctrl+O expand", reads, checks, output)
}

func (v *transcriptView) bottom() int    { return max(0, len(v.lines)-v.Inner.Dy()) }
func (v *transcriptView) AtBottom() bool { return v.offset >= v.bottom() }
func (v *transcriptView) ScrollPageUp()  { v.offset = max(0, v.offset-max(1, v.Inner.Dy()-2)) }
func (v *transcriptView) ScrollPageDown() {
	v.offset = min(v.bottom(), v.offset+max(1, v.Inner.Dy()-2))
}

func (v *transcriptView) Draw(buf *ui.Buffer) {
	v.Block.Draw(buf)
	margin := min(2, max(0, v.Inner.Dx()-1))
	for y := 0; y < v.Inner.Dy() && v.offset+y < len(v.lines); y++ {
		x := v.Inner.Min.X + margin
		for _, cell := range v.lines[v.offset+y] {
			if x >= v.Inner.Max.X {
				break
			}
			buf.SetCell(cell, image.Pt(x, v.Inner.Min.Y+y))
			x += runewidth.RuneWidth(cell.Rune)
		}
	}
}

// Keep mutations and errors visible; collapse repetitive reads and command output.
// The original transcript is retained for the full activity view.
func conversationRows(rows []string, details bool) []string {
	if details {
		return rows
	}
	var out []string
	var last string
	reads, checks, waits, output := 0, 0, 0, 0
	flush := func() {
		if reads+checks+waits+output == 0 {
			return
		}
		out = append(out, "", styledMarkdown("Activity", "fg:cyan,mod:bold")+
			activityCounts(reads, checks, output), last, "")
		reads, checks, waits, output = 0, 0, 0, 0
		last = ""
	}
	activity := false
	for _, row := range rows {
		switch {
		case strings.HasPrefix(row, "[READ]"):
			activity = true
			if strings.Contains(row, "Action:") {
				reads++
			}
			last = row
		case strings.HasPrefix(row, "[CHECK]"):
			activity = true
			checks++
			last = row
		case strings.HasPrefix(row, "[THINK]"):
			activity = true
			waits++
			last = row
		case strings.HasPrefix(row, "[AGENT]") || strings.HasPrefix(row, "[YOU]"):
			flush()
			activity = false
			out = append(out, row)
		case strings.HasPrefix(row, "[ERROR]") || strings.HasPrefix(row, "[EDIT]") ||
			strings.HasPrefix(row, "[+]") || strings.HasPrefix(row, "[~]") ||
			strings.HasPrefix(row, "[-]") || strings.HasPrefix(row, "[PASS]") ||
			strings.HasPrefix(row, "[SAVE]") || strings.HasPrefix(row, "[NOTE]") ||
			strings.HasPrefix(row, "[AI]"):
			flush()
			out = append(out, row)
		default:
			if activity && row != "" {
				output++
			} else {
				out = append(out, row)
			}
		}
	}
	flush()
	return out
}
