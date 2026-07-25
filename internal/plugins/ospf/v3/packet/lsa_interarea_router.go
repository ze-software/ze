// Design: plan/learned/969-ospfv3-2-wire.md -- OSPFv3 Inter-Area-Router-LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.6 Inter-Area-Router-LSA)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// Inter-Area-Router-LSA body field offsets (RFC 5340 §A.4.6, body-relative).
const (
	interAreaRouterOptionsOff = 1 // 24-bit Options
	interAreaRouterMetricOff  = 5 // 24-bit metric
	interAreaRouterDestOff    = 8 // Destination Router ID
	interAreaRouterLen        = 12
)

// InterAreaRouterLSA is the OSPFv3 Inter-Area-Router-LSA body (RFC 5340 §A.4.6):
// an area-scoped summary of the cost to reach an AS boundary router in another
// area. It is a fixed 12 octets.
type InterAreaRouterLSA struct {
	Options           types.Options
	Metric            uint32
	DestinationRouter types.RouterID
}

// decodeInterAreaRouterLSA parses an Inter-Area-Router-LSA body.
func decodeInterAreaRouterLSA(body []byte) (InterAreaRouterLSA, error) {
	if len(body) != interAreaRouterLen {
		return InterAreaRouterLSA{}, ErrLength
	}
	opts, err := types.OptionsFromBytes(body, interAreaRouterOptionsOff)
	if err != nil {
		return InterAreaRouterLSA{}, err
	}
	dest, err := types.RouterIDFromBytes(body[interAreaRouterDestOff : interAreaRouterDestOff+types.IDLen])
	if err != nil {
		return InterAreaRouterLSA{}, err
	}
	return InterAreaRouterLSA{
		Options:           opts,
		Metric:            readUint24(body, interAreaRouterMetricOff),
		DestinationRouter: dest,
	}, nil
}

// EncodedLen returns the Inter-Area-Router-LSA body length (fixed 12 octets).
func (l InterAreaRouterLSA) EncodedLen() int { return interAreaRouterLen }

// WriteTo serializes the Inter-Area-Router-LSA body into buf at off. The two
// reserved octets (before Options and before Metric) are written zero (RFC 5340
// §A.4.6).
func (l InterAreaRouterLSA) WriteTo(buf []byte, off int) int {
	buf[off] = 0
	l.Options.WriteTo(buf, off+interAreaRouterOptionsOff)
	buf[off+4] = 0
	writeUint24(buf, off+interAreaRouterMetricOff, l.Metric)
	l.DestinationRouter.WriteTo(buf, off+interAreaRouterDestOff)
	return off + interAreaRouterLen
}
