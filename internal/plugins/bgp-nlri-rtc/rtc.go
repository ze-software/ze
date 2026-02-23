// Design: docs/architecture/wire/nlri.md — route target constraint plugin
// RFC: rfc/short/rfc4684.md
//
// Package bgp_rtc implements a Route Target Constraint family plugin for ze.
// It handles RTC NLRI (RFC 4684, SAFI 132).
package bgp_nlri_rtc

import (
	"context"
	"log/slog"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// RunRTCPlugin runs the RTC plugin using the SDK RPC protocol.
func RunRTCPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("rtc plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-rtc", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/rtc", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("rtc plugin failed", "error", err)
		return 1
	}

	return 0
}
