package tui

import (
	"context"
	"fmt"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/llm"
	"projemble/internal/llm/factory"
	"strings"
	"time"
)

type modelResult struct {
	models []string
	err    error
}
type modelPicker struct {
	list     *widgets.List
	search   *widgets.Input
	provider llm.ProviderID
	key      string
	models   []string
	results  chan modelResult
	cancel   context.CancelFunc
	message  string
	custom   bool
	current  string
	querying bool
	query    string
}

func newModelPicker() *modelPicker {
	l := widgets.NewList()
	l.Border = true
	themeList(l)
	s := widgets.NewInput()
	s.Border = true
	s.Title = "Filter models"
	s.Placeholder = "Press / to search"
	themeInput(s)
	return &modelPicker{list: l, search: s}
}
func (p *modelPicker) close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.results = nil
}
func (p *modelPicker) prepare(options generationOptions) {
	p.current = options.Model
	if p.provider == options.Provider && p.key == options.APIKey {
		return
	}
	p.close()
	p.provider, p.key = options.Provider, options.APIKey
	p.models = nil
	p.custom = false
	p.query = ""
	p.querying = false
	p.message = "Loading provider models…"
	p.list.SelectedRow = 0
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	p.cancel = cancel
	p.results = make(chan modelResult, 1)
	results := p.results
	go func() {
		models, err := factory.ListModels(ctx, llm.Config{Provider: options.Provider, APIKey: options.APIKey})
		results <- modelResult{models, err}
	}()
}
func (p *modelPicker) receive(result modelResult) {
	p.models = result.models
	for i, model := range p.models {
		if model == p.current {
			p.list.SelectedRow = i + 1
			break
		}
	}
	p.message = fmt.Sprintf("%d available models", len(result.models))
	if result.err != nil {
		p.message = result.err.Error()
	}
	p.close()
}
func (p *modelPicker) render(input *widgets.Input, width, height int, validation string) {
	if p.custom {
		setInputLayout(input, width, height)
		setFooter(&input.Block, "Enter confirm model · Esc model list", false)
		if validation != "" {
			setFooter(&input.Block, validation, true)
		}
		ui.Render(input)
		return
	}
	selected := ""
	if p.list.SelectedRow >= 1 && p.list.SelectedRow < len(p.list.Rows) {
		selected = p.list.Rows[p.list.SelectedRow]
	}
	query := strings.ToLower(strings.TrimSpace(p.query))
	filtered := make([]string, 0, len(p.models))
	for _, model := range p.models {
		if query == "" || strings.Contains(strings.ToLower(model), query) {
			filtered = append(filtered, model)
		}
	}
	p.list.Title = "Provider models"
	rows := make([]string, 0, len(filtered)+1)
	rows = append(rows, "Enter a custom model ID")
	rows = append(rows, filtered...)
	p.list.Rows = rows
	if p.querying {
		p.list.SelectedRow = 0
		for i, model := range filtered {
			if model == selected {
				p.list.SelectedRow = i + 1
				break
			}
		}
	}
	if p.search.Text != p.query {
		p.search.Text = p.query
		p.search.Cursor = len([]rune(p.query))
	}
	p.search.Title = "Filter models · press / to search"
	p.search.CursorStyle = ui.NewStyle(ui.ColorClear)
	if p.querying {
		p.search.Title = "Filter models · type to narrow · Enter done · Esc clear"
		p.search.CursorStyle = focusedStyle()
	}
	searchHeight := 5
	if height < 12 {
		searchHeight = 4
	}
	p.search.SetRect(0, 0, width, searchHeight)
	p.list.SetRect(0, searchHeight, width, height)
	message := p.message
	if query != "" {
		message = fmt.Sprintf("%d matches", len(filtered))
	}
	footer := message + " · ↑/↓ select · Enter choose · / search · Ctrl+R reload · Esc back"
	if validation != "" {
		footer = validation
	}
	setFooter(&p.list.Block, footer, validation != "")
	ui.Render(p.search, p.list)
}
func (p *modelPicker) handle(event ui.Event, input *widgets.Input) (ui.Event, bool) {
	if p.custom {
		if isEscapeKey(event.ID) {
			p.custom = false
			return event, true
		}
		return event, false
	}
	if p.querying {
		switch event.ID {
		case "<Escape>":
			if p.query != "" {
				p.query = ""
				p.search.Text = ""
				p.search.Cursor = 0
			} else {
				p.querying = false
			}
			return event, true
		case "<Enter>":
			p.querying = false
			return event, true
		case "<Backspace>", "<C-h>":
			r := []rune(p.query)
			if len(r) > 0 {
				p.query = string(r[:len(r)-1])
			}
			p.search.Text = p.query
			p.search.Cursor = len([]rune(p.query))
			return event, true
		case "<C-u>":
			p.query = ""
			p.search.Text = ""
			p.search.Cursor = 0
			return event, true
		case "<Space>":
			p.query += " "
			p.search.Text = p.query
			return event, true
		case "<Up>":
			p.list.ScrollUp()
			return event, true
		case "<Down>":
			p.list.ScrollDown()
			return event, true
		default:
			if event.Type == ui.KeyboardEvent && len([]rune(event.ID)) == 1 {
				p.query += event.ID
				p.search.Text = p.query
				p.search.Cursor = len([]rune(p.query))
				return event, true
			}
		}
	}
	switch event.ID {
	case "/":
		p.querying = true
	case "<C-r>":
		p.provider = ""
	case "<Up>", "k":
		p.list.ScrollUp()
	case "<Down>", "j":
		p.list.ScrollDown()
	case "<PageUp>":
		p.list.ScrollPageUp()
	case "<PageDown>":
		p.list.ScrollPageDown()
	case "<Enter>":
		if p.list.SelectedRow == 0 {
			p.custom = true
			return event, true
		}
		i := p.list.SelectedRow - 1
		if i >= 0 && i < len(p.models) {
			selected := p.models[i]
			if i+1 < len(p.list.Rows) {
				selected = p.list.Rows[i+1]
			}
			input.Text = strings.TrimSpace(selected)
			return event, false
		}
	case "<Escape>", "<C-c>":
		return event, false
	default:
		if event.Type == ui.KeyboardEvent && len([]rune(event.ID)) == 1 {
			p.query = event.ID
			p.querying = true
			p.search.Text = p.query
		}
	}
	return event, true
}
