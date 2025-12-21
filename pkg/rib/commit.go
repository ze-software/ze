package rib

import (
	"errors"
	"net/netip"
	"sort"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// ErrNilNegotiated is returned when CommitService is used with nil Negotiated.
var ErrNilNegotiated = errors.New("commit: negotiated parameters required")

// UpdateSender is the interface for sending BGP UPDATE messages.
// This is implemented by Peer to allow CommitService to send updates.
type UpdateSender interface {
	SendUpdate(u *message.Update) error
}

// CommitOptions configures how a commit is performed.
type CommitOptions struct {
	SendEOR bool // Whether to send End-of-RIB after commit
}

// CommitServiceStats holds statistics from a commit operation.
type CommitServiceStats struct {
	UpdatesSent      int           // Number of UPDATE messages sent
	RoutesAnnounced  int           // Total routes announced
	RoutesWithdrawn  int           // Total routes withdrawn (future use)
	FamiliesAffected []nlri.Family // Address families that had routes
	EORSent          []nlri.Family // Families for which EOR was sent
}

// CommitService handles batched route commits with grouping and EOR.
//
// This provides a single abstraction for committing routes to peers,
// used by both config routes (on session establish) and API routes
// (on explicit commit command).
type CommitService struct {
	sender       UpdateSender
	negotiated   *message.Negotiated
	groupUpdates bool
}

// NewCommitService creates a new CommitService.
//
// sender: interface for sending UPDATE messages (typically a Peer)
// negotiated: session parameters for proper encoding (ASN4, families). Can be nil for defaults.
// groupUpdates: if true, routes with same attributes are grouped into fewer UPDATEs.
func NewCommitService(sender UpdateSender, negotiated *message.Negotiated, groupUpdates bool) *CommitService {
	return &CommitService{
		sender:       sender,
		negotiated:   negotiated,
		groupUpdates: groupUpdates,
	}
}

// Commit sends the given routes to the peer.
//
// If groupUpdates is enabled, routes with identical attributes are combined
// into fewer UPDATE messages. Otherwise, one UPDATE per route is sent.
//
// If SendEOR is true, End-of-RIB markers are sent for each affected family.
func (c *CommitService) Commit(routes []*Route, opts CommitOptions) (CommitServiceStats, error) {
	var stats CommitServiceStats

	if c.negotiated == nil {
		return stats, ErrNilNegotiated
	}

	if len(routes) == 0 {
		return stats, nil
	}

	// Track which families have routes
	familySeen := make(map[nlri.Family]bool)

	if c.groupUpdates {
		// Group routes by attributes for fewer UPDATEs
		groups := GroupByAttributes(routes)

		for _, group := range groups {
			update := c.buildGroupedUpdate(&group)
			if err := c.sender.SendUpdate(update); err != nil {
				return stats, err
			}
			stats.UpdatesSent++
			stats.RoutesAnnounced += len(group.Routes)
			familySeen[group.Family] = true
		}
	} else {
		// One UPDATE per route
		for _, route := range routes {
			update := c.buildSingleUpdate(route)
			if err := c.sender.SendUpdate(update); err != nil {
				return stats, err
			}
			stats.UpdatesSent++
			stats.RoutesAnnounced++
			familySeen[route.NLRI().Family()] = true
		}
	}

	// Collect affected families (sorted for determinism)
	for family := range familySeen {
		stats.FamiliesAffected = append(stats.FamiliesAffected, family)
	}
	sortFamilies(stats.FamiliesAffected)

	// Send EOR for each affected family if requested
	if opts.SendEOR {
		for _, family := range stats.FamiliesAffected {
			eor := message.BuildEOR(family)
			if err := c.sender.SendUpdate(eor); err != nil {
				return stats, err
			}
			stats.EORSent = append(stats.EORSent, family)
		}
	}

	return stats, nil
}

// buildGroupedUpdate builds an UPDATE message for a route group.
func (c *CommitService) buildGroupedUpdate(group *RouteGroup) *message.Update {
	family := group.Family
	nextHop := bytesToAddr(group.NextHop)

	// Collect all NLRIs
	var nlriBytes []byte
	for _, route := range group.Routes {
		nlriBytes = append(nlriBytes, route.NLRI().Bytes()...)
	}

	// Build path attributes
	attrBytes := c.packAttributes(group.Attributes, nextHop, family, nlriBytes)

	// Determine if NLRI goes in UPDATE.NLRI or MP_REACH_NLRI
	if c.useTraditionalNLRI(family, nextHop) {
		return &message.Update{
			PathAttributes: attrBytes,
			NLRI:           nlriBytes,
		}
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nil, // NLRI is in MP_REACH_NLRI
	}
}

// buildSingleUpdate builds an UPDATE message for a single route.
func (c *CommitService) buildSingleUpdate(route *Route) *message.Update {
	family := route.NLRI().Family()
	nextHop := route.NextHop()
	nlriBytes := route.NLRI().Bytes()

	attrBytes := c.packAttributes(route.Attributes(), nextHop, family, nlriBytes)

	if c.useTraditionalNLRI(family, nextHop) {
		return &message.Update{
			PathAttributes: attrBytes,
			NLRI:           nlriBytes,
		}
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nil,
	}
}

// useTraditionalNLRI returns true if NLRI should go in UPDATE.NLRI field.
// Returns false if NLRI should be in MP_REACH_NLRI attribute.
func (c *CommitService) useTraditionalNLRI(family nlri.Family, nextHop netip.Addr) bool {
	// Only IPv4 unicast with IPv4 next-hop uses traditional NLRI field
	// IPv4 unicast with IPv6 next-hop (RFC 5549) must use MP_REACH_NLRI
	return family.AFI == 1 && family.SAFI == 1 && nextHop.Is4()
}

// packAttributes packs path attributes into wire format.
func (c *CommitService) packAttributes(attrs []attribute.Attribute, nextHop netip.Addr, family nlri.Family, nlriBytes []byte) []byte {
	var result []byte

	// 1. ORIGIN (from provided attributes or default to IGP)
	hasOrigin := false
	for _, attr := range attrs {
		if attr.Code() == attribute.AttrOrigin {
			result = append(result, attribute.PackAttribute(attr)...)
			hasOrigin = true
			break
		}
	}
	if !hasOrigin {
		// Default to IGP origin
		result = append(result, attribute.PackAttribute(attribute.Origin(0))...)
	}

	// 2. AS_PATH (preserve existing if present, prepend local AS for eBGP)
	result = append(result, c.buildASPath(attrs)...)

	// 3. NEXT_HOP or MP_REACH_NLRI
	if c.useTraditionalNLRI(family, nextHop) {
		// Traditional IPv4 unicast with IPv4 next-hop: NEXT_HOP attribute
		nh := &attribute.NextHop{Addr: nextHop}
		result = append(result, attribute.PackAttribute(nh)...)
	} else {
		// All other cases: MP_REACH_NLRI with next-hop and NLRI
		mpReach := c.buildMPReachNLRI(family, nextHop, nlriBytes)
		result = append(result, attribute.PackAttribute(mpReach)...)
	}

	// 4. LOCAL_PREF for iBGP
	if c.isIBGP() {
		hasLocalPref := false
		for _, attr := range attrs {
			if attr.Code() == attribute.AttrLocalPref {
				result = append(result, attribute.PackAttribute(attr)...)
				hasLocalPref = true
				break
			}
		}
		if !hasLocalPref {
			result = append(result, attribute.PackAttribute(attribute.LocalPref(100))...)
		}
	}

	// 5. Other attributes (MED, communities, etc.)
	for _, attr := range attrs {
		code := attr.Code()
		// Skip attributes already handled above
		if code == attribute.AttrOrigin || code == attribute.AttrASPath ||
			code == attribute.AttrNextHop || code == attribute.AttrLocalPref {
			continue
		}
		result = append(result, attribute.PackAttribute(attr)...)
	}

	return result
}

// buildASPath builds the AS_PATH attribute.
// If an existing AS_PATH is provided, prepends local AS for eBGP.
// For iBGP, preserves the existing AS_PATH as-is.
func (c *CommitService) buildASPath(attrs []attribute.Attribute) []byte {
	// Find existing AS_PATH if any
	var existingASPath *attribute.ASPath
	for _, attr := range attrs {
		if attr.Code() == attribute.AttrASPath {
			if asp, ok := attr.(*attribute.ASPath); ok {
				existingASPath = asp
			}
			break
		}
	}

	if c.isIBGP() {
		// iBGP: preserve existing AS_PATH or use empty
		if existingASPath != nil {
			return attribute.PackASPathAttribute(existingASPath, c.negotiated.ASN4)
		}
		return attribute.PackASPathAttribute(&attribute.ASPath{Segments: nil}, c.negotiated.ASN4)
	}

	// eBGP: prepend local AS to existing path
	if existingASPath != nil && len(existingASPath.Segments) > 0 {
		// Prepend local AS to first segment if it's AS_SEQUENCE
		newSegments := make([]attribute.ASPathSegment, len(existingASPath.Segments))
		copy(newSegments, existingASPath.Segments)

		if len(newSegments) > 0 && newSegments[0].Type == attribute.ASSequence {
			// Prepend to first AS_SEQUENCE segment
			newASNs := make([]uint32, 0, len(newSegments[0].ASNs)+1)
			newASNs = append(newASNs, c.negotiated.LocalAS)
			newASNs = append(newASNs, newSegments[0].ASNs...)
			newSegments[0].ASNs = newASNs
		} else {
			// Insert new AS_SEQUENCE segment at beginning
			newSeg := attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{c.negotiated.LocalAS}}
			newSegments = append([]attribute.ASPathSegment{newSeg}, newSegments...)
		}

		return attribute.PackASPathAttribute(&attribute.ASPath{Segments: newSegments}, c.negotiated.ASN4)
	}

	// No existing AS_PATH: create new with just local AS
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{c.negotiated.LocalAS}},
		},
	}
	return attribute.PackASPathAttribute(asPath, c.negotiated.ASN4)
}

// buildMPReachNLRI builds an MP_REACH_NLRI attribute.
// Handles VPN next-hop encoding (RD prefix) per RFC 4364.
func (c *CommitService) buildMPReachNLRI(family nlri.Family, nextHop netip.Addr, nlriBytes []byte) attribute.Attribute {
	// Check if this is a VPN SAFI that needs RD in next-hop
	if isVPNSAFI(family.SAFI) {
		return c.buildVPNMPReachNLRI(family, nextHop, nlriBytes)
	}

	// Standard MP_REACH_NLRI
	return &attribute.MPReachNLRI{
		AFI:      attribute.AFI(family.AFI),
		SAFI:     attribute.SAFI(family.SAFI),
		NextHops: []netip.Addr{nextHop},
		NLRI:     nlriBytes,
	}
}

// buildVPNMPReachNLRI builds MP_REACH_NLRI for VPN routes with RD in next-hop.
// RFC 4364 Section 4.3.4: VPN next-hop is RD(8 bytes, all zeros) + IP.
func (c *CommitService) buildVPNMPReachNLRI(family nlri.Family, nextHop netip.Addr, nlriBytes []byte) attribute.Attribute {
	// Build next-hop with RD prefix
	var nhBytes []byte
	if nextHop.Is4() {
		// RD(8) + IPv4(4) = 12 bytes
		nhBytes = make([]byte, 12)
		// First 8 bytes are RD (all zeros)
		copy(nhBytes[8:], nextHop.AsSlice())
	} else {
		// RD(8) + IPv6(16) = 24 bytes
		nhBytes = make([]byte, 24)
		copy(nhBytes[8:], nextHop.AsSlice())
	}

	// Build the value manually since MPReachNLRI.Pack() doesn't handle RD prefix
	// Format: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(with RD) + Reserved(1) + NLRI
	valueLen := 2 + 1 + 1 + len(nhBytes) + 1 + len(nlriBytes)
	value := make([]byte, valueLen)

	// AFI
	value[0] = byte(family.AFI >> 8)
	value[1] = byte(family.AFI)
	// SAFI
	value[2] = byte(family.SAFI)
	// NH Length
	value[3] = byte(len(nhBytes))
	// Next-hop (with RD)
	copy(value[4:], nhBytes)
	// Reserved
	value[4+len(nhBytes)] = 0
	// NLRI
	copy(value[4+len(nhBytes)+1:], nlriBytes)

	// Return a custom MPReachNLRI that will pack correctly
	// We need to use the raw bytes approach since the standard Pack() doesn't handle RD
	return &vpnMPReachNLRI{
		afi:   attribute.AFI(family.AFI),
		safi:  attribute.SAFI(family.SAFI),
		value: value,
	}
}

// vpnMPReachNLRI is a custom MPReachNLRI for VPN routes that includes RD in next-hop.
type vpnMPReachNLRI struct {
	afi   attribute.AFI
	safi  attribute.SAFI
	value []byte
}

func (m *vpnMPReachNLRI) Code() attribute.AttributeCode { return attribute.AttrMPReachNLRI }
func (m *vpnMPReachNLRI) Flags() attribute.AttributeFlags {
	return attribute.FlagOptional
}
func (m *vpnMPReachNLRI) Len() int     { return len(m.value) }
func (m *vpnMPReachNLRI) Pack() []byte { return m.value }

// isVPNSAFI returns true if the SAFI indicates a VPN family.
func isVPNSAFI(safi nlri.SAFI) bool {
	return safi == 128 // MPLS VPN (RFC 4364)
}

// isIBGP returns true if this is an iBGP session.
func (c *CommitService) isIBGP() bool {
	return c.negotiated.LocalAS == c.negotiated.PeerAS
}

// bytesToAddr converts a byte slice to netip.Addr.
func bytesToAddr(b []byte) netip.Addr {
	switch len(b) {
	case 4:
		return netip.AddrFrom4([4]byte{b[0], b[1], b[2], b[3]})
	case 16:
		var arr [16]byte
		copy(arr[:], b)
		return netip.AddrFrom16(arr)
	default:
		return netip.Addr{}
	}
}

// sortFamilies sorts families for deterministic ordering.
func sortFamilies(families []nlri.Family) {
	sort.Slice(families, func(i, j int) bool {
		if families[i].AFI != families[j].AFI {
			return families[i].AFI < families[j].AFI
		}
		return families[i].SAFI < families[j].SAFI
	})
}
