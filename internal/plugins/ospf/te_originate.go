// Design: plan/learned/1030-ospf-ext-2-traffic-engineering.md -- TE LSA origination from config.
// RFC: rfc/short/rfc3630.md sec 2.4/2.5 (Router-Address + Link TLV), sec 3 (rate-limit);
// rfc/short/rfc5392.md sec 3/4 (inter-AS proxy advertisement, Type 10/11 scope).
//
// OnOriginate is a PULL model: each self-LSA pass this returns the FULL desired set of TE
// LSAs. The carrier assigns sequence numbers, rate-limits to MinLSInterval, installs, and
// floods; an unchanged body floods nothing (idempotent, AC-12). To promptly withdraw a
// link that has gone down or been de-configured (AC-13), the originator remembers the
// instances it emitted last pass and returns a Withdraw for any that are no longer desired.
// Type-1 (RFC 3630) builds a Router-Address LSA (Instance 0) plus one Link LSA per intra
// TE link from config + the live interface snapshot; type-6 (RFC 5392) builds one inter-AS
// Link LSA per configured inter-as link, with no Link ID and the per-link Type 10/11 scope.

package ospf

import (
	"sort"
	"sync"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// teOrigKey identifies one originated instance for withdraw tracking: its target area,
// flooding scope, and Opaque ID (Instance).
type teOrigKey struct {
	area  types.AreaID
	scope OpaqueScope
	inst  uint32
}

// teOriginator holds the TE origination state: a stable interface-name -> Instance mapping
// (so an interface keeps its Instance across passes, RFC 3630 sec 2.2 gives the Instance no
// topological meaning) and the previously-originated instance sets for withdraw diffing.
type teOriginator struct {
	mu        sync.Mutex
	intraInst map[string]uint32
	interInst map[string]uint32
	nextInst  uint32
	prevT1    map[teOrigKey]bool
	prevT6    map[teOrigKey]bool
	topology  func() []ospflsdb.InterfaceInfo
}

func newTEOriginator(topology func() []ospflsdb.InterfaceInfo) *teOriginator {
	return &teOriginator{
		intraInst: make(map[string]uint32),
		interInst: make(map[string]uint32),
		prevT1:    make(map[teOrigKey]bool),
		prevT6:    make(map[teOrigKey]bool),
		topology:  topology,
	}
}

// setTopology overrides the live-interface source (tests inject a canned snapshot).
func (o *teOriginator) setTopology(fn func() []ospflsdb.InterfaceInfo) {
	o.mu.Lock()
	o.topology = fn
	o.mu.Unlock()
}

// instanceFor returns the stable Instance for an interface's TE link (allocating a fresh
// one on first use). Router-Address uses the reserved Instance 0, never allocated here.
func (o *teOriginator) instanceFor(name string, interAS bool) uint32 {
	m := o.intraInst
	if interAS {
		m = o.interInst
	}
	if id, ok := m[name]; ok {
		return id
	}
	o.nextInst++
	m[name] = o.nextInst
	return o.nextInst
}

// teOriginateType1 builds the RFC 3630 Router-Address + per-link TE LSAs for the intra-area
// TE topology, plus withdraws for links no longer present. Called by the carrier on each
// self-LSA pass (RFC 5250 sec 3).
func (e *engine) teOriginateType1(router types.RouterID) []opaqueOrigination {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()

	o := e.teOrig
	o.mu.Lock()
	defer o.mu.Unlock()

	topo := topologyByName(o.topology)
	teCfg := teInterfaceConfigByName(cfg)

	var out []opaqueOrigination
	desired := make(map[teOrigKey]bool)
	teAreas := make(map[types.AreaID]bool)

	// One Link LSA per intra-area TE link that has a usable Link ID (RFC 3630 sec 2.4.2).
	names := sortedNames(teCfg)
	for _, name := range names {
		ic := teCfg[name]
		if !ic.TE.active() || ic.TE.InterAS != nil {
			continue
		}
		info, up := topo[name]
		if !up {
			continue
		}
		link, ok := buildIntraTELink(router, ic, info)
		if !ok {
			continue // R-4: refuse a Link TLV without a usable Link Type + Link ID.
		}
		inst := o.instanceFor(name, false)
		key := teOrigKey{area: ic.AreaID, scope: OpaqueScopeArea, inst: inst}
		lsa := packet.TELSA{IsLink: true, Link: link}
		out = append(out, opaqueOrigination{OpaqueID: inst, Area: ic.AreaID, Body: lsa.Encode()})
		desired[key] = true
		teAreas[ic.AreaID] = true
		// Install this router's own TE link into its own TED: a self-originated LSA is
		// short-circuited before the reception path, so it never reaches teOnReceive; without
		// this the local links are absent from `show ospf te-database` and from a future rsvpte
		// CSPF (which needs the full topology, own links included; FRR carries them too). Self
		// links are always usable.
		e.upsertSelfTED(router, ic.AreaID, OpaqueScopeArea, packet.TEOpaqueType, inst, lsa)
	}

	// RFC 3630 sec 2.4.1: one Router-Address TE LSA (Instance 0) per area that has a TE link,
	// so a multi-area TE router advertises its Router Address into EVERY TE area, not only the
	// lowest-numbered -- a receiver in another area needs the originator's stable address to
	// resolve that area's links. When TE is configured but no intra-area TE link is active yet,
	// fall back to a single backbone Router-Address so the address is still advertised.
	if anyTEActive(teCfg) {
		ra := cfg.TERouterAddress
		if !cfg.HasTERouterAddress {
			ra = [4]byte(router)
		}
		areas := sortedAreaIDs(teAreas)
		if len(areas) == 0 {
			areas = []types.AreaID{types.BackboneArea}
		}
		raLSA := packet.TELSA{IsRouterAddress: true, RouterAddress: ra}
		body := raLSA.Encode()
		for _, area := range areas {
			key := teOrigKey{area: area, scope: OpaqueScopeArea, inst: 0}
			out = append(out, opaqueOrigination{OpaqueID: 0, Area: area, Body: body})
			desired[key] = true
			e.upsertSelfTED(router, area, OpaqueScopeArea, packet.TEOpaqueType, 0, raLSA)
		}
	}

	wd := withdrawDiff(o.prevT1, desired)
	out = append(out, wd...)
	// Keep the self portion of the TED in step with origination: drop the link instances that
	// vanished. withdrawSelfTED skips Instance 0 (the Router-Address is one per-router TED entry
	// the per-area diff cannot manage); the self Router-Address is removed only when no TE is
	// active at all.
	e.withdrawSelfTED(router, packet.TEOpaqueType, wd)
	if e.ted != nil && !anyTEActive(teCfg) {
		e.ted.withdraw(router, packet.TEOpaqueType, 0)
	}
	o.prevT1 = desired
	return out
}

// teOriginateType6 builds the RFC 5392 inter-AS TE Link LSAs (Opaque type 6) for configured
// inter-as links, plus withdraws for links no longer present. Inter-AS links carry no OSPF
// adjacency (sec 4): the advertisement is proxied purely from config.
func (e *engine) teOriginateType6(router types.RouterID) []opaqueOrigination {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()

	o := e.teOrig
	o.mu.Lock()
	defer o.mu.Unlock()

	topo := topologyByName(o.topology)
	teCfg := teInterfaceConfigByName(cfg)

	var out []opaqueOrigination
	desired := make(map[teOrigKey]bool)
	for _, name := range sortedNames(teCfg) {
		ic := teCfg[name]
		if ic.TE == nil || ic.TE.InterAS == nil {
			continue
		}
		link := buildInterASTELink(ic, topo[name])
		inst := o.instanceFor(name, true)
		scope := ic.TE.InterAS.Scope
		// A Type-11 (AS-wide) inter-AS LSA uses the backbone as its sequence-key area
		// (mirroring the carrier's Type-11 handling); Type-10 uses the link's area.
		area := ic.AreaID
		if scope == OpaqueScopeAS {
			area = types.BackboneArea
		}
		key := teOrigKey{area: area, scope: scope, inst: inst}
		lsa := packet.TELSA{IsLink: true, Link: link}
		out = append(out, opaqueOrigination{OpaqueID: inst, Area: area, Scope: scope, Body: lsa.Encode()})
		desired[key] = true
		// Install this router's own inter-AS TE link into its own TED (self LSAs bypass the
		// reception path), so `show ospf te-database` and a future rsvpte CSPF see local links.
		e.upsertSelfTED(router, area, scope, packet.InterAsTEOpaqueType, inst, lsa)
	}

	wd := withdrawDiff(o.prevT6, desired)
	out = append(out, wd...)
	e.withdrawSelfTED(router, packet.InterAsTEOpaqueType, wd)
	o.prevT6 = desired
	return out
}

// upsertSelfTED installs a self-originated TE LSA into this router's own TED so `show ospf
// te-database` and a future rsvpte CSPF observe the local links; a self-originated LSA is
// short-circuited before teOnReceive, so it never enters the TED through the reception path.
// Self entries are always reachable. It is a no-op before the TED exists.
func (e *engine) upsertSelfTED(router types.RouterID, area types.AreaID, scope OpaqueScope, opaqueType uint8, inst uint32, lsa packet.TELSA) {
	if e.ted == nil {
		return
	}
	e.ted.applyLSA(router, area, scope, opaqueType, inst, lsa, true)
}

// withdrawSelfTED removes the self TED entries for the withdrawn originations of opaqueType
// (link-down / de-config). It skips Instance 0: the Router-Address is a single per-router TED
// entry (its key carries no area), so a per-area withdraw must not remove it while another
// area still advertises it -- teOriginateType1 drops it explicitly when TE goes fully inactive.
func (e *engine) withdrawSelfTED(router types.RouterID, opaqueType uint8, withdrawn []opaqueOrigination) {
	if e.ted == nil {
		return
	}
	for _, w := range withdrawn {
		if w.OpaqueID == 0 {
			continue
		}
		e.ted.withdraw(router, opaqueType, w.OpaqueID)
	}
}

// sortedAreaIDs returns the areas of m in ascending 4-octet order (deterministic origination).
func sortedAreaIDs(m map[types.AreaID]bool) []types.AreaID {
	out := make([]types.AreaID, 0, len(m))
	for a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return lessAreaID(out[i], out[j]) })
	return out
}

// buildIntraTELink builds an RFC 3630 Link TLV from config and the live interface snapshot.
// ok is false when the mandatory Link Type / Link ID cannot be filled (no Full neighbor on
// a point-to-point link, or no known DR on a multi-access link), so origination refuses to
// emit a malformed Link TLV (R-4).
func buildIntraTELink(self types.RouterID, ic interfaceConfig, info ospflsdb.InterfaceInfo) (packet.TELink, bool) {
	if info.State == ospflsdb.InterfaceStateDown {
		return packet.TELink{}, false
	}
	link := packet.TELink{HasLinkType: true}
	if ic.NetworkType == networkPointToPoint {
		link.LinkType = packet.TELinkTypePointToPoint
		nbr, ok := firstFullNeighbor(info)
		if !ok {
			return packet.TELink{}, false
		}
		// RFC 3630 sec 2.5.2: for a point-to-point link the Link ID is the neighbor Router ID.
		link.HasLinkID = true
		link.LinkID = [4]byte(nbr.RouterID)
		if nbr.Address.Is4() {
			link.RemoteIPs = [][4]byte{nbr.Address.As4()}
		}
	} else {
		link.LinkType = packet.TELinkTypeMultiAccess
		// RFC 3630 sec 2.5.2: for a multi-access link the Link ID is the DR interface address.
		drAddr, ok := drInterfaceAddress(self, info)
		if !ok {
			return packet.TELink{}, false
		}
		link.HasLinkID = true
		link.LinkID = drAddr
	}
	if info.Address != ([4]byte{}) {
		link.LocalIPs = [][4]byte{info.Address}
	}
	applyTELinkAttributes(&link, ic)
	return link, true
}

// buildInterASTELink builds an RFC 5392 inter-AS Link TLV: point-to-point Link Type, NO
// Link ID (sec 3.2.1 prohibited), the Remote AS Number + Remote ASBR ID(s), and the local
// interface address when known. The normal TE attributes are carried too (sec 4).
func buildInterASTELink(ic interfaceConfig, info ospflsdb.InterfaceInfo) packet.TELink {
	ia := ic.TE.InterAS
	link := packet.TELink{HasLinkType: true, LinkType: packet.TELinkTypePointToPoint}
	link.HasRemoteAS = ia.HasRemoteAS
	link.RemoteAS = ia.RemoteAS
	link.HasRemoteASBRv4 = ia.HasRemoteASBRv4
	link.RemoteASBRv4 = ia.RemoteASBRv4
	link.HasRemoteASBRv6 = ia.HasRemoteASBRv6
	link.RemoteASBRv6 = ia.RemoteASBRv6
	if info.Address != ([4]byte{}) {
		link.LocalIPs = [][4]byte{info.Address}
	}
	applyTELinkAttributes(&link, ic)
	return link
}

// applyTELinkAttributes fills the RFC 3630 sec 2.5 link attributes from config. The TE
// metric defaults to the standard OSPF interface cost when unset (RFC 3630 sec 2.5.5, AC-19)
// but an explicit te-metric is used verbatim and does not change the cost. The Maximum
// Reservable Bandwidth defaults to the Maximum Bandwidth (sec 2.5.7), and the eight
// Unreserved values initialize to the Maximum Reservable (sec 2.5.8).
func applyTELinkAttributes(link *packet.TELink, ic interfaceConfig) {
	te := ic.TE
	link.HasTEMetric = true
	if te.HasMetric {
		link.TEMetric = te.Metric
	} else {
		link.TEMetric = uint32(interfaceCost(ic))
	}
	if te.HasMaxBandwidth {
		link.HasMaxBandwidth = true
		link.MaxBandwidth = te.MaxBandwidth
	}
	reservable, hasReservable := te.MaxReservable, te.HasMaxReservable
	if !hasReservable && te.HasMaxBandwidth {
		reservable, hasReservable = te.MaxBandwidth, true
	}
	if hasReservable {
		link.HasMaxReservable = true
		link.MaxReservable = reservable
		link.HasUnreserved = true
		for i := range packet.TEUnreservedPriorities {
			link.Unreserved[i] = reservable
		}
	}
	if te.HasAdminGroup {
		link.HasAdminGroup = true
		link.AdminGroup = te.AdminGroup
	}
}

// withdrawDiff returns a Withdraw origination for every instance originated last pass but
// not in the desired set (AC-13: prompt MaxAge-flush on link-down / de-config).
func withdrawDiff(prev, desired map[teOrigKey]bool) []opaqueOrigination {
	var out []opaqueOrigination
	for key := range prev {
		if desired[key] {
			continue
		}
		out = append(out, opaqueOrigination{OpaqueID: key.inst, Area: key.area, Scope: key.scope, Withdraw: true})
	}
	return out
}

// topologyByName snapshots the live interfaces keyed by name.
func topologyByName(fn func() []ospflsdb.InterfaceInfo) map[string]ospflsdb.InterfaceInfo {
	out := map[string]ospflsdb.InterfaceInfo{}
	if fn == nil {
		return out
	}
	infos := fn()
	for i := range infos {
		out[infos[i].Name] = infos[i]
	}
	return out
}

// teInterfaceConfigByName returns the configured interfaces (which carry the TE block) by name.
func teInterfaceConfigByName(cfg ospfConfig) map[string]interfaceConfig {
	out := make(map[string]interfaceConfig, len(cfg.Interfaces))
	for i := range cfg.Interfaces {
		out[cfg.Interfaces[i].Name] = cfg.Interfaces[i]
	}
	return out
}

func sortedNames(m map[string]interfaceConfig) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func anyTEActive(m map[string]interfaceConfig) bool {
	for name := range m {
		if m[name].TE.active() {
			return true
		}
	}
	return false
}

// firstFullNeighbor returns the first Full neighbor on an interface (a usable Link ID
// source for a point-to-point TE link).
func firstFullNeighbor(info ospflsdb.InterfaceInfo) (ospflsdb.NeighborInfo, bool) {
	for _, n := range info.Neighbors {
		if n.State == ospflsdb.NeighborStateFull {
			return n, true
		}
	}
	return ospflsdb.NeighborInfo{}, false
}

// drInterfaceAddress returns the DR interface address for a multi-access link: this
// router's own address when it is the DR, otherwise the DR neighbor's source address.
func drInterfaceAddress(self types.RouterID, info ospflsdb.InterfaceInfo) ([4]byte, bool) {
	if info.DR == (types.RouterID{}) {
		return [4]byte{}, false
	}
	if info.DR == self {
		if info.Address == ([4]byte{}) {
			return [4]byte{}, false
		}
		return info.Address, true
	}
	for _, n := range info.Neighbors {
		if n.RouterID == info.DR && n.Address.Is4() {
			return n.Address.As4(), true
		}
	}
	return [4]byte{}, false
}

// interfaceCost returns the effective OSPF output cost (default 1 when unset), the value the
// TE metric falls back to (RFC 3630 sec 2.5.5).
func interfaceCost(ic interfaceConfig) uint16 {
	if ic.HasCost {
		return ic.Cost
	}
	return 1
}

// lessAreaID orders two Area IDs by their 4-octet big-endian value.
func lessAreaID(a, b types.AreaID) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
