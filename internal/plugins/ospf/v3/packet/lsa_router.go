// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Router-LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.3 Router-LSA)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// Router-LSA flag bits (RFC 5340 §A.4.3).
const (
	RouterFlagB  = 0x01 // B-bit: area border router
	RouterFlagE  = 0x02 // E-bit: AS boundary router
	RouterFlagV  = 0x04 // V-bit: virtual link endpoint
	RouterFlagNt = 0x10 // Nt-bit: NSSA border router translating Type-7 to Type-5
)

// Router-LSA link record types (RFC 5340 §A.4.3).
const (
	RouterLinkTypeP2P     = 1 // point-to-point connection to another router
	RouterLinkTypeTransit = 2 // connection to a transit network
	RouterLinkTypeVirtual = 4 // virtual link
)

const (
	routerLSAFixedLen = 4
	routerLinkLen     = 16
)

// RouterLSA is the OSPFv3 Router-LSA body (RFC 5340 §A.4.3). Unlike OSPFv2 it
// carries NO IP addresses and NO per-link count: the number of link records is
// derived from the LSA Length. Options is the 24-bit OSPFv3 Options field.
type RouterLSA struct {
	Flags   uint8
	Options types.Options
	Links   []RouterLink
}

// RouterLink is one 16-octet OSPFv3 Router-LSA link record (RFC 5340 §A.4.3).
// All three IDs are 32-bit; there are no IP addresses.
type RouterLink struct {
	Type                uint8
	Metric              uint16
	InterfaceID         types.InterfaceID
	NeighborInterfaceID types.InterfaceID
	NeighborRouterID    types.RouterID
}

// DecodeRouterLSA parses a Router-LSA body. The link count is derived from the
// remaining body length, which must be a whole number of 16-octet records.
func DecodeRouterLSA(body []byte) (RouterLSA, error) {
	if len(body) < routerLSAFixedLen {
		return RouterLSA{}, ErrTruncated
	}
	if (len(body)-routerLSAFixedLen)%routerLinkLen != 0 {
		return RouterLSA{}, ErrLength
	}
	opts, err := types.OptionsFromBytes(body, 1)
	if err != nil {
		return RouterLSA{}, err
	}
	count := (len(body) - routerLSAFixedLen) / routerLinkLen
	out := RouterLSA{Flags: body[0], Options: opts, Links: make([]RouterLink, 0, count)}
	off := routerLSAFixedLen
	for range count {
		iface, err := types.InterfaceIDFromBytes(body[off+4 : off+8])
		if err != nil {
			return RouterLSA{}, err
		}
		nbrIface, err := types.InterfaceIDFromBytes(body[off+8 : off+12])
		if err != nil {
			return RouterLSA{}, err
		}
		nbrRouter, err := types.RouterIDFromBytes(body[off+12 : off+16])
		if err != nil {
			return RouterLSA{}, err
		}
		out.Links = append(out.Links, RouterLink{
			Type:                body[off],
			Metric:              readUint16(body, off+2),
			InterfaceID:         iface,
			NeighborInterfaceID: nbrIface,
			NeighborRouterID:    nbrRouter,
		})
		off += routerLinkLen
	}
	return out, nil
}

// EncodedLen returns the Router-LSA body length.
func (l RouterLSA) EncodedLen() int { return routerLSAFixedLen + len(l.Links)*routerLinkLen }

// WriteTo serializes the Router-LSA body into buf at off. The reserved octet
// after Type in each link record is written zero (RFC 5340 §A.4.3).
func (l RouterLSA) WriteTo(buf []byte, off int) int {
	buf[off] = l.Flags
	off++
	off += l.Options.WriteTo(buf, off)
	for _, link := range l.Links {
		buf[off] = link.Type
		buf[off+1] = 0
		off += 2
		off += writeUint16(buf, off, link.Metric)
		off += link.InterfaceID.WriteTo(buf, off)
		off += link.NeighborInterfaceID.WriteTo(buf, off)
		off += link.NeighborRouterID.WriteTo(buf, off)
	}
	return off
}
