// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md -- the RFC 7684 Extended Prefix Opaque
// LSA (Opaque Type 7) consumer.
// RFC: rfc/short/rfc7684.md -- sec 2 (Extended Prefix Opaque LSA), sec 2.1 (Extended Prefix
// TLV, Route Type / A-Flag / N-Flag, lowest-Opaque-ID dedup), sec 5 (malformed rules).
// RFC: rfc/short/rfc5250.md sec 5 (Type-11 reachability, consumed from ext-1).
//
// This is the ext-4 consumer of the ext-1 opaque carrier for Opaque Type 7. Origination reads
// the router's advertised prefixes from the LSDB self-LSAs (Router-LSA stub links -> intra;
// self Type-3 summaries -> inter-area; self Type-5 -> AS-external) and emits one Extended
// Prefix Opaque LSA per prefix with the correct Route Type / scope / flags. Reception decodes
// the body, enforces the sec 5 malformed rules, ignores the N-Flag on a non-host prefix,
// dedups (first in one LSA, lowest Opaque ID across LSAs), and dispatches sub-TLVs to
// registered codecs. The carrier owns flooding, sequencing, the LS-ID split, and the
// reachability determination; this file interprets only the Type 7 body.

package ospf

import (
	"net/netip"
	"sort"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// extPrefixAdvert is one prefix this router advertises an Extended Prefix TLV for, with its
// RFC 7684 sec 2.1 Route Type, flooding scope, target area, and flags.
type extPrefixAdvert struct {
	prefix    netip.Prefix
	routeType uint8
	scope     OpaqueScope
	area      types.AreaID
	flags     uint8
}

// connectedPrefix is one intra-area connected prefix, tracked so an ABR can set the A-Flag
// (and preserve the N-Flag) on the inter-area advertisement of a prefix connected in another
// area (RFC 7684 sec 2.1).
type connectedPrefix struct {
	prefix netip.Prefix
	host   bool
}

// extPrefixOnOriginate is the RFC 5250 sec 3 pull-model OnOriginate for Opaque Type 7. Each
// self-LSA pass it returns the FULL desired set of Extended Prefix Opaque LSAs plus a withdraw
// for any instance no longer desired (AC-13). The carrier (ext-1) assigns sequence numbers,
// builds the Opaque LSA, installs, and floods; an unchanged body floods nothing.
func (e *engine) extPrefixOnOriginate(router types.RouterID) []opaqueOrigination {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()

	o := e.extOrig
	o.mu.Lock()
	defer o.mu.Unlock()

	var out []opaqueOrigination
	desired := map[extPrefixKey]extOrig{}
	if cfg.ExtendedPrefix {
		for _, adv := range e.selfPrefixAdverts(router) {
			key := extPrefixKey{scope: adv.scope, area: adv.area, routeType: adv.routeType, prefix: prefixKeyBytes(adv.prefix)}
			id := o.prefixIDFor(key)
			tlv := packet.ExtPrefixTLV{
				RouteType:     adv.routeType,
				PrefixLength:  uint8(adv.prefix.Bits()),
				AF:            packet.ExtPrefixAFIPv4Unicast,
				Flags:         adv.flags,
				AddressPrefix: adv.prefix.Addr().As4(),
			}
			// Registered sub-TLV builders (a downstream application) contribute bytes; none
			// until then, so a carrier-only router originates an empty container (AC-12).
			tlv.SubTLVs = buildPrefixSubTLVs(
				extSubTLVContext{Router: router, Prefix: adv.prefix, RouteType: adv.routeType},
				func() { e.ext.subtlvErrors.With(extRegistryPrefix).Inc() })
			body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{tlv}})
			out = append(out, opaqueOrigination{OpaqueID: id, Area: adv.area, Scope: adv.scope, Body: body})
			if _, existed := o.prevPrefix[key]; !existed {
				e.ext.originations.With(opaqueTypeLabel(packet.ExtPrefixOpaqueType)).Inc()
			}
			desired[key] = extOrig{id: id, area: adv.area, scope: adv.scope}
		}
	}
	// Withdraw instances originated last pass but no longer desired (AC-13).
	for key, prev := range o.prevPrefix {
		if _, ok := desired[key]; !ok {
			out = append(out, opaqueOrigination{OpaqueID: prev.id, Area: prev.area, Scope: prev.scope, Withdraw: true})
		}
	}
	o.prevPrefix = desired
	e.refreshExtMetrics()
	return out
}

// selfPrefixAdverts reads this router's advertised prefixes from the LSDB self-LSAs and maps
// each to an Extended Prefix advertisement (RFC 7684 sec 2.1 Route Type). It reuses the
// authoritative Router/Summary/External LSAs FRR sees rather than recomputing a prefix set, so
// the Route Type and prefix correlate with the base LSAs. Intra-area connected prefixes come
// from Router-LSA stub links (Route Type 1); ABR inter-area prefixes from self Type-3
// summaries (Route Type 3, A-Flag when connected in another area); AS-external prefixes from
// self Type-5 (Route Type 5, AS scope).
func (e *engine) selfPrefixAdverts(router types.RouterID) []extPrefixAdvert {
	if e.lsdb == nil {
		return nil
	}
	connected := map[types.AreaID][]connectedPrefix{}
	var out []extPrefixAdvert

	// Intra-area connected prefixes: Router-LSA stub links (RFC 2328 A.4.2 type 3).
	for _, v := range e.lsdb.LSAViewsByType(types.LSTypeRouter) {
		if v.AdvertisingRouter != router {
			continue
		}
		rl, err := packet.DecodeRouterLSA(v.Body)
		if err != nil {
			continue
		}
		for _, link := range rl.Links {
			if link.Type != packet.RouterLinkTypeStub {
				continue
			}
			pfx, ok := prefixFromNetMask([4]byte(link.LinkID), link.LinkData)
			if !ok {
				continue
			}
			flags := uint8(0)
			// RFC 7684 sec 2.1: the N-Flag MAY be set for a host prefix (a /32 loopback)
			// identifying the advertising router.
			if pfx.Bits() == 32 {
				flags |= packet.ExtPrefixFlagN
			}
			out = append(out, extPrefixAdvert{prefix: pfx, routeType: packet.ExtRouteTypeIntraArea, scope: extPrefixScope(packet.ExtRouteTypeIntraArea), area: v.Area, flags: flags})
			connected[v.Area] = append(connected[v.Area], connectedPrefix{prefix: pfx, host: pfx.Bits() == 32})
		}
	}

	// Inter-area prefixes: self Type-3 Summary LSAs (RFC 2328 sec 12.4.3), originated by ABRs.
	for _, v := range e.lsdb.LSAViewsByType(types.LSTypeSummaryNetwork) {
		if v.AdvertisingRouter != router {
			continue
		}
		sm, err := packet.DecodeSummaryLSA(v.Body)
		if err != nil {
			continue
		}
		pfx, ok := prefixFromNetMask([4]byte(v.LinkStateID), sm.NetworkMask)
		if !ok {
			continue
		}
		flags := uint8(0)
		if host, attached := connectedElsewhere(connected, v.Area, pfx); attached {
			// RFC 7684 sec 2.1: an ABR generating this TLV for an inter-area prefix locally
			// connected in another connected area SHOULD set the A-Flag.
			flags |= packet.ExtPrefixFlagA
			if host {
				// RFC 7684 sec 2.1: the N-Flag is preserved when propagated between areas.
				flags |= packet.ExtPrefixFlagN
			}
		}
		out = append(out, extPrefixAdvert{prefix: pfx, routeType: packet.ExtRouteTypeInterArea, scope: extPrefixScope(packet.ExtRouteTypeInterArea), area: v.Area, flags: flags})
	}

	// AS-external prefixes: self Type-5 AS-External LSAs (RFC 2328 sec 12.4.4), AS scope.
	for _, v := range e.lsdb.LSAViewsByType(types.LSTypeASExternal) {
		if v.AdvertisingRouter != router {
			continue
		}
		ext, err := packet.DecodeExternalLSA(v.Body)
		if err != nil {
			continue
		}
		pfx, ok := prefixFromNetMask([4]byte(v.LinkStateID), ext.NetworkMask)
		if !ok {
			continue
		}
		out = append(out, extPrefixAdvert{prefix: pfx, routeType: packet.ExtRouteTypeASExternal, scope: extPrefixScope(packet.ExtRouteTypeASExternal), area: types.BackboneArea})
	}

	sortExtPrefixAdverts(out)
	return out
}

// connectedElsewhere reports whether pfx is a connected prefix in an area OTHER than exclude,
// and whether that connected prefix is a host prefix (for the A-Flag / N-Flag decision).
func connectedElsewhere(connected map[types.AreaID][]connectedPrefix, exclude types.AreaID, pfx netip.Prefix) (host, attached bool) {
	for area, list := range connected {
		if area == exclude {
			continue
		}
		for _, c := range list {
			if c.prefix == pfx {
				return c.host, true
			}
		}
	}
	return false, false
}

// sortExtPrefixAdverts orders advertisements deterministically (scope, area, route type,
// prefix) so Opaque-ID assignment and re-origination are stable.
func sortExtPrefixAdverts(a []extPrefixAdvert) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].scope != a[j].scope {
			return a[i].scope < a[j].scope
		}
		if a[i].area != a[j].area {
			return lessAreaID(a[i].area, a[j].area)
		}
		if a[i].routeType != a[j].routeType {
			return a[i].routeType < a[j].routeType
		}
		return a[i].prefix.String() < a[j].prefix.String()
	})
}

// extPrefixOnReceive is the RFC 5250 sec 3 reception hook for Opaque Type 7. It decodes the
// body (a malformed body is counted and NOT applied, RFC 7684 sec 5), ignores the N-Flag on a
// non-host prefix (sec 2.1), dedups a duplicate prefix within one LSA (first wins, logged) and
// across LSAs (lowest Opaque ID wins), dispatches sub-TLVs to registered codecs, and records
// Type-11 reachability (RFC 5250 sec 5). It never applies a route (empty containers).
func (e *engine) extPrefixOnReceive(r opaqueReceived) {
	if r.OpaqueType != packet.ExtPrefixOpaqueType {
		return
	}
	if r.Withdrawn {
		e.extRecv.withdrawPrefixes(r.AdvertisingRouter, r.OpaqueID)
		e.refreshExtMetrics()
		return
	}
	lsa, err := packet.DecodeExtPrefixLSA(r.Body)
	if err != nil {
		// RFC 7684 sec 5: a malformed LSA MUST NOT be applied. The ext-1 carrier owns
		// storage/reflood of the raw opaque bytes; this consumer refuses to derive any
		// attribute from a malformed body and counts it.
		e.ext.malformed.With(opaqueTypeLabel(r.OpaqueType)).Inc()
		return
	}
	// RFC 5250 sec 5: a Type-11 LSA from an unreachable originator is present-but-unusable.
	usable := r.Scope != OpaqueScopeAS || r.Reachable
	seen := map[[5]byte]bool{}
	for i := range lsa.Prefixes {
		tlv := lsa.Prefixes[i]
		// RFC 7684 sec 2.1: only AF 0 (IPv4 unicast) is defined; other AFs are out of scope.
		if tlv.AF != packet.ExtPrefixAFIPv4Unicast {
			continue
		}
		pk := [5]byte{tlv.AddressPrefix[0], tlv.AddressPrefix[1], tlv.AddressPrefix[2], tlv.AddressPrefix[3], tlv.PrefixLength}
		if seen[pk] {
			// RFC 7684 sec 2.1: a duplicate Extended Prefix TLV for the same prefix in one LSA
			// -> use the first instance and log the situation as an error.
			logger().Warn("ospf ext-prefix: duplicate Extended Prefix TLV in one LSA, using first",
				"router", r.AdvertisingRouter.String(), "opaque-id", r.OpaqueID, "prefix", extPrefixString(tlv))
			continue
		}
		seen[pk] = true
		for _, s := range tlv.SubTLVs {
			dispatchPrefixSubTLV(s, func() { e.ext.subtlvErrors.With(extRegistryPrefix).Inc() })
		}
		e.extRecv.applyPrefix(r.AdvertisingRouter, r.OpaqueID, tlv.RouteType, extNormalizeFlags(tlv), pk, r.Scope, usable)
	}
	e.refreshExtMetrics()
}

// extNormalizeFlags returns the Extended Prefix TLV flags after RFC 7684 sec 2.1 normalization:
// the N-Flag is ignored (cleared) when it is set on a prefix that is not a host prefix (/32).
func extNormalizeFlags(tlv packet.ExtPrefixTLV) uint8 {
	flags := tlv.Flags
	if tlv.HasFlag(packet.ExtPrefixFlagN) && tlv.PrefixLength != 32 {
		flags &^= packet.ExtPrefixFlagN
	}
	return flags
}

// extPrefixString renders a decoded Extended Prefix TLV's prefix as a CIDR string for logging.
func extPrefixString(tlv packet.ExtPrefixTLV) string {
	return netip.PrefixFrom(netip.AddrFrom4(tlv.AddressPrefix), int(tlv.PrefixLength)).String()
}
