// Package log provides structured logging with a pretty terminal handler.
//
// Usage:
//
//	log.Info("server started", "port", 8080)
//	log.Error("failed to connect", "err", err)
//	log.With("component", "indexer").Info("processing block", "number", 123)
//
// The package uses slog under the hood with a custom terminal handler that
// provides colored output with clear level identification.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

var (
	// defaultLogger is the root logger used by package-level functions.
	defaultLogger atomic.Pointer[slog.Logger]
)

func init() {
	// Initialize with default terminal handler
	SetDefault(New(nil))
}

// New creates a new logger with the given options.
// If opts is nil, default options are used.
func New(opts *HandlerOptions) *slog.Logger {
	handler := NewTerminalHandler(os.Stderr, opts)
	return slog.New(handler)
}

// SetDefault sets the default logger used by package-level functions.
func SetDefault(l *slog.Logger) {
	defaultLogger.Store(l)
	// Also set as slog default for compatibility
	slog.SetDefault(l)
}

// Default returns the default logger.
func Default() *slog.Logger {
	return defaultLogger.Load()
}

// SetLevel sets the minimum log level for the default logger.
func SetLevel(level slog.Level) {
	handler := NewTerminalHandler(os.Stderr, &HandlerOptions{
		Level:      level,
		ShowSource: true,
	})
	SetDefault(slog.New(handler))
}

// With returns a logger with the given attributes added to every log.
// This is useful for adding context like component names:
//
//	logger := log.With("component", "indexer")
//	logger.Info("starting")
func With(args ...any) *slog.Logger {
	return Default().With(args...)
}

// WithGroup returns a logger that prefixes all messages with the group name.
//
//	logger := log.WithGroup("indexer")
//	logger.Info("starting") // Output: INF indexer: starting
func WithGroup(name string) *slog.Logger {
	return Default().WithGroup(name)
}

// Debug logs a message at debug level.
func Debug(msg string, args ...any) {
	log(context.Background(), slog.LevelDebug, msg, args...)
}

// Debugf logs a formatted message at debug level.
func Debugf(format string, args ...any) {
	log(context.Background(), slog.LevelDebug, fmt.Sprintf(format, args...))
}

// Info logs a message at info level.
func Info(msg string, args ...any) {
	log(context.Background(), slog.LevelInfo, msg, args...)
}

// Infof logs a formatted message at info level.
func Infof(format string, args ...any) {
	log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Warn logs a message at warn level.
func Warn(msg string, args ...any) {
	log(context.Background(), slog.LevelWarn, msg, args...)
}

// Warnf logs a formatted message at warn level.
func Warnf(format string, args ...any) {
	log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Error logs a message at error level.
func Error(msg string, args ...any) {
	log(context.Background(), slog.LevelError, msg, args...)
}

// Errorf logs a formatted message at error level.
func Errorf(format string, args ...any) {
	log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}

// Fatal logs a message at error level and then calls os.Exit(1).
func Fatal(msg string, args ...any) {
	log(context.Background(), slog.LevelError, msg, args...)
	os.Exit(1)
}

// Fatalf logs a formatted message at error level and then calls os.Exit(1).
func Fatalf(format string, args ...any) {
	log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Print logs a message at info level (compatibility with std log).
func Print(args ...any) {
	log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// Println logs a message at info level (compatibility with std log).
func Println(args ...any) {
	log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// Printf logs a formatted message at info level (compatibility with std log).
func Printf(format string, args ...any) {
	log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

// log is the internal logging function that captures the correct source location.
func log(ctx context.Context, level slog.Level, msg string, args ...any) {
	l := Default()
	if !l.Enabled(ctx, level) {
		return
	}

	// Get the caller's PC for source location (skip 3: log(), the public function, and runtime.Callers)
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)

	_ = l.Handler().Handle(ctx, r)
}

// Logger is a convenience wrapper around slog.Logger that provides
// the same API as the package-level functions.
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new Logger with the given group name.
func NewLogger(group string) *Logger {
	return &Logger{Logger: Default().WithGroup(group)}
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...any) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...any) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...any) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}
