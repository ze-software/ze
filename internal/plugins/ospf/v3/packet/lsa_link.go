// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Link-LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.9 Link-LSA)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// Link-LSA body field offsets (RFC 5340 §A.4.9, body-relative).
const (
	linkOptionsOff     = 1  // 24-bit Options
	linkLocalAddrOff   = 4  // 128-bit link-local interface address
	linkPrefixCountOff = 20 // 32-bit number of prefixes
	linkPrefixListOff  = 24 // start of the prefix list
	linkLocalAddrLen   = 16
)

// LinkLSA is the OSPFv3 Link-LSA body (RFC 5340 §A.4.9). It is link-local scope
// (never flooded off the originating link) and advertises the router's
// link-local address, its Options for the link, and the IPv6 prefixes the router
// associates with the link. Each prefix's 16-bit field is Reserved (0).
type LinkLSA struct {
	RtrPriority   uint8
	Options       types.Options
	LinkLocalAddr [16]byte
	Prefixes      []Prefix
}

// decodeLinkLSA parses a Link-LSA body. The 32-bit prefix count is validated
// against the maximum number of minimum-size prefix entries (4 octets each)
// before any allocation, then the variable-length prefixes are read in order and
// must consume the body exactly.
func decodeLinkLSA(body []byte) (LinkLSA, error) {
	if len(body) < linkPrefixListOff {
		return LinkLSA{}, ErrTruncated
	}
	opts, err := types.OptionsFromBytes(body, linkOptionsOff)
	if err != nil {
		return LinkLSA{}, err
	}
	count := readUint32(body, linkPrefixCountOff)
	maxCount := (len(body) - linkPrefixListOff) / prefixHeaderLen
	if count > uint32(maxCount) {
		return LinkLSA{}, ErrLength
	}
	out := LinkLSA{
		RtrPriority: body[0],
		Options:     opts,
		Prefixes:    make([]Prefix, 0, int(count)),
	}
	copy(out.LinkLocalAddr[:], body[linkLocalAddrOff:linkLocalAddrOff+linkLocalAddrLen])
	off := linkPrefixListOff
	for range count {
		pfx, n, err := decodePrefix(body, off)
		if err != nil {
			return LinkLSA{}, err
		}
		out.Prefixes = append(out.Prefixes, pfx)
		off += n
	}
	if off != len(body) {
		return LinkLSA{}, ErrLength
	}
	return out, nil
}

// EncodedLen returns the Link-LSA body length.
func (l LinkLSA) EncodedLen() int {
	n := linkPrefixListOff
	for _, p := range l.Prefixes {
		n += p.encodedLen()
	}
	return n
}

// WriteTo serializes the Link-LSA body into buf at off. Each prefix's 16-bit
// field is forced to Reserved (0) on the wire (RFC 5340 §A.4.9).
func (l LinkLSA) WriteTo(buf []byte, off int) int {
	start := off
	buf[start] = l.RtrPriority
	l.Options.WriteTo(buf, start+linkOptionsOff)
	copy(buf[start+linkLocalAddrOff:start+linkLocalAddrOff+linkLocalAddrLen], l.LinkLocalAddr[:])
	writeUint32(buf, start+linkPrefixCountOff, uint32(len(l.Prefixes)))
	off = start + linkPrefixListOff
	for _, p := range l.Prefixes {
		p.Field16 = 0
		off = p.writeTo(buf, off)
	}
	return off
}
