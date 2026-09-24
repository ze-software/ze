// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7311.md
//
// Package aigp registers AIGP attribute presentation. Session policy,
// accumulation and metric-triggered forwarding live in the reactor; best-path
// selection lives in the RIB.
package aigp

import (
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setAIGPLogger sets the package-level logger.
func setAIGPLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// runAIGPPlugin serves the attribute plugin's SDK lifecycle.
func runAIGPPlugin(conn net.Conn) int {
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
