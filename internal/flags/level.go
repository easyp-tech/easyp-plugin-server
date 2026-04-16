package flags

import (
	"flag"
	"fmt"
	"log/slog"
)

var _ flag.Value = (*Level)(nil)

// Level for setting level by flag.
type Level struct {
	Level slog.Level
}

// String implements flag.Value.
func (l *Level) String() string {
	return l.Level.String()
}

// Set implements flag.Value.
func (l *Level) Set(s string) error {
	err := l.Level.UnmarshalText([]byte(s))
	if err != nil {
		return fmt.Errorf("parse log level: %w", err)
	}

	return nil
}
