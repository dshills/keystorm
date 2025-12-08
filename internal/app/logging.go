// Package app provides the main application structure and coordination.
package app

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log message.
type LogLevel int

const (
	// LogLevelDebug is for detailed debugging information.
	LogLevelDebug LogLevel = iota
	// LogLevelInfo is for general informational messages.
	LogLevelInfo
	// LogLevelWarn is for warning messages.
	LogLevelWarn
	// LogLevelError is for error messages.
	LogLevelError
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a string into a LogLevel.
func ParseLogLevel(s string) LogLevel {
	switch s {
	case "debug", "DEBUG":
		return LogLevelDebug
	case "info", "INFO":
		return LogLevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return LogLevelWarn
	case "error", "ERROR":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// Logger provides structured logging for the application.
type Logger struct {
	mu       sync.Mutex
	level    LogLevel
	output   io.Writer
	prefix   string
	fields   map[string]any
	disabled bool
}

// LoggerConfig configures the logger.
type LoggerConfig struct {
	// Level is the minimum log level to output.
	Level LogLevel
	// Output is where logs are written. Defaults to os.Stderr.
	Output io.Writer
	// Prefix is prepended to all log messages.
	Prefix string
}

// DefaultLoggerConfig returns the default logger configuration.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  LogLevelInfo,
		Output: os.Stderr,
		Prefix: "keystorm",
	}
}

// NewLogger creates a new logger with the given configuration.
func NewLogger(cfg LoggerConfig) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	return &Logger{
		level:  cfg.Level,
		output: cfg.Output,
		prefix: cfg.Prefix,
		fields: make(map[string]any),
	}
}

// WithField returns a new logger with the given field added.
func (l *Logger) WithField(key string, value any) *Logger {
	newFields := make(map[string]any, len(l.fields)+1)
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &Logger{
		level:    l.level,
		output:   l.output,
		prefix:   l.prefix,
		fields:   newFields,
		disabled: l.disabled,
	}
}

// WithFields returns a new logger with the given fields added.
func (l *Logger) WithFields(fields map[string]any) *Logger {
	newFields := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		level:    l.level,
		output:   l.output,
		prefix:   l.prefix,
		fields:   newFields,
		disabled: l.disabled,
	}
}

// WithComponent returns a new logger with the component field set.
func (l *Logger) WithComponent(component string) *Logger {
	return l.WithField("component", component)
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// Disable disables all logging.
func (l *Logger) Disable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.disabled = true
}

// Enable enables logging.
func (l *Logger) Enable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.disabled = false
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...any) {
	l.log(LogLevelDebug, msg, args...)
}

// Info logs an info message.
func (l *Logger) Info(msg string, args ...any) {
	l.log(LogLevelInfo, msg, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...any) {
	l.log(LogLevelWarn, msg, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, args ...any) {
	l.log(LogLevelError, msg, args...)
}

// log writes a log message if the level is enabled.
func (l *Logger) log(level LogLevel, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.disabled || level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02T15:04:05.000")

	// Format message with args if provided
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	// Build log line
	var line string
	if l.prefix != "" {
		line = fmt.Sprintf("%s [%s] %s: %s", timestamp, level.String(), l.prefix, msg)
	} else {
		line = fmt.Sprintf("%s [%s] %s", timestamp, level.String(), msg)
	}

	// Append fields if any
	if len(l.fields) > 0 {
		line += " {"
		first := true
		for k, v := range l.fields {
			if !first {
				line += ", "
			}
			line += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		line += "}"
	}

	line += "\n"

	// Write to output
	_, _ = l.output.Write([]byte(line))
}

// NullLogger is a logger that discards all output.
var NullLogger = &Logger{disabled: true}

// appLogger is the application-wide logger instance.
var (
	appLogger     *Logger
	appLoggerOnce sync.Once
)

// GetLogger returns the application logger.
// Creates a default logger on first call if not set.
func GetLogger() *Logger {
	appLoggerOnce.Do(func() {
		if appLogger == nil {
			appLogger = NewLogger(DefaultLoggerConfig())
		}
	})
	return appLogger
}

// SetLogger sets the application-wide logger.
// Should be called early in application startup.
func SetLogger(l *Logger) {
	appLogger = l
}

// Logger returns the application's logger instance.
func (app *Application) Logger() *Logger {
	if app.logger == nil {
		return GetLogger()
	}
	return app.logger
}

// SetAppLogger sets the application's logger.
func (app *Application) SetAppLogger(l *Logger) {
	app.logger = l
}

// LogDebug logs a debug message using the application logger.
func (app *Application) LogDebug(msg string, args ...any) {
	app.Logger().Debug(msg, args...)
}

// LogInfo logs an info message using the application logger.
func (app *Application) LogInfo(msg string, args ...any) {
	app.Logger().Info(msg, args...)
}

// LogWarn logs a warning message using the application logger.
func (app *Application) LogWarn(msg string, args ...any) {
	app.Logger().Warn(msg, args...)
}

// LogError logs an error message using the application logger.
func (app *Application) LogError(msg string, args ...any) {
	app.Logger().Error(msg, args...)
}

// logComponentError logs an error with component context.
func (app *Application) logComponentError(component string, err error) {
	if err != nil {
		app.Logger().WithComponent(component).Error("error: %v", err)
	}
}
