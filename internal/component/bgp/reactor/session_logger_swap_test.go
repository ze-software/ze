package reactor

import "log/slog"

// swapSessionLogger overrides the session subsystem logger provider and returns a restore
// func (test-only). The override goes through the same atomic.Pointer that sessionLogger()
// reads, so it is safe against the cold-path callers on background goroutines (timer
// callbacks, the cancel goroutine) that a plain package-var swap would race under stress.
// Used by the RFC 7606 diagnostics cost tests to force a Debug-disabled logger.
func swapSessionLogger(fn func() *slog.Logger) (restore func()) {
	prev := sessionLoggerRef.Load()
	next := sessionLoggerProvider(fn)
	sessionLoggerRef.Store(&next)
	return func() { sessionLoggerRef.Store(prev) }
}
