package reactor

import (
	"maps"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// fwdContextIDWithASN4 returns a context ID matching the same wire NLRI framing
// as srcCtxID but with AS_PATH/AGGREGATOR ASN width rewritten to asn4. EBGP
// prepend and RS-client transcode paths alter only AS-width bytes, not ADD-PATH
// framing; preserving the source ADD-PATH map lets buildFwdBody still convert
// NLRI framing exactly once for the destination.
func fwdContextIDWithASN4(srcCtxID bgpctx.ContextID, asn4 bool) bgpctx.ContextID {
	if srcCtxID == 0 {
		return 0
	}
	srcCtx := bgpctx.Registry.Get(srcCtxID)
	if srcCtx == nil || srcCtx.ASN4() == asn4 {
		return srcCtxID
	}

	enc := srcCtx.Encoding()
	if enc == nil {
		id, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
		if err != nil {
			return srcCtxID
		}
		return id
	}

	encCopy := *enc
	encCopy.ASN4 = asn4
	encCopy.Families = append([]capability.Family(nil), enc.Families...)
	encCopy.AddPathMode = maps.Clone(enc.AddPathMode)
	encCopy.ExtendedNextHop = maps.Clone(enc.ExtendedNextHop)
	encCopy.PathsLimitSend = maps.Clone(enc.PathsLimitSend)
	encCopy.PathsLimitRecv = maps.Clone(enc.PathsLimitRecv)

	ctx := bgpctx.NewEncodingContext(srcCtx.Identity(), &encCopy, srcCtx.Direction())
	id, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		return srcCtxID
	}
	return id
}
