// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI / MP_UNREACH_NLRI wire access
// RFC: rfc/short/rfc2545.md — Section 3, the 16-or-32-octet IPv6 next-hop field

package wireu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// MPReachWire wraps MP_REACH_NLRI attribute bytes for zero-copy lazy parsing.
// RFC 4760 Section 3: AFI(2) + SAFI(1) + NH_Len(1) + NextHop + Reserved(1) + NLRI
//
// This is a view into the original wire bytes - do not modify.
type MPReachWire []byte

// AFI returns the Address Family Identifier (2 octets at offset 0).
// Returns 0 if data is too short.
func (m MPReachWire) AFI() uint16 {
	if len(m) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(m[0:2])
}

// SAFI returns the Subsequent Address Family Identifier (1 octet at offset 2).
// Returns 0 if data is too short.
func (m MPReachWire) SAFI() uint8 {
	if len(m) < 3 {
		return 0
	}
	return m[2]
}

// Family returns the combined AFI/SAFI as an family.Family.
func (m MPReachWire) Family() family.Family {
	return family.Family{
		AFI:  family.AFI(m.AFI()),
		SAFI: family.SAFI(m.SAFI()),
	}
}

// NextHopBytes returns the Network Address of Next Hop field exactly as the
// source framed it, as a view into the attribute bytes.
// RFC 4760 Section 3: the field's width is the octet that precedes it.
//
// The WHOLE field, never a decoded address. RFC 2545 Section 3 lets an IPv6 next
// hop carry a global address followed by a link-local one. NextHop below keeps
// only the first 16 octets of that pair.
//
// A caller that STORES a route for later re-advertisement MUST use this instead.
// The global half alone cannot rebuild the form the source sent (RFC2545-3-1,
// RFC2545-3-2).
//
// Returns nil when the attribute is shorter than the field it declares.
func (m MPReachWire) NextHopBytes() []byte {
	if len(m) < 4 {
		return nil
	}
	nhLen := int(m[3])
	if len(m) < 4+nhLen {
		return nil
	}
	return m[4 : 4+nhLen]
}

// NextHop returns the first next-hop address from the attribute.
// RFC 4760 Section 3: Next Hop Network Address field.
// Returns invalid Addr if data is malformed, too short, or the AFI is not
// IPv4/IPv6 (VPN/EVPN next-hops are family-specific and not decoded here).
// Callers MUST check IsValid() before use.
//
// A 32-octet RFC 2545 field yields the GLOBAL address alone. Use NextHopBytes
// when the link-local half matters, which it does for anything that will put
// these bytes back on the wire.
func (m MPReachWire) NextHop() netip.Addr {
	nhBytes := m.NextHopBytes()
	nhLen := len(nhBytes)
	if nhLen == 0 {
		return netip.Addr{}
	}

	// The LENGTH decides the address family of the next hop, not the AFI of the
	// NLRI it carries. RFC 8950 Section 3 (which obsoletes RFC 5549) advertises
	// IPv4 NLRI with an IPv6 next hop and states the field for it: "Length of
	// Next Hop Address = 16 or 32". So an AFI of 1 says nothing about how wide
	// this field is.
	//
	// Reading AFI alone truncated such a next hop to its first four octets and
	// answered a VALID IPv4 address for them, which is the silently-wrong value
	// ai/rules/principles.md bans: 0x0BADCAFE... came back as 11.173.202.254,
	// and a caller could not tell it from a real next hop. Measured against
	// test/exabgp-compat/encoding/extended-nexthop, whose own expectation states
	// bad:cafe:bad:cafe:bad:cafe:bad:cafe.
	//
	// 32 and 48 are the RFC 2545 Section 3 pair, a global address followed by a
	// link-local one; the global half alone is what this answers, as the
	// contract above says. 24 and 48 are the VPN-IPv6 forms, whose 8-octet RD
	// prefix is not an address, so they stay undecoded here.
	// The AFI is not consulted at all. It says what the NLRI is, and this field
	// is described by its own length: RFC 8950 Section 3 carries IPv4 NLRI
	// behind an IPv6 next hop, and RFC 4761 VPLS (AFI 25) carries an ordinary
	// one, so a switch on AFI answered nothing for every family but two.
	//
	// RFC 4364 Section 4.3.2 prefixes a VPN next hop with an 8-octet Route
	// Distinguisher set to zero. The RD is not part of the address, so it is
	// stepped over rather than decoded: 12 is RD+IPv4 and 24 is RD+IPv6, and
	// RFC 8950 Section 3 states 48 for the RD+IPv6 pair.
	switch nhLen {
	case 4:
		return netip.AddrFrom4([4]byte(nhBytes[:4]))
	case 16, 32:
		return netip.AddrFrom16([16]byte(nhBytes[:16]))
	case 12:
		return netip.AddrFrom4([4]byte(nhBytes[8:12]))
	case 24, 48:
		return netip.AddrFrom16([16]byte(nhBytes[8:24]))
	default:
		return netip.Addr{}
	}
}

// Prefixes parses and returns all NLRI prefixes from the attribute.
// RFC 4760 Section 3: NLRI field follows NextHop + Reserved byte.
// Returns nil if data is malformed, or for non-IPv4/IPv6 AFIs: only IP
// prefixes are representable as netip.Prefix, so VPN/EVPN/FlowSpec yield nil
// here by design (the sole caller, appendMPBlock, omits prefixes for
// non-CIDR families). Use NLRIs()/NLRIIterator() for full family support.
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
func (m MPReachWire) Prefixes() []netip.Prefix {
	if len(m) < 4 {
		return nil
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil
	}

	nlriBytes := m[nlriOffset:]
	afi := m.AFI()

	switch afi {
	case 1: // IPv4
		return ParseIPv4Prefixes(nlriBytes)
	case 2: // IPv6
		return parseIPv6Prefixes(nlriBytes)
	default:
		return nil
	}
}

// NLRIs parses and returns all NLRIs from the attribute, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if wire bytes are malformed.
func (m MPReachWire) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(m) < 5 {
		return nil, fmt.Errorf("MP_REACH_NLRI too short: %d bytes", len(m))
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil, fmt.Errorf("invalid next-hop length: %d", nhLen)
	}

	nlriBytes := m[nlriOffset:]
	fam := m.Family()

	return ParseNLRIs(nlriBytes, fam, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the NLRI section.
// RFC 7911 Section 3: When addPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns nil if data is malformed or empty.
//
// Use this for zero-copy iteration instead of NLRIs() which allocates a slice.
func (m MPReachWire) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(m) < 5 {
		return nil
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil
	}

	nlriBytes := m[nlriOffset:]
	if len(nlriBytes) == 0 {
		return nil
	}

	return nlri.NewNLRIIterator(nlriBytes, addPath)
}

// NLRIBytes returns the raw NLRI bytes without parsing.
// Returns the bytes after NextHop + Reserved, or nil if malformed.
// Use for raw wire byte extraction (pool storage).
func (m MPReachWire) NLRIBytes() []byte {
	if len(m) < 5 {
		return nil
	}

	nhLen := int(m[3])
	// NLRI starts after: AFI(2) + SAFI(1) + NHLen(1) + NextHop(nhLen) + Reserved(1)
	nlriOffset := 4 + nhLen + 1
	if nlriOffset > len(m) {
		return nil
	}

	return m[nlriOffset:]
}

// MPUnreachWire wraps MP_UNREACH_NLRI attribute bytes for zero-copy lazy parsing.
// RFC 4760 Section 4: AFI(2) + SAFI(1) + Withdrawn Routes
//
// This is a view into the original wire bytes - do not modify.
type MPUnreachWire []byte

// AFI returns the Address Family Identifier (2 octets at offset 0).
// Returns 0 if data is too short.
func (m MPUnreachWire) AFI() uint16 {
	if len(m) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(m[0:2])
}

// SAFI returns the Subsequent Address Family Identifier (1 octet at offset 2).
// Returns 0 if data is too short.
func (m MPUnreachWire) SAFI() uint8 {
	if len(m) < 3 {
		return 0
	}
	return m[2]
}

// Family returns the combined AFI/SAFI as an family.Family.
func (m MPUnreachWire) Family() family.Family {
	return family.Family{
		AFI:  family.AFI(m.AFI()),
		SAFI: family.SAFI(m.SAFI()),
	}
}

// Prefixes parses and returns all withdrawn prefixes from the attribute.
// RFC 4760 Section 4: Withdrawn Routes field follows AFI + SAFI.
// Returns nil if data is malformed, or for non-IPv4/IPv6 AFIs: only IP
// prefixes are representable as netip.Prefix, so VPN/EVPN/FlowSpec yield nil
// here by design (the sole caller, appendMPBlock, omits prefixes for
// non-CIDR families). Use NLRIs()/NLRIIterator() for full family support.
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
func (m MPUnreachWire) Prefixes() []netip.Prefix {
	if len(m) < 3 {
		return nil
	}

	// Withdrawn routes start after AFI(2) + SAFI(1)
	withdrawnBytes := m[3:]
	afi := m.AFI()

	switch afi {
	case 1: // IPv4
		return ParseIPv4Prefixes(withdrawnBytes)
	case 2: // IPv6
		return parseIPv6Prefixes(withdrawnBytes)
	default:
		return nil
	}
}

// NLRIs parses and returns all withdrawn NLRIs, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if wire bytes are malformed.
func (m MPUnreachWire) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(m) < 3 {
		return nil, fmt.Errorf("MP_UNREACH_NLRI too short: %d bytes", len(m))
	}

	// Withdrawn routes start after AFI(2) + SAFI(1)
	withdrawnBytes := m[3:]
	fam := m.Family()

	// RFC 8277 Section 2.4: the withdrawal is framed by the withdrawal splitter.
	return ParseWithdrawnNLRIs(withdrawnBytes, fam, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the withdrawn NLRI section.
// RFC 7911 Section 3: When addPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns nil if data is malformed or empty.
//
// Use this for zero-copy iteration instead of NLRIs() which allocates a slice.
func (m MPUnreachWire) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(m) < 3 {
		return nil
	}

	// Withdrawn routes start after AFI(2) + SAFI(1)
	withdrawnBytes := m[3:]
	if len(withdrawnBytes) == 0 {
		return nil
	}

	return nlri.NewNLRIIterator(withdrawnBytes, addPath)
}

// WithdrawnBytes returns the raw withdrawn NLRI bytes without parsing.
// Returns the bytes after AFI(2) + SAFI(1), or nil if malformed.
// Use for raw wire byte extraction (pool storage).
func (m MPUnreachWire) WithdrawnBytes() []byte {
	if len(m) < 3 {
		return nil
	}
	return m[3:]
}

// IPv4Reach holds zero-copy slices into UPDATE body for legacy IPv4 unicast.
// RFC 4271: IPv4 unicast uses body structure, not MP attributes.
type IPv4Reach struct {
	nh   []byte // slice to NEXT_HOP attribute value (4 bytes)
	nlri []byte // slice to body NLRI section
}

// NextHop returns the next-hop address from the NEXT_HOP attribute.
// Returns invalid Addr if nh is nil or wrong size.
func (r IPv4Reach) NextHop() netip.Addr {
	if len(r.nh) < 4 {
		return netip.Addr{}
	}
	var addr [4]byte
	copy(addr[:], r.nh[:4])
	return netip.AddrFrom4(addr)
}

// Prefixes parses and returns all IPv4 prefixes from the NLRI section.
// Returns nil if nlri is nil or empty.
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
func (r IPv4Reach) Prefixes() []netip.Prefix {
	if len(r.nlri) == 0 {
		return nil
	}
	return ParseIPv4Prefixes(r.nlri)
}

// NLRIs parses and returns all NLRIs, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if bytes are malformed.
func (r IPv4Reach) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(r.nlri) == 0 {
		return nil, nil
	}
	return ParseNLRIs(r.nlri, family.IPv4Unicast, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the NLRI section.
// Returns nil if nlri is empty.
func (r IPv4Reach) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(r.nlri) == 0 {
		return nil
	}
	return nlri.NewNLRIIterator(r.nlri, addPath)
}

// NewIPv4Reach creates an IPv4Reach from next-hop and NLRI byte slices.
// Used by filter to construct from parsed UPDATE body fields.
func NewIPv4Reach(nh, nlriData []byte) *IPv4Reach {
	return &IPv4Reach{nh: nh, nlri: nlriData}
}

// NLRISlice returns the raw NLRI bytes for cross-package access.
func (r IPv4Reach) NLRISlice() []byte {
	return r.nlri
}

// IPv4Withdraw holds zero-copy slice into UPDATE body for withdrawn routes.
type IPv4Withdraw struct {
	withdrawn []byte // slice to body withdrawn section
}

// NewIPv4Withdraw creates an IPv4Withdraw from withdrawn byte slice.
// Used by filter to construct from parsed UPDATE body fields.
func NewIPv4Withdraw(withdrawn []byte) *IPv4Withdraw {
	return &IPv4Withdraw{withdrawn: withdrawn}
}

// WithdrawnSlice returns the raw withdrawn bytes for cross-package access.
func (w IPv4Withdraw) WithdrawnSlice() []byte {
	return w.withdrawn
}

// Prefixes parses and returns all withdrawn IPv4 prefixes.
//
// Note: This method does NOT preserve ADD-PATH path-id. Use NLRIs() instead.
func (w IPv4Withdraw) Prefixes() []netip.Prefix {
	if len(w.withdrawn) == 0 {
		return nil
	}
	return ParseIPv4Prefixes(w.withdrawn)
}

// NLRIs parses and returns all withdrawn NLRIs, preserving path-id.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// Returns error if bytes are malformed.
func (w IPv4Withdraw) NLRIs(hasAddPath bool) ([]nlri.NLRI, error) {
	if len(w.withdrawn) == 0 {
		return nil, nil
	}
	return ParseWithdrawnNLRIs(w.withdrawn, family.IPv4Unicast, hasAddPath)
}

// NLRIIterator returns a zero-allocation iterator over the withdrawn section.
// Returns nil if withdrawn is empty.
func (w IPv4Withdraw) NLRIIterator(addPath bool) *nlri.NLRIIterator {
	if len(w.withdrawn) == 0 {
		return nil
	}
	return nlri.NewNLRIIterator(w.withdrawn, addPath)
}

// ParseNLRIs parses the NLRIs of an announcement using the nlri package.
// RFC 7911 Section 3: When hasAddPath is true, each NLRI is prefixed with 4-byte path-id.
// IPv4/IPv6 unicast/multicast parse into INET; every other family is framed
// by its registered splitter and carried as one WireNLRI per NLRI.
func ParseNLRIs(data []byte, fam family.Family, hasAddPath bool) ([]nlri.NLRI, error) {
	return parseNLRISection(data, fam, hasAddPath, false)
}

// ParseWithdrawnNLRIs is ParseNLRIs for the NLRIs of a withdrawal: MP_UNREACH_NLRI
// or the IPv4 Withdrawn Routes field. A family whose withdrawal framing differs
// from its announcement's is framed by nlrisplit.SplitWithdrawn, and a route it
// names by a CIDR (nlrisplit.RouteCIDR) parses into an INET of that prefix.
//
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." A labeled withdrawal carries that field where its
// announcement carried a label stack, so the announcement framing walks it as
// a label entry, finds the S bit clear in the 0x800000 a sender SHOULD use, and
// runs past the NLRI. Framed as a withdrawal, the field is skipped, and the
// INET carries what is left: the prefix and its path identifier, never a label.
func ParseWithdrawnNLRIs(data []byte, fam family.Family, hasAddPath bool) ([]nlri.NLRI, error) {
	return parseNLRISection(data, fam, hasAddPath, true)
}

// parseNLRISection is ParseNLRIs and ParseWithdrawnNLRIs; withdraw picks the
// framing.
func parseNLRISection(data []byte, fam family.Family, hasAddPath, withdraw bool) ([]nlri.NLRI, error) {
	var result []nlri.NLRI
	originalLen := len(data)

	for len(data) > 0 {
		offset := originalLen - len(data)
		var n nlri.NLRI
		var rest []byte
		var err error

		switch {
		case fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIUnicast:
			n, rest, err = nlri.ParseINET(family.AFIIPv4, family.SAFIUnicast, data, hasAddPath)
		case fam.AFI == family.AFIIPv6 && fam.SAFI == family.SAFIUnicast:
			n, rest, err = nlri.ParseINET(family.AFIIPv6, family.SAFIUnicast, data, hasAddPath)
		case fam.AFI == family.AFIIPv4 && fam.SAFI == family.SAFIMulticast:
			n, rest, err = nlri.ParseINET(family.AFIIPv4, family.SAFIMulticast, data, hasAddPath)
		case fam.AFI == family.AFIIPv6 && fam.SAFI == family.SAFIMulticast:
			n, rest, err = nlri.ParseINET(family.AFIIPv6, family.SAFIMulticast, data, hasAddPath)
		default: // VPN, EVPN, FlowSpec, etc. — no dedicated parser
			// Wrap NLRI bytes as opaque WireNLRI to preserve the family in JSON
			// output. Detailed parsing is delegated to plugin-registered decoders.
			//
			// MP_REACH_NLRI packs as many NLRIs as fit, and nlrisplit already
			// holds every family's framing, so ONE carrier is built per NLRI.
			// Wrapping the whole remainder in one carrier told every consumer
			// the peer had sent one route where it sent several: the JSON
			// renderer wrote the decoder's list of routes into the slot where
			// one route goes, and no reader could name the second one
			// (plan/journal/validated-value-discarded-by-its-caller.md).
			if !nlrisplit.Supported(fam) {
				// A family with no registered splitter has no framing this
				// package knows, and guessing at a boundary would publish a
				// route nobody sent. The whole remainder stays one carrier.
				w, wErr := nlri.NewWireNLRI(fam, data, hasAddPath)
				if wErr != nil {
					return result, fmt.Errorf("wrapping NLRI for %s: %w", fam, wErr)
				}
				return append(result, w), nil
			}

			// Split answers the NLRIs it read before any corruption, plus the
			// error. Both are carried out: the routes that parsed are real, and
			// the caller still learns the section was malformed.
			split := nlrisplit.Split
			if withdraw {
				split = nlrisplit.SplitWithdrawn
			}
			parts, splitErr := split(fam, data, hasAddPath)
			// One scratch for the section: RouteCIDR's indirect call moves it
			// to the heap, so a scratch per NLRI would be an allocation each.
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			for _, part := range parts {
				n, nErr := wrapNLRI(fam, part, hasAddPath, withdraw, scratch[:])
				if nErr != nil {
					return result, fmt.Errorf("wrapping NLRI for %s: %w", fam, nErr)
				}
				result = append(result, n)
			}
			if splitErr != nil {
				return result, fmt.Errorf("splitting NLRI section for %s: %w", fam, splitErr)
			}
			return result, nil
		}

		if err != nil {
			return result, fmt.Errorf("parsing NLRI at offset %d: %w", offset, err)
		}

		result = append(result, n)
		data = rest
	}

	return result, nil
}

// wrapNLRI carries one framed NLRI of a family with no dedicated parser. An
// announcement stays opaque for the family's decoder. A withdrawal of a family
// that names its routes by a CIDR becomes an INET of that prefix, because the
// field in front of it is the Compatibility field and carries nothing to decode
// (RFC 8277 Section 2.4); any other withdrawal stays opaque too. scratch has
// nlrisplit.PrefixKeyScratchSize bytes and is reused by the caller.
func wrapNLRI(fam family.Family, part []byte, hasAddPath, withdraw bool, scratch []byte) (nlri.NLRI, error) {
	if !withdraw {
		return nlri.NewWireNLRI(fam, part, hasAddPath)
	}
	pathID, payload, err := nlri.SplitPathID(part, hasAddPath)
	if err != nil {
		return nil, err
	}
	cidr, err := nlrisplit.RouteCIDR(fam, payload, scratch, true)
	if errors.Is(err, nlrisplit.ErrUnsupported) {
		return nlri.NewWireNLRI(fam, part, hasAddPath)
	}
	if err != nil {
		return nil, err
	}
	// The INET is parsed from a stack copy of [path id][CIDR], so the one
	// allocation per NLRI is the INET itself. ParseINET, not NewINET, builds it:
	// it records that the wire carried a path identifier (HasAddPath), which a
	// locally built INET reports false and the JSON event reads to print it.
	var framed [4 + nlrisplit.PrefixKeyScratchSize]byte
	off := 0
	if hasAddPath {
		binary.BigEndian.PutUint32(framed[:4], pathID)
		off = 4
	}
	off += copy(framed[off:], cidr)
	n, _, err := nlri.ParseINET(fam.AFI, fam.SAFI, framed[:off], hasAddPath)
	return n, err
}
