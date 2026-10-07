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
	height  int
	offset  int
	details bool
}

func newTranscriptView() *transcriptView {
	return &transcriptView{Block: *ui.NewBlock()}
}

func (v *transcriptView) content(rows []string, details, follow bool) {
	width, height := max(1, v.Inner.Dx()-4), v.Inner.Dy()
	oldBottom := max(0, len(v.lines)-v.height)
	wasAtBottom := v.offset >= oldBottom
	layoutChanged := width != v.width || height != v.height || details != v.details
	if layoutChanged || !slices.Equal(rows, v.rows) {
		v.rows = slices.Clone(rows)
		v.width, v.height, v.details = width, height, details
		v.lines = nil
		v.lines = formatTranscriptRows(borderedMessages(conversationRows(rows, details), width), width)
	}
	if follow {
		v.offset = v.bottom()
	} else if layoutChanged && wasAtBottom {
		v.offset = v.bottom()
	} else if layoutChanged && oldBottom > 0 {
		v.offset = v.offset * v.bottom() / oldBottom
	} else {
		v.offset = min(max(0, v.offset), v.bottom())
	}
	v.Title = "Conversation · compact · Ctrl+O details"
	if details {
		v.Title = "Conversation · full activity · Ctrl+O compact"
	}
	if len(v.lines) == 0 {
		v.TitleBottom = "0 lines"
	} else {
		first := min(v.offset+1, len(v.lines))
		last := min(v.offset+max(1, v.Inner.Dy()), len(v.lines))
		v.TitleBottom = fmt.Sprintf("%d-%d/%d PgUp/Dn/Wheel", first, last, len(v.lines))
	}
}

func formatTranscriptRows(rows []string, width int) [][]ui.Cell {
	var output [][]ui.Cell
	color := ""
	for _, row := range rows {
		if strings.HasPrefix(visibleStyledText(row), "╭─ YOU ") {
			color = colorUser
			output = append(output, wrapTranscriptCells(ui.ParseStyles(row, ui.NewStyle(ui.ColorClear)), width)...)
			continue
		}
		if strings.HasPrefix(visibleStyledText(row), "╭─ AGENT ") {
			color = colorThinking
			output = append(output, wrapTranscriptCells(ui.ParseStyles(row, ui.NewStyle(ui.ColorClear)), width)...)
			continue
		}
		if strings.HasPrefix(visibleStyledText(row), "╰─") {
			output = append(output, wrapTranscriptCells(ui.ParseStyles(row, ui.NewStyle(ui.ColorClear)), width)...)
			color = ""
			continue
		}
		cells := ui.ParseStyles(row, ui.NewStyle(ui.ColorClear))
		if color == "" {
			for _, line := range wrapTranscriptCells(cells, max(1, width)) {
				output = append(output, line)
			}
			continue
		}
		prefix := ui.ParseStyles(styledMarkdown("│ ", "fg:"+color+",mod:bold"), ui.NewStyle(ui.ColorClear))
		for _, line := range wrapTranscriptCells(cells, max(1, width-2)) {
			output = append(output, append(append([]ui.Cell(nil), prefix...), line...))
		}
	}
	return output
}

func borderedMessages(rows []string, width int) []string {
	var result, message []string
	speaker := ""
	flush := func() {
		if speaker == "" {
			return
		}
		color, label := colorUser, " YOU "
		if speaker == "AGENT" {
			color, label = colorThinking, " AGENT "
		}
		left := "╭─" + label
		if len([]rune(left)) < width {
			left += strings.Repeat("─", width-len([]rune(left)))
		}
		result = append(result, styledMarkdown(left, "fg:"+color+",mod:bold"))
		result = append(result, message...)
		result = append(result, styledMarkdown("╰"+strings.Repeat("─", max(0, width-1)), "fg:"+color+",mod:bold"))
		message = nil
		speaker = ""
	}
	for _, row := range rows {
		switch {
		case strings.Contains(row, "[YOU](fg:"):
			flush()
			speaker = "YOU"
		case strings.Contains(row, "[AGENT](fg:"):
			flush()
			speaker = "AGENT"
		case isActivityRow(row):
			flush()
			result = append(result, row)
		default:
			if speaker != "" {
				message = append(message, row)
			} else {
				result = append(result, row)
			}
		}
	}
	flush()
	return result
}

func isActivityRow(row string) bool {
	for _, prefix := range []string{
		"[... think]", "[> read]", "[> list]", "[> write]", "[> remove]", "[> run]", "[> call]",
		"[read]", "[+ ", "[~ ", "[- ", "[! failed]", "[· out]", "[· err]", "[ok]", "[· note]", "[· skipped]",
	} {
		if strings.HasPrefix(row, prefix) {
			return true
		}
	}
	return false
}

func wrapTranscriptCells(cells []ui.Cell, width int) [][]ui.Cell {
	var lines [][]ui.Cell
	for len(cells) > 0 {
		if cells[0].Rune == '\n' {
			lines = append(lines, nil)
			cells = cells[1:]
			continue
		}
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

func activityCounts(reads, checks, waits, output int) string {
	var counts []string
	if reads > 0 {
		counts = append(counts, pluralCount(reads, "read"))
	}
	if checks > 0 {
		counts = append(counts, pluralCount(checks, "check"))
	}
	if waits > 0 {
		counts = append(counts, pluralCount(waits, "model request"))
	}
	if output > 0 {
		counts = append(counts, pluralCount(output, "output line"))
	}
	counts = append(counts, "Ctrl+O expand")
	return " · " + strings.Join(counts, " · ")
}

func pluralCount(count int, singular string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ss", count, singular)
}

func (v *transcriptView) bottom() int    { return max(0, len(v.lines)-v.Inner.Dy()) }
func (v *transcriptView) AtBottom() bool { return v.offset >= v.bottom() }
func (v *transcriptView) ScrollPageUp()  { v.offset = max(0, v.offset-max(1, v.Inner.Dy()-2)) }
func (v *transcriptView) ScrollPageDown() {
	v.offset = min(v.bottom(), v.offset+max(1, v.Inner.Dy()-2))
}
func (v *transcriptView) ScrollLines(lines int) {
	v.offset = min(v.bottom(), max(0, v.offset+lines))
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
		out = append(out, styledMarkdown("Agent activity", "fg:cyan,mod:bold")+
			activityCounts(reads, checks, waits, output), last)
		reads, checks, waits, output = 0, 0, 0, 0
		last = ""
	}
	activity := false
	for _, row := range rows {
		switch {
		case strings.HasPrefix(row, "[> read]") || strings.HasPrefix(row, "[> list]"):
			activity = true
			reads++
			last = row
		case strings.HasPrefix(row, "[> run]"):
			activity = true
			checks++
			last = row
		case strings.HasPrefix(row, "[... think]"):
			activity = true
			waits++
			last = row
		case strings.HasPrefix(row, "[read]"):
			if activity {
				output++
				last = row
			} else {
				out = append(out, row)
			}
		case strings.HasPrefix(row, "[· out]") || strings.HasPrefix(row, "[· err]"):
			if activity {
				output++
			} else {
				out = append(out, row)
			}
		case strings.HasPrefix(row, "[AGENT]") || strings.HasPrefix(row, "[YOU]"):
			flush()
			activity = false
			out = append(out, row)
		case isActivityRow(row):
			flush()
			activity = false
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
