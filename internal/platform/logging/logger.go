package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type Logger struct {
	base *slog.Logger
}

func New(level, format string) *Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	if strings.EqualFold(format, "text") {
		h = slog.NewTextHandler(os.Stderr, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	return &Logger{base: slog.New(h)}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{base: l.base.With(args...)}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}
	traceID, _ := ctx.Value("trace_id").(string)
	if traceID == "" {
		return l
	}
	return l.With("trace_id", traceID)
}

func (l *Logger) Info(msg string, args ...any) {
	l.base.Info(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.base.Error(msg, args...)
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
