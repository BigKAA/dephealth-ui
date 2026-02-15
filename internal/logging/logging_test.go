package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger_JSONDefault(t *testing.T) {
	cfg := LogConfig{Format: "json", Level: "info"}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewLogger_Text(t *testing.T) {
	cfg := LogConfig{Format: "text", Level: "debug"}
	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewHandler_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	cfg := LogConfig{Format: "json", Level: "info"}
	logger := slog.New(newHandler(cfg, &buf))

	logger.Info("test message", "key", "value")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON output: %v, raw: %s", err, buf.String())
	}
	if m["msg"] != "test message" {
		t.Errorf("expected msg=test message, got %v", m["msg"])
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
}

func TestNewHandler_TextOutput(t *testing.T) {
	var buf bytes.Buffer
	cfg := LogConfig{Format: "text", Level: "info"}
	logger := slog.New(newHandler(cfg, &buf))

	logger.Info("hello text")

	if !strings.Contains(buf.String(), "hello text") {
		t.Errorf("text output should contain message, got: %s", buf.String())
	}
}

func TestNewHandler_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := LogConfig{Format: "json", Level: "warn"}
	logger := slog.New(newHandler(cfg, &buf))

	logger.Info("should be filtered")
	if buf.Len() > 0 {
		t.Errorf("info message should be filtered at warn level, got: %s", buf.String())
	}

	logger.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("warn message should appear at warn level")
	}
}

func TestNewHandler_CustomKeys(t *testing.T) {
	var buf bytes.Buffer
	cfg := LogConfig{
		Format:     "json",
		Level:      "info",
		TimeKey:    "@timestamp",
		LevelKey:   "severity",
		MessageKey: "message",
	}
	logger := slog.New(newHandler(cfg, &buf))

	logger.Info("custom keys test")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := m["@timestamp"]; !ok {
		t.Error("expected @timestamp key")
	}
	if _, ok := m["severity"]; !ok {
		t.Error("expected severity key")
	}
	if _, ok := m["message"]; !ok {
		t.Error("expected message key")
	}
	if _, ok := m["time"]; ok {
		t.Error("default time key should be replaced")
	}
	if _, ok := m["level"]; ok {
		t.Error("default level key should be replaced")
	}
	if _, ok := m["msg"]; ok {
		t.Error("default msg key should be replaced")
	}
}

func TestNewHandler_TimeFormatRFC3339(t *testing.T) {
	var buf bytes.Buffer
	cfg := LogConfig{Format: "json", Level: "info", TimeFormat: "rfc3339"}
	logger := slog.New(newHandler(cfg, &buf))

	logger.Info("time test")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	ts, ok := m["time"].(string)
	if !ok {
		t.Fatal("expected string time value")
	}
	if strings.Contains(ts, ".") {
		t.Errorf("rfc3339 should not have fractional seconds, got: %s", ts)
	}
}

func TestNewHandler_TimeFormatUnix(t *testing.T) {
	var buf bytes.Buffer
	cfg := LogConfig{Format: "json", Level: "info", TimeFormat: "unix"}
	logger := slog.New(newHandler(cfg, &buf))

	logger.Info("unix time test")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	ts, ok := m["time"].(float64)
	if !ok {
		t.Fatalf("expected numeric time value, got %T: %v", m["time"], m["time"])
	}
	if ts < 1e9 {
		t.Errorf("unix timestamp seems too small: %f", ts)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"INFO", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"invalid", "INFO"},
		{"", "INFO"},
	}
	for _, tt := range tests {
		level := parseLevel(tt.input)
		if level.String() != tt.want {
			t.Errorf("parseLevel(%q) = %s, want %s", tt.input, level.String(), tt.want)
		}
	}
}
