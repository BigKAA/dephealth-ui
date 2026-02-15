// Package logging provides configurable slog.Logger construction.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// LogConfig holds logging configuration.
type LogConfig struct {
	Format     string `yaml:"format"`     // "text" or "json" (default: "json")
	Level      string `yaml:"level"`      // "debug", "info", "warn", "error" (default: "info")
	TimeFormat string `yaml:"timeFormat"` // "rfc3339", "rfc3339nano", "unix", "unixmilli" (default: "rfc3339nano")
	AddSource  bool   `yaml:"addSource"`  // include file:line in log output (default: false)
	TimeKey    string `yaml:"timeKey"`    // JSON key for timestamp (empty = slog default "time")
	LevelKey   string `yaml:"levelKey"`   // JSON key for level (empty = slog default "level")
	MessageKey string `yaml:"messageKey"` // JSON key for message (empty = slog default "msg")
	SourceKey  string `yaml:"sourceKey"`  // JSON key for source (empty = slog default "source")
}

// NewLogger creates a configured *slog.Logger from LogConfig.
// Output is always os.Stdout.
func NewLogger(cfg LogConfig) *slog.Logger {
	return slog.New(newHandler(cfg, os.Stdout))
}

// newHandler creates a slog.Handler writing to w.
func newHandler(cfg LogConfig, w io.Writer) slog.Handler {
	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	switch cfg.Format {
	case "text":
		return slog.NewTextHandler(w, opts)
	default: // "json"
		opts.ReplaceAttr = buildReplaceAttr(cfg)
		return slog.NewJSONHandler(w, opts)
	}
}

// parseLevel converts a string to slog.Level. Defaults to Info on error.
func parseLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return slog.LevelInfo
	}
	return level
}

// buildReplaceAttr creates a ReplaceAttr function for the JSON handler
// that customizes time format and key names.
func buildReplaceAttr(cfg LogConfig) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) != 0 {
			return a
		}

		switch a.Key {
		case slog.TimeKey:
			if cfg.TimeKey != "" {
				a.Key = cfg.TimeKey
			}
			if t, ok := a.Value.Any().(time.Time); ok {
				switch cfg.TimeFormat {
				case "rfc3339":
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				case "unix":
					a = slog.Int64(a.Key, t.Unix())
				case "unixmilli":
					a = slog.Int64(a.Key, t.UnixMilli())
				// "rfc3339nano" = slog default, no transform needed
				}
			}
		case slog.LevelKey:
			if cfg.LevelKey != "" {
				a.Key = cfg.LevelKey
			}
		case slog.MessageKey:
			if cfg.MessageKey != "" {
				a.Key = cfg.MessageKey
			}
		case slog.SourceKey:
			if cfg.SourceKey != "" {
				a.Key = cfg.SourceKey
			}
		}
		return a
	}
}
