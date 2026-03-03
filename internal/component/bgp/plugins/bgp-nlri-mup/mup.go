// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin
// RFC: rfc/short/draft-ietf-bess-mup-safi.md
//
// Package bgp_mup implements a Mobile User Plane family plugin for ze.
// It handles MUP NLRI (draft-mpmz-bess-mup-safi, SAFI 85).
package bgp_nlri_mup

import (
	"context"
	"log/slog"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// RunMUPPlugin runs the MUP plugin using the SDK RPC protocol.
func RunMUPPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("mup plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-mup", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/mup", Mode: "decode"},
			{Name: "ipv6/mup", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("mup plugin failed", "error", err)
		return 1
	}

	return 0
}
