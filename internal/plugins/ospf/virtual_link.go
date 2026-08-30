// Design: docs/architecture/ospf/ospf-ext-7-virtual-links.md -- the engine-side virtual-link manager.
// Related: spf/transitarea.go -- the transit-area SPF resolution this consumes.
// Related: lsdb/origination.go -- the backbone Router-LSA virtual record this drives.
// RFC: rfc/short/rfc2328.md (sec 15), rfc/short/rfc5340.md (sec 4.2)
//
// A virtual link is modeled as a synthetic point-to-point interface in the backbone (Area
// 0) whose cost and next hop are computed from the transit area's intra-area SPF, not
// configured, and whose packets are routed (not link-local). This file owns the AF-neutral
// control plane: config -> SPF virtual-link requests, the SPF resolution callback -> per-link
// runtime state, and the reachable-link -> synthetic backbone interface surfaced to
// origination (which emits the Type-4 / RouterLinkTypeVirtual record and, in the transit
// area, the V-bit). The synthetic interface's ID lives in a reserved high range so it never
// collides with a real OS ifindex.

package ospf

import (
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/textbuf"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// virtualLinkNamePrefix marks synthetic virtual-interface names. A real interface name
// never starts with '*', so a synthetic virtual interface can never shadow a real one.
const virtualLinkNamePrefix = "*vlink-"

// virtualStatePointToPoint is the ISM state of a virtual link's synthetic interface (a
// point-to-point interface, no DR election).
const virtualStatePointToPoint = "point-to-point"

// virtualIfaceIDBase reserves the top of the Interface-ID space for synthetic virtual
// interfaces (OSPFv3 Router-LSA link Interface ID), well above any real OS ifindex.
const virtualIfaceIDBase uint32 = 0xF0000000

type virtualLinkKey struct {
	transit  types.AreaID
	neighbor types.RouterID
}

// virtualLinkRuntime is the engine-side state of one configured virtual link.
type virtualLinkRuntime struct {
	cfg       virtualLinkConfig
	name      string // synthetic backbone interface name (reserved namespace)
	ifaceID   uint32 // synthetic Interface ID for the OSPFv3 Router-LSA virtual record
	reachable bool   // the transit-area SPF reached the neighbor
	cost      uint16 // transit-area path cost = the virtual link's output cost
	// localAddr is the local source used to reach the neighbor: the local transit IPv4
	// interface address (OSPFv2 Router-LSA Link Data) or the local global IPv6 source.
	localAddr netip.Addr
	// neighborAddr is the neighbor's reachable unicast destination (IPv4 transit address /
	// IPv6 global address), distinct from the transit next hop.
	neighborAddr netip.Addr
	// transitNextHop is the real next hop toward the neighbor across the transit area.
	transitNextHop ospfspf.NextHop
	// iface is the synthetic backbone point-to-point interface running this virtual link's
	// ISM/NSM; nil while the link is down. Created when the neighbor becomes reachable.
	iface *ospfiface.Interface
}

func virtualLinkName(k virtualLinkKey) string {
	var tb textbuf.Buffer
	return tb.Str(virtualLinkNamePrefix).Str(k.transit.String()).Byte('-').Str(k.neighbor.String()).String()
}

// receiveTargetLocked maps a received packet to the interface that must process it. RFC
// 2328 section 15: a virtual-link packet is routed and arrives on the TRANSIT interface's
// ifindex, not a synthetic one, so it must be demuxed to the virtual interface by (source
// Router ID + backbone Area) rather than by ifindex. The predicate is deliberately narrow:
// the packet's Area is the backbone AND it did NOT arrive on a real backbone interface AND
// its source Router ID matches a reachable configured virtual link. A packet on a genuine
// backbone interface, or from any other router, falls through to the ordinary ifindex path,
// so real-interface demux (and the base Type 1-7 flooding) is undisturbed. Runs under mu.
func (e *engine) receiveTargetLocked(ifindex int, h Header) (string, *ospfiface.Interface, bool) {
	if rt := e.virtualLinkTargetLocked(ifindex, h); rt != nil {
		return rt.name, rt.iface, true
	}
	ic, ok := e.runningByIfIndexLocked(ifindex)
	if !ok {
		return "", nil, false
	}
	return ic.Name, e.interfaces[ic.Name], true
}

// virtualLinkTargetLocked returns the virtual-link runtime a received backbone packet must
// be demuxed to, or nil. A packet qualifies only when its Area is the backbone AND it
// arrived on a REAL, enrolled interface whose area is the virtual link's TRANSIT area AND
// its source Router ID matches a reachable virtual link with a live synthetic interface
// (RFC 2328 sec 15). Requiring a real transit interface rejects an unknown/non-enrolled
// ifindex and a real backbone interface (whose area is not the transit area), so base
// backbone demux is untouched. Runs under mu.
func (e *engine) virtualLinkTargetLocked(ifindex int, h Header) *virtualLinkRuntime {
	if h.AreaID != types.BackboneArea {
		return nil
	}
	ic, running := e.runningByIfIndexLocked(ifindex)
	if !running {
		return nil
	}
	// NOTE (#4): the demux keys on the neighbor Router ID plus the arrival interface's
	// transit area, never on the source address; the transit-area match disambiguates the
	// rare case of the same neighbor Router ID reached via two different transit areas.
	for key, rt := range e.virtualLinks {
		if key.neighbor == h.RouterID && key.transit == ic.AreaID && rt.reachable && rt.iface != nil {
			return rt
		}
	}
	return nil
}

// configureVirtualLinks (re)builds the virtual-link runtime map from config and hands the
// transit-area + neighbor pairs to the SPF computer so each is resolved every SPF run
// (RFC 2328 sec 16.1). Existing runtime state for an unchanged (transit, neighbor) key is
// preserved so a reconfigure of an unrelated link does not flap it.
func (e *engine) configureVirtualLinks(cfg ospfConfig) {
	requests := make([]ospfspf.VirtualLinkRequest, 0, len(cfg.VirtualLinks))
	next := make(map[virtualLinkKey]*virtualLinkRuntime, len(cfg.VirtualLinks))
	e.mu.Lock()
	old := e.virtualLinks
	idx := uint32(0)
	for _, vl := range cfg.VirtualLinks {
		key := virtualLinkKey{transit: vl.TransitArea, neighbor: vl.RemoteRouterID}
		rt := old[key]
		if rt == nil {
			rt = &virtualLinkRuntime{name: virtualLinkName(key), ifaceID: virtualIfaceIDBase + idx}
		}
		rt.cfg = vl
		next[key] = rt
		requests = append(requests, ospfspf.VirtualLinkRequest{TransitArea: vl.TransitArea, Neighbor: vl.RemoteRouterID})
		idx++
	}
	e.virtualLinks = next
	e.mu.Unlock()
	if e.spf != nil {
		e.spf.SetVirtualLinks(requests)
	}
}

// onVirtualLinksResolved is the SPF callback (RFC 2328 sec 16.1): it drives each configured
// virtual link up (reachable, with the transit cost + next hop) or down from its transit
// area's SPF result, then re-originates the backbone Router-LSA (the virtual record / V-bit
// changes with reachability) and re-runs SPF. It fires only when the resolved set changed
// (the SPF computer suppresses no-op runs), so an unchanged cost does not flap the link.
func (e *engine) onVirtualLinksResolved(results []ospfspf.VirtualNeighborResult) {
	var toStart, toStop []*virtualLinkRuntime
	changed := false
	e.mu.Lock()
	for _, r := range results {
		key := virtualLinkKey{transit: r.TransitArea, neighbor: r.Neighbor}
		rt := e.virtualLinks[key]
		if rt == nil {
			continue
		}
		wasReachable := rt.reachable
		rt.reachable = r.Reachable
		if r.Reachable {
			rt.cost = clampVirtualCost(r.Cost)
			if len(r.NextHops) > 0 {
				rt.transitNextHop = r.NextHops[0]
			}
			if e.dispatch != nil && e.dispatch.codec.IsV6() {
				// RFC 5340 sec 2.9: resolve the global source + global destination from the
				// transit area's Intra-Area-Prefix-LSAs; keep the prior addresses until both
				// are learned (re-resolved on the next transit SPF run).
				if src, dst, resolved := e.v6ResolveVirtualEndpointLocked(rt); resolved {
					rt.localAddr, rt.neighborAddr = src, dst
				}
			} else {
				rt.localAddr = e.virtualLocalAddrLocked(rt)
				rt.neighborAddr = virtualNeighborAddr(r)
			}
			if rt.iface == nil {
				toStart = append(toStart, rt)
			}
		} else if rt.iface != nil {
			toStop = append(toStop, rt)
		}
		if wasReachable != r.Reachable {
			changed = true
			e.recordVirtualAdjChangeLocked(key)
		}
	}
	e.mu.Unlock()
	// Create/destroy the synthetic interface outside the lock (Start spawns goroutines).
	for _, rt := range toStart {
		e.startVirtualInterface(rt)
	}
	for _, rt := range toStop {
		e.stopVirtualInterface(rt)
	}
	e.recordVirtualMetrics()
	if changed {
		// A virtual link coming up or down changes the backbone Router-LSA (its virtual
		// record and the transit area's V-bit) and the routing topology.
		e.originateSelfLSAs()
		e.triggerSPF(types.BackboneArea)
	}
}

// virtualNeighborAddr is the routed unicast destination for a virtual link: the neighbor's
// reachable address across the transit area. On a directly-adjacent transit (the shipped
// interop topology) the transit next hop IS the neighbor's transit/global address; a
// multi-hop transit path refines this to the neighbor's own address (its Intra-Area-Prefix
// for IPv6) under QEMU.
func virtualNeighborAddr(r ospfspf.VirtualNeighborResult) netip.Addr {
	if len(r.NextHops) == 0 {
		return netip.Addr{}
	}
	return r.NextHops[0].Addr
}

// startVirtualInterface creates and starts the synthetic backbone point-to-point interface
// for a reachable virtual link (RFC 2328 section 15): it runs the shared ISM/NSM, sends its
// packets ROUTED to the neighbor's address (via the transit egress), and inherits the
// transit area's authentication (AC-18) because its sends go out the transit egress and its
// receives are verified against the transit interface.
func (e *engine) startVirtualInterface(rt *virtualLinkRuntime) {
	e.mu.Lock()
	if rt.iface != nil {
		e.mu.Unlock()
		return
	}
	isV6 := e.dispatch != nil && e.dispatch.codec.IsV6()
	cfg := ospfiface.Config{
		Name:               rt.name,
		RouterID:           e.cfg.RouterID,
		AreaID:             types.BackboneArea,
		AreaType:           areaTypeNormal,
		NetworkType:        ospfiface.NetworkPointToPoint,
		Cost:               rt.cost,
		HelloInterval:      rt.cfg.HelloInterval,
		DeadInterval:       rt.cfg.DeadInterval,
		RetransmitInterval: rt.cfg.RetransmitInterval,
		// RFC 2328 App A.3.3: a DD over a virtual link carries Interface MTU 0, so the MTU
		// match is skipped on the synthetic interface.
		MTUIgnore:   true,
		IsV6:        isV6,
		InterfaceID: rt.ifaceID,
	}
	if !isV6 && rt.localAddr.Is4() {
		cfg.InterfaceAddress = rt.localAddr.As4()
	}
	ifc := ospfiface.New(cfg, e.virtualSender(), e.ifaceMetric)
	if isV6 {
		ifc.SetEncoder(v6Encoder{instanceID: e.cfg.InstanceID})
	}
	if e.sink != nil {
		ifc.SetEventSink(e.sink)
	}
	ifc.SetNeighborSink(nsmAdapter{table: e.neighbors, onChange: e.originateSelfLSAs, auth: e.auth})
	rt.iface = ifc
	e.interfaces[rt.name] = ifc
	if e.neighbors != nil {
		e.neighbors.ConfigureInterface(neighborInterfaceConfig(cfg, e.cfg.Opaque))
	}
	e.mu.Unlock()
	ifc.Start()
}

// stopVirtualInterface tears the synthetic interface down when the neighbor becomes
// unreachable, removing its neighbor-table entries and releasing its adjacency.
func (e *engine) stopVirtualInterface(rt *virtualLinkRuntime) {
	e.mu.Lock()
	ifc := rt.iface
	rt.iface = nil
	delete(e.interfaces, rt.name)
	if e.neighbors != nil {
		e.neighbors.DeleteInterface(rt.name)
	}
	e.mu.Unlock()
	if ifc != nil {
		ifc.Stop()
	}
}

// stopVirtualInterfaces stops every synthetic virtual interface (engine shutdown).
func (e *engine) stopVirtualInterfaces() {
	e.mu.Lock()
	ifaces := make([]*ospfiface.Interface, 0, len(e.virtualLinks))
	for _, rt := range e.virtualLinks {
		if rt.iface != nil {
			ifaces = append(ifaces, rt.iface)
			rt.iface = nil
		}
	}
	e.mu.Unlock()
	for _, ifc := range ifaces {
		ifc.Stop()
	}
}

// virtualSender returns the routed sender used by the synthetic virtual interfaces and the
// neighbor table: it recognizes the reserved virtual-interface names and routes their
// packets (SendPacketRouted) to the resolved neighbor address, delegating every other name
// to the ordinary link-local transport.
func (e *engine) virtualSender() virtualAwareSender { return virtualAwareSender{eng: e} }

// virtualAwareSender routes virtual-link packets and passes real-interface packets through.
type virtualAwareSender struct{ eng *engine }

func (s virtualAwareSender) SendPacket(name string, dst netip.Addr, payload []byte) error {
	if strings.HasPrefix(name, virtualLinkNamePrefix) {
		return s.eng.sendVirtualLink(name, payload)
	}
	if s.eng.transport == nil {
		return nil
	}
	return s.eng.transport.SendPacket(name, dst, payload)
}

func (s virtualAwareSender) JoinAllDRouters(name string) error {
	if strings.HasPrefix(name, virtualLinkNamePrefix) || s.eng.transport == nil {
		return nil // a virtual link is point-to-point: no DR multicast group
	}
	return s.eng.transport.JoinAllDRouters(name)
}

func (s virtualAwareSender) LeaveAllDRouters(name string) error {
	if strings.HasPrefix(name, virtualLinkNamePrefix) || s.eng.transport == nil {
		return nil
	}
	return s.eng.transport.LeaveAllDRouters(name)
}

// sendVirtualLink routes an outgoing virtual-link packet to the neighbor's address over the
// transit egress with a routed TTL/hop-limit (RFC 2328 section 8.1 / RFC 5340 section 2.9).
// A packet whose destination or egress is not yet resolved is dropped (the adjacency retries).
func (e *engine) sendVirtualLink(name string, payload []byte) error {
	e.mu.Lock()
	var rt *virtualLinkRuntime
	for _, r := range e.virtualLinks {
		if r.name == name {
			rt = r
			break
		}
	}
	isV6 := e.dispatch != nil && e.dispatch.codec.IsV6()
	// The IPv6 routed send requires a valid GLOBAL source (RFC 5340 sec 2.9); the IPv4 send
	// lets the kernel pick the source, so src may be unset.
	if rt == nil || e.transport == nil || !rt.neighborAddr.IsValid() || rt.transitNextHop.Interface == "" || (isV6 && !rt.localAddr.Is6()) {
		e.mu.Unlock()
		return nil
	}
	egress, dst, src := rt.transitNextHop.Interface, rt.neighborAddr, rt.localAddr
	e.mu.Unlock()
	return e.transport.SendPacketRouted(egress, dst, src, payload)
}

// clampVirtualCost maps the 64-bit SPF cost to the 16-bit interface cost the origination
// topology carries; an at/over-ceiling cost means the neighbor is effectively unreachable.
func clampVirtualCost(cost uint64) uint16 {
	if cost >= uint64(^uint16(0)) {
		return ^uint16(0) - 1
	}
	if cost == 0 {
		return 1
	}
	return uint16(cost)
}

// virtualLocalAddrLocked resolves the OSPFv2 local source address used to reach the
// neighbor: the local transit IPv4 interface address (the Router-LSA Link Data of the
// Type-4 record). The OSPFv3 global source is resolved separately (virtuallink_v6.go).
// Runs under mu.
func (e *engine) virtualLocalAddrLocked(rt *virtualLinkRuntime) netip.Addr {
	if rt.transitNextHop.Interface == "" {
		return netip.Addr{}
	}
	local := interfaceIPv4Address(rt.transitNextHop.Interface)
	if local == ([4]byte{}) {
		return netip.Addr{}
	}
	return netip.AddrFrom4(local)
}

// virtualLinkTopology surfaces each reachable virtual link as a synthetic backbone
// point-to-point interface for origination (RFC 2328 sec 15: a virtual link belongs to the
// backbone). Its neighbor list reflects the synthetic interface's adjacency in the neighbor
// table, so origination emits the Type-4 / RouterLinkTypeVirtual record only once the
// adjacency is Full. A Down (unreachable) link contributes nothing.
func (e *engine) virtualLinkTopology() []ospflsdb.InterfaceInfo {
	// The runtime map is guarded by e.mu, but the *virtualLinkRuntime objects it holds are
	// mutated in place under e.mu by onVirtualLinksResolved (cost, localAddr, cfg on
	// reconfigure). Snapshotting pointers and reading their fields after the unlock would
	// race those writes, so snapshot VALUES here: the copies below share nothing mutable
	// (virtualLinkConfig is a pure-value struct) and are safe to read lock-free.
	type virtualLinkView struct {
		name      string
		cfg       virtualLinkConfig
		cost      uint16
		ifaceID   uint32
		localAddr netip.Addr
	}
	e.mu.Lock()
	views := make([]virtualLinkView, 0, len(e.virtualLinks))
	for _, rt := range e.virtualLinks {
		if !rt.reachable {
			continue
		}
		views = append(views, virtualLinkView{
			name:      rt.name,
			cfg:       rt.cfg,
			cost:      rt.cost,
			ifaceID:   rt.ifaceID,
			localAddr: rt.localAddr,
		})
	}
	rid := e.cfg.RouterID
	isV6 := e.dispatch != nil && e.dispatch.codec.IsV6()
	e.mu.Unlock()

	out := make([]ospflsdb.InterfaceInfo, 0, len(views))
	for i := range views {
		v := &views[i]
		info := ospflsdb.InterfaceInfo{
			Name:               v.name,
			AreaID:             types.BackboneArea,
			AreaType:           areaTypeNormal,
			NetworkType:        ospflsdb.NetworkVirtual,
			State:              virtualStatePointToPoint,
			VirtualTransitArea: v.cfg.TransitArea,
			Cost:               v.cost,
			RouterID:           rid,
			InterfaceID:        v.ifaceID,
			RetransmitInterval: v.cfg.RetransmitInterval,
			TransmitDelay:      v.cfg.TransmitDelay,
			Neighbors:          e.virtualNeighbors(v.name),
			IsV6:               isV6,
		}
		if !isV6 && v.localAddr.Is4() {
			info.Address = v.localAddr.As4()
		}
		out = append(out, info)
	}
	return out
}

// virtualNeighbors reads the synthetic interface's neighbor adjacency from the neighbor
// table, so origination sees the neighbor Full only when the routed adjacency has formed.
// It takes the (immutable) synthetic interface name rather than a *virtualLinkRuntime so no
// runtime pointer escapes e.mu: virtualLinkTopology calls it lock-free from a value snapshot.
func (e *engine) virtualNeighbors(name string) []ospflsdb.NeighborInfo {
	if e.neighbors == nil {
		return nil
	}
	flood := e.neighbors.FloodNeighbors(name)
	out := make([]ospflsdb.NeighborInfo, 0, len(flood))
	for _, n := range flood {
		out = append(out, ospflsdb.NeighborInfo{RouterID: n.RouterID, Address: n.Address, State: n.State, InterfaceID: n.InterfaceID})
	}
	return out
}

// registerVirtualLinkMetrics registers the virtual-link metric series (spec-ospf-ext-7). The
// IPv4 family uses the ze_ospf_virtual_* prefix; the IPv6 family the ze_ospfv3_virtual_link*
// prefix. Called from setMetrics; a nil registry leaves the nop metrics.
func (e *engine) registerVirtualLinkMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		e.mVirtualLinks = reg.GaugeVec("ze_ospfv3_virtual_links", "Current OSPFv3 virtual links by transit area and state.", []string{labelTransitArea, labelState})
		e.mVirtualCost = reg.GaugeVec("ze_ospfv3_virtual_link_cost", "Current OSPFv3 virtual-link cost (transit-area path cost) by transit area and remote router.", []string{labelTransitArea, "remote_router_id"})
		e.mVirtualAdjChgs = reg.CounterVec("ze_ospfv3_virtual_link_reresolves_total", "Total OSPFv3 virtual-link reachability changes by transit area.", []string{labelTransitArea})
		return
	}
	e.mVirtualLinks = reg.GaugeVec("ze_ospf_virtual_links", "Current OSPF virtual links by transit area and state.", []string{labelTransitArea, labelState})
	e.mVirtualCost = reg.GaugeVec("ze_ospf_virtual_link_cost", "Current OSPF virtual-link cost (transit-area path cost) by transit area and neighbor.", []string{labelTransitArea, "neighbor"})
	e.mVirtualAdjChgs = reg.CounterVec("ze_ospf_virtual_link_adjacency_changes_total", "Total OSPF virtual-link adjacency (reachability) changes by transit area and neighbor.", []string{labelTransitArea, "neighbor"})
}

func (e *engine) recordVirtualAdjChangeLocked(key virtualLinkKey) {
	if e.mVirtualAdjChgs == nil {
		return
	}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		e.mVirtualAdjChgs.With(key.transit.String()).Inc()
		return
	}
	e.mVirtualAdjChgs.With(key.transit.String(), key.neighbor.String()).Inc()
}

func (e *engine) recordVirtualMetrics() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.mVirtualLinks == nil {
		return
	}
	up := make(map[types.AreaID]float64)
	down := make(map[types.AreaID]float64)
	for key, rt := range e.virtualLinks {
		if rt.reachable {
			up[key.transit]++
			if e.mVirtualCost != nil {
				e.mVirtualCost.With(key.transit.String(), key.neighbor.String()).Set(float64(rt.cost))
			}
		} else {
			down[key.transit]++
		}
	}
	for area, n := range up {
		e.mVirtualLinks.With(area.String(), "up").Set(n)
	}
	for area, n := range down {
		e.mVirtualLinks.With(area.String(), ospflsdb.InterfaceStateDown).Set(n)
	}
}

// virtualLinkSnapshot renders the configured virtual links for `show ospf virtual-links`.
func (e *engine) virtualLinkSnapshot() []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]any, 0, len(e.virtualLinks))
	for key, rt := range e.virtualLinks {
		state := ospflsdb.InterfaceStateDown
		if rt.reachable {
			state = virtualStatePointToPoint
		}
		nextHop := ""
		if rt.transitNextHop.Addr.IsValid() {
			nextHop = rt.transitNextHop.Addr.String()
		}
		neighborAddr := ""
		if rt.neighborAddr.IsValid() {
			neighborAddr = rt.neighborAddr.String()
		}
		out = append(out, virtualLinkRow{
			TransitArea:     key.transit.String(),
			RemoteRouterID:  key.neighbor.String(),
			State:           state,
			Cost:            rt.cost,
			NextHop:         nextHop,
			Interface:       rt.transitNextHop.Interface,
			NeighborAddress: neighborAddr,
		})
	}
	return out
}

// virtualLinkRow is one `show ospf virtual-links` row.
type virtualLinkRow struct {
	TransitArea     string `json:"transit-area"`
	RemoteRouterID  string `json:"remote-router-id"`
	State           string `json:"state"`
	Cost            uint16 `json:"cost"`
	NextHop         string `json:"next-hop,omitempty"`
	Interface       string `json:"interface,omitempty"`
	NeighborAddress string `json:"neighbor-address,omitempty"`
}
