// Design: docs/architecture/core-design.md — zero-allocation wire UPDATE builders
// RFC: rfc/short/rfc4271.md — path attribute order and LOCAL_PREF (Sections 5, 5.1.5)
// Overview: reactor.go — Reactor struct, lifecycle, and connection management
// Related: reactor_api.go — reactorAPIAdapter for plugin integration
// Related: reactor_api_forward.go — forwarding uses wire builders

package reactor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// Skip-and-backfill encoding pattern:
// All UPDATE builders in this file use skip-and-backfill instead of Len()-then-WriteTo().
// 1. Write fixed marker/type bytes
// 2. Skip variable-length fields (save position for backfill)
// 3. Write payload forward at advancing offset
// 4. Backfill length fields at saved positions
// This avoids the double traversal of computing Len() then calling WriteTo().

// ErrNextHopUnencodable reports a next-hop address that cannot fill the Next Hop
// field the encoder is about to declare a length for.
//
// Every next-hop field on this rail writes its own length octet before the
// address, so the writer is the one symbol that knows what it has promised. It
// refuses rather than writes, because neither alternative is available to it:
// netip.Addr.As4 PANICS on an address that is not IPv4, and netip.Addr.As16
// silently renders 192.0.2.1 as ::ffff:192.0.2.1 and the zero Addr as ::, which
// fill the declared octets and reach the peer as a well-formed attribute naming a
// host that does not exist. The second failure is the expensive one: the peer
// raises no NOTIFICATION and installs the route.
//
// The forwarding rail already refuses in the same place. mpReachNextHopHandler
// (filter_delta_handlers.go) synthesizes the length octet from the buffer it
// holds and keeps the route unchanged when the replacement is not 4, 16 or 32
// octets. Both rails therefore put the refusal in the function that writes the
// length, and neither trusts a caller to have checked.
var ErrNextHopUnencodable = errors.New("next hop cannot be encoded")

// validateAnnounceNextHop reports why WriteAnnounceUpdate would refuse this route
// and this link-local address, and nil when it would encode them.
//
// It is the reason behind the writer's zero return, not a second gate in front of
// it: both call announceNextHopOctets, so a caller that skips this function still
// cannot get bad octets out of the writer. What it loses is the diagnosis.
func validateAnnounceNextHop(route bgptypes.RouteSpec, linkLocalNextHop netip.Addr) error {
	_, err := announceNextHopOctets(route, linkLocalNextHop)
	return err
}

// announceNextHop holds both halves of the Next Hop field, already reduced to the
// octets that go on the wire.
type announceNextHop struct {
	v4            [4]byte
	global        [16]byte
	linkLocal     [16]byte
	withLinkLocal bool
}

// announceNextHopOctets settles both halves of the Next Hop field for one route.
//
// It is the only place the route's family selects a next-hop form, so the writer
// and ValidateAnnounceNextHop cannot come to different answers about the same
// route.
func announceNextHopOctets(route bgptypes.RouteSpec, linkLocalNextHop netip.Addr) (announceNextHop, error) {
	var nh announceNextHop
	var err error

	// RFC 4271 Section 5.1.3 gives an IPv4 route the four-octet NEXT_HOP
	// attribute; RFC 4760 Section 3 and RFC 2545 Section 3 give every IPv6 route
	// the MP_REACH_NLRI Next Hop field instead.
	if !route.Prefix.Addr().Is6() {
		nh.v4, err = nextHopV4Octets(route.NextHop.Addr)
		return nh, err
	}
	if nh.global, err = nextHopGlobalOctets(route.NextHop.Addr); err != nil {
		return nh, err
	}
	nh.linkLocal, nh.withLinkLocal, err = nextHopLinkLocalOctets(linkLocalNextHop)
	return nh, err
}

// nextHopV4Octets returns the four octets RFC 4271 Section 5.1.3 gives the
// NEXT_HOP attribute of an IPv4 route.
//
// An IPv4-mapped address names the same host as its IPv4 form, so it is accepted
// and unmapped. Every other address is refused: there are no four octets that
// name it, which is why the standard library panics rather than guesses.
func nextHopV4Octets(addr netip.Addr) ([4]byte, error) {
	if !addr.Is4() && !addr.Is4In6() {
		return [4]byte{}, fmt.Errorf("%w: NEXT_HOP %v is not an IPv4 address (RFC 4271 Section 5.1.3)", ErrNextHopUnencodable, addr)
	}
	return addr.As4(), nil
}

// nextHopGlobalOctets returns the sixteen octets RFC 2545 Section 3 calls "the
// global IPv6 address of the next hop", the first address of the MP_REACH_NLRI
// Next Hop field.
//
// attribute.ValidateGlobalNextHop owns the link-local half of that phrase and is
// reused here rather than restated. It returns nil for an IPv4 address and for
// the zero Addr, on the stated ground that an unset next hop is the caller's own
// defect and naming it there would name the wrong producer. This function is that
// caller, so those two cases are refused here.
func nextHopGlobalOctets(addr netip.Addr) ([16]byte, error) {
	if !addr.Is6() || addr.Is4In6() {
		return [16]byte{}, fmt.Errorf("%w: MP_REACH next hop %v is not an IPv6 address (RFC 2545 Section 3)", ErrNextHopUnencodable, addr)
	}
	if err := attribute.ValidateGlobalNextHop(addr); err != nil {
		return [16]byte{}, fmt.Errorf("%w: %w", ErrNextHopUnencodable, err)
	}
	return addr.As16(), nil
}

// nextHopLinkLocalOctets returns the sixteen octets RFC 2545 Section 3 calls "the
// link-local IPv6 address of the next hop", and reports whether the 32-octet form
// is owed at all.
//
// The zero Addr is not an error. It is Section 3's "in all other cases" answer,
// which the caller has already decided against the host interface table
// (Peer.linkLocalNextHopFor, link_scope.go), and it selects the 16-octet form.
//
// An address that is valid but not link-local unicast IS an error. The caller
// asked for the second slot to be filled, and Section 3 permits exactly one kind
// of address there. Quietly falling back to the 16-octet form would encode a
// different answer than the caller gave, which is the failure this guard exists
// to stop rather than a safe default.
func nextHopLinkLocalOctets(addr netip.Addr) ([16]byte, bool, error) {
	if !addr.IsValid() {
		return [16]byte{}, false, nil
	}
	if !addr.Is6() || addr.Is4In6() || !addr.IsLinkLocalUnicast() {
		return [16]byte{}, false, fmt.Errorf("%w: MP_REACH second next hop %v is not a link-local IPv6 address (RFC 2545 Section 3)", ErrNextHopUnencodable, addr)
	}
	return addr.As16(), true, nil
}

// Zero-allocation attribute writers.
// These functions write attributes directly to the buffer without allocating structs.

// writeOriginAttr writes ORIGIN attribute directly to buf.
// RFC 4271 §5.1.1: Well-known mandatory, 1 byte value.
func writeOriginAttr(buf []byte, off int, origin uint8) int {
	// Header: Transitive(0x40) | code(1) | len(1)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrOrigin)
	buf[off+2] = 1
	buf[off+3] = origin
	return 4
}

// writeASPathAttr writes AS_PATH attribute directly to buf.
// RFC 4271 §5.1.2: Well-known mandatory.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers.
// RFC 4271 §4.3: Handles segment splitting for >255 ASNs and extended length.
func writeASPathAttr(buf []byte, off int, asns []uint32, asn4 bool) int {
	start := off
	asnSize := 2
	if asn4 {
		asnSize = 4
	}

	// RFC 4271: Max 255 ASNs per segment, split if needed
	// Calculate total value length accounting for segment splitting
	var valueLen int
	remaining := len(asns)
	for remaining > 0 {
		chunk := min(remaining, attribute.MaxASPathSegmentLength)
		valueLen += 2 + chunk*asnSize // type(1) + count(1) + asns
		remaining -= chunk
	}
	// Empty AS_PATH for iBGP has valueLen=0

	// RFC 4271 §4.3: Use extended length if > 255 bytes
	if valueLen > 255 {
		buf[off] = byte(attribute.FlagTransitive | attribute.FlagExtLength)
		buf[off+1] = byte(attribute.AttrASPath)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec // valueLen validated ≤ max attr len
		off += 4
	} else {
		buf[off] = byte(attribute.FlagTransitive)
		buf[off+1] = byte(attribute.AttrASPath)
		buf[off+2] = byte(valueLen)
		off += 3
	}

	// Value: write segments, splitting at 255 ASNs
	remaining = len(asns)
	idx := 0
	for remaining > 0 {
		chunk := min(remaining, attribute.MaxASPathSegmentLength)

		buf[off] = byte(attribute.ASSequence) // Type
		buf[off+1] = byte(chunk)              // Count
		off += 2

		for i := range chunk {
			asn := asns[idx+i]
			if asn4 {
				binary.BigEndian.PutUint32(buf[off:], asn)
				off += 4
			} else {
				// RFC 6793: Map to AS_TRANS if > 65535
				if asn > 65535 {
					binary.BigEndian.PutUint16(buf[off:], 23456) // AS_TRANS
				} else {
					binary.BigEndian.PutUint16(buf[off:], uint16(asn)) //nolint:gosec // asn checked ≤ 65535 in else branch
				}
				off += 2
			}
		}

		idx += chunk
		remaining -= chunk
	}

	return off - start
}

// isNonMappableAS reports whether asn cannot be represented in two octets.
// RFC 6793 §4.2.1: a four-octet AS is "mappable" only when its high two octets
// are zero; a non-mappable AS is encoded as AS_TRANS (23456) in a two-octet
// AS_PATH, which obliges the sender to also emit an AS4_PATH (§4.2.2).
func isNonMappableAS(asn uint32) bool { return asn > 0xFFFF }

// anyNonMappableAS reports whether asns contains a non-mappable four-octet AS.
// Callers use it to decide whether a two-octet AS_PATH they are about to send
// needs an accompanying AS4_PATH (RFC 6793 §4.2.2): when every AS is mappable the
// AS4_PATH MUST NOT be sent.
func anyNonMappableAS(asns []uint32) bool {
	return slices.ContainsFunc(asns, isNonMappableAS)
}

// asPathHasNonMappableAS reports whether asPath carries a non-mappable AS in a
// non-confederation segment. Confederation segments are skipped because RFC 6793
// §3 forbids them in AS4_PATH, so a non-mappable AS confined to a confed segment
// would never appear in the AS4_PATH we would emit and must not trigger one.
func asPathHasNonMappableAS(asPath *attribute.ASPath) bool {
	if asPath == nil {
		return false
	}
	for _, seg := range asPath.Segments {
		if seg.Type == attribute.ASConfedSequence || seg.Type == attribute.ASConfedSet {
			continue
		}
		if anyNonMappableAS(seg.ASNs) {
			return true
		}
	}
	return false
}

// writeAS4PathForASNs writes an AS4_PATH attribute (RFC 6793 §4.2.2) carrying
// asns as a single four-octet AS_SEQUENCE into buf at off, returning bytes
// written. It emits only toward an OLD (2-octet) peer (asn4 == false) and only
// when asns holds a non-mappable AS, i.e. exactly when the two-octet AS_PATH just
// written had to substitute AS_TRANS; otherwise it writes nothing. The origination
// encoders synthesize a single-sequence path, so a single AS_SEQUENCE suffices
// (multi-segment stored paths use asPathHasNonMappableAS + an AS4Path built from
// the segments directly). Reuses the tested AS4Path encoder.
func writeAS4PathForASNs(buf []byte, off int, asn4 bool, asns []uint32) int {
	as4 := as4PathForASNs(asn4, asns)
	if as4 == nil {
		return 0
	}
	return attribute.WriteAttrTo(as4, buf, off)
}

// as4PathForASNs returns the AS4_PATH attribute writeAS4PathForASNs would write,
// or nil when none is owed. Split out so a caller that must place the attribute at
// its type-code position (rather than at the current end of the block) can ask for
// the attribute and hand it to an ordered insert, without duplicating the RFC 6793
// §4.2.2 "only toward an OLD peer, only for a non-mappable AS" condition.
func as4PathForASNs(asn4 bool, asns []uint32) *attribute.AS4Path {
	if asn4 || !anyNonMappableAS(asns) {
		return nil
	}
	return &attribute.AS4Path{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}}}
}

// writeNextHopAttr writes NEXT_HOP attribute directly to buf.
// RFC 4271 §5.1.3: Well-known mandatory, 4 bytes for IPv4.
//
// It takes the four octets rather than the address, so the length octet it writes
// and the value that follows cannot disagree and there is no address form left
// for it to reject. nextHopV4Octets is where an address becomes those octets, and
// where one that is not an IPv4 address is refused.
func writeNextHopAttr(buf []byte, off int, a4 [4]byte) int {
	// Header: Transitive(0x40) | code(3) | len(4)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrNextHop)
	buf[off+2] = 4
	copy(buf[off+3:], a4[:])
	return 7
}

// writeMEDAttr writes MED attribute directly to buf.
// RFC 4271 §5.1.4: Optional non-transitive, 4 bytes.
func writeMEDAttr(buf []byte, off int, med uint32) int {
	// Header: Optional(0x80) | code(4) | len(4)
	buf[off] = byte(attribute.FlagOptional)
	buf[off+1] = byte(attribute.AttrMED)
	buf[off+2] = 4
	binary.BigEndian.PutUint32(buf[off+3:], med)
	return 7
}

// writeLocalPrefAttr writes LOCAL_PREF attribute directly to buf.
// RFC 4271 §5.1.5: Well-known for iBGP, 4 bytes.
func writeLocalPrefAttr(buf []byte, off int, localPref uint32) int {
	// Header: Transitive(0x40) | code(5) | len(4)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrLocalPref)
	buf[off+2] = 4
	binary.BigEndian.PutUint32(buf[off+3:], localPref)
	return 7
}

// writeCommunitiesAttr writes COMMUNITIES attribute directly to buf.
// RFC 1997: Optional transitive, 4 bytes per community.
// RFC 4271 §4.3: Uses extended length for >63 communities (>255 bytes).
func writeCommunitiesAttr(buf []byte, off int, communities []uint32) int {
	start := off
	valueLen := len(communities) * 4

	// RFC 4271 §4.3: Use extended length if > 255 bytes
	flags := attribute.FlagOptional | attribute.FlagTransitive
	if valueLen > 255 {
		buf[off] = byte(flags | attribute.FlagExtLength)
		buf[off+1] = byte(attribute.AttrCommunity)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec // valueLen validated ≤ max attr len
		off += 4
	} else {
		buf[off] = byte(flags)
		buf[off+1] = byte(attribute.AttrCommunity)
		buf[off+2] = byte(valueLen)
		off += 3
	}

	for _, c := range communities {
		binary.BigEndian.PutUint32(buf[off:], c)
		off += 4
	}

	return off - start
}

// writeAnnounceUpdate writes a complete BGP UPDATE message for announcing a route
// directly into buf at offset off. Returns total bytes written.
//
// True zero-allocation: writes all attributes directly to the buffer.
//
// RFC 4271 Section 4.3 - UPDATE message format.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
//
// RFC 2545 Section 3: linkLocalNextHop is the link-local address to write after
// the global one in the MP_REACH Next Hop field of an IPv6 route. The zero Addr
// means the field carries the global address alone. The caller has already
// decided the section's condition against the host interface table
// (Peer.linkLocalNextHopFor, link_scope.go); this writer only encodes the answer.
//
// It writes NOTHING and returns 0 for a next hop that cannot fill the field it is
// about to declare a length for. A successful write is never shorter than the BGP
// header, so 0 names the refusal and no partial message reaches the buffer.
// ValidateAnnounceNextHop gives the reason, and Session.SendAnnounce returns it.
//
// The refusal is here rather than only in a caller-side guard because this
// function is exported: a guard in Session.SendAnnounce would bind one caller,
// while the length octet is written here for every caller there will ever be.
// The int return is the buffer-first writer contract this package keeps
// (WriteWithdrawUpdate, nlri.WriteNLRI, attribute.WriteAttrTo), so the reason
// travels beside it rather than inside it.
//
//nolint:unparam // buffer-first wire contract write(buf, off) int (ai/rules/performance.md): off says how far the caller's pooled buffer is already filled, which is what lets this writer and every one it calls (writeOriginAttr, writeASPathAttr, writeCommunitiesAttr, nlri.WriteNLRI) write forward with no reslice and no allocation. session_write.go passes 0 because one UPDATE owns the whole send buffer
func writeAnnounceUpdate(buf []byte, off int, route bgptypes.RouteSpec, linkLocalNextHop netip.Addr, localAS uint32, isIBGP, asn4, addPath bool) int {
	start := off

	// Both halves of the Next Hop field are settled before the first byte is
	// written, so a refusal leaves the caller's buffer untouched rather than a
	// part-built UPDATE in a session write buffer.
	nextHop, err := announceNextHopOctets(route, linkLocalNextHop)
	if err != nil {
		return 0
	}
	isIPv6 := route.Prefix.Addr().Is6()

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := range message.MarkerLen {
		buf[off+i] = 0xFF
	}
	off += message.MarkerLen

	// Length placeholder (backfill after body)
	lengthPos := off
	off += 2

	// Type = UPDATE
	buf[off] = byte(msgtype.TypeUPDATE)
	off++

	// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0 (announce, not withdraw)
	buf[off] = 0
	buf[off+1] = 0
	off += 2

	// Path Attributes Length placeholder (backfill after attrs)
	attrLenPos := off
	off += 2
	attrStart := off

	// Extract attributes from Wire (wire-first approach)
	origin := uint8(attribute.OriginIGP)
	var med *uint32
	var localPref *uint32
	var communities []uint32
	var largeCommunities []attribute.LargeCommunity
	var extCommunities []attribute.ExtendedCommunity
	var userASPath []uint32

	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil && originAttr != nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				origin = uint8(o)
			}
		}
		// Extract AS_PATH (all segments)
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok {
				for _, seg := range asp.Segments {
					userASPath = append(userASPath, seg.ASNs...)
				}
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil && medAttr != nil {
			if m, ok := medAttr.(attribute.MED); ok {
				v := uint32(m)
				med = &v
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil && lpAttr != nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				v := uint32(lp)
				localPref = &v
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				communities = make([]uint32, len(comms))
				for i, c := range comms {
					communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lc, ok := lcAttr.(attribute.LargeCommunities); ok {
				largeCommunities = lc
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ec, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				extCommunities = ec
			}
		}
	}

	// 1. ORIGIN - RFC 4271 §5.1.1: Well-known mandatory attribute.
	off += writeOriginAttr(buf, off, origin)

	// 2. AS_PATH - RFC 4271 §5.1.2: Well-known mandatory attribute.
	// Zero-alloc: write directly without creating ASPath struct.
	var asPathASNs []uint32
	switch {
	case len(userASPath) > 0:
		asPathASNs = userASPath // Use caller's slice directly
	case isIBGP:
		asPathASNs = nil // Empty AS_PATH for iBGP
	default: // eBGP: prepend local AS - use stack-allocated array
		asPathASNs = []uint32{localAS}
	}
	off += writeASPathAttr(buf, off, asPathASNs, asn4)

	// 3. NEXT_HOP - RFC 4271 §5.1.3 (IPv4 only; IPv6 uses MP_REACH_NLRI)
	if !isIPv6 {
		off += writeNextHopAttr(buf, off, nextHop.v4)
	}

	// 4. MED - RFC 4271 §5.1.4: Optional non-transitive attribute.
	if med != nil {
		off += writeMEDAttr(buf, off, *med)
	}

	// 5. LOCAL_PREF - RFC 4271 §5.1.5: Well-known attribute for iBGP only.
	// localPrefAllowedTo (forward_local_pref.go) owns that answer for every rail.
	if localPrefAllowedTo(isIBGP) {
		lpVal := uint32(100)
		if localPref != nil {
			lpVal = *localPref
		}
		off += writeLocalPrefAttr(buf, off, lpVal)
	}

	// 6. COMMUNITY - RFC 1997: Optional transitive attribute.
	if len(communities) > 0 {
		off += writeCommunitiesAttr(buf, off, communities)
	}

	// 7. LARGE_COMMUNITY - RFC 8092: Optional transitive attribute.
	// Type conversion only, no allocation.
	if len(largeCommunities) > 0 {
		lcomms := attribute.LargeCommunities(largeCommunities)
		off += attribute.WriteAttrTo(lcomms, buf, off)
	}

	// 8. EXTENDED_COMMUNITIES - RFC 4360: Optional transitive attribute.
	// Type conversion only, no allocation.
	if len(extCommunities) > 0 {
		extComms := attribute.ExtendedCommunities(extCommunities)
		off += attribute.WriteAttrTo(extComms, buf, off)
	}

	// NLRI handling - MP_REACH_NLRI (14) goes at end per our pattern
	if !isIPv6 {
		// AS4_PATH (17) last, after every lower-numbered attribute, when the
		// two-octet AS_PATH above had to carry AS_TRANS toward an OLD peer.
		off += writeAS4PathForASNs(buf, off, asn4, asPathASNs)

		// IPv4: Write NLRI directly after attributes (zero-alloc)
		// Backfill attr length first
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)

		// RFC 7911: WriteNLRI handles ADD-PATH encoding
		inet := nlri.NewINET(family.IPv4Unicast, route.Prefix, 0)
		off += nlri.WriteNLRI(inet, buf, off, addPath)
	} else {
		// RFC 4760 Section 3 - IPv6: Write MP_REACH_NLRI directly (zero-alloc)
		// Wire format: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(16 or 32) + Reserved(1) + NLRI(var)
		inet := nlri.NewINET(family.IPv6Unicast, route.Prefix, 0)
		nlriPayloadLen := nlri.LenWithContext(inet, addPath)

		// RFC 2545 Section 3: "The value of the Length of Next Hop Network Address
		// field on a MP_REACH_NLRI attribute shall be set to 16, when only a global
		// address is present, or 32 if a link-local address is also included in the
		// Next Hop field." The caller decided inclusion; the length follows it, and
		// nextHopLinkLocalOctets has already refused an address that would fill the
		// second slot with something Section 3 does not name.
		nhLen := 16
		if nextHop.withLinkLocal {
			nhLen = 32
		}
		mpValueLen := 2 + 1 + 1 + nhLen + 1 + nlriPayloadLen

		// RFC 4760 Section 3 - Attribute header (Optional, non-transitive)
		off += attribute.WriteHeaderTo(buf, off, attribute.FlagOptional, attribute.AttrMPReachNLRI, uint16(mpValueLen)) //nolint:gosec // mpValueLen bounded by UPDATE max

		// RFC 4760 Section 3 - AFI (2 octets)
		buf[off] = 0
		buf[off+1] = byte(attribute.AFIIPv6)
		off += 2

		// RFC 4760 Section 3 - SAFI (1 octet)
		buf[off] = byte(attribute.SAFIUnicast)
		off++

		// RFC 4760 Section 3 - Length of Next Hop (1 octet)
		buf[off] = byte(nhLen)
		off++

		// RFC 4760 Section 3 - Network Address of Next Hop (variable)
		// RFC 2545 Section 3: the global address is always first, the link-local
		// address second. Both were settled at entry, so each one fills the sixteen
		// octets the length above promises and names the host the caller gave.
		off += copy(buf[off:], nextHop.global[:])
		if nextHop.withLinkLocal {
			off += copy(buf[off:], nextHop.linkLocal[:])
		}

		// RFC 4760 Section 3 - Reserved (1 octet, MUST be 0)
		buf[off] = 0
		off++

		// RFC 4760 Section 3 - NLRI (variable)
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// AS4_PATH (17) after MP_REACH_NLRI (14) keeps type-code order, when the
		// two-octet AS_PATH above had to carry AS_TRANS toward an OLD peer.
		off += writeAS4PathForASNs(buf, off, asn4, asPathASNs)

		// Backfill attr length (no inline NLRI for IPv6)
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)
	}

	// Backfill total message length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

	return totalLen
}

// writeWithdrawUpdate writes a complete BGP UPDATE message for withdrawing a route
// directly into buf at offset off. Returns total bytes written.
//
// Eliminates large buffer allocations by writing directly to the provided buffer.
//
// RFC 4271 Section 4.3 - UPDATE message format.
// RFC 4760 Section 4: IPv6 withdrawals use MP_UNREACH_NLRI attribute.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
//
//nolint:unparam // buffer-first wire contract write(buf, off) int (ai/rules/performance.md): off says how far the caller's pooled buffer is already filled, so the marker, the length field and the NLRI go in at an advancing offset with no reslice and no allocation. session_write.go passes 0 because one UPDATE owns the whole send buffer
func writeWithdrawUpdate(buf []byte, off int, prefix netip.Prefix, addPath bool) int {
	start := off

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := range message.MarkerLen {
		buf[off+i] = 0xFF
	}
	off += message.MarkerLen

	// Length placeholder
	lengthPos := off
	off += 2

	// Type = UPDATE
	buf[off] = byte(msgtype.TypeUPDATE)
	off++

	if prefix.Addr().Is4() {
		// RFC 4271 Section 4.3 - IPv4: Use WithdrawnRoutes field (zero-alloc)
		// Withdrawn Routes Length placeholder
		withdrawnLenPos := off
		off += 2
		withdrawnStart := off

		// RFC 4271 Section 4.3 - Withdrawn Routes: list of IP address prefixes
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		inet := nlri.NewINET(family.IPv4Unicast, prefix, 0)
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// RFC 4271 Section 4.3 - Backfill Withdrawn Routes Length
		withdrawnLen := off - withdrawnStart
		buf[withdrawnLenPos] = byte(withdrawnLen >> 8)
		buf[withdrawnLenPos+1] = byte(withdrawnLen)

		// RFC 4271 Section 4.3 - Total Path Attribute Length = 0 (withdrawal only)
		buf[off] = 0
		buf[off+1] = 0
		off += 2
	} else {
		// RFC 4760 Section 4 - IPv6: Use MP_UNREACH_NLRI attribute (zero-alloc)
		// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0 (using MP_UNREACH instead)
		buf[off] = 0
		buf[off+1] = 0
		off += 2

		// RFC 4271 Section 4.3 - Path Attributes Length placeholder
		attrLenPos := off
		off += 2
		attrStart := off

		// RFC 4760 Section 4 - MP_UNREACH_NLRI wire format:
		//   AFI(2) + SAFI(1) + Withdrawn_NLRI(var)
		inet := nlri.NewINET(family.IPv6Unicast, prefix, 0)
		nlriPayloadLen := nlri.LenWithContext(inet, addPath)
		mpValueLen := 2 + 1 + nlriPayloadLen

		// RFC 4760 Section 4 - Attribute header (Optional, non-transitive)
		off += attribute.WriteHeaderTo(buf, off, attribute.FlagOptional, attribute.AttrMPUnreachNLRI, uint16(mpValueLen)) //nolint:gosec // mpValueLen bounded by UPDATE max

		// RFC 4760 Section 4 - AFI (2 octets)
		buf[off] = 0
		buf[off+1] = byte(attribute.AFIIPv6)
		off += 2

		// RFC 4760 Section 4 - SAFI (1 octet)
		buf[off] = byte(attribute.SAFIUnicast)
		off++

		// RFC 4760 Section 4 - Withdrawn Routes (variable)
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// Backfill attr length
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)
	}

	// Backfill total message length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

	return totalLen
}
