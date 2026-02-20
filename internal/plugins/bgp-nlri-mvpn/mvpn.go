// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin
//
// Package bgp_mvpn implements a Multicast VPN family plugin for ze.
// It handles MVPN NLRI (RFC 6514, SAFI 5).
package bgp_nlri_mvpn

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

// RunMVPNPlugin runs the MVPN plugin using the SDK RPC protocol.
func RunMVPNPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("mvpn plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-mvpn", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/mvpn", Mode: "decode"},
			{Name: "ipv6/mvpn", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("mvpn plugin failed", "error", err)
		return 1
	}

	return 0
}
