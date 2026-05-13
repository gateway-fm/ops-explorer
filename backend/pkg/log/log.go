// Package log is the project's structured logging facade.
//
// It is a thin wrapper over log/slog that ships a pretty terminal handler
// (see terminal_handler.go) and exposes a process-wide root logger (see
// root.go) so callers can write log.Info(...) from anywhere without threading
// a *slog.Logger through every function.
//
// Usage:
//
//	log.Info("server started", "port", 8080)
//	log.Error("failed to connect", "error", err)
//	log.With("component", "indexer").Info("processing block", "number", 123)
//
// Levels: Debug, Info, Warn, Error. Fatal is provided for unrecoverable
// startup failures and exits the process — never call it from a request path.
package log

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// Debug logs at debug level. Use for verbose diagnostics that are off by default.
func Debug(msg string, args ...any) {
	write(context.Background(), slog.LevelDebug, msg, args...)
}

// Info logs at info level. Use for routine operational events.
func Info(msg string, args ...any) {
	write(context.Background(), slog.LevelInfo, msg, args...)
}

// Warn logs at warn level. Use for recoverable problems that deserve attention.
func Warn(msg string, args ...any) {
	write(context.Background(), slog.LevelWarn, msg, args...)
}

// Error logs at error level. Use for failures the caller could not handle.
func Error(msg string, args ...any) {
	write(context.Background(), slog.LevelError, msg, args...)
}

// Fatal logs at error level and exits the process with status 1.
// Reserve for unrecoverable startup failures.
func Fatal(msg string, args ...any) {
	write(context.Background(), slog.LevelError, msg, args...)
	os.Exit(1)
}

// With returns a child logger that carries the supplied attrs on every record.
// Pair keys and values as you would for slog: With("component", "rpc").
func With(args ...any) *slog.Logger {
	return root().With(args...)
}

// WithGroup returns a child logger that nests subsequent attrs under name.
func WithGroup(name string) *slog.Logger {
	return root().WithGroup(name)
}

// write is the shared entry point for the level-specific helpers above.
// It captures the caller's PC (skipping write + the public helper) so the
// terminal handler can render an accurate source location.
func write(ctx context.Context, level slog.Level, msg string, args ...any) {
	l := root()
	if !l.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)

	_ = l.Handler().Handle(ctx, r)
}
