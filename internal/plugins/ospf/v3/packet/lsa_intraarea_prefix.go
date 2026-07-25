// Design: plan/learned/969-ospfv3-2-wire.md -- OSPFv3 Intra-Area-Prefix-LSA body codec.
// RFC: rfc/short/rfc5340.md (§A.4.10 Intra-Area-Prefix-LSA)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// Intra-Area-Prefix-LSA body field offsets (RFC 5340 §A.4.10, body-relative).
const (
	intraAreaPrefixCountOff  = 0  // 16-bit number of prefixes
	intraAreaRefTypeOff      = 2  // Referenced LS Type
	intraAreaRefLSIDOff      = 4  // Referenced Link State ID
	intraAreaRefAdvRouterOff = 8  // Referenced Advertising Router
	intraAreaPrefixListOff   = 12 // start of the prefix list
)

// IntraAreaPrefixLSA is the OSPFv3 Intra-Area-Prefix-LSA body (RFC 5340 §A.4.10).
// It attaches IPv6 prefixes to the Router-LSA or Network-LSA named by the
// Referenced (LS Type, Link State ID, Advertising Router) triple. Each prefix's
// 16-bit field carries the prefix's Metric.
type IntraAreaPrefixLSA struct {
	ReferencedLSType      types.LSType
	ReferencedLinkStateID types.LinkStateID
	ReferencedAdvRouter   types.RouterID
	Prefixes              []Prefix
}

// decodeIntraAreaPrefixLSA parses an Intra-Area-Prefix-LSA body. The 16-bit
// prefix count is validated against the maximum number of minimum-size prefix
// entries before allocation, then the variable-length prefixes are read in order
// and must consume the body exactly.
func decodeIntraAreaPrefixLSA(body []byte) (IntraAreaPrefixLSA, error) {
	if len(body) < intraAreaPrefixListOff {
		return IntraAreaPrefixLSA{}, ErrTruncated
	}
	refType, err := types.LSTypeFromBytes(body, intraAreaRefTypeOff)
	if err != nil {
		return IntraAreaPrefixLSA{}, err
	}
	refLSID, err := types.LinkStateIDFromBytes(body[intraAreaRefLSIDOff : intraAreaRefLSIDOff+types.IDLen])
	if err != nil {
		return IntraAreaPrefixLSA{}, err
	}
	refAdv, err := types.RouterIDFromBytes(body[intraAreaRefAdvRouterOff : intraAreaRefAdvRouterOff+types.IDLen])
	if err != nil {
		return IntraAreaPrefixLSA{}, err
	}
	count := readUint16(body, intraAreaPrefixCountOff)
	maxCount := (len(body) - intraAreaPrefixListOff) / prefixHeaderLen
	if int(count) > maxCount {
		return IntraAreaPrefixLSA{}, ErrLength
	}
	out := IntraAreaPrefixLSA{
		ReferencedLSType:      refType,
		ReferencedLinkStateID: refLSID,
		ReferencedAdvRouter:   refAdv,
		Prefixes:              make([]Prefix, 0, int(count)),
	}
	off := intraAreaPrefixListOff
	for range count {
		pfx, n, err := decodePrefix(body, off)
		if err != nil {
			return IntraAreaPrefixLSA{}, err
		}
		out.Prefixes = append(out.Prefixes, pfx)
		off += n
	}
	if off != len(body) {
		return IntraAreaPrefixLSA{}, ErrLength
	}
	return out, nil
}

// EncodedLen returns the Intra-Area-Prefix-LSA body length.
func (l IntraAreaPrefixLSA) EncodedLen() int {
	n := intraAreaPrefixListOff
	for _, p := range l.Prefixes {
		n += p.encodedLen()
	}
	return n
}

// WriteTo serializes the Intra-Area-Prefix-LSA body into buf at off. Each
// prefix's 16-bit field is its Metric (RFC 5340 §A.4.10), preserved from the
// decoded Prefix.Field16.
func (l IntraAreaPrefixLSA) WriteTo(buf []byte, off int) int {
	start := off
	writeUint16(buf, start+intraAreaPrefixCountOff, uint16(len(l.Prefixes)))
	l.ReferencedLSType.WriteTo(buf, start+intraAreaRefTypeOff)
	l.ReferencedLinkStateID.WriteTo(buf, start+intraAreaRefLSIDOff)
	l.ReferencedAdvRouter.WriteTo(buf, start+intraAreaRefAdvRouterOff)
	off = start + intraAreaPrefixListOff
	for _, p := range l.Prefixes {
		off = p.writeTo(buf, off)
	}
	return off
}
