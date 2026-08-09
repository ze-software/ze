// Design: docs/architecture/ospf/ospf-2-wire.md -- Router-LSA body codec
// RFC 2328 Appendix A.4.2: Router-LSA.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

const (
	RouterFlagB = 0x01
	RouterFlagE = 0x02
	RouterFlagV = 0x04
	// RouterFlagNt is the RFC 3101 Section 3.5 "Nt" bit: set in an NSSA border router's
	// Router-LSA to advertise that it is a Type-7 -> Type-5 translator candidate. The
	// elected translator is the candidate (Nt set) with the highest Router ID. (Bit 0x08,
	// the MOSPF "W" bit, is unused by Ze.)
	RouterFlagNt = 0x10

	RouterLinkTypeP2P     = 1
	RouterLinkTypeTransit = 2
	RouterLinkTypeStub    = 3
	RouterLinkTypeVirtual = 4

	routerLSAFixedLen  = 4
	routerLinkLen      = 12
	routerTOSMetricLen = 4
)

// RouterLSA is the Type 1 Router-LSA body.
type RouterLSA struct {
	Flags    uint8
	Reserved uint8
	Links    []RouterLink
}

// RouterLink is one 12-octet Router-LSA link record. LinkData is deliberately a
// raw 4-octet field because its meaning depends on LinkType.
type RouterLink struct {
	LinkID   types.LinkStateID
	LinkData [4]byte
	Type     uint8
	TOSCount uint8
	Metric   types.Metric
}

// DecodeRouterLSA parses a Router-LSA body. Obsolete per-TOS metric blocks are
// skipped after each base link record, so #TOS does not misalign following links.
func DecodeRouterLSA(body []byte) (RouterLSA, error) {
	if len(body) < routerLSAFixedLen {
		return RouterLSA{}, ErrTruncated
	}
	count := int(readUint16(body, 2))
	out := RouterLSA{Flags: body[0], Reserved: body[1], Links: make([]RouterLink, 0, count)}
	off := routerLSAFixedLen
	for range count {
		if off+routerLinkLen > len(body) {
			return RouterLSA{}, ErrTruncated
		}
		lsid, err := types.LinkStateIDFromBytes(body[off : off+types.LinkStateIDLen])
		if err != nil {
			return RouterLSA{}, err
		}
		metric, err := types.MetricFromBytes(body[off+10 : off+12])
		if err != nil {
			return RouterLSA{}, err
		}
		link := RouterLink{
			LinkID:   lsid,
			LinkData: readIPv4(body, off+4),
			Type:     body[off+8],
			TOSCount: body[off+9],
			Metric:   metric,
		}
		out.Links = append(out.Links, link)
		off += routerLinkLen
		tosBytes := int(link.TOSCount) * routerTOSMetricLen
		if off+tosBytes > len(body) {
			return RouterLSA{}, ErrTruncated
		}
		off += tosBytes
	}
	if off != len(body) {
		return RouterLSA{}, ErrLength
	}
	return out, nil
}

// EncodedLen returns the Router-LSA body length.
func (l RouterLSA) EncodedLen() int { return routerLSAFixedLen + len(l.Links)*routerLinkLen }

// WriteTo serializes the Router-LSA body into buf at off.
func (l RouterLSA) WriteTo(buf []byte, off int) int {
	buf[off] = l.Flags
	buf[off+1] = l.Reserved
	writeUint16(buf, off+2, uint16(len(l.Links)))
	off += routerLSAFixedLen
	for _, link := range l.Links {
		off += link.LinkID.WriteTo(buf, off)
		off += writeIPv4(buf, off, link.LinkData)
		buf[off] = link.Type
		buf[off+1] = link.TOSCount
		off += 2
		off += link.Metric.WriteTo(buf, off)
	}
	return off
}
