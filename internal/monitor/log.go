package monitor

import (
	"context"
	"log/slog"
	"os"
)

type contextKey struct{}

var loggerKey = contextKey{}

// NewLogger creates a new default JSON logger.
func NewLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

// WithContext returns a new context with the logger attached.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from the context.
// If no logger is found, returns a default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}

	return sLogger
}

var sLogger = NewLogger(slog.LevelInfo)
