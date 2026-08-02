// Design: plan/learned/961-ospf-7-lsdb-flooding.md -- Router-LSA and Network-LSA origination.
// RFC: rfc/short/rfc2328.md -- Appendix A.4.2-A.4.3 (Router/Network bodies), sec 12.4.3 (Summary), sec 12.4.4 (AS-External)
// RFC: rfc/short/rfc6987.md -- sec 2 max-metric sets all non-stub Router-LSA links to 0xffff.

package lsdb

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const LSInfinity types.Metric = 0xffff

// ErrExternalStoreFull is returned by OriginateExternal when the AS-wide Type 5 store
// is at capacity (MaxASExternalLSAs) and the LSA could not be installed. RFC 2328
// origination must not silently drop: the caller logs it and (for redistribution) does
// NOT count the route as injected.
var ErrExternalStoreFull = errors.New("ospf lsdb: AS-external store full, Type 5 origination rejected")

type ownRecord struct {
	sequence types.LSSequenceNumber
	last     time.Time
}

// OriginInput is the value snapshot used to regenerate self LSAs. It mirrors the
// IS-IS originator pattern: full regeneration from live state instead of partial
// link-record edits.
type OriginInput struct {
	AreaID              types.AreaID
	RouterID            types.RouterID
	Options             types.Options
	ABR                 bool
	ASBR                bool
	VirtualLinkEndpoint bool
	NSSATranslator      bool // RFC 3101 Nt-bit: this router translates Type 7 -> Type 5 here
	MaxMetric           bool
	Interfaces          []InterfaceInfo
}

// OriginateFromTopology regenerates this router's Router-LSA for every active
// area in the topology snapshot and Network-LSAs for segments where this router
// is DR.
func (d *LSDB) OriginateFromTopology(router types.RouterID, maxMetric bool) int {
	ifs := d.topologySnapshot()
	byArea := make(map[types.AreaID][]InterfaceInfo)
	for idx := range ifs {
		if ifs[idx].AreaID == (types.AreaID{}) && ifs[idx].Name == "" {
			continue
		}
		byArea[ifs[idx].AreaID] = append(byArea[ifs[idx].AreaID], ifs[idx])
	}
	areas := make([]types.AreaID, 0, len(byArea))
	activeAreas := make([]types.AreaID, 0, len(byArea))
	for area, ifaces := range byArea {
		areas = append(areas, area)
		if AreaHasAdvertisedLinks(ifaces) {
			activeAreas = append(activeAreas, area)
		}
	}
	sort.Slice(areas, func(i, j int) bool { return compareAreaID(areas[i], areas[j]) < 0 })
	sort.Slice(activeAreas, func(i, j int) bool { return compareAreaID(activeAreas[i], activeAreas[j]) < 0 })
	count := 0
	abr := isAreaBorderRouter(activeAreas)
	// RFC 2328 App A.4.2 / section 16.3: the V-bit is set in the Router-LSA for the TRANSIT
	// area of a fully adjacent virtual link (it marks that area as a transit area, driving
	// the far ABR's TransitCapability), NOT in the backbone Router-LSA that carries the
	// Type-4 virtual link record itself.
	fullTransitAreas := fullVirtualTransitAreas(ifs)
	// RFC 2328 Section 3.3 / 12.4.4: the node is an ASBR (Router-LSA E-bit) exactly
	// when it currently originates at least one non-purged Type 5 AS-External-LSA.
	// This clears the E-bit automatically when the last external is withdrawn (AC-6).
	asbr := d.selfOriginatesExternal(router)
	for _, area := range areas {
		opts := types.Options(0)
		if len(byArea[area]) > 0 {
			opts = byArea[area][0].Options
		}
		nt := abr && d.isNSSATranslatorArea(area)
		// RFC 2328 App A.4.2 / section 16.3: set the V-bit in the Router-LSA for a TRANSIT
		// area (an area a Full virtual link runs through), never in the backbone Router-LSA
		// that carries the Type-4 virtual link record.
		vle := fullTransitAreas[area]
		if _, ok := d.OriginateRouter(OriginInput{AreaID: area, RouterID: router, Options: opts, ABR: abr, ASBR: asbr, VirtualLinkEndpoint: vle, NSSATranslator: nt, MaxMetric: maxMetric, Interfaces: byArea[area]}); ok {
			count++
		}
		desiredNetworks := make(map[types.LSAKey]struct{})
		for idx := range byArea[area] {
			iface := &byArea[area][idx]
			if !advertiseInterfaceLinks(*iface) || iface.DR != router {
				continue
			}
			key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(iface.Address), AdvertisingRouter: router}
			desiredNetworks[key] = struct{}{}
			if _, ok := d.OriginateNetwork(area, router, opts, *iface); ok {
				count++
			}
		}
		count += d.flushStaleNetworkLSAs(area, router, desiredNetworks)
	}
	return count
}

// OriginateRouter builds and installs this router's Type 1 Router-LSA.
func (d *LSDB) OriginateRouter(in OriginInput) (packet.LSAHeader, bool) {
	if in.RouterID == (types.RouterID{}) {
		return packet.LSAHeader{}, false
	}
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(in.RouterID), AdvertisingRouter: in.RouterID}
	flags := uint8(0)
	if in.VirtualLinkEndpoint {
		flags |= packet.RouterFlagV
	}
	if in.ASBR {
		flags |= packet.RouterFlagE
	}
	if in.ABR {
		flags |= packet.RouterFlagB
	}
	if in.NSSATranslator {
		flags |= packet.RouterFlagNt
	}
	body := packet.RouterLSA{Flags: flags, Links: routerLinks(in)}
	if h, same := d.existingSelfBodyUnchanged(in.AreaID, key, encodedRouterBody(body)); same {
		return h, false
	}
	seq, ok, purge := d.nextOwnSequence(in.AreaID, key)
	if !ok {
		h, _ := d.Lookup(in.AreaID, key)
		return h, false
	}
	h := packet.LSAHeader{
		Age:               types.LSAge(0),
		Options:           in.Options,
		Type:              types.LSTypeRouter,
		LinkStateID:       key.LinkStateID,
		AdvertisingRouter: in.RouterID,
		Sequence:          seq,
	}
	if purge {
		h.Age = types.LSAge(types.MaxAge)
	}
	return d.installOriginated(in.AreaID, packet.LSA{Header: h, Router: &body}, key, purge)
}

// AreaHasAdvertisedLinks reports whether an area has a live or passive
// interface, or a Full virtual link, that contributes to its Router-LSA.
func AreaHasAdvertisedLinks(ifaces []InterfaceInfo) bool {
	for idx := range ifaces {
		if ifaces[idx].NetworkType == NetworkVirtual {
			// RFC 2328 section 15 / RFC 5340 section 3.5: a virtual link makes its area (the
			// backbone) active for ABR/backbone-attachment only when the adjacency is Full.
			if virtualLinkFull(ifaces[idx]) {
				return true
			}
			continue
		}
		if advertiseInterfaceLinks(ifaces[idx]) {
			return true
		}
	}
	return false
}

func advertiseInterfaceLinks(iface InterfaceInfo) bool {
	return iface.State != InterfaceStateDown || iface.Passive
}

// fullVirtualTransitAreas returns the set of transit areas that carry a fully adjacent
// virtual link. The Router-LSA V-bit is set in each such TRANSIT area's Router-LSA (RFC
// 2328 App A.4.2 / section 16.3): it marks the area as a transit area for a virtual link,
// which is what a far ABR reads to set TransitCapability. The Type-4 record itself is
// emitted in the backbone Router-LSA (routerLinks), so the two placements differ.
func fullVirtualTransitAreas(ifaces []InterfaceInfo) map[types.AreaID]bool {
	var out map[types.AreaID]bool
	for idx := range ifaces {
		if ifaces[idx].NetworkType != NetworkVirtual || !virtualLinkFull(ifaces[idx]) {
			continue
		}
		if out == nil {
			out = make(map[types.AreaID]bool)
		}
		out[ifaces[idx].VirtualTransitArea] = true
	}
	return out
}

// virtualLinkFull reports whether a synthetic virtual interface has a Full neighbor.
func virtualLinkFull(iface InterfaceInfo) bool {
	for _, nbr := range iface.Neighbors {
		if nbr.State == NeighborStateFull {
			return true
		}
	}
	return false
}

func routerLinks(in OriginInput) []packet.RouterLink {
	links := make([]packet.RouterLink, 0, len(in.Interfaces)*2)
	for idx := range in.Interfaces {
		iface := &in.Interfaces[idx]
		if !advertiseInterfaceLinks(*iface) {
			continue
		}
		metric := types.Metric(iface.Cost)
		if metric == 0 {
			metric = 1
		}
		local := iface.Address
		// RFC 2328 App A.4.2: a virtual link is a Type-4 link record with Link ID = the
		// neighbor Router ID, Link Data = the local transit interface address, and Metric =
		// the transit-area path cost. It has no stub/transit link, so `continue` after
		// emitting the record. Only emitted when the adjacency is Full.
		if iface.NetworkType == NetworkVirtual {
			for _, nbr := range iface.Neighbors {
				if nbr.State != NeighborStateFull {
					continue
				}
				m := metric
				if in.MaxMetric {
					m = LSInfinity
				}
				links = append(links, packet.RouterLink{LinkID: types.LinkStateID(nbr.RouterID), LinkData: local, Type: packet.RouterLinkTypeVirtual, Metric: m})
			}
			continue
		}
		// Point-to-multipoint is a collection of point-to-point links (RFC 2328 sec
		// 12.4.1.4): one Type-1 link per Full neighbor, exactly like point-to-point.
		isPtMP := iface.NetworkType == NetworkPointToMultipoint
		if iface.NetworkType == NetworkPointToPoint || isPtMP {
			for _, nbr := range iface.Neighbors {
				if nbr.State != NeighborStateFull {
					continue
				}
				m := metric
				// RFC 6987 router-wide max-metric OR RFC 5443 §2 per-interface LDP-sync
				// cost-out raises the p2p link to LSInfinity; the connected-subnet stub
				// (below) keeps the configured cost either way.
				if in.MaxMetric || iface.LDPSyncMaxMetric {
					m = LSInfinity
				}
				links = append(links, packet.RouterLink{LinkID: types.LinkStateID(nbr.RouterID), LinkData: local, Type: packet.RouterLinkTypeP2P, Metric: m})
			}
		}
		// PtMP advertises a host route (its own interface address, mask 255.255.255.255,
		// cost 0) instead of a subnet stub so other routers can reach the interface; it
		// never advertises a transit/subnet prefix (RFC 2328 sec 12.4.1.4).
		if isPtMP {
			if iface.Address != ([4]byte{}) {
				links = append(links, packet.RouterLink{LinkID: types.LinkStateID(iface.Address), LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub, Metric: 0})
			}
			continue
		}
		// RFC 6138 Section 4: "the Router-LSA is not updated with a 'Link Type 2' (link
		// to transit network) for that subnet until LDP is operational with all
		// neighboring routers on that subnet." The engine sets LDPSyncWithholdTransit
		// only for a non-cut-edge broadcast interface not yet LDP-synchronized; a
		// cut-edge is advertised immediately (the RFC 6138 MUST NOT-delay rule) and so
		// never carries this flag. The stub link (below) for the subnet is unaffected.
		if (iface.NetworkType == NetworkBroadcast || iface.NetworkType == NetworkNBMA) && iface.DR != (types.RouterID{}) && !iface.LDPSyncWithholdTransit {
			drAddr := iface.Address
			if iface.DR != in.RouterID {
				for _, nbr := range iface.Neighbors {
					// OSPFv2 Network-LSA Link State ID is the DR's IPv4 interface
					// address; derive it from the neighbor's reachable address.
					if nbr.RouterID == iface.DR && nbr.Address.Is4() {
						drAddr = nbr.Address.As4()
						break
					}
				}
			}
			m := metric
			if in.MaxMetric || iface.LDPSyncMaxMetric {
				m = LSInfinity
			}
			links = append(links, packet.RouterLink{LinkID: types.LinkStateID(drAddr), LinkData: local, Type: packet.RouterLinkTypeTransit, Metric: m})
		}
		if iface.NetworkMask != ([4]byte{}) && iface.Address != ([4]byte{}) {
			links = append(links, packet.RouterLink{LinkID: types.LinkStateID(networkAddress(iface.Address, iface.NetworkMask)), LinkData: iface.NetworkMask, Type: packet.RouterLinkTypeStub, Metric: metric})
		}
	}
	return links
}

// OriginateNetwork builds and installs the Type 2 Network-LSA for a segment where
// this router is the DR.
func (d *LSDB) OriginateNetwork(area types.AreaID, router types.RouterID, opts types.Options, iface InterfaceInfo) (packet.LSAHeader, bool) {
	if router == (types.RouterID{}) || iface.Address == ([4]byte{}) {
		return packet.LSAHeader{}, false
	}
	key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID(iface.Address), AdvertisingRouter: router}
	attached := []types.RouterID{router}
	for _, nbr := range iface.Neighbors {
		if nbr.State == NeighborStateFull {
			attached = append(attached, nbr.RouterID)
		}
	}
	sort.Slice(attached, func(i, j int) bool { return compareRouterID(attached[i], attached[j]) < 0 })
	body := packet.NetworkLSA{NetworkMask: iface.NetworkMask, AttachedRouters: attached}
	if h, same := d.existingSelfBodyUnchanged(area, key, encodedNetworkBody(body)); same {
		return h, false
	}
	seq, ok, purge := d.nextOwnSequence(area, key)
	if !ok {
		h, _ := d.Lookup(area, key)
		return h, false
	}
	h := packet.LSAHeader{Age: 0, Options: opts, Type: types.LSTypeNetwork, LinkStateID: key.LinkStateID, AdvertisingRouter: router, Sequence: seq}
	if purge {
		h.Age = types.LSAge(types.MaxAge)
	}
	return d.installOriginated(area, packet.LSA{Header: h, Network: &body}, key, purge)
}

// SelfLSAEncoder returns the address-family-neutral on-wire form of a self-originated
// LSA for the sequence number the LSDB assigned. The address family owns the LSA wire
// format (OSPFv2 typed bodies vs the OSPFv3 byte stream), so the caller supplies the
// encoder while the LSDB keeps ownership of sequencing, rate-limiting, install and
// flooding. purge is true when the assigned sequence is a MaxAge flush.
type SelfLSAEncoder func(seq types.LSSequenceNumber, purge bool) packet.LSA

// OriginateSelf installs a caller-encoded self-originated LSA, running the same
// change-detection, MinLSInterval rate-limit (RFC 2328 Section 9.5), sequence
// assignment, install and flood machinery as the typed Originate* builders. body is the
// LSA payload after the 20-octet header and is used only for the "unchanged since the
// last origination" comparison, so an idempotent re-run with the same topology floods
// nothing. It exists so the OSPFv3 engine can originate the address-free Router-LSA and
// the Intra-Area-Prefix-LSA (built with ospfv3/packet) without the LSDB depending on
// the OSPFv3 wire codec.
func (d *LSDB) OriginateSelf(area types.AreaID, key types.LSAKey, body []byte, enc SelfLSAEncoder) (packet.LSAHeader, bool) {
	if enc == nil || key.AdvertisingRouter == (types.RouterID{}) {
		return packet.LSAHeader{}, false
	}
	if h, same := d.existingSelfBodyUnchanged(area, key, body); same {
		return h, false
	}
	seq, ok, purge := d.nextOwnSequence(area, key)
	if !ok {
		h, _ := d.Lookup(area, key)
		return h, false
	}
	return d.installOriginated(area, enc(seq, purge), key, purge)
}

// SelfLSARef identifies a self-originated LSA by area and key, for the stale-flush set.
type SelfLSARef struct {
	Area types.AreaID
	Key  types.LSAKey
}

// FlushStaleSelfLSAs MaxAge-flushes (RFC 2328 Section 14.1) every self-originated LSA for
// router whose LS type is in manage but whose (area, key) is not in keep. A caller that
// regenerates its full current self-LSA set passes that set as keep and the types it owns
// as manage, so a Router-LSA or Intra-Area-Prefix-LSA for an area or prefix set that
// disappeared is withdrawn from the domain instead of lingering until it ages out. manage
// scopes the sweep to the caller's own LS types, leaving self LSAs originated by other
// paths (e.g. summaries) untouched. It returns the number of LSAs flushed.
func (d *LSDB) FlushStaleSelfLSAs(router types.RouterID, manage map[types.LSType]struct{}, keep map[SelfLSARef]struct{}) int {
	d.mu.RLock()
	var stale []SelfLSARef
	for area, own := range d.own {
		for key := range own {
			if key.AdvertisingRouter != router {
				continue
			}
			if _, ok := manage[key.Type]; !ok {
				continue
			}
			if _, ok := keep[SelfLSARef{Area: area, Key: key}]; !ok {
				stale = append(stale, SelfLSARef{Area: area, Key: key})
			}
		}
	}
	d.mu.RUnlock()
	count := 0
	for _, ref := range stale {
		if d.flushSelfLSA(ref.Area, ref.Key) {
			count++
		}
	}
	return count
}

// OriginateSummary builds and installs this router's Type 3/4 Summary-LSA.
// RFC 2328 Section 12.4.3: Summary-LSAs are originated by ABRs, Type 3
// describes an IP network and Type 4 describes an ASBR. Type 4 uses a zero mask.
func (d *LSDB) OriginateSummary(area types.AreaID, router types.RouterID, opts types.Options, typ types.LSType, lsid types.LinkStateID, mask [4]byte, metric uint32) (packet.LSAHeader, bool) {
	if router == (types.RouterID{}) || (typ != types.LSTypeSummaryNetwork && typ != types.LSTypeSummaryASBR) {
		return packet.LSAHeader{}, false
	}
	key := types.LSAKey{Type: typ, LinkStateID: lsid, AdvertisingRouter: router}
	body := packet.SummaryLSA{NetworkMask: mask, TOS: 0, Metric: metric}
	if typ == types.LSTypeSummaryASBR {
		body.NetworkMask = [4]byte{}
	}
	if h, same := d.existingSelfBodyUnchanged(area, key, encodedSummaryBody(body)); same {
		return h, false
	}
	seq, ok, purge := d.nextOwnSequence(area, key)
	if !ok {
		h, _ := d.Lookup(area, key)
		return h, false
	}
	h := packet.LSAHeader{Age: 0, Options: opts, Type: typ, LinkStateID: key.LinkStateID, AdvertisingRouter: router, Sequence: seq}
	if purge {
		h.Age = types.LSAge(types.MaxAge)
	}
	return d.installOriginated(area, packet.LSA{Header: h, Summary: &body}, key, purge)
}

// OriginateExternal builds and installs this router's Type 5 AS-External-LSA into
// the AS-wide store. RFC 2328 Section 12.4.4: the Link State ID is the external
// network address, the E-bit (type2) selects the metric type, the metric is a
// 24-bit field, and the Forwarding Address / External Route Tag are carried
// unchanged. Type 5 has no area scope; sequence bookkeeping uses the backbone area
// as a fixed key while the store routes by LS type to the AS-wide store.
func (d *LSDB) OriginateExternal(router types.RouterID, network, mask [4]byte, opts types.Options, type2 bool, metric uint32, fwd [4]byte, tag uint32) (packet.LSAHeader, bool, error) {
	if router == (types.RouterID{}) {
		return packet.LSAHeader{}, false, nil
	}
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(network), AdvertisingRouter: router}
	body := packet.ExternalLSA{
		NetworkMask:      mask,
		ExternalType2:    type2,
		Metric:           metric & packet.ExternalMetricMax,
		ForwardingAddr:   fwd,
		ExternalRouteTag: tag,
	}
	if h, same := d.existingSelfBodyUnchanged(types.BackboneArea, key, encodedExternalBody(body)); same {
		return h, false, nil // idempotent: identical body already originated, no change
	}
	seq, ok, purge := d.nextOwnSequence(types.BackboneArea, key)
	if !ok {
		h, _ := d.Lookup(types.BackboneArea, key)
		return h, false, nil // rate-limited by MinLSInterval, not a failure
	}
	h := packet.LSAHeader{Age: 0, Options: opts, Type: types.LSTypeASExternal, LinkStateID: key.LinkStateID, AdvertisingRouter: router, Sequence: seq}
	if purge {
		h.Age = types.LSAge(types.MaxAge)
	}
	rh, stored := d.installOriginated(types.BackboneArea, packet.LSA{Header: h, External: &body}, key, purge)
	if !stored {
		// The Type 5 was NOT installed (AS-external store at capacity). installOriginated
		// has already logged the drop; surface it so redistribution does not count an
		// uninstalled route as injected.
		return rh, false, ErrExternalStoreFull
	}
	return rh, true, nil
}

// PurgeExternal MaxAge-purges this router's self-originated Type 5 for network
// (re-originate at LS Age MaxAge, flood, drop -- RFC 2328 Section 14), reporting
// whether a non-purged self LSA existed.
func (d *LSDB) PurgeExternal(router types.RouterID, network [4]byte) bool {
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(network), AdvertisingRouter: router}
	return d.flushSelfLSA(types.BackboneArea, key)
}

// SelfExternalCount returns the number of non-purged Type 5 AS-External-LSAs this
// router currently originates (the ze_ospf_external_lsas gauge value).
func (d *LSDB) SelfExternalCount(router types.RouterID) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := 0
	for key, e := range d.asExternal.entries {
		// The AS-wide store also holds non-external AS-scope LSAs (the RFC 7770 Router
		// Information LSA, function code 12); count only true AS-External-LSAs.
		if key.Type.ASExternal() && key.AdvertisingRouter == router && e.self && !e.purged {
			n++
		}
	}
	return n
}

// selfIsASBRLocked reports whether this router is an AS boundary router -- it originates a
// non-purged Type 5 AS-External-LSA, OR a non-purged Type 7 NSSA-LSA (an NSSA ABR default or
// redistributed NSSA external). Both make the router an ASBR and require the Router-LSA E-bit
// (RFC 2328 sec 12.4.1); without the E-bit a receiver will not compute routes from the router's
// Type 7s ("originating router is not an ASBR"). It walks the self-LSA index d.own (the small
// set this router originates) rather than every area store, so it stays O(self-LSAs) on the
// per-second origination path. AF-agnostic: the .ASExternal()/.NSSA() classifiers match both
// OSPFv2 (0x0005/0x0007) and OSPFv3 (0x4005/0x2007). Caller holds d.mu.
func (d *LSDB) selfIsASBRLocked(router types.RouterID) bool {
	// Type 5 AS-External: scan the AS-wide store directly (small). It also holds non-external
	// AS-scope LSAs now (the RFC 7770 Router Information LSA, function code 12), which do NOT
	// make the router an ASBR, so filter to true AS-External-LSAs (ai/rules/no-fabrication:
	// only a real external/NSSA LSA sets the E-bit).
	for key, e := range d.asExternal.entries {
		if key.Type.ASExternal() && key.AdvertisingRouter == router && e.self && !e.purged {
			return true
		}
	}
	// Type 7 NSSA: walk the self-LSA index d.own (the small set this router originates) rather
	// than every area store, so this stays O(self-LSAs) on the per-second origination path.
	for area, own := range d.own {
		for key := range own {
			if !key.Type.NSSA() || key.AdvertisingRouter != router {
				continue
			}
			if adb := d.areas[area]; adb != nil {
				if e := adb.entries[key]; e != nil && e.self && !e.purged {
					return true
				}
			}
		}
	}
	return false
}

// SelfIsASBR is the exported, AF-agnostic ASBR test used to drive the Router-LSA E-bit from
// both the OSPFv2 (lsdb-internal) and OSPFv3 (engine) origination paths, so the E-bit
// determination is identical across address families (AC-6: the E-bit clears when the last
// external/NSSA LSA is withdrawn).
func (d *LSDB) SelfIsASBR(router types.RouterID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.selfIsASBRLocked(router)
}

// selfOriginatesExternal is the OSPFv2 origination path's ASBR test (see selfIsASBRLocked).
func (d *LSDB) selfOriginatesExternal(router types.RouterID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.selfIsASBRLocked(router)
}

// FlushStaleSummaryLSAs prematurely ages self-originated Type 3/4 LSAs in area
// that are not present in keep. Callers pass the desired current summary set
// after re-running ABR origination.
func (d *LSDB) FlushStaleSummaryLSAs(area types.AreaID, router types.RouterID, keep map[types.LSAKey]struct{}) int {
	d.mu.RLock()
	own := d.own[area]
	stale := make([]types.LSAKey, 0, len(own))
	for key := range own {
		if key.AdvertisingRouter != router || (key.Type != types.LSTypeSummaryNetwork && key.Type != types.LSTypeSummaryASBR) {
			continue
		}
		if _, ok := keep[key]; !ok {
			stale = append(stale, key)
		}
	}
	d.mu.RUnlock()
	count := 0
	for _, key := range stale {
		if d.flushSelfLSA(area, key) {
			count++
		}
	}
	return count
}

func (d *LSDB) flushStaleNetworkLSAs(area types.AreaID, router types.RouterID, keep map[types.LSAKey]struct{}) int {
	d.mu.RLock()
	own := d.own[area]
	stale := make([]types.LSAKey, 0, len(own))
	for key := range own {
		if key.Type != types.LSTypeNetwork || key.AdvertisingRouter != router {
			continue
		}
		if _, ok := keep[key]; !ok {
			stale = append(stale, key)
		}
	}
	d.mu.RUnlock()
	count := 0
	for _, key := range stale {
		if d.flushSelfLSA(area, key) {
			count++
		}
	}
	return count
}

// flushReceivedSelfLSA flushes a self-originated LSA this router has no local record of
// originating -- typically a stale instance neighbors still flood after this router
// restarted (RFC 2328 §13.4). It re-originates the received body at MaxAge with a sequence
// greater than the received instance, so neighbors accept the flush as the newest copy and
// purge the stale instance from the routing domain.
func (d *LSDB) flushReceivedSelfLSA(area types.AreaID, lsa packet.LSA) bool {
	if lsa.Header.Age.IsMaxAge() {
		return true // already a flush in flight; normal flooding propagates it
	}
	key := lsa.Header.Key()
	body := make([]byte, len(lsa.Body))
	copy(body, lsa.Body)
	next := lsa.Header.Sequence.Next()
	if lsa.Header.Sequence.IsMax() {
		next = types.MaxSequenceNumber
	}
	h := lsa.Header
	h.Age = types.LSAge(types.MaxAge)
	h.Sequence = next
	h.Checksum = 0
	res, ok := d.install(area, packet.LSA{Header: h, Body: body}, true, false)
	if ok && res.Entry != nil {
		d.mu.Lock()
		own := d.own[area]
		if own == nil {
			own = make(map[types.LSAKey]ownRecord)
			d.own[area] = own
		}
		own[key] = ownRecord{sequence: next, last: d.now()}
		d.mu.Unlock()
		d.floodExcept("", types.RouterID{}, area, key)
		d.notifyChange(area)
	}
	return true
}

// WithdrawSelf MaxAge-flushes a self-originated area/AS-scope LSA identified by key and
// returns its flushed header. It is the ext-14 debug-inject withdraw path for OSPFv3 native
// LSAs (RFC 2328 Section 14 purge), mirroring OriginateOpaque's opaque withdraw.
func (d *LSDB) WithdrawSelf(area types.AreaID, key types.LSAKey) (packet.LSAHeader, bool) {
	ok := d.flushSelfLSA(area, key)
	h, _ := d.Lookup(area, key)
	return h, ok
}

// WithdrawLinkSelf MaxAge-flushes a self-originated link-local LSA (RFC 5340 Link-LSA scope)
// identified by key on iface. It reuses the same in-place re-stamp path as the opaque
// link-scope withdraw, valid for any self-originated link LSA.
func (d *LSDB) WithdrawLinkSelf(iface string, key types.LSAKey) (packet.LSAHeader, bool) {
	return d.flushSelfLinkOpaque(iface, key)
}

func (d *LSDB) flushSelfLSA(area types.AreaID, key types.LSAKey) bool {
	lsa, ok := d.LookupLSA(area, key)
	if !ok || lsa.Header.Age.IsMaxAge() || len(lsa.RawBytes) == 0 {
		return false
	}
	seq, ok, _ := d.nextOwnSequenceForce(area, key, true)
	if !ok {
		return false
	}
	// Re-stamp the stored bytes (Age -> MaxAge, new sequence, fresh checksum) in place
	// rather than re-encoding from the typed/Body form: re-encoding would run the
	// OSPFv2 codec, which cannot reproduce an OSPFv3 LSA's 16-bit LS Type. The Age,
	// Sequence and Checksum offsets are identical across both address families.
	raw := make([]byte, len(lsa.RawBytes))
	copy(raw, lsa.RawBytes)
	cksum, ok := packet.RefreshLSAInPlace(raw, types.LSAge(types.MaxAge), seq)
	if !ok {
		return false
	}
	h := lsa.Header
	h.Age = types.LSAge(types.MaxAge)
	h.Sequence = seq
	h.Checksum = cksum
	_, installed := d.installOriginated(area, packet.LSA{Header: h, RawBytes: raw}, key, true)
	return installed
}

func encodedRouterBody(body packet.RouterLSA) []byte {
	buf := make([]byte, body.EncodedLen())
	body.WriteTo(buf, 0)
	return buf
}

func encodedNetworkBody(body packet.NetworkLSA) []byte {
	buf := make([]byte, body.EncodedLen())
	body.WriteTo(buf, 0)
	return buf
}

func encodedSummaryBody(body packet.SummaryLSA) []byte {
	buf := make([]byte, body.EncodedLen())
	body.WriteTo(buf, 0)
	return buf
}

func encodedExternalBody(body packet.ExternalLSA) []byte {
	buf := make([]byte, body.EncodedLen())
	body.WriteTo(buf, 0)
	return buf
}

func (d *LSDB) existingSelfBodyUnchanged(area types.AreaID, key types.LSAKey, body []byte) (packet.LSAHeader, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.dbForReadLocked(area, key)
	if store == nil {
		return packet.LSAHeader{}, false
	}
	entry := store.entries[key]
	if entry == nil || !entry.self || entry.purged || len(entry.raw) < types.LSAHeaderLen {
		return packet.LSAHeader{}, false
	}
	if !bytes.Equal(entry.raw[types.LSAHeaderLen:], body) {
		return packet.LSAHeader{}, false
	}
	return entry.Header(d.now()), true
}

func (d *LSDB) nextOwnSequence(area types.AreaID, key types.LSAKey) (types.LSSequenceNumber, bool, bool) {
	return d.nextOwnSequenceForce(area, key, false)
}

// nextOwnSequenceForce is nextOwnSequence with an optional MinLSInterval bypass.
// RFC 2328 Section 9.5 rate-limits successive originations of the same LSA, but
// flushing a no-longer-reachable LSA (premature MaxAge aging, Section 14.1) must
// not be delayed -- a withdrawn route would otherwise stay advertised for up to
// MinLSInterval (R-4). Purge callers pass force=true; normal origination does not.
func (d *LSDB) nextOwnSequenceForce(area types.AreaID, key types.LSAKey, force bool) (types.LSSequenceNumber, bool, bool) {
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	own := d.own[area]
	if own == nil {
		own = make(map[types.LSAKey]ownRecord)
		d.own[area] = own
	}
	rec, exists := own[key]
	if !force && exists && !rec.last.IsZero() && now.Sub(rec.last) < d.timers.MinLSInterval {
		return rec.sequence, false, false
	}
	switch {
	case !exists || rec.sequence == 0:
		rec.sequence = types.InitialSequenceNumber
	case rec.sequence.IsMax():
		own[key] = ownRecord{sequence: rec.sequence, last: now}
		return rec.sequence, true, true
	default:
		rec.sequence = rec.sequence.Next()
	}
	rec.last = now
	own[key] = rec
	return rec.sequence, true, false
}

func (d *LSDB) installOriginated(area types.AreaID, lsa packet.LSA, key types.LSAKey, purge bool) (packet.LSAHeader, bool) {
	raw, hdr, ok := normaliseLSA(lsa)
	if !ok {
		// A self-built LSA that fails to re-encode/verify is an internal encoding bug,
		// not a runtime condition; surface it loudly rather than dropping silently.
		slog.Error("ospf lsdb: self-originated LSA failed to normalise, origination dropped",
			"lsa-type", key.Type.String(), "area", area.String())
		return packet.LSAHeader{}, false
	}
	// Hold d.mu across install + Entry read/mutation so the markPurged write (and the
	// Header read) do not race readers (SelfExternalCount, Tick) that touch the same
	// Entry fields under the lock. install/installLocked locks internally, so we use
	// installLocked with the lock already held.
	d.mu.Lock()
	res, ok := d.installLocked(area, raw, hdr, true, false)
	if !ok || res.Entry == nil {
		d.mu.Unlock()
		// installLocked rejected the install: the destination store is at capacity
		// (MaxLSAsPerArea / MaxASExternalLSAs). RFC 2328 origination must not silently
		// drop -- log it so an operator sees the store exhaustion.
		slog.Warn("ospf lsdb: self-originated LSA install rejected, store at capacity",
			"lsa-type", key.Type.String(), "area", area.String())
		return packet.LSAHeader{}, false
	}
	h := res.Entry.Header(d.now())
	if purge {
		res.Entry.markPurged(d.now())
	}
	d.mu.Unlock()
	if purge {
		d.mPurges.With(key.Type.String()).Inc()
	} else {
		d.mOriginations.With(key.Type.String()).Inc()
	}
	d.floodExcept("", types.RouterID{}, area, key)
	d.notifyChange(area)
	return h, true
}

func (d *LSDB) handleSelfReceived(area types.AreaID, lsa packet.LSA) bool {
	// RFC 3623 sec 2: while in graceful restart the restarting router MUST NOT modify or
	// flush received self-originated LSAs; it accepts them as valid (they are its pre-restart
	// LSAs, held by neighbors). Return false so the normal install path stores the LSA
	// instead of the fight-back MaxAge flush below.
	if d.selfFlushSuppressed() {
		return false
	}
	d.mu.RLock()
	self := d.selfRouter
	d.mu.RUnlock()
	if self == (types.RouterID{}) {
		return false
	}
	key := lsa.Header.Key()
	selfOriginated := key.AdvertisingRouter == self
	if !selfOriginated && key.Type == types.LSTypeNetwork {
		topology := d.topologySnapshot()
		for idx := range topology {
			if topology[idx].AreaID == area && types.LinkStateID(topology[idx].Address) == key.LinkStateID {
				selfOriginated = true
				break
			}
		}
	}
	if !selfOriginated {
		return false
	}
	local, ok := d.Lookup(area, key)
	if !ok {
		// RFC 2328 §13.4: a self-originated LSA we hold no local record of originating (a
		// stale instance neighbors still flood after this router restarted) is flushed from
		// the routing domain by premature aging, not silently dropped.
		return d.flushReceivedSelfLSA(area, lsa)
	}
	if CompareHeaders(lsa.Header, local) != Newer {
		return true
	}
	current, ok := d.LookupLSA(area, key)
	if !ok {
		return true
	}
	next := lsa.Header.Sequence.Next()
	if lsa.Header.Sequence.IsMax() {
		next = types.MaxSequenceNumber
	}
	current.RawBytes = nil
	current.Header.Sequence = next
	current.Header.Age = 0
	current.Header.Checksum = 0
	res, ok := d.install(area, current, true, false)
	if ok && res.Entry != nil {
		d.mu.Lock()
		own := d.own[area]
		if own == nil {
			own = make(map[types.LSAKey]ownRecord)
			d.own[area] = own
		}
		own[key] = ownRecord{sequence: next, last: d.now()}
		d.mu.Unlock()
		d.floodExcept("", types.RouterID{}, area, key)
		d.notifyChange(area)
	}
	return true
}

func isAreaBorderRouter(areas []types.AreaID) bool {
	return len(areas) >= 2 && slices.Contains(areas, types.BackboneArea)
}

func networkAddress(addr, mask [4]byte) [4]byte {
	return [4]byte{addr[0] & mask[0], addr[1] & mask[1], addr[2] & mask[2], addr[3] & mask[3]}
}

func compareAreaID(a, b types.AreaID) int     { return compare4([4]byte(a), [4]byte(b)) }
func compareRouterID(a, b types.RouterID) int { return compare4([4]byte(a), [4]byte(b)) }

func compare4(a, b [4]byte) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
