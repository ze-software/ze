package rib

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
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
		// Two-level grouping: first by attributes, then by AS_PATH
		// Each ASPathGroup produces one UPDATE (RFC 4271: same attrs per UPDATE)
		attrGroups := GroupByAttributesTwoLevel(routes)

		for _, attrGroup := range attrGroups {
			for _, aspGroup := range attrGroup.ByASPath {
				update := c.buildGroupedUpdateTwoLevel(&attrGroup, &aspGroup)
				if err := c.sender.SendUpdate(update); err != nil {
					return stats, err
				}
				stats.UpdatesSent++
				stats.RoutesAnnounced += len(aspGroup.Routes)
				familySeen[attrGroup.Family] = true
			}
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

// buildGroupedUpdateTwoLevel builds an UPDATE message for a two-level group.
// Uses explicit AS_PATH from ASPathGroup instead of searching in attributes.
func (c *CommitService) buildGroupedUpdateTwoLevel(attrGroup *AttributeGroup, aspGroup *ASPathGroup) *message.Update {
	family := attrGroup.Family
	nextHop := bytesToAddr(attrGroup.NextHop)

	// Create PackContext for capability-aware NLRI encoding (RFC 7911 ADD-PATH)
	ctx := c.packContext(family)

	// Collect all NLRIs from the ASPathGroup
	var nlriBytes []byte
	for _, route := range aspGroup.Routes {
		nlriBytes = append(nlriBytes, route.NLRI().Pack(ctx)...)
	}

	// Build path attributes with explicit AS_PATH
	attrBytes := c.packAttributesWithASPath(attrGroup.Attributes, aspGroup.ASPath, nextHop, family, nlriBytes)

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

	// Create PackContext for capability-aware NLRI encoding (RFC 7911 ADD-PATH)
	ctx := c.packContext(family)
	nlriBytes := route.NLRI().Pack(ctx)

	// Use getRouteASPath to get AS_PATH (explicit field or from attrs)
	asPath := getRouteASPath(route)
	attrBytes := c.packAttributesWithASPath(route.Attributes(), asPath, nextHop, family, nlriBytes)

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

// packContext creates a PackContext for capability-aware NLRI encoding.
// RFC 7911: Checks if ADD-PATH is negotiated for the given family.
// RFC 6793: Includes ASN4 for attribute encoding decisions.
func (c *CommitService) packContext(family nlri.Family) *nlri.PackContext {
	if c.negotiated == nil || c.negotiated.AddPath == nil {
		return nil
	}
	msgFamily := message.Family{AFI: uint16(family.AFI), SAFI: uint8(family.SAFI)}
	return &nlri.PackContext{
		AddPath: c.negotiated.AddPath[msgFamily],
		ASN4:    c.negotiated.ASN4,
	}
}

// packAttributesWithASPath packs path attributes with an explicit AS_PATH.
// This is the preferred method for two-level grouping.
// Zero-allocation: calculates size, pre-allocates, writes with copy.
func (c *CommitService) packAttributesWithASPath(attrs []attribute.Attribute, asPath *attribute.ASPath, nextHop netip.Addr, family nlri.Family, nlriBytes []byte) []byte {
	// Build encoding context for ASN4-aware encoding
	var dstCtx *bgpctx.EncodingContext
	if c.negotiated != nil {
		dstCtx = &bgpctx.EncodingContext{ASN4: c.negotiated.ASN4}
	}

	// Phase 1: Identify attributes and calculate total size
	var origin attribute.Attribute
	var localPref attribute.Attribute
	var otherAttrs []attribute.Attribute

	for _, attr := range attrs {
		switch attr.Code() { //nolint:exhaustive // default handles all other attributes
		case attribute.AttrOrigin:
			origin = attr
		case attribute.AttrLocalPref:
			localPref = attr
		case attribute.AttrASPath, attribute.AttrNextHop:
			// Skip - we handle these explicitly
		default:
			otherAttrs = append(otherAttrs, attr)
		}
	}

	// Use defaults if not provided
	if origin == nil {
		origin = attribute.Origin(0) // IGP
	}

	// Build AS_PATH attribute
	asPathAttr := c.buildASPathFromExplicit(asPath)

	// Build NEXT_HOP or MP_REACH_NLRI
	var nhAttr attribute.Attribute
	if c.useTraditionalNLRI(family, nextHop) {
		nhAttr = &attribute.NextHop{Addr: nextHop}
	} else {
		nhAttr = c.buildMPReachNLRI(family, nextHop, nlriBytes)
	}

	// For iBGP, ensure LOCAL_PREF
	includeLocalPref := c.isIBGP()
	if includeLocalPref && localPref == nil {
		localPref = attribute.LocalPref(100)
	}

	// Phase 2: Calculate total size
	totalLen := attrSize(origin) +
		attrSizeWithContext(asPathAttr, dstCtx) +
		attrSize(nhAttr)

	if includeLocalPref {
		totalLen += attrSize(localPref)
	}

	for _, attr := range otherAttrs {
		totalLen += attrSize(attr)
	}

	// Phase 3: Pre-allocate and write using copy
	buf := make([]byte, totalLen)
	off := 0

	// 1. ORIGIN
	off += attribute.WriteAttrTo(origin, buf, off)

	// 2. AS_PATH (context-dependent for ASN4)
	off += attribute.WriteAttrToWithContext(asPathAttr, buf, off, nil, dstCtx)

	// 3. NEXT_HOP or MP_REACH_NLRI
	off += attribute.WriteAttrTo(nhAttr, buf, off)

	// 4. LOCAL_PREF for iBGP
	if includeLocalPref {
		off += attribute.WriteAttrTo(localPref, buf, off)
	}

	// 5. Other attributes
	for _, attr := range otherAttrs {
		off += attribute.WriteAttrTo(attr, buf, off)
	}

	// Invariant: attrSize must match WriteAttrTo
	if off != totalLen {
		slog.Error("attribute size mismatch: attrSize disagrees with WriteAttrTo",
			"predicted", totalLen,
			"actual", off,
			"attrCount", len(otherAttrs)+4) // origin, aspath, nh, localpref + others
		panic(fmt.Sprintf("BUG: attribute size mismatch: predicted=%d actual=%d", totalLen, off))
	}

	return buf
}

// attrSize returns the total wire size of an attribute (header + value).
func attrSize(attr attribute.Attribute) int {
	valueLen := attr.Len()
	if valueLen > 255 {
		return 4 + valueLen // Extended length header
	}
	return 3 + valueLen // Normal header
}

// attrSizeWithContext returns the total wire size with context-dependent encoding.
//
// Context-dependent attributes (RFC 6793):
//   - AS_PATH: 2-byte vs 4-byte ASN encoding
//   - AGGREGATOR: 6-byte vs 8-byte format
func attrSizeWithContext(attr attribute.Attribute, dstCtx *bgpctx.EncodingContext) int {
	asn4 := dstCtx == nil || dstCtx.ASN4

	var valueLen int
	switch a := attr.(type) {
	case *attribute.ASPath:
		valueLen = a.LenWithASN4(asn4)
	case *attribute.Aggregator:
		// RFC 6793: 8-byte (4-byte ASN + 4-byte IP) or 6-byte (2-byte ASN + 4-byte IP)
		if asn4 {
			valueLen = 8
		} else {
			valueLen = 6
		}
	default:
		return attrSize(attr)
	}

	if valueLen > 255 {
		return 4 + valueLen
	}
	return 3 + valueLen
}

// buildASPathFromExplicit builds AS_PATH from an explicit AS_PATH parameter.
// For eBGP: prepends local AS. For iBGP: preserves as-is.
// Returns the AS_PATH attribute object (not packed).
func (c *CommitService) buildASPathFromExplicit(asPath *attribute.ASPath) *attribute.ASPath {
	if c.isIBGP() {
		// iBGP: preserve existing AS_PATH or use empty
		if asPath != nil {
			return asPath
		}
		return &attribute.ASPath{Segments: nil}
	}

	// eBGP: prepend local AS to existing path
	if asPath != nil && len(asPath.Segments) > 0 {
		// Prepend local AS to first segment if it's AS_SEQUENCE
		newSegments := make([]attribute.ASPathSegment, len(asPath.Segments))
		copy(newSegments, asPath.Segments)

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

		return &attribute.ASPath{Segments: newSegments}
	}

	// No existing AS_PATH: create new with just local AS
	return &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{c.negotiated.LocalAS}},
		},
	}
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

// PackWithContext returns Pack() - raw attribute value is pre-encoded.
func (m *vpnMPReachNLRI) PackWithContext(_, _ *bgpctx.EncodingContext) []byte { return m.value }

// WriteTo writes the pre-encoded value into buf at offset.
func (m *vpnMPReachNLRI) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], m.value)
}

// WriteToWithContext writes pre-encoded value - context-independent.
func (m *vpnMPReachNLRI) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return m.WriteTo(buf, off)
}

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
