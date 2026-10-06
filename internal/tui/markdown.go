package tui

import (
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

func cleanAssistantMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, value)
}

func appendAssistantMarkdown(rows []string, content string) []string {
	rows = appendTranscriptRow(rows, "")
	rows = appendTranscriptRow(rows, "[AGENT](fg:"+colorThinking+",mod:bold)")
	rows = appendTranscriptRow(rows, "")
	content = strings.TrimSpace(cleanAssistantMarkdown(content))
	if len(content) > maxActivityOutput {
		content = content[:maxActivityOutput] + "\n\nResponse clipped for display."
	}
	source := []byte(normalizeLegacyMarkdown(content))
	markdown := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	document := markdown.Parser().Parse(text.NewReader(source))
	renderer := markdownTranscript{source: source}
	renderer.children(document, "")
	for _, line := range renderer.trimmedRows() {
		rows = appendTranscriptRow(rows, line)
	}
	return rows
}

func appendTranscriptRow(rows []string, row string) []string {
	if len(rows) >= maxActivityRows {
		rows = rows[len(rows)-maxActivityRows+1:]
	}
	return append(rows, row)
}

// Older model replies sometimes joined headings and sentences without line breaks.
// Only repair structural markers outside fenced and inline code; Goldmark parses the rest.
func normalizeLegacyMarkdown(content string) string {
	var output []string
	fenced := false
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
			output = append(output, raw)
			continue
		}
		if fenced {
			output = append(output, raw)
			continue
		}
		start, code := 0, false
		var pieces []string
		for i := 0; i < len(raw); i++ {
			if raw[i] == '`' {
				code = !code
				continue
			}
			if code {
				continue
			}
			if raw[i] == '#' && (i == 0 || raw[i-1] != '#') {
				end := i
				for end < len(raw) && raw[end] == '#' {
					end++
				}
				if end-i <= 6 && end < len(raw) && raw[end] == ' ' {
					if i > start && strings.TrimSpace(raw[start:i]) != "" {
						pieces = append(pieces, raw[start:i])
					}
					start = i
					i = end - 1
				}
			} else if raw[i] == '-' && i > start && i+1 < len(raw) && raw[i+1] == ' ' && sentenceBoundaryBefore(raw, i) {
				pieces = append(pieces, raw[start:i])
				start = i
			}
		}
		pieces = append(pieces, raw[start:])
		for _, piece := range pieces {
			trimmed := strings.TrimSpace(piece)
			if strings.HasPrefix(trimmed, "#") {
				end := strings.IndexByte(trimmed, ' ')
				if end > 0 {
					title, body := splitGluedHeading(trimmed[end+1:])
					if body != "" {
						output = append(output, trimmed[:end]+" "+title, "", body)
						continue
					}
				}
			}
			output = append(output, piece)
		}
	}
	return strings.Join(output, "\n")
}

func splitGluedHeading(title string) (string, string) {
	for _, candidate := range []string{"Architecture Diagram", "Implementation Summary", "Expected Endpoints", "Final Report", "Test Results", "Summary", "Overview", "Architecture", "Features", "Testing", "Security", "Checks"} {
		if len(title) <= len(candidate) || !strings.EqualFold(title[:len(candidate)], candidate) {
			continue
		}
		rest := title[len(candidate):]
		for _, first := range rest {
			if unicode.IsUpper(first) || first == ':' {
				return candidate, strings.TrimLeft(rest, ": \t")
			}
			break
		}
	}
	return title, ""
}

func sentenceBoundaryBefore(line string, at int) bool {
	for at > 0 && (line[at-1] == ' ' || line[at-1] == '\t') {
		at--
	}
	return at > 0 && strings.ContainsRune(".:;!?)]", rune(line[at-1]))
}

func styledMarkdown(value, style string) string {
	if value == "" {
		return ""
	}
	if style == "" {
		style = "mod:clear"
	}
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
