package tui

import (
	"strings"
	"unicode"
)

func cleanAssistantMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) && r != '\n' {
			return -1
		}
		return r
	}, value)
}

func appendAssistantMarkdown(rows []string, content string) []string {
	rows = appendTranscriptRow(rows, "[AI](fg:"+colorThinking+",mod:bold) Agent")
	content = strings.TrimSpace(cleanAssistantMarkdown(content))
	if len(content) > maxActivityOutput {
		content = content[:maxActivityOutput] + "\n[response clipped for display]"
	}

	var paragraph []string
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		rows = appendTranscriptRow(rows, formatInlineMarkdown(strings.Join(paragraph, " "), true))
		paragraph = nil
	}
	spacer := func() {
		if len(rows) > 0 && rows[len(rows)-1] != "" {
			rows = appendTranscriptRow(rows, "")
		}
	}

	inCode := false
	for _, raw := range splitAssistantHeadings(content) {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			flushParagraph()
			spacer()
			if inCode {
				inCode = false
			} else {
				language := strings.TrimSpace(strings.TrimPrefix(line, "```"))
				label := "Code"
				if language != "" {
					label += " · " + language
				}
				rows = appendTranscriptRow(rows, "["+label+"](fg:"+colorChanged+",mod:bold)")
				inCode = true
			}
			continue
		}
		if line == "" {
			flushParagraph()
			spacer()
			continue
		}
		if inCode {
			rows = appendTranscriptRow(rows, "  [│](fg:darkgrey) "+line)
			continue
		}
		if level, title, ok := markdownHeading(line); ok {
			flushParagraph()
			title, body := splitGluedHeading(title)
			spacer()
			prefix := strings.Repeat("  ", level-1)
			rows = appendTranscriptRow(rows, prefix+styledMarkdown(stripInlineMarkdown(title), "fg:"+colorAccent+",mod:bold))
			if body != "" {
				body = strings.TrimLeft(body, ":- \t")
				if body != "" {
					paragraph = append(paragraph, body)
					flushParagraph()
				}
			}
			spacer()
			continue
		}
		if isMarkdownRule(line) {
			flushParagraph()
			spacer()
			rows = appendTranscriptRow(rows, "[────────────────](fg:darkgrey)")
			spacer()
			continue
		}
		if isMarkdownTableRow(line) {
			flushParagraph()
			cells := tableCells(line)
			if !isTableDivider(cells) {
				for i := range cells {
					cells[i] = formatInlineMarkdown(cells[i], true)
				}
				rows = appendTranscriptRow(rows, "  "+strings.Join(cells, "  │  "))
			}
			continue
		}
		if item, ok := markdownListItem(line); ok {
			flushParagraph()
			rows = appendTranscriptRow(rows, "  "+item.prefix+formatInlineMarkdown(item.text, true))
			continue
		}
		if strings.HasPrefix(line, "> ") {
			flushParagraph()
			rows = appendTranscriptRow(rows, "[│](fg:lightblue) "+formatInlineMarkdown(strings.TrimSpace(line[2:]), true))
			continue
		}
		paragraph = append(paragraph, line)
	}
	flushParagraph()
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	return rows
}

func appendTranscriptRow(rows []string, row string) []string {
	if len(rows) == maxActivityRows {
		copy(rows, rows[1:])
		rows[len(rows)-1] = row
		return rows
	}
	return append(rows, row)
}

func splitAssistantHeadings(content string) []string {
	var lines []string
	inCode := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			lines = append(lines, line)
			continue
		}
		if inCode {
			lines = append(lines, line)
			continue
		}
		start := 0
		for i := 0; i < len(line); i++ {
			if line[i] == '#' && isMarkdownHeadingAt(line, i) && i > start {
				if before := strings.TrimSpace(line[start:i]); before != "" {
					lines = append(lines, before)
				}
				start = i
				i += headingMarkerLength(line, i) - 1
				continue
			}
			if line[i] == '-' && i > start && i+1 < len(line) && line[i+1] == ' ' && isSentenceBoundary(line[i-1]) {
				if before := strings.TrimSpace(line[start:i]); before != "" {
					lines = append(lines, before)
				}
				start = i
			}
		}
		if tail := strings.TrimSpace(line[start:]); tail != "" {
			lines = append(lines, tail)
		} else if line == "" {
			lines = append(lines, "")
		}
	}
	return lines
}

func markdownHeading(line string) (int, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level := headingMarkerLength(line, 0)
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level:]), true
}

func isMarkdownHeadingAt(line string, at int) bool {
	level := headingMarkerLength(line, at)
	return level > 0 && level <= 6 && at+level < len(line) && line[at+level] == ' '
}

func headingMarkerLength(line string, at int) int {
	end := at
	for end < len(line) && line[end] == '#' {
		end++
	}
	return end - at
}

func splitGluedHeading(title string) (string, string) {
	known := []string{"Architecture Diagram", "Implementation Summary", "Expected Endpoints", "Final Report", "Test Results", "Summary", "Overview", "Architecture", "Features", "Testing", "Security", "Checks"}
	for _, candidate := range known {
		if len(title) <= len(candidate) || !strings.EqualFold(title[:len(candidate)], candidate) {
			continue
		}
		rest := title[len(candidate):]
		first := strings.TrimLeft(rest, ":- \t")
		if len(first) < len(rest) || (len(rest) > 0 && firstRuneIsUpper(rest)) {
			return candidate, first
		}
	}
	return title, ""
}

func firstRuneIsUpper(value string) bool {
	for _, r := range value {
		return unicode.IsUpper(r)
	}
	return false
}

func isSentenceBoundary(char byte) bool {
	return strings.ContainsRune(".:;!?)]", rune(char))
}

func isMarkdownRule(line string) bool {
	line = strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	return len(line) >= 3 && (strings.Trim(line, "-") == "" || strings.Trim(line, "*") == "" || strings.Trim(line, "_") == "")
}

func isMarkdownTableRow(line string) bool {
	return strings.Count(line, "|") >= 2 && (strings.HasPrefix(strings.TrimSpace(line), "|") || strings.HasSuffix(strings.TrimSpace(line), "|"))
}

func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isTableDivider(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.Trim(cell, ":-")
		if cell != "" {
			return false
		}
	}
	return true
}

type markdownItem struct {
	prefix string
	text   string
}

func markdownListItem(line string) (markdownItem, bool) {
	line = strings.TrimSpace(line)
	if len(line) > 2 && strings.ContainsRune("-*+", rune(line[0])) && line[1] == ' ' {
		return markdownItem{prefix: "• ", text: strings.TrimSpace(line[2:])}, true
	}
	for i := 0; i < len(line); i++ {
		if line[i] < '0' || line[i] > '9' {
			break
		}
		if i+1 < len(line) && (line[i+1] == '.' || line[i+1] == ')') && i+2 < len(line) && line[i+2] == ' ' {
			return markdownItem{prefix: line[:i+1] + ". ", text: strings.TrimSpace(line[i+3:])}, true
		}
	}
	return markdownItem{}, false
}

func formatInlineMarkdown(value string, styled bool) string {
	var result strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '[' {
			linkEnd := strings.Index(value[i:], "](")
			if linkEnd > 0 {
				urlEnd := strings.Index(value[i+linkEnd+2:], ")")
				if urlEnd >= 0 {
					label := value[i+1 : i+linkEnd]
					if styled {
						result.WriteString(formatInlineMarkdown(label, false))
					} else {
						result.WriteString(label)
					}
					i += linkEnd + 2 + urlEnd + 1
					continue
				}
			}
		}
		marker := ""
		style := ""
		switch {
		case strings.HasPrefix(value[i:], "***"):
			marker, style = "***", "mod:bold"
		case strings.HasPrefix(value[i:], "**"):
			marker, style = "**", "mod:bold"
		case strings.HasPrefix(value[i:], "__"):
			marker, style = "__", "mod:bold"
		case value[i] == '`':
			marker, style = "`", "fg:"+colorChanged
		case value[i] == '*' && i+1 < len(value) && value[i+1] != ' ':
			marker, style = "*", "mod:italic"
		}
		if marker != "" {
			end := strings.Index(value[i+len(marker):], marker)
			if end >= 0 {
				body := value[i+len(marker) : i+len(marker)+end]
				if styled {
					result.WriteString(styledMarkdown(stripInlineMarkdown(body), style))
				} else {
					result.WriteString(stripInlineMarkdown(body))
				}
				i += len(marker) + end + len(marker)
				continue
			}
		}
		result.WriteByte(value[i])
		i++
	}
	return result.String()
}

func stripInlineMarkdown(value string) string {
	return formatInlineMarkdown(value, false)
}

func styledMarkdown(value, style string) string {
	return "[" + escapeUnbalancedBrackets(value) + "](" + style + ")"
}

func escapeUnbalancedBrackets(value string) string {
	runes := []rune(value)
	var openings []int
	for i, char := range runes {
		switch char {
		case '[':
			openings = append(openings, i)
		case ']':
			if len(openings) == 0 {
				runes[i] = '］'
			} else {
				openings = openings[:len(openings)-1]
			}
		}
	}
	for _, i := range openings {
		runes[i] = '［'
	}
	return string(runes)
}
