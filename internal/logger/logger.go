// Package logger provides structured logging for the DAST crawler.
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Level represents log levels.
type Level = zerolog.Level

// Log levels.
const (
	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
	FatalLevel = zerolog.FatalLevel
)

// Logger wraps zerolog for structured logging.
type Logger struct {
	zl zerolog.Logger
}

// Config holds logger configuration.
type Config struct {
	Level      Level
	Pretty     bool   // Use console writer (colored output)
	Output     io.Writer
	TimeFormat string
	Component  string // Component name (e.g., "crawler", "browser", "queue")
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Level:      InfoLevel,
		Pretty:     true,
		Output:     os.Stderr,
		TimeFormat: time.RFC3339,
	}
}

// New creates a new logger with the given configuration.
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = time.RFC3339
	}

	zerolog.TimeFieldFormat = cfg.TimeFormat

	var output io.Writer = cfg.Output

	if cfg.Pretty {
		output = zerolog.ConsoleWriter{
			Out:        cfg.Output,
			TimeFormat: "15:04:05",
			NoColor:    false,
		}
	}

	zl := zerolog.New(output).
		With().
		Timestamp().
		Logger().
		Level(cfg.Level)

	if cfg.Component != "" {
		zl = zl.With().Str("component", cfg.Component).Logger()
	}

	return &Logger{zl: zl}
}

// NewDefault creates a logger with default configuration.
func NewDefault() *Logger {
	return New(DefaultConfig())
}

// NewJSON creates a JSON-only logger (no pretty printing).
func NewJSON(level Level) *Logger {
	return New(Config{
		Level:  level,
		Pretty: false,
		Output: os.Stderr,
	})
}

// WithComponent returns a new logger with the component field set.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		zl: l.zl.With().Str("component", component).Logger(),
	}
}

// WithField returns a new logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		zl: l.zl.With().Interface(key, value).Logger(),
	}
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.zl.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{zl: ctx.Logger()}
}

// WithURL returns a new logger with URL field.
func (l *Logger) WithURL(url string) *Logger {
	return &Logger{
		zl: l.zl.With().Str("url", url).Logger(),
	}
}

// WithWorker returns a new logger with worker ID field.
func (l *Logger) WithWorker(workerID int) *Logger {
	return &Logger{
		zl: l.zl.With().Int("worker_id", workerID).Logger(),
	}
}

// WithDepth returns a new logger with depth field.
func (l *Logger) WithDepth(depth int) *Logger {
	return &Logger{
		zl: l.zl.With().Int("depth", depth).Logger(),
	}
}

// WithError returns a new logger with error field.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		zl: l.zl.With().Err(err).Logger(),
	}
}

// WithDuration returns a new logger with duration field.
func (l *Logger) WithDuration(d time.Duration) *Logger {
	return &Logger{
		zl: l.zl.With().Dur("duration", d).Logger(),
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string) {
	l.zl.Debug().Msg(msg)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.zl.Debug().Msgf(format, args...)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	l.zl.Info().Msg(msg)
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.zl.Info().Msgf(format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	l.zl.Warn().Msg(msg)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.zl.Warn().Msgf(format, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.zl.Error().Msg(msg)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.zl.Error().Msgf(format, args...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string) {
	l.zl.Fatal().Msg(msg)
}

// Fatalf logs a formatted fatal message and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.zl.Fatal().Msgf(format, args...)
}

// Event returns a zerolog Event for complex logging.
func (l *Logger) Event(level Level) *zerolog.Event {
	switch level {
	case DebugLevel:
		return l.zl.Debug()
	case InfoLevel:
		return l.zl.Info()
	case WarnLevel:
		return l.zl.Warn()
	case ErrorLevel:
		return l.zl.Error()
	case FatalLevel:
		return l.zl.Fatal()
	default:
		return l.zl.Info()
	}
}

// CrawlEvent logs a crawl-related event with standard fields.
func (l *Logger) CrawlEvent(level Level, url string, depth int, workerID int) *zerolog.Event {
	return l.Event(level).
		Str("url", url).
		Int("depth", depth).
		Int("worker_id", workerID)
}

// RequestEvent logs an HTTP request event.
func (l *Logger) RequestEvent(method, url string, statusCode int, duration time.Duration) {
	l.zl.Info().
		Str("method", method).
		Str("url", url).
		Int("status_code", statusCode).
		Dur("duration", duration).
		Msg("HTTP request")
}

// DiscoveryEvent logs a discovery event.
func (l *Logger) DiscoveryEvent(discoveryType, url, source string) {
	l.zl.Info().
		Str("type", discoveryType).
		Str("url", url).
		Str("source", source).
		Msg("Discovered endpoint")
}

// ErrorEvent logs an error event with context.
func (l *Logger) ErrorEvent(err error, url string, operation string) {
	l.zl.Error().
		Err(err).
		Str("url", url).
		Str("operation", operation).
		Msg("Operation failed")
}

// StatsEvent logs statistics.
func (l *Logger) StatsEvent(stats map[string]interface{}) {
	event := l.zl.Info()
	for k, v := range stats {
		event = event.Interface(k, v)
	}
	event.Msg("Crawl statistics")
}

// SetLevel changes the log level.
func (l *Logger) SetLevel(level Level) {
	l.zl = l.zl.Level(level)
}

// ParseLevel parses a level string.
func ParseLevel(levelStr string) (Level, error) {
	return zerolog.ParseLevel(levelStr)
}

// Global logger instance.
var globalLogger = NewDefault()

// SetGlobal sets the global logger.
func SetGlobal(l *Logger) {
	globalLogger = l
}

// Global returns the global logger.
func Global() *Logger {
	return globalLogger
}

// Package-level convenience functions using global logger.

// Debug logs a debug message using the global logger.
func Debug(msg string) {
	globalLogger.Debug(msg)
}

// Debugf logs a formatted debug message using the global logger.
func Debugf(format string, args ...interface{}) {
	globalLogger.Debugf(format, args...)
}

// Info logs an info message using the global logger.
func Info(msg string) {
	globalLogger.Info(msg)
}

// Infof logs a formatted info message using the global logger.
func Infof(format string, args ...interface{}) {
	globalLogger.Infof(format, args...)
}

// Warn logs a warning message using the global logger.
func Warn(msg string) {
	globalLogger.Warn(msg)
}

// Warnf logs a formatted warning message using the global logger.
func Warnf(format string, args ...interface{}) {
	globalLogger.Warnf(format, args...)
}

// Error logs an error message using the global logger.
func Error(msg string) {
	globalLogger.Error(msg)
}

// Errorf logs a formatted error message using the global logger.
func Errorf(format string, args ...interface{}) {
	globalLogger.Errorf(format, args...)
}
