package tui

import (
	"fmt"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxMentionEntries = 500

type fileMention struct {
	list        *widgets.List
	root, token string
	start, end  int
	paths       []string
	visible     bool
	dismissed   string
	error       string
}

func newFileMention() *fileMention {
	l := widgets.NewList()
	l.Border = true
	l.Title = "Project files"
	themeList(l)
	return &fileMention{list: l}
}

// refresh lists only the current @path directory; query and result counts stay bounded.
func (m *fileMention) refresh(root, token string, start, end int) {
	m.root, m.token, m.start, m.end = root, token, start, end
	m.visible = true
	m.error = ""
	fragment := strings.TrimPrefix(token, "@")
	directory, query := filepath.Split(filepath.FromSlash(fragment))
	directory = filepath.Clean(directory)
	if directory == "." {
		directory = ""
	}
	base := filepath.Join(root, directory)
	relative, err := filepath.Rel(root, base)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		m.visible = false
		m.error = "Path must stay within this project"
		return
	}
	entries, err := readMentionEntries(base)
	if err != nil {
		m.visible = false
		m.error = fmt.Sprintf("Cannot list %s: %s", filepath.ToSlash(directory), err)
		return
	}
	needle := strings.ToLower(query)
	m.paths = m.paths[:0]
	for _, entry := range entries {
		name := entry.Name()
		if !strings.Contains(strings.ToLower(name), needle) {
			continue
		}
		path := filepath.ToSlash(filepath.Join(directory, name))
		if entry.IsDir() {
			path += "/"
		}
		m.paths = append(m.paths, path)
		if len(m.paths) >= maxMentionEntries {
			break
		}
	}
	m.list.SelectedRow = 0
}

func readMentionEntries(path string) ([]os.DirEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(maxMentionEntries + 1)
	if err != nil && len(entries) == 0 {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if len(entries) > maxMentionEntries {
		entries = entries[:maxMentionEntries]
	}
	return entries, nil
}

func mentionAtCursor(c *messageComposer) (string, int, int, bool) {
	lines := strings.Split(c.Text, "\n")
	if c.Cursor.Y < 0 || c.Cursor.Y >= len(lines) {
		return "", 0, 0, false
	}
	line := []rune(lines[c.Cursor.Y])
	x := min(c.Cursor.X, len(line))
	start := -1
	for i := x - 1; i >= 0; i-- {
		if line[i] == ' ' || line[i] == '\t' {
			break
		}
		if line[i] == '@' {
			start = i
			break
		}
	}
	if start < 0 {
		return "", 0, 0, false
	}
	var absolute int
	for i := 0; i < c.Cursor.Y; i++ {
		absolute += len([]rune(lines[i])) + 1
	}
	absolute += start
	return string(line[start:x]), absolute, absolute + x - start, true
}

func (m *fileMention) active(root string, c *messageComposer) bool {
	token, start, end, ok := mentionAtCursor(c)
	signature := fmt.Sprintf("%s:%d:%d:%s", c.Text, c.Cursor.Y, c.Cursor.X, root)
	if !ok || signature == m.dismissed {
		m.visible = false
		return false
	}
	if token != m.token || root != m.root || start != m.start || end != m.end {
		m.refresh(root, token, start, end)
	}
	return m.visible
}

func (m *fileMention) draw(root string, c *messageComposer, width, end int) int {
	if !m.active(root, c) {
		return 0
	}
	if !m.visible {
		return 0
	}
	rows := make([]string, 0, len(m.paths))
	for _, path := range m.paths {
		if strings.HasSuffix(path, "/") {
			rows = append(rows, styledMarkdown("DIR  ", "fg:cyan,mod:bold")+path)
		} else {
			rows = append(rows, styledMarkdown("FILE ", "fg:lightblue,mod:bold")+path)
		}
	}
	if len(rows) == 0 {
		rows = []string{"No matching project files"}
	}
	m.list.Rows = rows
	m.list.Title = "@ files and folders · select to attach a path"
	m.list.SelectedRow = min(m.list.SelectedRow, len(rows)-1)
	visibleRows := min(8, len(rows))
	popupHeight := visibleRows + 2
	m.list.SetRect(0, end-popupHeight, width, end)
	ui.Render(m.list)
	return popupHeight
}

func (m *fileMention) choose(c *messageComposer) bool {
	if !m.visible || len(m.paths) == 0 || m.list.SelectedRow < 0 || m.list.SelectedRow >= len(m.paths) {
		return false
	}
	selected := m.paths[m.list.SelectedRow]
	replacement := "@" + selected
	if strings.HasSuffix(selected, "/") {
		replacement += ""
	} else {
		replacement += " "
	}
	runes := []rune(c.Text)
	insert := []rune(replacement)
	next := make([]rune, 0, len(runes)+len(insert))
	next = append(next, runes[:m.start]...)
	next = append(next, insert...)
	next = append(next, runes[m.end:]...)
	c.Text = string(next)
	cursor := m.start + len(insert)
	prefix := string(next[:cursor])
	parts := strings.Split(prefix, "\n")
	c.Cursor = image.Pt(len([]rune(parts[len(parts)-1])), len(parts)-1)
	m.visible = false
	m.token = ""
	return true
}

func (m *fileMention) dismiss(c *messageComposer) {
	_, _, _, ok := mentionAtCursor(c)
	if ok {
		m.dismissed = fmt.Sprintf("%s:%d:%d:%s", c.Text, c.Cursor.Y, c.Cursor.X, m.root)
	}
	m.visible = false
}
