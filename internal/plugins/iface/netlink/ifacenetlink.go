// Design: docs/features/interfaces.md -- Netlink interface backend

// Package ifacenetlink implements the netlink-based interface management
// backend for Linux. It registers itself with the iface component as the
// "netlink" backend via iface.RegisterBackend.
//
// On non-Linux platforms, stub implementations return "not supported" errors.
package ifacenetlink

import (
	"log/slog"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}
