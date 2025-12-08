package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		result := tt.level.String()
		if result != tt.expected {
			t.Errorf("LogLevel(%d).String() = '%s', expected '%s'", tt.level, result, tt.expected)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", LogLevelDebug},
		{"DEBUG", LogLevelDebug},
		{"info", LogLevelInfo},
		{"INFO", LogLevelInfo},
		{"warn", LogLevelWarn},
		{"WARN", LogLevelWarn},
		{"warning", LogLevelWarn},
		{"WARNING", LogLevelWarn},
		{"error", LogLevelError},
		{"ERROR", LogLevelError},
		{"unknown", LogLevelInfo}, // Default
		{"", LogLevelInfo},        // Default
	}

	for _, tt := range tests {
		result := ParseLogLevel(tt.input)
		if result != tt.expected {
			t.Errorf("ParseLogLevel('%s') = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestNewLogger(t *testing.T) {
	cfg := LoggerConfig{
		Level:  LogLevelDebug,
		Prefix: "test",
	}

	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}
}

func TestNewLogger_DefaultOutput(t *testing.T) {
	cfg := LoggerConfig{
		Output: nil, // Should default to stderr
	}

	logger := NewLogger(cfg)
	if logger.output == nil {
		t.Error("expected default output to be set")
	}
}

func TestLogger_Log(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelDebug,
		Output: &buf,
		Prefix: "test",
	})

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG]") {
		t.Error("expected DEBUG in output")
	}
	if !strings.Contains(output, "[INFO]") {
		t.Error("expected INFO in output")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("expected WARN in output")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("expected ERROR in output")
	}
	if !strings.Contains(output, "test:") {
		t.Error("expected prefix in output")
	}
}

func TestLogger_LogLevel_Filtering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelWarn,
		Output: &buf,
	})

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	output := buf.String()
	if strings.Contains(output, "[DEBUG]") {
		t.Error("expected DEBUG to be filtered out")
	}
	if strings.Contains(output, "[INFO]") {
		t.Error("expected INFO to be filtered out")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("expected WARN in output")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("expected ERROR in output")
	}
}

func TestLogger_Format(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelInfo,
		Output: &buf,
	})

	logger.Info("formatted %s %d", "test", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted test 42") {
		t.Errorf("expected formatted message, got: %s", output)
	}
}

func TestLogger_WithField(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelInfo,
		Output: &buf,
	})

	logger2 := logger.WithField("key", "value")
	logger2.Info("test")

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected field in output, got: %s", output)
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelInfo,
		Output: &buf,
	})

	logger2 := logger.WithFields(map[string]any{
		"key1": "value1",
		"key2": 42,
	})
	logger2.Info("test")

	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Errorf("expected key1 in output, got: %s", output)
	}
	if !strings.Contains(output, "key2=42") {
		t.Errorf("expected key2 in output, got: %s", output)
	}
}

func TestLogger_WithComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelInfo,
		Output: &buf,
	})

	logger2 := logger.WithComponent("lsp")
	logger2.Info("test")

	output := buf.String()
	if !strings.Contains(output, "component=lsp") {
		t.Errorf("expected component in output, got: %s", output)
	}
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelError,
		Output: &buf,
	})

	logger.Info("should not appear")
	if buf.Len() != 0 {
		t.Error("expected no output at error level")
	}

	logger.SetLevel(LogLevelInfo)
	logger.Info("should appear")
	if buf.Len() == 0 {
		t.Error("expected output after SetLevel")
	}
}

func TestLogger_SetOutput(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelInfo,
		Output: &buf1,
	})

	logger.Info("to buf1")
	if buf1.Len() == 0 {
		t.Error("expected output to buf1")
	}

	logger.SetOutput(&buf2)
	logger.Info("to buf2")
	if buf2.Len() == 0 {
		t.Error("expected output to buf2")
	}
}

func TestLogger_DisableEnable(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LoggerConfig{
		Level:  LogLevelInfo,
		Output: &buf,
	})

	logger.Disable()
	logger.Info("should not appear")
	if buf.Len() != 0 {
		t.Error("expected no output when disabled")
	}

	logger.Enable()
	logger.Info("should appear")
	if buf.Len() == 0 {
		t.Error("expected output when enabled")
	}
}

func TestNullLogger(t *testing.T) {
	// NullLogger should not panic
	NullLogger.Debug("test")
	NullLogger.Info("test")
	NullLogger.Warn("test")
	NullLogger.Error("test")
}

func TestGetLogger(t *testing.T) {
	logger := GetLogger()
	if logger == nil {
		t.Fatal("GetLogger() returned nil")
	}

	// Should return same instance
	logger2 := GetLogger()
	if logger != logger2 {
		t.Error("expected GetLogger() to return same instance")
	}
}

func TestDefaultLoggerConfig(t *testing.T) {
	cfg := DefaultLoggerConfig()

	if cfg.Level != LogLevelInfo {
		t.Errorf("expected default level INFO, got %d", cfg.Level)
	}
	if cfg.Output == nil {
		t.Error("expected default output to be set")
	}
	if cfg.Prefix != "keystorm" {
		t.Errorf("expected prefix 'keystorm', got '%s'", cfg.Prefix)
	}
}
