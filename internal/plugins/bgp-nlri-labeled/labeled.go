// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin
// RFC: rfc/short/rfc8277.md
//
// Package bgp_labeled implements a Labeled Unicast family plugin for ze.
// It handles Labeled Unicast NLRI (RFC 8277, SAFI 4).
package bgp_nlri_labeled

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

// RunLabeledPlugin runs the labeled unicast plugin using the SDK RPC protocol.
func RunLabeledPlugin(engineConn, callbackConn net.Conn) int {
	logger.Debug("labeled plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-labeled", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Families: []sdk.FamilyDecl{
			{Name: "ipv4/mpls-label", Mode: "decode"},
			{Name: "ipv6/mpls-label", Mode: "decode"},
		},
	})
	if err != nil {
		logger.Error("labeled plugin failed", "error", err)
		return 1
	}

	return 0
}
