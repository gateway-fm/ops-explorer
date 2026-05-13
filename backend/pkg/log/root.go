package log

import (
	"log/slog"
	"os"
	"sync/atomic"
)

// defaultLogger holds the process-wide root logger used by the package-level
// helpers (log.Info, log.Error, ...). It is read on every log call, so the
// atomic.Pointer keeps reconfiguration lock-free.
var defaultLogger atomic.Pointer[slog.Logger]

func init() {
	SetDefault(New(nil))
}

// New constructs a logger that writes to stderr through the terminal handler.
// Pass nil for opts to accept the defaults (info level, no source location).
func New(opts *HandlerOptions) *slog.Logger {
	return slog.New(NewTerminalHandler(os.Stderr, opts))
}

// SetDefault replaces the root logger. It also installs the logger as slog's
// own default so anything that reaches for slog.Default() picks up the same
// formatting.
func SetDefault(l *slog.Logger) {
	defaultLogger.Store(l)
	slog.SetDefault(l)
}

// Default returns the current root logger.
func Default() *slog.Logger {
	return defaultLogger.Load()
}

// SetLevel rebuilds the root logger with the supplied threshold and enables
// source locations for error-level records.
func SetLevel(level slog.Level) {
	SetDefault(New(&HandlerOptions{Level: level, ShowSource: true}))
}

// root is the internal accessor used by the level helpers. Kept separate from
// Default() so the hot path can stay unexported and inlineable.
func root() *slog.Logger {
	return defaultLogger.Load()
}
