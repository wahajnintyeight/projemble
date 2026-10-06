package tui

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
)

const maxMentionEntries = 120

type fileMention struct {
	list            *widgets.List
	root, token     string
	start, end      int
	paths           []string
	visible         bool
	dismissed       string
	error           string
	indexRoot       string
	indexGeneration uint64
	index           []string
	indexReady      bool
	indexing        bool
	truncated       bool
	indexCancel     context.CancelFunc
	updates         chan mentionIndexUpdate
}

func newFileMention() *fileMention {
	l := widgets.NewList()
	l.Border = true
	l.Title = "Project files"
	themeList(l)
	return &fileMention{list: l, updates: make(chan mentionIndexUpdate, 1)}
}

func (m *fileMention) refresh(root, token string, start, end int) {
	selectedPath := ""
	if m.list.SelectedRow >= 0 && m.list.SelectedRow < len(m.paths) {
		selectedPath = m.paths[m.list.SelectedRow]
	}
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
	if directory == "" {
		m.ensureIndex(root)
	}
	var candidates []string
	if directory != "" {
		entries, err := readMentionEntries(base)
		if err != nil {
			m.visible = false
			m.error = fmt.Sprintf("Cannot list %s: %s", filepath.ToSlash(directory), err)
			return
		}
		for _, entry := range entries {
			if entry.IsDir() && ignoredMentionDirectory(entry.Name()) {
				continue
			}
			path := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			if entry.IsDir() {
				path += "/"
			}
			candidates = append(candidates, path)
		}
	} else {
		candidates = append(candidates, m.index...)
		if len(candidates) == 0 {
			entries, err := readMentionEntries(base)
			if err != nil {
				m.visible = false
				m.error = err.Error()
				return
			}
			for _, entry := range entries {
				if entry.IsDir() && ignoredMentionDirectory(entry.Name()) {
					continue
				}
				path := entry.Name()
				if entry.IsDir() {
					path += "/"
				}
				candidates = append(candidates, path)
			}
		}
	}
	m.paths = rankMentionPaths(candidates, query, maxMentionEntries)
	m.list.SelectedRow = 0
	for i, path := range m.paths {
		if path == selectedPath {
			m.list.SelectedRow = i
			break
		}
	}
}

func (m *fileMention) ensureIndex(root string) {
	if m.indexRoot == root && (m.indexing || m.indexReady) {
		return
	}
	if m.indexCancel != nil {
		m.indexCancel()
	}
	m.indexRoot, m.index, m.indexReady, m.truncated, m.indexing = root, nil, false, false, true
	m.indexGeneration++
	generation := m.indexGeneration
	ctx, cancel := context.WithCancel(context.Background())
	m.indexCancel = cancel
	updates := m.updates
	go func() {
		paths, truncated, err := scanMentionTree(ctx, root, maxIndexedEntries)
		if ctx.Err() != nil {
			return
		}
		updates <- mentionIndexUpdate{root: root, generation: generation, paths: paths, truncated: truncated, err: err}
	}()
}

func (m *fileMention) receive(update mentionIndexUpdate) {
	if update.root != m.indexRoot || update.generation != m.indexGeneration {
		return
	}
	m.index, m.indexReady, m.truncated, m.indexing = update.paths, true, update.truncated, false
	if m.indexCancel != nil {
		m.indexCancel()
		m.indexCancel = nil
	}
	if update.err != nil {
		m.error = update.err.Error()
	}
	m.token = "" // refresh the active query with the finished tree index.
}

func (m *fileMention) reloadIndex() {
	if m.indexCancel != nil {
		m.indexCancel()
		m.indexCancel = nil
	}
	m.indexRoot, m.index, m.indexReady, m.truncated, m.indexing = "", nil, false, false, false
	m.indexGeneration++
	m.token = ""
}

func readMentionEntries(path string) ([]os.DirEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(maxMentionEntries + 1)
	if err != nil && err != io.EOF && len(entries) == 0 {
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
		if line[i] == '\n' {
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

func (m *fileMention) draw(root string, c *messageComposer, width, end, available int) int {
	if !m.active(root, c) {
		return 0
	}
	if !m.visible {
		return 0
	}
	rows := make([]string, 0, len(m.paths))
	for _, path := range m.paths {
		if strings.HasSuffix(path, "/") {
			rows = append(rows, styledMarkdown(filepath.FromSlash(path), "fg:cyan,mod:bold"))
		} else {
			rows = append(rows, filepath.FromSlash(path))
		}
	}
	if len(rows) == 0 {
		rows = []string{"No matching project files"}
	}
	m.list.Rows = rows
	m.list.Title = fmt.Sprintf("@%s | %d matches | Up/Down select | Enter attach/open | Ctrl+R refresh | Esc close", strings.TrimPrefix(m.token, "@"), len(m.paths))
	if m.indexing {
		m.list.Title = fmt.Sprintf("@%s | indexing in background | current-folder matches | Ctrl+R refresh", strings.TrimPrefix(m.token, "@"))
	}
	if m.truncated {
		m.list.Title += " | index limit reached; browse folders to continue"
	}
	m.list.SelectedRow = min(m.list.SelectedRow, len(rows)-1)
	visibleRows := min(8, len(rows), max(1, available-2))
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
