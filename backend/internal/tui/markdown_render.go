package tui

import (
	"fmt"
	"html"
	"strings"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

type markdownTranscript struct {
	source []byte
	rows   []string
}

func (r *markdownTranscript) add(line string) { r.rows = append(r.rows, line) }
func (r *markdownTranscript) blank() {
	if len(r.rows) > 0 && r.rows[len(r.rows)-1] != "" {
		r.add("")
	}
}
func (r *markdownTranscript) trimmedRows() []string {
	for len(r.rows) > 0 && r.rows[len(r.rows)-1] == "" {
		r.rows = r.rows[:len(r.rows)-1]
	}
	return r.rows
}
func (r *markdownTranscript) children(node ast.Node, prefix string) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		r.block(child, prefix)
	}
}
func (r *markdownTranscript) block(node ast.Node, prefix string) {
	switch n := node.(type) {
	case *ast.Heading:
		r.blank()
		r.add(prefix + r.inline(n, "fg:"+colorAccent+",mod:bold"))
		r.blank()
	case *ast.Paragraph, *ast.TextBlock:
		r.add(prefix + r.inline(node, ""))
		r.blank()
	case *ast.List:
		index := n.Start
		for item := n.FirstChild(); item != nil; item = item.NextSibling() {
			marker := "• "
			if n.IsOrdered() {
				marker = fmt.Sprintf("%d. ", index)
				index++
			}
			child := markdownTranscript{source: r.source}
			child.children(item, "")
			for i, line := range child.trimmedRows() {
				if i == 0 {
					r.add(prefix + marker + line)
				} else {
					r.add(prefix + strings.Repeat(" ", len(marker)) + line)
				}
			}
		}
		r.blank()
	case *ast.Blockquote:
		r.children(n, prefix+"│ ")
	case *ast.FencedCodeBlock:
		r.code(n, prefix, string(n.Language(r.source)))
	case *ast.CodeBlock:
		r.code(n, prefix, "")
	case *ast.ThematicBreak:
		r.blank()
		r.add(prefix + styledMarkdown("────────────────────────", "fg:darkgrey"))
		r.blank()
	case *extast.Table:
		r.table(n, prefix)
	default:
		if node.FirstChild() != nil {
			r.children(node, prefix)
		} else if node.Type() == ast.TypeBlock {
			for i := 0; i < node.Lines().Len(); i++ {
				segment := node.Lines().At(i)
				r.add(prefix + styledMarkdown(strings.TrimSuffix(string(segment.Value(r.source)), "\n"), ""))
			}
		}
	}
}
func (r *markdownTranscript) code(node ast.Node, prefix, language string) {
	r.blank()
	label := "Code"
	if language != "" {
		label += " · " + language
	}
	r.add(prefix + styledMarkdown(label, "fg:"+colorChanged+",mod:bold"))
	for i := 0; i < node.Lines().Len(); i++ {
		segment := node.Lines().At(i)
		line := strings.TrimSuffix(string(segment.Value(r.source)), "\n")
		line = strings.ReplaceAll(line, "\t", "    ")
		r.add(prefix + styledMarkdown("│ ", "fg:darkgrey") + styledMarkdown(line, "fg:"+colorChanged))
	}
	r.blank()
}
func (r *markdownTranscript) inline(node ast.Node, style string) string {
	var out strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *ast.Text:
			out.WriteString(styledMarkdown(html.UnescapeString(string(n.Segment.Value(r.source))), style))
			if n.HardLineBreak() {
				out.WriteByte('\n')
			} else if n.SoftLineBreak() {
				out.WriteByte(' ')
			}
		case *ast.String:
			out.WriteString(styledMarkdown(string(n.Value), style))
		case *ast.Emphasis:
			childStyle := style
			if childStyle == "" {
				if n.Level == 2 {
					childStyle = "mod:bold"
				} else {
					childStyle = "mod:italic"
				}
			}
			out.WriteString(r.inline(n, childStyle))
		case *ast.CodeSpan:
			out.WriteString(styledMarkdown(strings.ReplaceAll(string(n.Text(r.source)), "\n", " "), "fg:"+colorChanged))
		case *ast.Link:
			out.WriteString(r.inline(n, "fg:"+colorRead))
			if destination := string(n.Destination); destination != "" {
				out.WriteString(styledMarkdown(" ("+destination+")", "fg:darkgrey"))
			}
		case *ast.AutoLink:
			out.WriteString(styledMarkdown(string(n.Label(r.source)), "fg:"+colorRead))
		case *extast.Strikethrough:
			out.WriteString(r.inline(n, "mod:strike"))
		default:
			if child.FirstChild() != nil {
				out.WriteString(r.inline(child, style))
			} else {
				out.WriteString(styledMarkdown(string(child.Text(r.source)), style))
			}
		}
	}
	return out.String()
}

func visibleStyledText(value string) string {
	var out strings.Builder
	for _, cell := range ui.ParseStyles(value, ui.NewStyle(ui.ColorClear)) {
		out.WriteRune(cell.Rune)
	}
	return out.String()
}

func (r *markdownTranscript) table(table *extast.Table, prefix string) {
	var rows [][]string
	var widths []int
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			style := ""
			if len(rows) == 0 {
				style = "mod:bold"
			}
			value := r.inline(cell, style)
			index := len(cells)
			if index >= len(widths) {
				widths = append(widths, 0)
			}
			widths[index] = max(widths[index], runewidth.StringWidth(visibleStyledText(value)))
			cells = append(cells, value)
		}
		rows = append(rows, cells)
	}
	total := max(0, len(widths)-1) * 3
	for _, width := range widths {
		total += width
	}
	r.blank()
	if total > 96 && len(rows) > 1 {
		// Wide tables become labelled rows instead of overflowing the reading column.
		for _, row := range rows[1:] {
			for i, value := range row {
				label := ""
				if i < len(rows[0]) {
					label = visibleStyledText(rows[0][i]) + ": "
				}
				r.add(prefix + styledMarkdown(label, "mod:bold") + value)
			}
			r.blank()
		}
		return
	}
	for _, row := range rows {
		var out strings.Builder
		for i, value := range row {
			if i > 0 {
				out.WriteString(" │ ")
			}
			out.WriteString(value)
			out.WriteString(strings.Repeat(" ", max(0, widths[i]-runewidth.StringWidth(visibleStyledText(value)))))
		}
		r.add(prefix + out.String())
	}
	r.blank()
}
