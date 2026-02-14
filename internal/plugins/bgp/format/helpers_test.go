package format

import (
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
)

// testEncodingContext creates an encoding context for tests.
func testEncodingContext() bgpctx.ContextID {
	ctx := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{
			LocalASN: 65001,
			PeerASN:  65001,
		},
		&capability.EncodingCaps{
			ASN4: true,
		},
		bgpctx.DirectionRecv,
	)
	return bgpctx.Registry.Register(ctx)
}
