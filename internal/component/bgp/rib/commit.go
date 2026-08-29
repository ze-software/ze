// Design: docs/architecture/pool-architecture.md — RIB wire storage
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI wire format and the Length of Next Hop Network Address (Section 3)
// RFC: rfc/short/rfc4364.md — VPN next hop carries an 8-octet Route Distinguisher (Section 4.3.4)

package rib

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// EncodingContext is an alias for bgpctx.EncodingContext.
type EncodingContext = bgpctx.EncodingContext

// ErrNilContext is returned when CommitService is used with nil EncodingContext.
var ErrNilContext = errors.New("commit: encoding context required")

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
	UpdatesSent      int             // Number of UPDATE messages sent
	RoutesAnnounced  int             // Total routes announced
	RoutesWithdrawn  int             // Total routes withdrawn (future use)
	FamiliesAffected []family.Family // Address families that had routes
	EORSent          []family.Family // Families for which EOR was sent
}

// CommitService handles batched route commits with grouping and EOR.
//
// This provides a single abstraction for committing routes to peers,
// used by both config routes (on session establish) and API routes
// (on explicit commit command).
type CommitService struct {
	sender       UpdateSender
	ctx          *EncodingContext
	groupUpdates bool
}

// NewCommitService creates a new CommitService.
//
// sender: interface for sending UPDATE messages (typically a Peer)
// ctx: encoding context for proper wire encoding (ASN4, ADD-PATH, etc.)
// groupUpdates: if true, routes with same attributes are grouped into fewer UPDATEs.
func NewCommitService(sender UpdateSender, ctx *EncodingContext, groupUpdates bool) *CommitService {
	return &CommitService{
		sender:       sender,
		ctx:          ctx,
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

	if c.ctx == nil {
		return stats, ErrNilContext
	}

	if len(routes) == 0 {
		return stats, nil
	}

	// draft-abraitis-idr-addpath-paths-limit: enforce per-prefix path count limits.
	routes = c.enforcePathsLimit(routes)

	// Track which families have routes
	familySeen := make(map[family.Family]bool)

	if c.groupUpdates {
		// Two-level grouping: first by attributes, then by AS_PATH
		// Each ASPathGroup produces one UPDATE (RFC 4271: same attrs per UPDATE)
		attrGroups := GroupByAttributesTwoLevel(routes)

		for _, attrGroup := range attrGroups {
			for _, aspGroup := range attrGroup.ByASPath {
				update, err := c.buildGroupedUpdateTwoLevel(&attrGroup, &aspGroup)
				if err != nil {
					return stats, fmt.Errorf("build update: %w", err)
				}
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
			update, err := c.buildSingleUpdate(route)
			if err != nil {
				return stats, fmt.Errorf("build update: %w", err)
			}
			if err := c.sender.SendUpdate(update); err != nil {
				return stats, err
			}
			stats.UpdatesSent++
			stats.RoutesAnnounced++
			familySeen[route.NLRI().Family()] = true
		}
	}

	// Collect affected families (sorted for determinism)
	for fam := range familySeen {
		stats.FamiliesAffected = append(stats.FamiliesAffected, fam)
	}
	sortFamilies(stats.FamiliesAffected)

	// Send EOR for each affected family if requested
	if opts.SendEOR {
		for _, fam := range stats.FamiliesAffected {
			eor := message.BuildEOR(fam)
			if err := c.sender.SendUpdate(eor); err != nil {
				return stats, err
			}
			stats.EORSent = append(stats.EORSent, fam)
		}
	}

	return stats, nil
}

// buildGroupedUpdateTwoLevel builds an UPDATE message for a two-level group.
// Uses explicit AS_PATH from ASPathGroup instead of searching in attributes.
func (c *CommitService) buildGroupedUpdateTwoLevel(attrGroup *AttributeGroup, aspGroup *ASPathGroup) (*message.Update, error) {
	fam := attrGroup.Family
	nextHop := bytesToAddr(attrGroup.NextHop)

	// Check if ADD-PATH is negotiated for capability-aware NLRI encoding (RFC 7911)
	addPath := c.addPathFor(fam)

	// Collect all NLRIs from the ASPathGroup
	// Calculate total size first
	totalSize := 0
	for _, route := range aspGroup.Routes {
		totalSize += nlri.LenWithContext(route.NLRI(), addPath)
	}
	nlriBytes := make([]byte, totalSize)
	off := 0
	for _, route := range aspGroup.Routes {
		off += nlri.WriteNLRI(route.NLRI(), nlriBytes, off, addPath)
	}

	// Build path attributes with explicit AS_PATH
	attrBytes, err := c.packAttributesWithASPath(attrGroup.Attributes, aspGroup.ASPath, nextHop, fam, nlriBytes)
	if err != nil {
		return nil, err
	}

	// Determine if NLRI goes in UPDATE.NLRI or MP_REACH_NLRI
	if c.useTraditionalNLRI(fam, nextHop) {
		return &message.Update{
			PathAttributes: attrBytes,
			NLRI:           nlriBytes,
		}, nil
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nil, // NLRI is in MP_REACH_NLRI
	}, nil
}

// buildSingleUpdate builds an UPDATE message for a single route.
func (c *CommitService) buildSingleUpdate(route *Route) (*message.Update, error) {
	fam := route.NLRI().Family()
	nextHop := route.NextHop()

	// Check if ADD-PATH is negotiated for capability-aware NLRI encoding (RFC 7911)
	addPath := c.addPathFor(fam)
	nlriLen := nlri.LenWithContext(route.NLRI(), addPath)
	nlriBytes := make([]byte, nlriLen)
	nlri.WriteNLRI(route.NLRI(), nlriBytes, 0, addPath)

	// Use getRouteASPath to get AS_PATH (explicit field or from attrs)
	asPath := getRouteASPath(route)
	attrBytes, err := c.packAttributesWithASPath(route.Attributes(), asPath, nextHop, fam, nlriBytes)
	if err != nil {
		return nil, err
	}

	if c.useTraditionalNLRI(fam, nextHop) {
		return &message.Update{
			PathAttributes: attrBytes,
			NLRI:           nlriBytes,
		}, nil
	}

	return &message.Update{
		PathAttributes: attrBytes,
		NLRI:           nil,
	}, nil
}

// useTraditionalNLRI returns true if NLRI should go in UPDATE.NLRI field.
// Returns false if NLRI should be in MP_REACH_NLRI attribute.
func (c *CommitService) useTraditionalNLRI(fam family.Family, nextHop netip.Addr) bool {
	// Only IPv4 unicast with IPv4 next-hop uses traditional NLRI field
	// IPv4 unicast with IPv6 next-hop (RFC 5549) must use MP_REACH_NLRI
	return fam.AFI == 1 && fam.SAFI == 1 && nextHop.Is4()
}

// enforcePathsLimit filters routes to respect per-prefix path count limits.
// draft-abraitis-idr-addpath-paths-limit Section 4: the sender MUST NOT send
// more paths per prefix than the receiver's advertised limit.
// For VPN/labeled NLRIs, NLRI.Bytes() includes RD/label, so counting is per-RD+prefix.
func (c *CommitService) enforcePathsLimit(routes []*Route) []*Route {
	if c.ctx == nil {
		return routes
	}

	// Fast check: scan families present in routes. Skip entirely if none has a limit.
	var seenFamilies [4]family.Family
	nSeen := 0
	hasLimit := false
	for _, r := range routes {
		fam := r.NLRI().Family()
		found := false
		for i := range nSeen {
			if seenFamilies[i] == fam {
				found = true
				break
			}
		}
		if !found {
			if c.ctx.PathsLimit(fam) > 0 {
				hasLimit = true
				break
			}
			if nSeen < len(seenFamilies) {
				seenFamilies[nSeen] = fam
				nSeen++
			}
		}
	}
	if !hasLimit {
		return routes
	}

	// Count paths per (family, prefix-bytes) key; drop excess.
	type prefixKey struct {
		fam    family.Family
		prefix string
	}
	counts := make(map[prefixKey]uint16, len(routes))
	n := 0
	for _, r := range routes {
		fam := r.NLRI().Family()
		limit := c.ctx.PathsLimit(fam)
		if limit == 0 {
			routes[n] = r
			n++
			continue
		}
		pk := prefixKey{fam: fam, prefix: string(r.NLRI().Bytes())}
		if counts[pk] < limit {
			counts[pk]++
			routes[n] = r
			n++
		}
	}
	return routes[:n]
}

// addPathFor returns whether ADD-PATH is negotiated for the given family.
// RFC 7911: Checks if ADD-PATH is negotiated.
func (c *CommitService) addPathFor(fam family.Family) bool {
	if c.ctx == nil {
		return false
	}
	return c.ctx.AddPath(fam)
}

// packAttributesWithASPath packs path attributes with an explicit AS_PATH.
// This is the preferred method for two-level grouping.
// Zero-allocation: calculates size, pre-allocates, writes with copy.
func (c *CommitService) packAttributesWithASPath(attrs []attribute.Attribute, asPath *attribute.ASPath, nextHop netip.Addr, fam family.Family, nlriBytes []byte) ([]byte, error) {
	// Use the stored encoding context for ASN4-aware encoding
	dstCtx := c.ctx

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

	// RFC 6793 Section 4.2.2 states one obligation as a PAIR: "if the NEW BGP
	// speaker has to send the AGGREGATOR attribute, and if the aggregating
	// Autonomous System's AS number is a non-mappable four-octet AS number, then
	// the speaker MUST use the AS4_AGGREGATOR attribute and set the AS number
	// field in the existing AGGREGATOR attribute to the reserved AS number,
	// AS_TRANS." Aggregator.WriteToWithContext writes the AS_TRANS half; nothing
	// wrote the companion, so the real ASN was lost on the way to an OLD speaker.
	//
	// The companion is synthesized here rather than forwarded, because an upstream
	// that saw ze negotiate ASN4 had no reason to send one: it put the four-octet
	// ASN in AGGREGATOR itself.
	otherAttrs = appendAS4AggregatorFor(otherAttrs, dstCtx)

	// Build AS_PATH attribute
	asPathAttr := c.buildASPathFromExplicit(asPath)

	// Build NEXT_HOP or MP_REACH_NLRI. The MP_REACH branch refuses a next hop
	// with no wire form, before any buffer is sized for it.
	var nhAttr attribute.Attribute
	if c.useTraditionalNLRI(fam, nextHop) {
		nhAttr = &attribute.NextHop{Addr: nextHop}
	} else {
		mpReach, err := c.buildMPReachNLRI(fam, nextHop, nlriBytes)
		if err != nil {
			return nil, err
		}
		nhAttr = mpReach
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

	// Sized with the destination's context, and written with it below. AS_PATH is
	// not the only context-dependent attribute that reaches this rail: a FORWARDED
	// AGGREGATOR sits in otherAttrs, and a context-free write sends the 8-octet
	// form to a peer for which RFC 4271 defines a 6-octet attribute.
	for _, attr := range otherAttrs {
		totalLen += attrSizeWithContext(attr, dstCtx)
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

	// 5. Other attributes, in the destination's encoding
	for _, attr := range otherAttrs {
		off += attribute.WriteAttrToWithContext(attr, buf, off, nil, dstCtx)
	}

	// Invariant: the sizers must match the writers, context for context
	if off != totalLen {
		slog.Error("attribute size mismatch: the sizers disagree with the writers",
			"predicted", totalLen,
			"actual", off,
			"attrCount", len(otherAttrs)+4) // origin, aspath, nh, localpref + others
		return nil, fmt.Errorf("BUG: attribute size mismatch: predicted=%d actual=%d", totalLen, off)
	}

	return buf, nil
}

// appendAS4AggregatorFor adds the RFC 6793 Section 4.2.2 companion to attrs when
// an AGGREGATOR is about to be downgraded to AS_TRANS for this destination.
//
// It is a no-op unless all three hold: the destination did not negotiate
// four-octet AS support, an AGGREGATOR is present, and its ASN is non-mappable.
// Section 4.2.2 is explicit about the last one: "if the AS number is mappable,
// then the AS4_AGGREGATOR attribute MUST NOT be sent."
//
// An AS4_AGGREGATOR already in attrs is left as the only one. Emitting a second
// would make the attribute appear twice, which RFC 7606 Section 3(g) treats as
// malformed.
func appendAS4AggregatorFor(attrs []attribute.Attribute, dstCtx *bgpctx.EncodingContext) []attribute.Attribute {
	if dstCtx == nil || dstCtx.ASN4() {
		return attrs
	}

	var aggregator *attribute.Aggregator
	for _, attr := range attrs {
		switch a := attr.(type) {
		case *attribute.Aggregator:
			aggregator = a
		case *attribute.AS4Aggregator:
			return attrs // the upstream already supplied one
		}
	}

	if aggregator == nil || aggregator.ASN <= 65535 {
		return attrs
	}

	return append(attrs, &attribute.AS4Aggregator{
		ASN:     aggregator.ASN,
		Address: aggregator.Address,
	})
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
	asn4 := dstCtx == nil || dstCtx.ASN4()

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

	localAS := c.ctx.LocalASN()

	// eBGP: prepend local AS to existing path
	if asPath != nil && len(asPath.Segments) > 0 {
		// Prepend local AS to first segment if it's AS_SEQUENCE
		newSegments := make([]attribute.ASPathSegment, len(asPath.Segments))
		copy(newSegments, asPath.Segments)

		if len(newSegments) > 0 && newSegments[0].Type == attribute.ASSequence {
			// Prepend to first AS_SEQUENCE segment
			newASNs := make([]uint32, 0, len(newSegments[0].ASNs)+1)
			newASNs = append(newASNs, localAS)
			newASNs = append(newASNs, newSegments[0].ASNs...)
			newSegments[0].ASNs = newASNs
		} else {
			// Insert new AS_SEQUENCE segment at beginning
			newSeg := attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: []uint32{localAS}}
			newSegments = append([]attribute.ASPathSegment{newSeg}, newSegments...)
		}

		return &attribute.ASPath{Segments: newSegments}
	}

	// No existing AS_PATH: create new with just local AS
	return &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{localAS}},
		},
	}
}

// buildMPReachNLRI builds an MP_REACH_NLRI attribute for one family, and refuses
// a next hop that cannot be encoded.
//
// One encoder serves every family, the VPN one included: (*MPReachNLRI).nextHopOctets
// counts the 8-octet Route Distinguisher for SAFI 128 and WriteTo writes it, both
// per RFC 4364 Section 4.3.4.
//
// RFC 4760 Section 3: "Length of Next Hop Network Address" is the field a peer
// reads to determine the next hop's network-layer protocol. The zero netip.Addr
// has no wire form, so it yields a length that attribute.ValidNextHopLens admits
// for no AFI/SAFI pair, and the peer answers a malformed MP_REACH_NLRI with a
// session reset (RFC 7606 Section 7.11). This rail sizes with attrSize and writes
// with attribute.WriteAttrTo, so no CheckedWriteTo sits between it and the socket
// and the refusal has to happen here. The two announce rails in the reactor call
// the same ValidateNextHops for the same reason.
func (c *CommitService) buildMPReachNLRI(fam family.Family, nextHop netip.Addr, nlriBytes []byte) (attribute.Attribute, error) {
	mpReach := attribute.NewMPReachNLRI(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI), []netip.Addr{nextHop}, nlriBytes)

	if err := mpReach.ValidateNextHops(); err != nil {
		// The caller above the reactor discards this error, so the record is what
		// reaches an operator.
		slog.Warn("commit: announce refused, the next hop has no wire form",
			"family", fam,
			"nextHop", nextHop,
			"error", err)
		return nil, err
	}

	return mpReach, nil
}

// isIBGP returns true if this is an iBGP session.
func (c *CommitService) isIBGP() bool {
	return c.ctx.IsIBGP()
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
func sortFamilies(families []family.Family) {
	sort.Slice(families, func(i, j int) bool {
		if families[i].AFI != families[j].AFI {
			return families[i].AFI < families[j].AFI
		}
		return families[i].SAFI < families[j].SAFI
	})
}
