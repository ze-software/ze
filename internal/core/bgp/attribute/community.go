// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc1997.md — BGP communities
// RFC: rfc/short/rfc4360.md — extended communities
// RFC: rfc/short/rfc8092.md — large communities
// Related: flowspec_action.go -- keyword table producing ExtendedCommunity values

package attribute

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strings"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// Community represents a standard BGP community.
//
// RFC 1997 - BGP Communities Attribute:
// Communities are treated as 32 bit values. The community attribute values
// ranging from 0x00000000 through 0x0000FFFF and 0xFFFF0000 through 0xFFFFFFFF
// are reserved. Other values encode an AS number in the first two octets.
//
// RFC 1997 Section "COMMUNITIES attribute":
// The COMMUNITIES path attribute is an optional transitive attribute of
// variable length. The attribute consists of a set of four octet values,
// each of which specify a community.
type Community uint32

// Well-known communities.
//
// RFC 1997 Section "Well-known Communities":
// The following communities have global significance and their operations
// shall be implemented in any community-attribute-aware BGP speaker.
const (
	// CommunityNoExport - RFC 1997: All routes received carrying a communities
	// attribute containing this value MUST NOT be advertised outside a BGP
	// confederation boundary.
	CommunityNoExport Community = 0xFFFFFF01 // NO_EXPORT

	// CommunityNoAdvertise - RFC 1997: All routes received carrying a communities
	// attribute containing this value MUST NOT be advertised to other BGP peers.
	CommunityNoAdvertise Community = 0xFFFFFF02 // NO_ADVERTISE

	// CommunityNoExportSubconfed - RFC 1997: All routes received carrying a
	// communities attribute containing this value MUST NOT be advertised to
	// external BGP peers (this includes peers in other member autonomous systems
	// inside a BGP confederation).
	CommunityNoExportSubconfed Community = 0xFFFFFF03 // NO_EXPORT_SUBCONFED

	// CommunityNoPeer - RFC 3765: Routes with this community should not be
	// advertised to peers.
	CommunityNoPeer Community = 0xFFFFFF04 // NOPEER (RFC 3765)

	// CommunityGracefulShutdown - RFC 8326: Signals that a BGP session or
	// link is about to go down. Receivers should de-preference tagged routes.
	CommunityGracefulShutdown Community = 0xFFFF0000 // GRACEFUL_SHUTDOWN (RFC 8326)

	// CommunityAcceptOwn - RFC 7611: Allows a router to accept routes with
	// its own AS in the AS_PATH for VRF-to-VRF re-import on the same PE.
	CommunityAcceptOwn Community = 0xFFFF0001 // ACCEPT_OWN (RFC 7611)

	// CommunityRouteFilterTranslatedV4 - IANA: Used by route servers to
	// signal translated IPv4 route filter results to clients.
	CommunityRouteFilterTranslatedV4 Community = 0xFFFF0002

	// CommunityRouteFilterV4 - IANA: Used by route servers to signal
	// IPv4 route filter results to clients.
	CommunityRouteFilterV4 Community = 0xFFFF0003

	// CommunityRouteFilterTranslatedV6 - IANA: Used by route servers to
	// signal translated IPv6 route filter results to clients.
	CommunityRouteFilterTranslatedV6 Community = 0xFFFF0004

	// CommunityRouteFilterV6 - IANA: Used by route servers to signal
	// IPv6 route filter results to clients.
	CommunityRouteFilterV6 Community = 0xFFFF0005

	// CommunityLLGRStale - RFC 9494 Section 3: Routes retained beyond the
	// Graceful Restart timer are marked with this community. LLGR_STALE routes
	// are treated as least preferred in route selection.
	CommunityLLGRStale Community = 0xFFFF0006 // LLGR_STALE (RFC 9494)

	// CommunityNoLLGR - RFC 9494 Section 3: Routes carrying this community
	// MUST NOT be retained when Long-Lived Graceful Restart procedures are in
	// effect. They are deleted immediately when the LLGR period begins.
	CommunityNoLLGR Community = 0xFFFF0007 // NO_LLGR (RFC 9494)

	// CommunityAcceptOwnNexthop - IANA (draft): Allows accepting routes
	// where the next-hop is the receiving router's own address.
	CommunityAcceptOwnNexthop Community = 0xFFFF0008

	// CommunityStandbyPE - RFC 9026: Used in EVPN multi-homing to signal
	// that the advertising PE is the standby for the Ethernet Segment.
	CommunityStandbyPE Community = 0xFFFF0009 // STANDBY_PE (RFC 9026)

	// CommunityBlackhole - RFC 7999: Signals to upstream providers to
	// null-route traffic destined for the tagged prefix (RTBH filtering).
	CommunityBlackhole Community = 0xFFFF029A // BLACKHOLE (RFC 7999)
)

// communityNames maps well-known community values to display names.
// Pre-populated with RFC 1997/3765 built-ins. Plugins register additional
// names via RegisterCommunityName during init().
// Read-only after init phase completes; no mutex needed.
var communityNames = map[Community]string{
	CommunityNoExport:                "no-export",
	CommunityNoAdvertise:             "no-advertise",
	CommunityNoExportSubconfed:       "no-export-subconfed",
	CommunityNoPeer:                  "nopeer",
	CommunityGracefulShutdown:        "graceful-shutdown",
	CommunityAcceptOwn:               "accept-own",
	CommunityRouteFilterTranslatedV4: "route-filter-translated-v4",
	CommunityRouteFilterV4:           "route-filter-v4",
	CommunityRouteFilterTranslatedV6: "route-filter-translated-v6",
	CommunityRouteFilterV6:           "route-filter-v6",
	CommunityLLGRStale:               "llgr-stale",
	CommunityNoLLGR:                  "no-llgr",
	CommunityAcceptOwnNexthop:        "accept-own-nexthop",
	CommunityStandbyPE:               "standby-pe",
	CommunityBlackhole:               "blackhole",
}

// communityAliases holds accepted spellings the canonical name does not
// generate. The underscore form of every canonical name is derived. Only
// irregular spellings belong here: "gshut" for graceful-shutdown, and the
// hyphenated forms of "nopeer", whose canonical name carries no hyphen to
// convert.
//
// Aliases are accepted on parse and never produced on output, so the reverse
// table is deliberately larger than the forward one.
var communityAliases = map[string]Community{
	"gshut":   CommunityGracefulShutdown,
	"no-peer": CommunityNoPeer,
	"no_peer": CommunityNoPeer,
}

// communityValues maps every accepted text spelling to its wire value. It is
// DERIVED from communityNames plus communityAliases, never written by hand: a
// second hand-maintained table is what let two parsers drift to 5 names and 31.
//
// Text keys this map. Every other lookup in this package keys on the numeric
// value. Parsing is the one boundary where the input is text.
// Read-only after the init phase completes, so no mutex is needed.
var communityValues = func() map[string]Community {
	m := make(map[string]Community, len(communityNames)*2+len(communityAliases))
	for value, name := range communityNames {
		addCommunityText(m, value, name)
	}
	maps.Copy(m, communityAliases)
	return m
}()

// addCommunityText records a canonical name and its underscore spelling.
func addCommunityText(m map[string]Community, value Community, name string) {
	m[name] = value
	if underscored := strings.ReplaceAll(name, "-", "_"); underscored != name {
		m[underscored] = value
	}
}

// CommunityValue resolves a well-known community name to its wire value.
// The lookup accepts the canonical name, its underscore spelling, and any
// registered alias. Callers that also accept ASN:value or hex forms use
// ParseCommunity, which consults this first.
func CommunityValue(name string) (Community, bool) {
	v, ok := communityValues[strings.ToLower(name)]
	return v, ok
}

// CommunityFrom4 constructs a Community from 4 raw bytes (network byte order).
func CommunityFrom4(b [4]byte) Community {
	return Community(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]))
}

// RegisterCommunityName registers a display name for a well-known community value.
// Called by plugins during init() to add their community names without modifying
// the attribute package. Idempotent: re-registering the same value+name is a no-op.
// Returns an error if the value is already registered with a different name.
// Must only be called during init() (not thread-safe for concurrent writes).
func RegisterCommunityName(value Community, name string) error {
	if existing, ok := communityNames[value]; ok {
		if existing == name {
			return nil // idempotent
		}
		return fmt.Errorf("community 0x%08X already registered as %q, cannot re-register as %q", uint32(value), existing, name)
	}
	communityNames[value] = name
	// Both directions come from this one call, so a registered name is parseable
	// as soon as it is renderable. Registering only the forward map is how a
	// plugin's name became visible to String() and invisible to every parser.
	addCommunityText(communityValues, value, name)
	return nil
}

// RegisterCommunityAlias registers one more accepted spelling for a
// community that is already named. The alias parses to the value and is never
// rendered: output always uses the canonical name from RegisterCommunityName.
// Called by plugins during init(). Idempotent for the same alias and value.
// Must only be called during init() (not thread-safe for concurrent writes).
func RegisterCommunityAlias(value Community, alias string) error {
	if _, ok := communityNames[value]; !ok {
		return fmt.Errorf("community 0x%08X has no registered name, cannot add alias %q", uint32(value), alias)
	}
	if existing, ok := communityValues[alias]; ok && existing != value {
		return fmt.Errorf("community alias %q already resolves to 0x%08X, cannot point it at 0x%08X", alias, uint32(existing), uint32(value))
	}
	communityValues[alias] = value
	return nil
}

// String returns the community in human-readable format.
// Checks the community name registry first, then falls back to ASN:value format.
//
// RFC 1997: Community values encode an AS number in the first two octets
// and a locally defined value in the last two octets, displayed as ASN:value.
func (c Community) String() string {
	if name, ok := communityNames[c]; ok {
		return name
	}
	var buf [11]byte
	return string(appendCommunityText(buf[:0], uint32(c)))
}

// Communities represents the COMMUNITIES attribute.
//
// RFC 1997 Section "COMMUNITIES attribute":
// The COMMUNITIES path attribute has Type Code 8. It is an optional
// transitive attribute of variable length consisting of a set of
// four octet values.
type Communities []Community

// Code returns the attribute type code (8) per RFC 1997.
func (c Communities) Code() AttributeCode { return AttrCommunity }

// Flags returns the attribute flags (optional transitive) per RFC 1997.
func (c Communities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (4 bytes per community).
func (c Communities) Len() int { return len(c) * 4 }

// WriteTo writes the communities into buf at offset.
func (c Communities) WriteTo(buf []byte, off int) int {
	for i, comm := range c {
		binary.BigEndian.PutUint32(buf[off+i*4:], uint32(comm))
	}
	return len(c) * 4
}

// WriteToWithContext writes communities - context-independent.
func (c Communities) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return c.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (c Communities) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := c.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return c.WriteTo(buf, off), nil
}

// ParseCommunities parses a COMMUNITIES attribute.
//
// RFC 1997: The attribute consists of a set of four octet values.
// Length must be a multiple of 4 bytes.
func ParseCommunities(data []byte) (Communities, error) {
	if len(data)%4 != 0 {
		return nil, ErrInvalidLength
	}
	comms := make(Communities, len(data)/4)
	for i := range comms {
		comms[i] = Community(binary.BigEndian.Uint32(data[i*4:]))
	}
	return comms, nil
}

// Contains returns true if the community list contains the given community.
func (c Communities) Contains(comm Community) bool {
	return slices.Contains(c, comm)
}

// ExtendedCommunity represents an extended community.
//
// RFC 4360 Section 2 - BGP Extended Communities Attribute:
// Each Extended Community is encoded as an 8-octet quantity:
//   - Type Field:  1 or 2 octets
//   - Value Field: Remaining octets
//
// RFC 4360 Section 2:
// The high-order octet of the Type Field contains:
//   - Bit 0 (I): IANA authority bit
//   - Bit 1 (T): Transitive bit (0=transitive, 1=non-transitive across AS)
//   - Bits 2-7: Structure of the community
//
// Two extended communities are declared equal only when all 8 octets are equal.
type ExtendedCommunity [8]byte

// ExtendedCommunities represents the EXTENDED_COMMUNITIES attribute.
//
// RFC 4360 Section 2:
// The Extended Communities Attribute is a transitive optional BGP attribute,
// with the Type Code 16. The attribute consists of a set of "extended communities".
type ExtendedCommunities []ExtendedCommunity

// Code returns the attribute type code (16) per RFC 4360 Section 2.
func (e ExtendedCommunities) Code() AttributeCode { return AttrExtCommunity }

// Flags returns the attribute flags (transitive optional) per RFC 4360 Section 2.
func (e ExtendedCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (8 bytes per extended community).
func (e ExtendedCommunities) Len() int { return len(e) * 8 }

// WriteTo writes the extended communities into buf at offset.
func (e ExtendedCommunities) WriteTo(buf []byte, off int) int {
	for i, ec := range e {
		copy(buf[off+i*8:], ec[:])
	}
	return len(e) * 8
}

// WriteToWithContext writes extended communities - context-independent.
func (e ExtendedCommunities) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return e.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (e ExtendedCommunities) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := e.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return e.WriteTo(buf, off), nil
}

// ParseExtendedCommunities parses an EXTENDED_COMMUNITIES attribute.
//
// RFC 4360 Section 2: Each Extended Community is encoded as an 8-octet quantity.
// Length must be a multiple of 8 bytes.
func ParseExtendedCommunities(data []byte) (ExtendedCommunities, error) {
	if len(data)%8 != 0 {
		return nil, ErrInvalidLength
	}
	comms := make(ExtendedCommunities, len(data)/8)
	for i := range comms {
		copy(comms[i][:], data[i*8:])
	}
	return comms, nil
}

// LargeCommunity represents a large community.
//
// RFC 8092 Section 3 - BGP Large Communities Attribute:
// Each BGP Large Community value is encoded as a 12-octet quantity:
//
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                      Global Administrator                     |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       Local Data Part 1                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       Local Data Part 2                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 8092 Section 3:
// Global Administrator: A four-octet namespace identifier (SHOULD be an ASN).
// Local Data Part 1:    A four-octet operator-defined value.
// Local Data Part 2:    A four-octet operator-defined value.
type LargeCommunity struct {
	GlobalAdmin uint32 // RFC 8092: Global Administrator (typically ASN)
	LocalData1  uint32 // RFC 8092: Local Data Part 1
	LocalData2  uint32 // RFC 8092: Local Data Part 2
}

// String returns the large community in canonical representation.
//
// RFC 8092 Section 5 - Canonical Representation:
// The canonical representation is three separate unsigned integers in decimal
// notation: Global Administrator, Local Data 1, Local Data 2. Each number is
// separated from the next by a single colon. Numbers MUST NOT contain leading
// zeros; a zero value MUST be represented with a single zero.
func (l LargeCommunity) String() string {
	var buf [32]byte
	return string(l.AppendText(buf[:0]))
}

// LargeCommunities represents the LARGE_COMMUNITIES attribute.
//
// RFC 8092 Section 3:
// The BGP Large Communities attribute is an optional transitive path attribute
// of variable length. All routes with this attribute belong to the communities
// specified in the attribute.
//
// RFC 8092 Section 8 - IANA Considerations:
// IANA has assigned the value 32 (LARGE_COMMUNITY) in the "BGP Path Attributes"
// subregistry.
type LargeCommunities []LargeCommunity

// Code returns the attribute type code (32) per RFC 8092 Section 8.
func (l LargeCommunities) Code() AttributeCode { return AttrLargeCommunity }

// Flags returns the attribute flags (optional transitive) per RFC 8092 Section 3.
func (l LargeCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (12 bytes per unique large community).
func (l LargeCommunities) Len() int { return len(l.unique()) * 12 }

// WriteTo writes the unique large communities into buf at offset.
func (l LargeCommunities) WriteTo(buf []byte, off int) int {
	u := l.unique()
	for i, lc := range u {
		pos := off + i*12
		binary.BigEndian.PutUint32(buf[pos:], lc.GlobalAdmin)
		binary.BigEndian.PutUint32(buf[pos+4:], lc.LocalData1)
		binary.BigEndian.PutUint32(buf[pos+8:], lc.LocalData2)
	}
	return len(u) * 12
}

// unique returns the deduplicated communities. If the slice has no duplicates
// (the common case after ParseLargeCommunities), it returns the original slice
// with zero allocation. Uses O(n^2) scan for small slices, falls back to
// map-based dedup for large ones.
func (l LargeCommunities) unique() LargeCommunities {
	if len(l) <= 1 {
		return l
	}
	if len(l) <= 16 {
		for i := 1; i < len(l); i++ {
			for j := range i {
				if l[i] == l[j] {
					return l.deduplicate()
				}
			}
		}
		return l
	}
	return l.deduplicate()
}

// WriteToWithContext writes large communities - context-independent.
func (l LargeCommunities) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return l.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (l LargeCommunities) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := l.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return l.WriteTo(buf, off), nil
}

// deduplicate returns a new slice with duplicate communities removed.
// Preserves order of first occurrence.
func (l LargeCommunities) deduplicate() LargeCommunities {
	if len(l) <= 1 {
		return l
	}

	seen := make(map[LargeCommunity]struct{}, len(l))
	result := make(LargeCommunities, 0, len(l))

	for _, lc := range l {
		if _, exists := seen[lc]; !exists {
			seen[lc] = struct{}{}
			result = append(result, lc)
		}
	}

	return result
}

// ParseLargeCommunities parses a LARGE_COMMUNITIES attribute.
//
// RFC 8092 Section 5:
//
//	"A receiving speaker MUST silently remove redundant BGP Large Community
//	 values from a BGP Large Community attribute."
//
// RFC 8092 Section 6 - Error Handling:
// A BGP Large Communities attribute SHALL be considered malformed if the
// length, expressed in octets, is not a non-zero multiple of 12.
//
// A BGP UPDATE message with a malformed BGP Large Communities attribute
// SHALL be handled using the approach of "treat-as-withdraw" (RFC 7606).
func ParseLargeCommunities(data []byte) (LargeCommunities, error) {
	if len(data)%12 != 0 {
		return nil, ErrInvalidLength
	}

	count := len(data) / 12
	if count == 0 {
		return LargeCommunities{}, nil
	}

	// Parse all communities first
	comms := make(LargeCommunities, count)
	for i := range comms {
		offset := i * 12
		comms[i] = LargeCommunity{
			GlobalAdmin: binary.BigEndian.Uint32(data[offset:]),
			LocalData1:  binary.BigEndian.Uint32(data[offset+4:]),
			LocalData2:  binary.BigEndian.Uint32(data[offset+8:]),
		}
	}

	// RFC 8092 Section 5: MUST silently remove redundant values
	return comms.deduplicate(), nil
}

// IPv6ExtendedCommunity represents an IPv6 Address Specific Extended Community.
//
// RFC 5701 Section 2 - IPv6 Address Specific BGP Extended Community Attribute:
// Each IPv6 Address Specific extended community is encoded as a 20-octet quantity:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| 0x00 or 0x40  |    Sub-Type   |    Global Administrator       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|          Global Administrator (cont.)                         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|          Global Administrator (cont.)                         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|          Global Administrator (cont.)                         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Global Administrator (cont.)  |    Local Administrator        |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 5701 Section 2:
// The first high-order octet indicates whether the community is transitive (0x00)
// or non-transitive (0x40).
type IPv6ExtendedCommunity [20]byte

// IPv6ExtendedCommunities represents the IPV6_EXTENDED_COMMUNITIES attribute.
//
// RFC 5701 Section 2:
// The IPv6 Address Specific Extended Community Attribute is a transitive,
// optional BGP attribute. The attribute consists of a set of
// "IPv6 Address Specific extended communities".
type IPv6ExtendedCommunities []IPv6ExtendedCommunity

// Code returns the attribute type code (25) per RFC 5701 Section 3.
func (e IPv6ExtendedCommunities) Code() AttributeCode { return AttrIPv6ExtCommunity }

// Flags returns the attribute flags (transitive optional) per RFC 5701 Section 2.
func (e IPv6ExtendedCommunities) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the length in bytes (20 bytes per IPv6 extended community).
func (e IPv6ExtendedCommunities) Len() int { return len(e) * 20 }

// WriteTo writes the IPv6 extended communities into buf at offset.
func (e IPv6ExtendedCommunities) WriteTo(buf []byte, off int) int {
	for i, ec := range e {
		copy(buf[off+i*20:], ec[:])
	}
	return len(e) * 20
}

// WriteToWithContext writes IPv6 extended communities - context-independent.
func (e IPv6ExtendedCommunities) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return e.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (e IPv6ExtendedCommunities) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := e.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return e.WriteTo(buf, off), nil
}

// ParseIPv6ExtendedCommunities parses an IPV6_EXTENDED_COMMUNITIES attribute.
//
// RFC 5701 Section 2: Each IPv6 Extended Community is encoded as a 20-octet quantity.
// Length must be a multiple of 20 bytes.
func ParseIPv6ExtendedCommunities(data []byte) (IPv6ExtendedCommunities, error) {
	if len(data)%20 != 0 {
		return nil, ErrInvalidLength
	}
	comms := make(IPv6ExtendedCommunities, len(data)/20)
	for i := range comms {
		copy(comms[i][:], data[i*20:])
	}
	return comms, nil
}
