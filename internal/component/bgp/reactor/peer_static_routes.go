// Design: docs/architecture/core-design.md — static route building for BGP UPDATEs
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"slices"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

func toVPLSParams(r VPLSRoute) message.VPLSParams {
	return message.VPLSParams{
		RD: r.RD, Endpoint: r.Endpoint, Base: r.Base, Offset: r.Offset,
		Size: r.Size, NextHop: r.NextHop, Origin: attribute.Origin(r.Origin),
		LocalPreference: r.LocalPreference, MED: r.MED, ASPath: r.ASPath,
		Communities: r.Communities, ExtCommunityBytes: r.ExtCommunityBytes,
		OriginatorID: r.OriginatorID, ClusterList: r.ClusterList,
	}
}

func toFlowSpecParams(r FlowSpecRoute) message.FlowSpecParams {
	return message.FlowSpecParams{
		IsIPv6: r.IsIPv6, RD: r.RD, NLRI: r.NLRI, NextHop: r.NextHop,
		CommunityBytes: r.CommunityBytes, ExtCommunityBytes: r.ExtCommunityBytes,
		IPv6ExtCommunityBytes: r.IPv6ExtCommunityBytes,
	}
}

func toMUPParams(r MUPRoute) message.MUPParams {
	return message.MUPParams{
		RouteType: r.RouteType, IsIPv6: r.IsIPv6, NLRI: r.NLRI,
		NextHop: r.NextHop, ExtCommunityBytes: r.ExtCommunityBytes,
		PrefixSID: r.PrefixSID,
	}
}

func toMVPNParams(routes []MVPNRoute) []message.MVPNParams {
	params := make([]message.MVPNParams, len(routes))
	for i := range routes {
		r := &routes[i]
		params[i] = message.MVPNParams{
			RouteType: r.RouteType, IsIPv6: r.IsIPv6, RD: r.RD,
			SourceAS: r.SourceAS, Source: r.Source, Group: r.Group,
			NextHop: r.NextHop, Origin: attribute.Origin(r.Origin),
			LocalPreference: r.LocalPreference, MED: r.MED,
			ExtCommunityBytes: r.ExtCommunityBytes,
			OriginatorID:      r.OriginatorID,
			ClusterList:       r.ClusterList,
		}
	}
	return params
}

// toStaticRouteUnicastParams converts a StaticRoute to UnicastParams.
// Used for IPv4/IPv6 unicast routes (not VPN).
// nextHop is the resolved next-hop address (from RouteNextHop policy).
// linkLocal is the peer's IPv6 link-local address for 32-byte MP_REACH next-hop (RFC 2545 Section 3).
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
// linkLocal is the peer's IPv6 link-local for 32-byte MP_REACH next-hop (RFC 2545 Section 3).
//
// The returned *Update's PathAttributes/NLRI alias ub.scratch. Caller MUST
// fully consume the Update (send, copy, hand to sendUpdateWithSplit) before
// calling message.PutUpdateBuilder(ub) or reusing ub for another Build*.
func buildStaticRouteUpdateNew(ub *message.UpdateBuilder, route *StaticRoute, nextHop, linkLocal netip.Addr, sendCtx *bgpctx.EncodingContext) *message.Update {
	if route.IsVPN() {
		p := toStaticRouteVPNParams(route, nextHop)
		return ub.BuildVPN(&p)
	}
	if route.IsLabeledUnicast() {
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
	if route.IsLabeledUnicast() {
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
	return fmt.Sprintf("%s|%d|%d|%d|%v|%v|%s|%s|%v|%s|%v|%v|%d|%v|%d|%v",
		r.NextHop.String(),
		r.Origin,
		r.LocalPreference,
		r.MED,
		comms,
		lcs,
		hex.EncodeToString(r.ExtCommunityBytes),
		r.RD,
		r.Prefix.Addr().Is4(),
		prefixKey,
		r.ASPath,
		r.AtomicAggregate,
		r.AggregatorASN,
		r.AggregatorIP,
		r.OriginatorID,
		r.ClusterList,
	)
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
