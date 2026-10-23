package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type textHandler struct {
	out   io.Writer
	mu    *sync.Mutex
	level slog.Level
	attrs []slog.Attr
	group string
}

func newTextHandler(out io.Writer, level slog.Level) *textHandler {
	return &textHandler{out: out, mu: &sync.Mutex{}, level: level}
}
func (h *textHandler) Enabled(_ context.Context, level slog.Level) bool { return level >= h.level }
func (h *textHandler) Handle(_ context.Context, record slog.Record) error {
	var line strings.Builder
	fmt.Fprintf(&line, "level=%s msg=%q", record.Level, record.Message)
	appendAttr := func(a slog.Attr) bool { h.writeAttr(&line, h.group, a); return true }
	for _, a := range h.attrs {
		h.writeAttr(&line, "", a)
	}
	record.Attrs(appendAttr)
	line.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, line.String())
	return err
}
func (h *textHandler) writeAttr(line *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, child := range a.Value.Group() {
			h.writeAttr(line, prefix, child)
		}
		return
	}
	fmt.Fprintf(line, " %s=%q", safeKey(prefix+a.Key), a.Value.String())
}
func safeKey(key string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, key)
}
func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append([]slog.Attr(nil), h.attrs...)
	for _, a := range attrs {
		if h.group != "" {
			a.Key = h.group + a.Key
		}
		clone.attrs = append(clone.attrs, a)
	}
	return &clone
}
func (h *textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.group += name + "."
	return &clone
}
