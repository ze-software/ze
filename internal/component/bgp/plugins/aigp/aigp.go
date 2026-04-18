// Design: (none -- predates documentation)
// RFC: rfc/short/rfc7311.md
//
// Package aigp implements a stub plugin for the AIGP (Accumulated IGP) attribute.
// RFC 7311: The Accumulated IGP Metric Attribute for BGP.
//
// This is a placeholder that registers the AIGP attribute type (code 26).
// Full AIGP processing will be added when the spec-aigp work is implemented.
package aigp

import (
	"log/slog"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetAIGPLogger sets the package-level logger.
func SetAIGPLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RunAIGPPlugin is the in-process entry point. Stub: runs the SDK event loop with no handlers.
func RunAIGPPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-aigp", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		logger().Error("aigp plugin failed", "error", err)
		return 1
	}
	return 0
}
