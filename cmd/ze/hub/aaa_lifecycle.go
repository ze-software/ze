// Design: .claude/patterns/registration.md -- AAA registry (VFS-like)
// Related: infra_setup.go -- infraSetup installs the bundle on config load
// Related: main.go -- runYANGConfig defers closeAAABundle on exit

package hub

import (
	"log/slog"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// aaaBundle holds the live AAA bundle.
//
// Swapped atomically on every infraSetup invocation (which runs on initial
// startup and on each config reload). The previously installed bundle is
// Close()d by swapAAABundle so backend workers (TACACS+ accounting) drain
// cleanly across reloads.
//
// closeAAABundle is wired as a defer at the top of runYANGConfig so the
// currently-installed bundle is Close()d on any exit path (clean shutdown,
// error return, panic recovery).
var aaaBundle atomic.Pointer[aaa.Bundle]

// swapAAABundle installs the new bundle as the live one and closes the
// previously installed bundle (if any). Safe to call concurrently; safe
// with a nil b (treated as "unregister without replacement").
func swapAAABundle(b *aaa.Bundle, logger *slog.Logger) {
	prev := aaaBundle.Swap(b)
	if prev != nil && prev != b {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: previous bundle close error on swap", "error", err)
		}
	}
}

// closeAAABundle closes whatever bundle is currently installed and clears
// the slot. Called via defer from runYANGConfig so the TACACS+ accounting
// worker drains on every exit path.
func closeAAABundle(logger *slog.Logger) {
	prev := aaaBundle.Swap(nil)
	if prev != nil {
		if err := prev.Close(); err != nil && logger != nil {
			logger.Warn("aaa: bundle close error on shutdown", "error", err)
		}
	}
}
