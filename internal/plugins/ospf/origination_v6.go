// Design: plan/learned/972-ospf-af-unify.md -- OSPFv3 (IPv6) self-origination.
// RFC: rfc/short/rfc5340.md (App A.4.3 Router-LSA, A.4.4 Network-LSA, A.4.10 Intra-Area-Prefix-LSA)
//
// originateSelfLSAs routes the IPv6 family here instead of the OSPFv2
// lsdb.OriginateFromTopology path. OSPFv3 LSAs are address-free (RFC 5340 App A.4.3):
// the Router-LSA carries adjacencies (neighbor Router ID + Interface ID) with no IP
// addresses, and the router's own IPv6 prefixes live in a separate Intra-Area-Prefix-LSA
// (App A.4.10) that references the Router-LSA. The LSDB owns sequencing, rate-limiting
// and flooding through OriginateSelf; this file owns the OSPFv3 LSA construction
// (ospfv3/packet) and the neutral header mapping shared with the v6 codec.
//
// Broadcast: a DR also originates a Network-LSA (App A.4.4) for its segment and every router
// on the segment carries a transit link in its Router-LSA. The transit network's prefixes
// (carried in a Network-referencing Intra-Area-Prefix-LSA built from the segment's Link-LSAs)
// are a route-data-plane follow-up; the control plane (adjacency, Network-LSA, transit links)
// is here.

package ospf

import (
	"net/netip"
	"slices"
	"sort"

	ifcomp "codeberg.org/thomas-mangin/ze/internal/component/iface"
	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
	ospfv3packet "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
)

// v6MaxLinkMetric is the RFC 6987 max-metric value applied to every non-stub link.
const v6MaxLinkMetric uint16 = 0xffff

// v6ManagedSelfTypes are the OSPFv3 self-LSA types v6OriginateSelf owns; the stale-flush
// sweeps only these (Router, Network for DR segments, Intra-Area-Prefix), leaving any other
// self LSA (e.g. the inter-area summaries) untouched.
var v6ManagedSelfTypes = map[types.LSType]struct{}{
	types.LSType(ospfv3types.LSTypeRouter):          {},
	types.LSType(ospfv3types.LSTypeNetwork):         {},
	types.LSType(ospfv3types.LSTypeIntraAreaPrefix): {},
}

// v6NetworkKey is the LSDB key for this router's OSPFv3 Network-LSA on a broadcast segment it
// is the DR for; the Link State ID is the DR's own Interface ID (RFC 5340 App A.4.4).
func v6NetworkKey(router types.RouterID, ifaceID uint32) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeNetwork), LinkStateID: v6SummaryLSID(ifaceID), AdvertisingRouter: router}
}

// v6RouterKey is the LSDB key for this router's OSPFv3 Router-LSA (Link State ID 0, the
// single fragment; RFC 5340 App A.4.3).
func v6RouterKey(router types.RouterID) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeRouter), AdvertisingRouter: router}
}

// v6IntraAreaPrefixKey is the LSDB key for this router's Intra-Area-Prefix-LSA.
func v6IntraAreaPrefixKey(router types.RouterID) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeIntraAreaPrefix), AdvertisingRouter: router}
}

// v6OriginateSelf regenerates this router's OSPFv3 self-LSAs for every active area from
// the live topology snapshot: per-link Link-LSAs, the address-free Router-LSA
// (adjacencies), Router-referencing Intra-Area-Prefix-LSAs for the router's global
// prefixes, DR Network-LSAs, and DR Network-referencing Intra-Area-Prefix-LSAs built
// from link-scoped Link-LSAs. Each area is independent, so the iteration order does
// not matter. After regenerating the current set it MaxAge-flushes stale area-scope
// LSAs and releases stale self Link-LSAs, so withdrawn reachability does not linger.
// It returns the number of LSAs (re)originated; an unchanged topology re-originates nothing.
func (e *engine) v6OriginateSelf(router types.RouterID, maxMetric bool) int {
	if e.lsdb == nil || router == (types.RouterID{}) {
		return 0
	}
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	byArea := make(map[types.AreaID][]ospflsdb.InterfaceInfo)
	topology := e.lsdbTopology()
	for idx := range topology {
		if topology[idx].AreaID == (types.AreaID{}) && topology[idx].Name == "" {
			continue
		}
		byArea[topology[idx].AreaID] = append(byArea[topology[idx].AreaID], topology[idx])
	}
	activeAreas := make([]types.AreaID, 0, len(byArea))
	for area, ifaces := range byArea {
		if v6AreaHasAdvertisedLinks(ifaces) {
			activeAreas = append(activeAreas, area)
		}
	}
	abr := v6IsAreaBorderRouter(activeAreas)
	count := 0
	linkKeep := make(map[ospflsdb.LinkLSARef]struct{})
	for _, ifaces := range byArea {
		for idx := range ifaces {
			iface := &ifaces[idx]
			if !v6ShouldOriginateLinkLSA(*iface) {
				continue
			}
			key, changed := e.v6OriginateLinkLSA(router, *iface)
			if key.Type != 0 {
				linkKeep[ospflsdb.LinkLSARef{Interface: iface.Name, Key: key}] = struct{}{}
			}
			if changed {
				count++
			}
		}
	}
	keep := make(map[ospflsdb.SelfLSARef]struct{})
	for area, ifaces := range byArea {
		opts := ospfv3types.Options(0)
		if len(ifaces) > 0 {
			opts = neutralToV6Options(ifaces[0].Options)
		}
		if _, ok := e.v6OriginateRouter(area, router, opts, ifaces, maxMetric, abr, v6NSSATranslatorArea(cfg, area) && abr); ok {
			count++
		}
		keep[ospflsdb.SelfLSARef{Area: area, Key: v6RouterKey(router)}] = struct{}{}

		if prefixes := v6InterfacePrefixes(ifaces); len(prefixes) > 0 {
			if _, ok := e.v6OriginateIntraAreaPrefix(area, router, prefixes); ok {
				count++
			}
			keep[ospflsdb.SelfLSARef{Area: area, Key: v6IntraAreaPrefixKey(router)}] = struct{}{}
		}

		// Broadcast segments where this router is the DR: originate the Network-LSA (RFC
		// 5340 App A.4.4) naming the attached routers and the Network-referencing
		// Intra-Area-Prefix-LSA (RFC 5340 App A.4.9) aggregating attached routers'
		// Link-LSA prefixes. A segment where this router is not the DR contributes only
		// the transit link in the Router-LSA above.
		for idx := range ifaces {
			iface := &ifaces[idx]
			if iface.NetworkType != ospflsdb.NetworkBroadcast || iface.DR != router || !v6AdvertiseInterface(*iface) {
				continue
			}
			key, changed := e.v6OriginateNetwork(area, router, opts, *iface)
			if changed {
				count++
			}
			keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
			if key, changed := e.v6OriginateNetworkIntraAreaPrefix(area, router, *iface); key.Type != 0 {
				if changed {
					count++
				}
				keep[ospflsdb.SelfLSARef{Area: area, Key: key}] = struct{}{}
			}
		}
	}
	count += e.lsdb.FlushStaleLinkSelfLSAs(router, linkKeep)
	count += e.lsdb.FlushStaleSelfLSAs(router, v6ManagedSelfTypes, keep)
	return count
}

// v6OriginateRouter builds and originates this router's OSPFv3 Router-LSA for one area.
// The Link State ID of an OSPFv3 Router-LSA is a fragment number (0 for the single
// fragment), not the Router ID as in OSPFv2 (RFC 5340 App A.4.3).
func (e *engine) v6OriginateRouter(area types.AreaID, router types.RouterID, opts ospfv3types.Options, ifaces []ospflsdb.InterfaceInfo, maxMetric, abr, nssaTranslator bool) (packet.LSAHeader, bool) {
	// RFC 5340 App A.4.3: set B when this router is an ABR, E when it is an
	// ASBR, and Nt in directly attached NSSAs where Ze participates in Type-7
	// translation. SelfIsASBR is AF-neutral, so v6 Type-7 originators also set E.
	asbr := e.lsdb != nil && e.lsdb.SelfIsASBR(router)
	body := v6RouterLSABody(opts, ifaces, maxMetric, asbr, abr, nssaTranslator)
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := v6RouterKey(router)
	return e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(ospfv3types.LSTypeRouter, ospfv3types.LinkStateID{}, router, seq, purge),
			Router: &body,
		})
	})
}

// v6OriginateIntraAreaPrefix builds and originates the Intra-Area-Prefix-LSA that attaches
// this router's global IPv6 interface prefixes (already collected by the caller) to its
// Router-LSA (RFC 5340 App A.4.10). The caller originates it only when there is at least
// one prefix; the empty case is withdrawn by the stale-flush in v6OriginateSelf.
func (e *engine) v6OriginateIntraAreaPrefix(area types.AreaID, router types.RouterID, prefixes []ospfv3packet.Prefix) (packet.LSAHeader, bool) {
	if len(prefixes) == 0 {
		return packet.LSAHeader{}, false
	}
	body := ospfv3packet.IntraAreaPrefixLSA{
		ReferencedLSType:    ospfv3types.LSTypeRouter,
		ReferencedAdvRouter: ospfv3types.RouterID(router),
		Prefixes:            prefixes,
	}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := v6IntraAreaPrefixKey(router)
	return e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:       v6OriginHeader(ospfv3types.LSTypeIntraAreaPrefix, ospfv3types.LinkStateID{}, router, seq, purge),
			IntraAreaPfx: &body,
		})
	})
}

// v6OriginateNetwork originates the OSPFv3 Network-LSA for a broadcast segment where this
// router is the DR (RFC 5340 App A.4.4): the address-free body lists the attached routers
// (this router plus every Full neighbor on the segment), sorted for a stable, idempotent
// body. The Link State ID is this router's Interface ID. It returns the LSDB key (for the
// caller's stale-flush keep set) and whether the LSA was (re)originated.
func (e *engine) v6OriginateNetwork(area types.AreaID, router types.RouterID, opts ospfv3types.Options, iface ospflsdb.InterfaceInfo) (types.LSAKey, bool) {
	attached := []ospfv3types.RouterID{ospfv3types.RouterID(router)}
	for _, nbr := range iface.Neighbors {
		if nbr.State == ospflsdb.NeighborStateFull {
			attached = append(attached, ospfv3types.RouterID(nbr.RouterID))
		}
	}
	sort.Slice(attached, func(i, j int) bool { return v6CompareRID(types.RouterID(attached[i]), types.RouterID(attached[j])) < 0 })
	body := ospfv3packet.NetworkLSA{Options: opts, AttachedRouters: attached}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	lsid := v6SummaryLSID(iface.InterfaceID)
	key := v6NetworkKey(router, iface.InterfaceID)
	_, ok := e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:  v6OriginHeader(ospfv3types.LSTypeNetwork, ospfv3types.LinkStateID(lsid), router, seq, purge),
			Network: &body,
		})
	})
	return key, ok
}

// v6RouterLSABody builds the address-free OSPFv3 Router-LSA body from an area's
// interfaces: one point-to-point link record per Full neighbor (RFC 5340 App A.4.3). The
// link's Interface ID is this router's ifindex for the interface -- the same value the
// interface advertises in its Hellos (RFC 5340 sec 3.4.3) -- and the Neighbor Interface ID
// is the Interface ID that neighbor advertised in its Hellos (tracked through the adjacency
// table); the point-to-point two-way check keys on the Neighbor Router ID.
func v6RouterLSABody(opts ospfv3types.Options, ifaces []ospflsdb.InterfaceInfo, maxMetric, asbr, abr, nssaTranslator bool) ospfv3packet.RouterLSA {
	body := ospfv3packet.RouterLSA{Options: opts}
	if abr {
		body.Flags |= ospfv3packet.RouterFlagB
	}
	if asbr {
		body.Flags |= ospfv3packet.RouterFlagE
	}
	if nssaTranslator {
		body.Flags |= ospfv3packet.RouterFlagNt
	}
	for idx := range ifaces {
		iface := &ifaces[idx]
		if !v6AdvertiseInterface(*iface) {
			continue
		}
		metric := iface.Cost
		if metric == 0 {
			metric = 1
		}
		if maxMetric {
			metric = v6MaxLinkMetric
		}
		switch iface.NetworkType {
		case ospflsdb.NetworkPointToPoint:
			for _, nbr := range iface.Neighbors {
				if nbr.State != ospflsdb.NeighborStateFull {
					continue
				}
				body.Links = append(body.Links, ospfv3packet.RouterLink{
					Type:                ospfv3packet.RouterLinkTypeP2P,
					Metric:              metric,
					InterfaceID:         ospfv3types.InterfaceID(iface.InterfaceID),
					NeighborInterfaceID: ospfv3types.InterfaceID(nbr.InterfaceID),
					NeighborRouterID:    ospfv3types.RouterID(nbr.RouterID),
				})
			}
		case ospflsdb.NetworkBroadcast:
			if link, ok := v6TransitLink(*iface, metric); ok {
				body.Links = append(body.Links, link)
			}
		}
	}
	return body
}

// v6TransitLink builds the Type 2 (transit) Router-LSA link for a broadcast segment with an
// elected DR (RFC 5340 App A.4.3): InterfaceID is this router's, and NeighborRouterID /
// NeighborInterfaceID identify the DR -- this router's own IDs when it is the DR, otherwise
// the DR neighbor's (learned from its Hellos). With no DR elected (or the DR not yet a known
// neighbor) there is no transit link, mirroring OSPFv2 routerLinks.
func v6TransitLink(iface ospflsdb.InterfaceInfo, metric uint16) (ospfv3packet.RouterLink, bool) {
	if iface.DR == (types.RouterID{}) {
		return ospfv3packet.RouterLink{}, false
	}
	drIfaceID := iface.InterfaceID
	if iface.DR != iface.RouterID {
		drIfaceID = 0
		found := false
		for _, nbr := range iface.Neighbors {
			if nbr.RouterID == iface.DR {
				drIfaceID = nbr.InterfaceID
				found = true
				break
			}
		}
		if !found {
			return ospfv3packet.RouterLink{}, false
		}
	}
	return ospfv3packet.RouterLink{
		Type:                ospfv3packet.RouterLinkTypeTransit,
		Metric:              metric,
		InterfaceID:         ospfv3types.InterfaceID(iface.InterfaceID),
		NeighborInterfaceID: ospfv3types.InterfaceID(drIfaceID),
		NeighborRouterID:    ospfv3types.RouterID(iface.DR),
	}, true
}

// v6InterfacePrefixes collects the area interfaces' global IPv6 prefixes as OSPFv3 prefix
// records, the 16-bit field carrying the interface metric (RFC 5340 App A.4.10).
// Duplicate prefixes (the same subnet on multiple interfaces) are advertised once.
func v6InterfacePrefixes(ifaces []ospflsdb.InterfaceInfo) []ospfv3packet.Prefix {
	seen := make(map[string]struct{})
	var out []ospfv3packet.Prefix
	for idx := range ifaces {
		iface := &ifaces[idx]
		if !v6AdvertiseInterface(*iface) {
			continue
		}
		metric := iface.Cost
		if metric == 0 {
			metric = 1
		}
		for _, pfx := range interfaceIPv6Prefixes(iface.Name) {
			if _, ok := seen[pfx.String()]; ok {
				continue
			}
			seen[pfx.String()] = struct{}{}
			p, ok := netipToV6Prefix(pfx, metric)
			if !ok {
				continue
			}
			out = append(out, p)
		}
	}
	return out
}

// v6AdvertiseInterface mirrors the OSPFv2 advertiseInterfaceLinks rule: an interface
// contributes to origination unless it is administratively down (a passive interface
// still advertises its prefix).
func v6AdvertiseInterface(iface ospflsdb.InterfaceInfo) bool {
	return iface.State != ospflsdb.InterfaceStateDown || iface.Passive
}

func v6AreaHasAdvertisedLinks(ifaces []ospflsdb.InterfaceInfo) bool {
	return slices.ContainsFunc(ifaces, v6AdvertiseInterface)
}

func v6IsAreaBorderRouter(areas []types.AreaID) bool {
	return len(areas) >= 2 && slices.Contains(areas, types.BackboneArea)
}

func v6NSSATranslatorArea(cfg ospfConfig, area types.AreaID) bool {
	for _, a := range cfg.Areas {
		if a.AreaID == area {
			return a.AreaType == areaTypeNSSA && a.NSSATranslateRole != translateRoleNever
		}
	}
	return false
}

// interfaceIPv6Prefixes returns the interface's global (non-link-local, non-loopback,
// non-multicast) IPv6 prefixes as masked netip.Prefix values: the prefixes this router
// advertises in its OSPFv3 Intra-Area-Prefix-LSA. Link-local addresses carry no routing
// reachability and belong in the Link-LSA, not the Intra-Area-Prefix-LSA.
func interfaceIPv6Prefixes(name string) []netip.Prefix {
	addrs, err := ifcomp.Addresses(name)
	if err != nil {
		return nil
	}
	var out []netip.Prefix
	for _, addr := range addrs {
		if addr.Family != "ipv6" || addr.PrefixLength < 0 || addr.PrefixLength > 128 {
			continue
		}
		parsed, err := netip.ParseAddr(addr.Address)
		if err != nil || !parsed.Is6() || parsed.Is4In6() {
			continue
		}
		if parsed.IsLinkLocalUnicast() || parsed.IsLoopback() || parsed.IsUnspecified() || parsed.IsMulticast() {
			continue
		}
		out = append(out, netip.PrefixFrom(parsed, addr.PrefixLength).Masked())
	}
	return out
}

func interfaceIPv6LinkLocal(name string) netip.Addr {
	addrs, err := ifcomp.Addresses(name)
	if err != nil {
		return netip.Addr{}
	}
	for _, addr := range addrs {
		if addr.Family != "ipv6" || !addr.LinkLocal {
			continue
		}
		parsed, err := netip.ParseAddr(addr.Address)
		if err == nil && parsed.Is6() && parsed.IsLinkLocalUnicast() {
			return parsed
		}
	}
	return netip.Addr{}
}

// netipToV6Prefix converts a netip.Prefix to the OSPFv3 word-padded wire prefix (RFC
// 5340 App A.4.1) with metric in the 16-bit field. It is the inverse of v6PrefixToNetip.
func netipToV6Prefix(pfx netip.Prefix, metric uint16) (ospfv3packet.Prefix, bool) {
	if !pfx.Addr().Is6() || pfx.Bits() < 0 || pfx.Bits() > 128 {
		return ospfv3packet.Prefix{}, false
	}
	plen, err := ospfv3types.NewPrefixLength(uint8(pfx.Bits()))
	if err != nil {
		return ospfv3packet.Prefix{}, false
	}
	full := pfx.Masked().Addr().As16()
	addr := make([]byte, plen.ByteLen())
	copy(addr, full[:plen.ByteLen()])
	return ospfv3packet.Prefix{Length: plen, Field16: metric, Address: addr}, true
}

// v6OriginHeader builds the OSPFv3 LSA header for a self-originated LSA: age 0 (or MaxAge
// for a purge), the LSDB-assigned sequence reinterpreted into the OSPFv3 signed-32-bit
// space (bit pattern preserved, matching neutralToV6LSAHeader), Length and Checksum left
// zero for WriteTo to finalize.
func v6OriginHeader(t ospfv3types.LSType, lsid ospfv3types.LinkStateID, router types.RouterID, seq types.LSSequenceNumber, purge bool) ospfv3packet.LSAHeader {
	age := ospfv3types.LSAge(0)
	if purge {
		age = ospfv3types.MaxAge
	}
	return ospfv3packet.LSAHeader{
		Age:               age,
		Type:              t,
		LinkStateID:       lsid,
		AdvertisingRouter: ospfv3types.RouterID(router),
		Sequence:          ospfv3types.LSSequenceNumber(int32(uint32(seq))),
	}
}

// v6SelfLSA serializes a constructed OSPFv3 LSA (Length + Fletcher checksum finalized by
// WriteTo) and wraps the bytes as an AF-neutral packet.LSA for the LSDB. The neutral
// header is read after WriteTo so it carries the finalized Length and Checksum; the
// typed body is dropped so the LSDB's AF-agnostic normaliseLSA trusts the raw bytes.
func v6SelfLSA(lsa ospfv3packet.LSA) packet.LSA {
	raw := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(raw, 0)
	return packet.LSA{Header: v6LSAHeaderToNeutral(lsa.Header), RawBytes: raw}
}
