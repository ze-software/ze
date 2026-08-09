// Design: docs/architecture/ospf/ospf-ext-2-traffic-engineering.md -- RFC 3630 TE LSA body codec.
// RFC: rfc/short/rfc3630.md -- Traffic Engineering (TE) Extensions to OSPFv2.
//
// The TE LSA is an Opaque LSA (Opaque type 1, LS type 10 area-local) whose body is a
// single top-level TLV: either the Router Address TLV (type 1) or the Link TLV (type 2),
// the latter carrying nested sub-TLVs 1-9. This file codes that body ON TOP of the
// generic 4-byte-aligned TLV iterator/builder from opaque_tlv.go (spec-ospf-ext-1); it
// never re-implements TLV framing or the 4-octet alignment. The two load-bearing wire
// details are the alignment (padding excluded from Length, RFC 3630 sec 2.3.2) and the
// IEEE-754 single-precision bandwidth in bytes/sec (sec 2.4.2), both handled here.

package packet

import "math"

// RFC 3630 sec 2.2: the Traffic Engineering LSA uses Opaque type 1; RFC 5392 sec 3.1.1:
// the Inter-AS-TE-v2 LSA uses Opaque type 6. Both select TE bodies inside the shared
// 8-bit Opaque Type namespace; the carrier owns the Link State ID split.
const (
	// TEOpaqueType is the RFC 3630 Traffic Engineering LSA Opaque type.
	TEOpaqueType uint8 = 1
	// InterAsTEOpaqueType is the RFC 5392 Inter-AS-TE-v2 LSA Opaque type.
	InterAsTEOpaqueType uint8 = 6
)

// RFC 3630 sec 2.4: an LSA contains exactly one top-level TLV, either a Router Address
// TLV or a Link TLV.
const (
	// TETLVRouterAddress is the top-level Router Address TLV (RFC 3630 sec 2.4.1).
	TETLVRouterAddress uint16 = 1
	// TETLVLink is the top-level Link TLV (RFC 3630 sec 2.4.2).
	TETLVLink uint16 = 2
)

// RFC 3630 sec 2.5: the Link sub-TLV type codes carried inside the Link TLV.
const (
	TESubLinkType          uint16 = 1 // sec 2.5.1, 1 octet
	TESubLinkID            uint16 = 2 // sec 2.5.2, 4 octets
	TESubLocalInterfaceIP  uint16 = 3 // sec 2.5.3, 4N octets
	TESubRemoteInterfaceIP uint16 = 4 // sec 2.5.4, 4N octets
	TESubTEMetric          uint16 = 5 // sec 2.5.5, 4 octets
	TESubMaxBandwidth      uint16 = 6 // sec 2.5.6, 4 octets IEEE float
	TESubMaxReservableBW   uint16 = 7 // sec 2.5.7, 4 octets IEEE float
	TESubUnreservedBW      uint16 = 8 // sec 2.5.8, 32 octets (8 IEEE floats)
	TESubAdminGroup        uint16 = 9 // sec 2.5.9, 4 octets bit mask
)

// RFC 3630 sec 2.5.1: the Link Type sub-TLV values.
const (
	TELinkTypePointToPoint uint8 = 1
	TELinkTypeMultiAccess  uint8 = 2
)

// TEUnreservedPriorities is the fixed count of priority levels in the Unreserved
// Bandwidth sub-TLV (RFC 3630 sec 2.5.8): priority 0 first, priority 7 last.
const TEUnreservedPriorities = 8

// TELink is one decoded TE Link TLV: the RFC 3630 sub-TLVs 1-9 plus the RFC 5392
// inter-AS sub-TLVs (21/22/24). Every attribute is optional (Has* gates presence); the
// value types are plain (no pointers) so a TELink crosses the plugin boundary by value.
// Bandwidth is stored as float64: the wire encoding is IEEE-754 single precision, but
// float32 rounds small reservations away, so the in-memory representation is float64,
// mirroring internal/plugins/rsvpte admission.
type TELink struct {
	HasLinkType bool
	LinkType    uint8
	HasLinkID   bool
	LinkID      [4]byte
	LocalIPs    [][4]byte
	RemoteIPs   [][4]byte

	HasTEMetric bool
	TEMetric    uint32

	HasMaxBandwidth bool
	MaxBandwidth    float64 // bytes/sec

	HasMaxReservable bool
	MaxReservable    float64 // bytes/sec

	HasUnreserved bool
	// Unreserved holds the eight priority levels, priority 0 first (RFC 3630 sec 2.5.8).
	Unreserved [TEUnreservedPriorities]float64 // bytes/sec

	HasAdminGroup bool
	AdminGroup    uint32

	// RFC 5392 inter-AS sub-TLVs; see te_interas.go.
	HasRemoteAS     bool
	RemoteAS        uint32
	HasRemoteASBRv4 bool
	RemoteASBRv4    [4]byte
	HasRemoteASBRv6 bool
	RemoteASBRv6    [16]byte
}

// IsInterAS reports whether this Link TLV carries the RFC 5392 Remote AS Number sub-TLV,
// which is REQUIRED in any inter-AS TE Link TLV (RFC 5392 sec 3.3.1).
func (l TELink) IsInterAS() bool { return l.HasRemoteAS }

// TELSA is one decoded TE LSA body: exactly one top-level TLV (RFC 3630 sec 2.4), either
// a Router Address (IsRouterAddress) or a Link (IsLink).
type TELSA struct {
	IsRouterAddress bool
	RouterAddress   [4]byte
	IsLink          bool
	Link            TELink
}

// TEAdminGroupHasGroup reports whether administrative group n (0..31) is set in the mask.
// RFC 3630 sec 2.5.9: the least significant bit is group 0, the most significant group 31.
func TEAdminGroupHasGroup(mask uint32, n uint) bool {
	if n > 31 {
		return false
	}
	return mask&(uint32(1)<<n) != 0
}

// teFloat32Bytes encodes a bandwidth in bytes/sec as 4-octet IEEE-754 single-precision,
// network byte order (RFC 3630 sec 2.4.2). Values beyond float32 range become +/-Inf,
// which round-trips without panicking.
func teFloat32Bytes(v float64) []byte {
	b := make([]byte, 4)
	writeUint32(b, 0, math.Float32bits(float32(v)))
	return b
}

// teReadFloat32 decodes a 4-octet IEEE-754 single-precision bandwidth to float64. The
// caller guarantees len(b) >= 4.
func teReadFloat32(b []byte) float64 { return float64(math.Float32frombits(readUint32(b, 0))) }

// teU32Bytes returns v as a 4-octet big-endian value slice.
func teU32Bytes(v uint32) []byte {
	b := make([]byte, 4)
	writeUint32(b, 0, v)
	return b
}

// teIPBytes returns a 4-octet copy of an IPv4 address for a TLV value.
func teIPBytes(ip [4]byte) []byte {
	b := make([]byte, 4)
	copy(b, ip[:])
	return b
}

// teIPsBytes concatenates N IPv4 addresses into a 4N-octet value (RFC 3630 sec 2.5.3/4).
func teIPsBytes(ips [][4]byte) []byte {
	b := make([]byte, 4*len(ips))
	for i, ip := range ips {
		writeIPv4(b, i*4, ip)
	}
	return b
}

// linkSubTLVs assembles the ordered sub-TLV set for a Link TLV. RFC 3630 sec 2.4.2:
// sub-TLVs have no ordering requirement, so a fixed order (1,2,3,4,5,6,7,8,9 then the
// RFC 5392 21/22/24) is chosen for a deterministic, idempotent re-origination.
func (l TELink) linkSubTLVs() []opaqueTLV {
	tlvs := make([]opaqueTLV, 0, 12)
	if l.HasLinkType {
		tlvs = append(tlvs, opaqueTLV{Type: TESubLinkType, Value: []byte{l.LinkType}})
	}
	if l.HasLinkID {
		tlvs = append(tlvs, opaqueTLV{Type: TESubLinkID, Value: teIPBytes(l.LinkID)})
	}
	if len(l.LocalIPs) > 0 {
		tlvs = append(tlvs, opaqueTLV{Type: TESubLocalInterfaceIP, Value: teIPsBytes(l.LocalIPs)})
	}
	if len(l.RemoteIPs) > 0 {
		tlvs = append(tlvs, opaqueTLV{Type: TESubRemoteInterfaceIP, Value: teIPsBytes(l.RemoteIPs)})
	}
	if l.HasTEMetric {
		tlvs = append(tlvs, opaqueTLV{Type: TESubTEMetric, Value: teU32Bytes(l.TEMetric)})
	}
	if l.HasMaxBandwidth {
		tlvs = append(tlvs, opaqueTLV{Type: TESubMaxBandwidth, Value: teFloat32Bytes(l.MaxBandwidth)})
	}
	if l.HasMaxReservable {
		tlvs = append(tlvs, opaqueTLV{Type: TESubMaxReservableBW, Value: teFloat32Bytes(l.MaxReservable)})
	}
	if l.HasUnreserved {
		// RFC 3630 sec 2.5.8: eight IEEE floats, priority 0 first through priority 7 last.
		v := make([]byte, 4*TEUnreservedPriorities)
		for i := range TEUnreservedPriorities {
			writeUint32(v, i*4, math.Float32bits(float32(l.Unreserved[i])))
		}
		tlvs = append(tlvs, opaqueTLV{Type: TESubUnreservedBW, Value: v})
	}
	if l.HasAdminGroup {
		tlvs = append(tlvs, opaqueTLV{Type: TESubAdminGroup, Value: teU32Bytes(l.AdminGroup)})
	}
	tlvs = appendInterAsSubTLVs(tlvs, l)
	return tlvs
}

// topTLVs returns the single top-level TLV for this TE LSA body, or nil for an empty body.
func (l TELSA) topTLVs() []opaqueTLV {
	switch {
	case l.IsRouterAddress:
		return []opaqueTLV{{Type: TETLVRouterAddress, Value: teIPBytes(l.RouterAddress)}}
	case l.IsLink:
		sub := l.Link.linkSubTLVs()
		val := make([]byte, opaqueTLVsLen(sub))
		writeOpaqueTLVs(val, sub)
		return []opaqueTLV{{Type: TETLVLink, Value: val}}
	default:
		return nil
	}
}

// Encode renders the TE LSA body (the bytes after the 20-byte LSA header) using the
// ext-1 4-byte-aligned TLV builder. The result is handed to the opaque carrier verbatim.
//
// Cold-path exception to ai/rules/buffer-first: linkSubTLVs / topTLVs build the TLV set with
// make/append and this allocates the body slice. This runs only on TE-LSA origination and
// refresh (a config/topology change, rate-limited to MinLSInterval), never on packet
// forwarding, so the allocation is negligible and the readable TLV-set construction is kept
// deliberately rather than hand-rolling a two-pass buffer-first encoder for the wire.
func (l TELSA) Encode() []byte {
	tlvs := l.topTLVs()
	b := make([]byte, opaqueTLVsLen(tlvs))
	writeOpaqueTLVs(b, tlvs)
	return b
}

// DecodeTELSA parses a TE LSA body into a TELSA. It walks the top-level TLVs with the
// bound-checked ext-1 iterator and never panics on malformed input (R-8/AC-18): a
// truncated header, an over-long length, or a bad sub-TLV length returns an error. An
// unrecognized top-level TLV is ignored (RFC 3630 sec 2.3.2). A body with no recognized
// top-level TLV returns ErrTruncated so the caller skips it.
func DecodeTELSA(body []byte) (TELSA, error) {
	var out TELSA
	it := newOpaqueTLVIterator(body)
	recognized := false
	for it.Next() {
		v := it.Value()
		switch it.Type() {
		case TETLVRouterAddress:
			// RFC 3630 sec 2.4.1: the Router Address TLV value is a 4-octet IP address.
			if len(v) < 4 {
				return out, ErrLength
			}
			out.IsRouterAddress = true
			out.RouterAddress = readIPv4(v, 0)
			recognized = true
		case TETLVLink:
			link, err := decodeTELink(v)
			if err != nil {
				return out, err
			}
			out.IsLink = true
			out.Link = link
			recognized = true
		}
	}
	if it.Err() != nil {
		return out, it.Err()
	}
	if !recognized {
		return out, ErrTruncated
	}
	return out, nil
}

// decodeTELink parses the nested sub-TLVs of a Link TLV value. Unrecognized sub-TLVs are
// ignored (RFC 3630 sec 2.4.2); a defined sub-TLV with a wrong fixed length is malformed.
func decodeTELink(value []byte) (TELink, error) {
	var l TELink
	it := newOpaqueTLVIterator(value)
	for it.Next() {
		v := it.Value()
		switch it.Type() {
		case TESubLinkType:
			if len(v) < 1 {
				return l, ErrLength
			}
			l.HasLinkType = true
			l.LinkType = v[0]
		case TESubLinkID:
			if len(v) != 4 {
				return l, ErrLength
			}
			l.HasLinkID = true
			l.LinkID = readIPv4(v, 0)
		case TESubLocalInterfaceIP:
			ips, err := decodeIPList(v)
			if err != nil {
				return l, err
			}
			l.LocalIPs = ips
		case TESubRemoteInterfaceIP:
			ips, err := decodeIPList(v)
			if err != nil {
				return l, err
			}
			l.RemoteIPs = ips
		case TESubTEMetric:
			if len(v) != 4 {
				return l, ErrLength
			}
			l.HasTEMetric = true
			l.TEMetric = readUint32(v, 0)
		case TESubMaxBandwidth:
			if len(v) != 4 {
				return l, ErrLength
			}
			l.HasMaxBandwidth = true
			l.MaxBandwidth = teReadFloat32(v)
		case TESubMaxReservableBW:
			if len(v) != 4 {
				return l, ErrLength
			}
			l.HasMaxReservable = true
			l.MaxReservable = teReadFloat32(v)
		case TESubUnreservedBW:
			// RFC 3630 sec 2.5.8: exactly eight IEEE floats (32 octets), priority 0 first.
			if len(v) != 4*TEUnreservedPriorities {
				return l, ErrLength
			}
			l.HasUnreserved = true
			for i := range TEUnreservedPriorities {
				l.Unreserved[i] = teReadFloat32(v[i*4:])
			}
		case TESubAdminGroup:
			if len(v) != 4 {
				return l, ErrLength
			}
			l.HasAdminGroup = true
			l.AdminGroup = readUint32(v, 0)
		default:
			// RFC 5392 sec 3.3: the inter-AS sub-TLVs (21/22/24). Any other sub-TLV type is
			// an unrecognized sub-TLV and is ignored (RFC 3630 sec 2.4.2).
			handled, err := parseInterAsSubTLV(&l, it.Type(), v)
			if handled && err != nil {
				return l, err
			}
		}
	}
	if it.Err() != nil {
		return l, it.Err()
	}
	return l, nil
}

// decodeIPList decodes a 4N-octet Local/Remote Interface IP Address sub-TLV value into a
// list of IPv4 addresses (RFC 3630 sec 2.5.3/2.5.4). A length that is not a positive
// multiple of 4 is malformed.
func decodeIPList(v []byte) ([][4]byte, error) {
	if len(v) == 0 || len(v)%4 != 0 {
		return nil, ErrLength
	}
	out := make([][4]byte, 0, len(v)/4)
	for i := 0; i < len(v); i += 4 {
		out = append(out, readIPv4(v, i))
	}
	return out, nil
}
