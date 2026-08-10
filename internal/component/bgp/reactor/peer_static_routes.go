// Design: docs/architecture/core-design.md — static route building for BGP UPDATEs
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"net/netip"
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func toPluginParams(r PluginRoute, fam family.Family) message.PluginParams {
	return message.PluginParams{
		AFI: uint16(fam.AFI), SAFI: byte(fam.SAFI), IsIPv6: r.IsIPv6, NLRI: r.NLRI,
		NextHop: r.NextHop, RawAttrs: r.RawAttrs,
		ASPath: r.ASPath, LocalPreference: r.LocalPreference,
		MapV4NextHop: r.MapV4NextHop,
	}
}

// toStaticRouteUnicastParams converts a StaticRoute to UnicastParams.
// Used for IPv4/IPv6 unicast routes (not VPN).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
// linkLocal is the link-local address to append after nextHop in the MP_REACH
// Next Hop field, already decided against RFC 2545 Section 3's condition by
// Peer.linkLocalNextHopFor (link_scope.go). The zero Addr means the field carries
// the global address alone.
func toStaticRouteUnicastParams(r *StaticRoute, nextHop, linkLocal netip.Addr, sendCtx *bgpctx.EncodingContext) message.UnicastParams {
	// RFC 8950: Extended next-hop for cross-AFI next-hop
	var useExtNH bool
	if sendCtx != nil {
		if r.Prefix.Addr().Is4() && nextHop.Is6() {
			useExtNH = sendCtx.ExtendedNextHopFor(family.IPv4Unicast) != 0
		} else if r.Prefix.Addr().Is6() && nextHop.Is4() {
			useExtNH = sendCtx.ExtendedNextHopFor(family.IPv6Unicast) != 0
		}
	}

	// Write raw attributes into a single contiguous buffer
	rawAttrs := packRawAttributes(r.RawAttributes)
	if r.AIGPMetric != nil {
		rawAttrs = appendAIGPRaw(rawAttrs, *r.AIGPMetric)
	}

	return message.UnicastParams{
		Prefix:             r.Prefix,
		PathID:             r.PathID,
		NextHop:            nextHop,
		LinkLocalNextHop:   linkLocal,
		Origin:             attribute.Origin(r.Origin),
		ASPath:             r.ASPath,
		MED:                r.MED,
		LocalPreference:    r.LocalPreference,
		Communities:        r.Communities,
		ExtCommunityBytes:  r.ExtCommunityBytes,
		LargeCommunities:   r.LargeCommunities,
		AtomicAggregate:    r.AtomicAggregate,
		HasAggregator:      r.HasAggregator,
		AggregatorASN:      r.AggregatorASN,
		AggregatorIP:       r.AggregatorIP,
		UseExtendedNextHop: useExtNH,
		RawAttributeBytes:  rawAttrs,
		OriginatorID:       r.OriginatorID,
		ClusterList:        r.ClusterList,
	}
}

// toStaticRouteLabeledUnicastParams converts a StaticRoute to LabeledUnicastParams.
// Used for labeled unicast routes (SAFI 4).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
func toStaticRouteLabeledUnicastParams(r *StaticRoute, nextHop netip.Addr) message.LabeledUnicastParams {
	// Write raw attributes into a single contiguous buffer
	rawAttrs := packRawAttributes(r.RawAttributes)
	if r.AIGPMetric != nil {
		rawAttrs = appendAIGPRaw(rawAttrs, *r.AIGPMetric)
	}

	return message.LabeledUnicastParams{
		Prefix:            r.Prefix,
		PathID:            r.PathID,
		NextHop:           nextHop,
		Labels:            r.Labels,
		Origin:            attribute.Origin(r.Origin),
		ASPath:            r.ASPath,
		MED:               r.MED,
		LocalPreference:   r.LocalPreference,
		Communities:       r.Communities,
		ExtCommunityBytes: r.ExtCommunityBytes,
		LargeCommunities:  r.LargeCommunities,
		AtomicAggregate:   r.AtomicAggregate,
		HasAggregator:     r.HasAggregator,
		AggregatorASN:     r.AggregatorASN,
		AggregatorIP:      r.AggregatorIP,
		OriginatorID:      r.OriginatorID,
		ClusterList:       r.ClusterList,
		PrefixSID:         r.PrefixSIDBytes,
		RawAttributeBytes: rawAttrs,
	}
}

// toStaticRouteVPNParams converts a StaticRoute to VPNParams.
// Used for VPN routes (SAFI 128).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
func toStaticRouteVPNParams(r *StaticRoute, nextHop netip.Addr) message.VPNParams {
	return message.VPNParams{
		Prefix:            r.Prefix,
		PathID:            r.PathID,
		NextHop:           nextHop,
		Labels:            r.Labels,
		RDBytes:           r.RDBytes,
		Origin:            attribute.Origin(r.Origin),
		ASPath:            r.ASPath,
		MED:               r.MED,
		LocalPreference:   r.LocalPreference,
		Communities:       r.Communities,
		ExtCommunityBytes: r.ExtCommunityBytes,
		LargeCommunities:  r.LargeCommunities,
		AtomicAggregate:   r.AtomicAggregate,
		HasAggregator:     r.HasAggregator,
		AggregatorASN:     r.AggregatorASN,
		AggregatorIP:      r.AggregatorIP,
		OriginatorID:      r.OriginatorID,
		ClusterList:       r.ClusterList,
		PrefixSID:         r.PrefixSIDBytes,
	}
}

// buildStaticRouteUpdateNew builds an UPDATE for a static route using ub.
// nextHop is the resolved next-hop address (from RouteNextHop policy).
// linkLocal is the address to append after nextHop, already decided against RFC
// 2545 Section 3's condition by Peer.linkLocalNextHopFor (link_scope.go). It is
// not "the peer's link-local": the configured leaf reaches this parameter only
// when the section's condition holds, and the zero Addr means the 16-octet form.
//
// The returned *Update's PathAttributes/NLRI alias ub.scratch. Caller MUST
// fully consume the Update (send, copy, hand to sendUpdateWithSplit) before
// calling message.PutUpdateBuilder(ub) or reusing ub for another Build*.
func buildStaticRouteUpdateNew(ub *message.UpdateBuilder, route *StaticRoute, nextHop, linkLocal netip.Addr, sendCtx *bgpctx.EncodingContext) *message.Update {
	if route.IsVPN() {
		p := toStaticRouteVPNParams(route, nextHop)
		return ub.BuildVPN(&p)
	}
	if route.isLabeledUnicast() {
		p := toStaticRouteLabeledUnicastParams(route, nextHop)
		return ub.BuildLabeledUnicast(&p)
	}
	p := toStaticRouteUnicastParams(route, nextHop, linkLocal, sendCtx)
	return ub.BuildUnicast(&p)
}

// routeFamily returns the NLRI family for a StaticRoute.
// Used to track which families had routes sent for EOR purposes.
func routeFamily(route *StaticRoute) family.Family {
	if route.IsVPN() {
		if route.Prefix.Addr().Is6() {
			return family.Family{AFI: family.AFIIPv6, SAFI: 128} // VPNv6
		}
		return family.Family{AFI: family.AFIIPv4, SAFI: 128} // VPNv4
	}
	if route.isLabeledUnicast() {
		if route.Prefix.Addr().Is6() {
			return family.Family{AFI: family.AFIIPv6, SAFI: 4} // IPv6 Labeled Unicast
		}
		return family.Family{AFI: family.AFIIPv4, SAFI: 4} // IPv4 Labeled Unicast
	}
	if route.Prefix.Addr().Is6() {
		return family.IPv6Unicast
	}
	return family.IPv4Unicast
}

// appendAIGPRaw packs an AIGP metric as a complete wire attribute and appends it
// to the raw attribute list. RFC 7311: optional transitive, type 26.
func appendAIGPRaw(rawAttrs [][]byte, metric uint64) [][]byte {
	const hdrLen = 3                                  // flags(1) + code(1) + length(1)
	buf := make([]byte, hdrLen+attribute.AIGPWireLen) // pool-fallback
	buf[0] = byte(attribute.FlagOptional | attribute.FlagTransitive)
	buf[1] = byte(attribute.AttrAIGP)
	buf[2] = byte(attribute.AIGPWireLen)
	attribute.WriteAIGPMetric(buf, hdrLen, metric)
	return append(rawAttrs, buf)
}

// writeRawAttribute writes a raw attribute into buf at off, returning bytes written.
// Format: flags (1 byte) + code (1 byte) + length (1 or 2 bytes) + value.
func writeRawAttribute(buf []byte, off int, ra RawAttribute) int {
	flags := ra.Flags
	valueLen := len(ra.Value)

	// Use extended length if value > 255 bytes OR if extended length flag is set
	if valueLen > 255 || (flags&0x10) != 0 {
		flags |= 0x10 // Ensure extended length flag is set
		buf[off] = flags
		buf[off+1] = ra.Code
		buf[off+2] = byte((valueLen >> 8) & 0xFF)
		buf[off+3] = byte(valueLen & 0xFF)
		copy(buf[off+4:], ra.Value)
		return 4 + valueLen
	}

	buf[off] = flags
	buf[off+1] = ra.Code
	buf[off+2] = byte(valueLen & 0xFF)
	copy(buf[off+3:], ra.Value)
	return 3 + valueLen
}

// rawAttributeLen returns the wire length of a raw attribute.
func rawAttributeLen(ra RawAttribute) int {
	valueLen := len(ra.Value)
	if valueLen > 255 || (ra.Flags&0x10) != 0 {
		return 4 + valueLen
	}
	return 3 + valueLen
}

// packRawAttributes packs multiple raw attributes into a single contiguous buffer,
// returning sub-slices for each attribute. Reduces N allocations to 1.
func packRawAttributes(attrs []RawAttribute) [][]byte {
	if len(attrs) == 0 {
		return nil
	}
	totalSize := 0
	for i := range attrs {
		totalSize += rawAttributeLen(attrs[i])
	}
	buf := make([]byte, totalSize)
	result := make([][]byte, len(attrs))
	off := 0
	for i := range attrs {
		n := writeRawAttribute(buf, off, attrs[i])
		result[i] = buf[off : off+n]
		off += n
	}
	return result
}

// routeGroupKey generates a string key for grouping routes by attributes.
// Routes with the same key can be combined into a single UPDATE.
func routeGroupKey(r *StaticRoute) string {
	// Sort communities for consistent key.
	comms := make([]uint32, len(r.Communities))
	copy(comms, r.Communities)
	slices.Sort(comms)

	// Sort large communities.
	lcs := make([][3]uint32, len(r.LargeCommunities))
	copy(lcs, r.LargeCommunities)
	sort.Slice(lcs, func(i, j int) bool {
		if lcs[i][0] != lcs[j][0] {
			return lcs[i][0] < lcs[j][0]
		}
		if lcs[i][1] != lcs[j][1] {
			return lcs[i][1] < lcs[j][1]
		}
		return lcs[i][2] < lcs[j][2]
	})

	// Key includes: nexthop, origin, localpref, med, communities, large-communities, ext-communities, vpn, ipv4/ipv6,
	// as-path, atomic-aggregate, aggregator, originator-id, cluster-list.
	// For IPv6 routes, include prefix in key to prevent grouping (each needs separate MP_REACH_NLRI UPDATE).
	// IPv4 routes can be grouped since multiple NLRIs can be in one UPDATE.
	prefixKey := ""
	if !r.Prefix.Addr().Is4() {
		prefixKey = r.Prefix.String()
	}
	var b keyBuilder
	b.Grow(128)
	if r.NextHop.IsSelf() {
		b.WriteString("self")
	} else {
		b.Addr(r.NextHop.Addr)
	}
	b.Sep()
	b.Uint(uint64(r.Origin))
	b.Sep()
	b.Uint(uint64(r.LocalPreference))
	b.Sep()
	b.Uint(uint64(r.MED))
	b.Sep()
	b.uint32Slice(comms)
	b.Sep()
	b.largeComms(lcs)
	b.Sep()
	b.Hex(r.ExtCommunityBytes)
	b.Sep()
	b.WriteString(r.RD)
	b.Sep()
	b.Bool(r.Prefix.Addr().Is4())
	b.Sep()
	b.WriteString(prefixKey)
	b.Sep()
	b.uint32Slice(r.ASPath)
	b.Sep()
	b.Bool(r.AtomicAggregate)
	b.Sep()
	b.Uint(uint64(r.AggregatorASN))
	b.Sep()
	b.Addr(netip.AddrFrom4(r.AggregatorIP))
	b.Sep()
	b.Uint(uint64(r.OriginatorID))
	b.Sep()
	b.uint32Slice(r.ClusterList)
	if r.AIGPMetric != nil {
		b.Sep()
		b.Uint(*r.AIGPMetric)
	}
	return b.String()
}

// groupRoutesByAttributes groups routes by their attribute key.
// Returns groups sorted: multi-route groups first (by first prefix), then singletons (by prefix).
// This matches ExaBGP's behavior for UPDATE grouping.
func groupRoutesByAttributes(routes []StaticRoute) [][]StaticRoute {
	groups := make(map[string][]StaticRoute)

	for i := range routes {
		key := routeGroupKey(&routes[i])
		groups[key] = append(groups[key], routes[i])
	}

	// Collect groups into slice.
	result := make([][]StaticRoute, 0, len(groups))
	for _, g := range groups {
		// Sort routes within group by prefix.
		sort.Slice(g, func(i, j int) bool {
			return g[i].Prefix.Addr().Compare(g[j].Prefix.Addr()) < 0
		})
		result = append(result, g)
	}

	// Sort groups: multi-route first, then singletons, each ordered by first prefix.
	sort.Slice(result, func(i, j int) bool {
		// Multi-route groups come before singletons.
		if len(result[i]) > 1 && len(result[j]) == 1 {
			return true
		}
		if len(result[i]) == 1 && len(result[j]) > 1 {
			return false
		}
		// Same category: sort by first prefix.
		return result[i][0].Prefix.Addr().Compare(result[j][0].Prefix.Addr()) < 0
	})

	return result
}

type keyBuilder struct{ strings.Builder }

func (b *keyBuilder) Sep()          { b.WriteByte('|') }
func (b *keyBuilder) Uint(v uint64) { var buf [20]byte; b.Write(textbuf.Uint(buf[:0], v)) }
func (b *keyBuilder) Addr(addr netip.Addr) {
	var buf [39]byte
	b.Write(textbuf.Addr(buf[:0], addr))
}
func (b *keyBuilder) Hex(data []byte) { var buf [64]byte; b.Write(textbuf.Hex(buf[:0], data)) }

func (b *keyBuilder) Bool(v bool) {
	if v {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
}

func (b *keyBuilder) uint32Slice(s []uint32) {
	b.WriteByte('[')
	for i, v := range s {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.Uint(uint64(v))
	}
	b.WriteByte(']')
}

func (b *keyBuilder) largeComms(lcs [][3]uint32) {
	b.WriteByte('[')
	for i, lc := range lcs {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('[')
		b.Uint(uint64(lc[0]))
		b.WriteByte(' ')
		b.Uint(uint64(lc[1]))
		b.WriteByte(' ')
		b.Uint(uint64(lc[2]))
		b.WriteByte(']')
	}
	b.WriteByte(']')
}
