// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin
// Design: rfc/short/rfc4761.md
//
// Package bgp_vpls implements a VPLS family plugin for ze.
// It handles VPLS NLRI (RFC 4761, SAFI 65).
package bgp_nlri_vpls

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

// RunVPLSPlugin runs the VPLS plugin using the SDK RPC protocol.
func RunVPLSPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("vpls plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-vpls", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "l2vpn/vpls", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("vpls plugin failed", "error", err)
		return 1
	}

	return 0
}
