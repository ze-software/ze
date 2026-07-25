package format

import (
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
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
	id, _ := bgpctx.Registry.Register(ctx)
	return id
}
