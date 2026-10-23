package logging

import (
	"context"
	"io"
	"log/slog"
)

type Logger interface {
	Debug(string, ...any)
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
	With(...any) Logger
}

type logger struct{ inner *slog.Logger }

func New(out io.Writer, level string) Logger {
	var minimum slog.Level
	_ = minimum.UnmarshalText([]byte(level))
	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: minimum})
	return logger{slog.New(handler)}
}
func (l logger) Debug(msg string, values ...any) {
	l.inner.DebugContext(context.Background(), msg, values...)
}
func (l logger) Info(msg string, values ...any) {
	l.inner.InfoContext(context.Background(), msg, values...)
}
func (l logger) Warn(msg string, values ...any) {
	l.inner.WarnContext(context.Background(), msg, values...)
}
func (l logger) Error(msg string, values ...any) {
	l.inner.ErrorContext(context.Background(), msg, values...)
}
func (l logger) With(values ...any) Logger { return logger{l.inner.With(values...)} }
