// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPF database export.
// Related: lsdb/native_view.go -- consistent, owned LSDB views.
package ospf

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	v3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	v3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
	"github.com/ze-software/ze/pkg/ze"
)

var bgplsSnapshots = linkstateevents.RegisterSource(Namespace)
var bgplsGeneration atomic.Uint64

// bgplsSource serializes capture, publication and withdrawal. Safe for concurrent
// use. startBGPLS MUST be paired with stopBGPLS before the engine is released.
// Nothing in a published snapshot aliases mutable LSDB storage.
type bgplsSource struct {
	mu          sync.Mutex
	bus         ze.EventBus
	unsubscribe func()
	domains     []linkstateevents.Domain
	unreachable map[linkstateevents.Domain]map[bgplsNodeKey]struct{}
	stopped     bool
	worker      atomic.Pointer[bgplsWorker]
}

type bgplsWorker struct {
	wake chan struct{}
	stop chan struct{}
	done chan struct{}
}

// requestBGPLS is safe while holding engine.mu or LSDB locks. The one-slot
// queue coalesces mutations; the worker always captures the current full DB.
func (e *engine) requestBGPLS() {
	worker := e.bgpls.worker.Load()
	if worker == nil {
		return
	}
	select {
	case worker.wake <- struct{}{}:
	default:
	}
}

func (e *engine) runBGPLS(worker *bgplsWorker) {
	defer close(worker.done)
	// The engine owns this worker until stopBGPLS closes stop and joins done.
	for {
		select {
		case <-worker.stop:
			return
		case <-worker.wake:
			e.publishBGPLS()
		}
	}
}

func (e *engine) startBGPLS(bus ze.EventBus) {
	if bus == nil {
		return
	}
	e.bgpls.mu.Lock()
	defer e.bgpls.mu.Unlock()
	if e.bgpls.stopped || e.bgpls.bus != nil {
		return
	}
	e.bgpls.bus = bus
	worker := &bgplsWorker{wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	e.bgpls.worker.Store(worker)
	go e.runBGPLS(worker)
	e.bgpls.unsubscribe = linkstateevents.Request.Subscribe(bus, e.publishBGPLS)
	e.publishBGPLSLocked()
}

func (e *engine) publishBGPLS() {
	e.bgpls.mu.Lock()
	defer e.bgpls.mu.Unlock()
	if e.bgpls.bus == nil || e.bgpls.stopped {
		return
	}
	e.publishBGPLSLocked()
}

// publishBGPLSLocked MUST be called with bgpls.mu held. Holding the publication
// lock before capture prevents a delayed callback from restoring an older view.
func (e *engine) publishBGPLSLocked() {
	e.mu.Lock()
	protocol := linkstateevents.OSPFv2
	if e.dispatch.codec.IsV6() {
		protocol = linkstateevents.OSPFv3
	}
	instance := uint64(e.cfg.InstanceID)
	areas := make([]types.AreaID, 0, len(e.areas))
	for area := range e.areas {
		areas = append(areas, area)
	}
	views := e.lsdb.AllLSAViews()
	var reachability ospfspf.ReachabilitySnapshot
	if e.spf != nil {
		reachability = e.spf.Reachability()
	}
	e.mu.Unlock()
	current := make(map[uint32]*bgplsArea, len(areas))
	for _, area := range areas {
		id := binary.BigEndian.Uint32(area[:])
		current[id] = newBGPLSArea(protocol, instance, id, e.af)
		current[id].configured = true
	}
	// Correlation indexes see exactly the same live, configured database as
	// emitted objects; retired area records must not resurrect an identity.
	views = slices.DeleteFunc(views, func(view ospflsdb.NativeLSAView) bool {
		if view.Age >= types.MaxAge {
			return true
		}
		if bgplsASWide(view.Type) {
			return false
		}
		area := current[binary.BigEndian.Uint32(view.Area[:])]
		return area == nil || !area.configured
	})
	// Decode base LSAs before extensions so route types and the two directions
	// of numbered links are available when attaching extension attributes.
	for pass := range 2 {
		for i := range views {
			view := &views[i]
			extended := view.Type.IsOpaque() && protocol == linkstateevents.OSPFv2
			if protocol == linkstateevents.OSPFv3 {
				extended = view.Type&0x1fff >= 12
			}
			if extended != (pass == 1) {
				continue
			}
			id := binary.BigEndian.Uint32(view.Area[:])
			if bgplsASWide(view.Type) {
				id = 0
			}
			area := current[id]
			if area == nil {
				area = newBGPLSArea(protocol, instance, id, e.af)
				current[id] = area
			}
			if protocol == linkstateevents.OSPFv2 {
				area.v2(view, views)
			} else {
				area.v3(view)
			}
		}
	}
	for _, old := range e.bgpls.domains {
		area := current[old.Area]
		if area != nil && area.snapshot.Domain == old {
			continue
		}
		e.emitBGPLS(&linkstateevents.Snapshot{Domain: old})
		delete(e.bgpls.unreachable, old)
	}
	e.bgpls.domains = e.bgpls.domains[:0]
	areaIDs := make([]uint32, 0, len(current))
	for id := range current {
		areaIDs = append(areaIDs, id)
	}
	slices.Sort(areaIDs)
	routerIDs := make(map[[4]byte][]linkstateevents.TLV)
	for _, area := range current {
		for _, node := range area.snapshot.Nodes {
			if len(node.ID.RouterID) != 4 {
				continue
			}
			var router [4]byte
			copy(router[:], node.ID.RouterID)
			for _, attr := range node.Attributes {
				if attr.Type == 1028 || attr.Type == 1029 {
					routerIDs[router] = append(routerIDs[router], attr)
				}
			}
		}
	}
	for _, id := range areaIDs {
		area := current[id]
		area.resolveV3Links(views)
		area.linkAttributes(routerIDs)
		domain := area.snapshot.Domain
		unreachable := area.markUnreachable(reachability, e.bgpls.unreachable[domain])
		if len(unreachable) == 0 {
			delete(e.bgpls.unreachable, domain)
		} else {
			if e.bgpls.unreachable == nil {
				e.bgpls.unreachable = make(map[linkstateevents.Domain]map[bgplsNodeKey]struct{})
			}
			e.bgpls.unreachable[domain] = unreachable
		}
		for _, missing := range area.unresolvedInterAS {
			e.log.Warn("ospf: withholding BGP-LS inter-AS link without a unique 4-octet remote Router-ID",
				"area", id, "advertising-router", missing.router.String(), "link-state-id", missing.id.String(),
				"remote-as", missing.remote.asn, "remote-asbr-ipv6", netip.AddrFrom16(missing.remote.address).String())
		}
		e.emitBGPLS(&area.snapshot)
		e.bgpls.domains = append(e.bgpls.domains, area.snapshot.Domain)
	}
}

func (e *engine) emitBGPLS(snapshot *linkstateevents.Snapshot) {
	snapshot.Generation = bgplsGeneration.Add(1)
	if _, err := bgplsSnapshots.Emit(e.bgpls.bus, snapshot); err != nil {
		e.log.Warn("ospf: link-state snapshot delivery failed", "error", err)
	}
}

// stopBGPLS MUST be called for an engine whose startBGPLS was called. The stop
// flag also rejects callbacks already dispatched by the replay subscription.
func (e *engine) stopBGPLS() {
	e.bgpls.mu.Lock()
	e.bgpls.stopped = true
	worker := e.bgpls.worker.Swap(nil)
	if worker != nil {
		close(worker.stop)
	}
	unsubscribe := e.bgpls.unsubscribe
	e.bgpls.unsubscribe = nil
	if e.bgpls.bus != nil {
		for _, domain := range e.bgpls.domains {
			e.emitBGPLS(&linkstateevents.Snapshot{Domain: domain})
		}
	}
	e.bgpls.domains = nil
	e.bgpls.unreachable = nil
	e.bgpls.bus = nil
	e.bgpls.mu.Unlock()
	if unsubscribe != nil {
		unsubscribe()
	}
	if worker != nil {
		<-worker.done
	}
}

type bgplsNodeKey struct {
	router [8]byte
	length uint8
	area   bool
	asn    uint32
}

type bgplsPrefixKey struct {
	node     bgplsNodeKey
	prefix   netip.Prefix
	topology uint16
	route    uint8
}

type bgplsLinkKey struct {
	local    bgplsNodeKey
	remote   bgplsNodeKey
	localID  uint32
	remoteID uint32
	address  netip.Addr
	topology uint16
}

type bgplsASIPv6 struct {
	asn     uint32
	address [16]byte
}
type bgplsUnresolvedInterAS struct {
	router types.RouterID
	id     types.LinkStateID
	remote bgplsASIPv6
}

type bgplsArea struct {
	snapshot          linkstateevents.Snapshot
	af                addressFamily
	configured        bool
	nodes             map[bgplsNodeKey]int
	links             map[bgplsLinkKey]int
	prefixes          map[bgplsPrefixKey]int
	v2Routers         map[types.RouterID]*packet.RouterLSA
	v2Networks        map[types.LinkStateID]*ospflsdb.NativeLSAView
	v2Reverse         map[[8]byte][][4]byte
	v2TE              map[*ospflsdb.NativeLSAView]packet.TELSA
	v2RemoteIPv6      map[bgplsASIPv6]types.RouterID
	unresolvedInterAS []bgplsUnresolvedInterAS
}

func newBGPLSArea(protocol linkstateevents.Protocol, instance uint64, area uint32, af addressFamily) *bgplsArea {
	return &bgplsArea{
		snapshot: linkstateevents.Snapshot{Domain: linkstateevents.Domain{
			Protocol: protocol, Instance: instance, Area: area, Identifier: instance,
		}},
		af:       af,
		nodes:    make(map[bgplsNodeKey]int),
		links:    make(map[bgplsLinkKey]int),
		prefixes: make(map[bgplsPrefixKey]int),
	}
}

func bgplsASWide(t types.LSType) bool { return t.ASWide() || t == types.LSTypeOpaqueAS }

// indexV2 decodes each Router-LSA once per replacement, rather than scanning
// and decoding the entire LSDB for every point-to-point adjacency.
func (a *bgplsArea) indexV2(views []ospflsdb.NativeLSAView) {
	if a.v2Routers != nil {
		return
	}
	a.v2Routers = make(map[types.RouterID]*packet.RouterLSA)
	a.v2Networks = make(map[types.LinkStateID]*ospflsdb.NativeLSAView)
	a.v2Reverse = make(map[[8]byte][][4]byte)
	a.v2TE = make(map[*ospflsdb.NativeLSAView]packet.TELSA)
	for i := range views {
		v := &views[i]
		if v.Age >= types.MaxAge {
			continue
		}
		if v.Type.IsOpaque() && (v.LinkStateID[0] == packet.TEOpaqueType || v.LinkStateID[0] == packet.InterAsTEOpaqueType) {
			te, err := packet.DecodeTELSA(v.Body)
			if err == nil {
				a.v2TE[v] = te
				if te.IsLink && te.Link.HasRemoteAS && te.Link.HasRemoteASBRv4 && te.Link.HasRemoteASBRv6 {
					if a.v2RemoteIPv6 == nil {
						a.v2RemoteIPv6 = make(map[bgplsASIPv6]types.RouterID)
					}
					key := bgplsASIPv6{asn: te.Link.RemoteAS, address: te.Link.RemoteASBRv6}
					router := types.RouterID(te.Link.RemoteASBRv4)
					if old, found := a.v2RemoteIPv6[key]; found && old != router {
						router = types.RouterID{}
					}
					a.v2RemoteIPv6[key] = router
				}
			}
		}
		if binary.BigEndian.Uint32(v.Area[:]) != a.snapshot.Domain.Area {
			continue
		}
		switch v.Type {
		case types.LSTypeRouter:
			router, err := packet.DecodeRouterLSA(v.Body)
			if err != nil {
				continue
			}
			a.v2Routers[v.AdvertisingRouter] = &router
			for _, link := range router.Links {
				if link.Type != packet.RouterLinkTypeP2P {
					continue
				}
				var key [8]byte
				copy(key[:4], v.AdvertisingRouter[:])
				copy(key[4:], link.LinkID[:])
				a.v2Reverse[key] = append(a.v2Reverse[key], link.LinkData)
			}
		case types.LSTypeNetwork:
			if _, err := packet.DecodeNetworkLSA(v.Body); err == nil {
				a.v2Networks[v.LinkStateID] = v
			}
		default:
			// Other LSA types carry no topology this index reads.
		}
	}
}

func (a *bgplsArea) routerOptions(id linkstateevents.NodeID, options uint32) {
	node := a.node(id)
	for i := range node.Attributes {
		if node.Attributes[i].Type != 1024 {
			continue
		}
		if options&uint32(v3types.OptR) != 0 {
			node.Attributes[i].Value[0] |= 0x08
		}
		if options&uint32(v3types.OptV6) != 0 {
			node.Attributes[i].Value[0] |= 0x04
		}
	}
}

func (a *bgplsArea) nodeID(router types.RouterID, pseudo *types.LinkStateID, area bool) linkstateevents.NodeID {
	id := linkstateevents.NodeID{Area: a.snapshot.Domain.Area, HasArea: area, RouterID: router[:]}
	if pseudo != nil {
		id.RouterID = make([]byte, 8)
		copy(id.RouterID, router[:])
		copy(id.RouterID[4:], pseudo[:])
	}
	return id
}

func bgplsNodeIdentity(id *linkstateevents.NodeID) bgplsNodeKey {
	key := bgplsNodeKey{length: uint8(len(id.RouterID)), area: id.HasArea, asn: id.ASN}
	copy(key.router[:], id.RouterID)
	return key
}

func (a *bgplsArea) node(id linkstateevents.NodeID) *linkstateevents.Node {
	key := bgplsNodeIdentity(&id)
	if index, ok := a.nodes[key]; ok {
		return &a.snapshot.Nodes[index]
	}
	a.nodes[key] = len(a.snapshot.Nodes)
	a.snapshot.Nodes = append(a.snapshot.Nodes, linkstateevents.Node{ID: id})
	return &a.snapshot.Nodes[len(a.snapshot.Nodes)-1]
}

func (a *bgplsArea) link(link *linkstateevents.Link, topology uint16) *linkstateevents.Link {
	key := bgplsLinkKey{local: bgplsNodeIdentity(&link.Local), remote: bgplsNodeIdentity(&link.Remote),
		localID: link.LocalID, remoteID: link.RemoteID, topology: topology}
	if len(link.LocalAddresses) != 0 {
		key.address = link.LocalAddresses[0]
	}
	if index, ok := a.links[key]; ok {
		return &a.snapshot.Links[index]
	}
	a.links[key] = len(a.snapshot.Links)
	a.snapshot.Links = append(a.snapshot.Links, *link)
	stored := &a.snapshot.Links[len(a.snapshot.Links)-1]
	stored.Topologies = []uint16{topology}
	return stored
}

func (a *bgplsArea) prefix(node linkstateevents.NodeID, prefix netip.Prefix, topology uint16, route uint8) *linkstateevents.Prefix {
	key := bgplsPrefixKey{node: bgplsNodeIdentity(&node), prefix: prefix, topology: topology, route: route}
	if index, ok := a.prefixes[key]; ok {
		return &a.snapshot.Prefixes[index]
	}
	a.prefixes[key] = len(a.snapshot.Prefixes)
	a.snapshot.Prefixes = append(a.snapshot.Prefixes, linkstateevents.Prefix{
		Node: node, Prefix: prefix, Topology: topology, RouteType: route,
	})
	return &a.snapshot.Prefixes[len(a.snapshot.Prefixes)-1]
}

func bgplsAttribute(attrs *[]linkstateevents.TLV, typ uint16, value []byte) {
	for i := range *attrs {
		if (*attrs)[i].Type == typ {
			(*attrs)[i].Value = value
			return
		}
	}
	*attrs = append(*attrs, linkstateevents.TLV{Type: typ, Value: value})
}

func bgplsMetric(attrs *[]linkstateevents.TLV, typ uint16, metric uint32) {
	var value [4]byte
	binary.BigEndian.PutUint32(value[:], metric)
	// RFC 9552 Section 5.3.2.4: "OSPF link metrics have a length of 2 octets."
	if typ == 1095 {
		bgplsAttribute(attrs, typ, value[2:])
		return
	}
	bgplsAttribute(attrs, typ, value[:])
}

func bgplsV4Prefix(id, mask [4]byte) (netip.Prefix, bool) {
	value := binary.BigEndian.Uint32(mask[:])
	length := bits.OnesCount32(value)
	if value != ^uint32(0)<<(32-length) {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(netip.AddrFrom4(id), length).Masked(), true
}

func (a *bgplsArea) routerFlags(id linkstateevents.NodeID, flags byte) {
	var value byte
	if flags&packet.RouterFlagB != 0 {
		value |= 0x10
	}
	if flags&packet.RouterFlagE != 0 {
		value |= 0x20
	}
	bgplsAttribute(&a.node(id).Attributes, 1024, []byte{value})
}

func (a *bgplsArea) v2(v *ospflsdb.NativeLSAView, views []ospflsdb.NativeLSAView) {
	a.indexV2(views)
	id := a.nodeID(v.AdvertisingRouter, nil, !bgplsASWide(v.Type))
	switch v.Type {
	case types.LSTypeRouter:
		router := a.v2Routers[v.AdvertisingRouter]
		if router == nil {
			return
		}
		a.routerFlags(id, router.Flags)
		offset := 4
		for _, native := range router.Links {
			a.v2RouterLink(v, &native, 0, uint32(native.Metric), views)
			for i := range int(native.TOSCount) {
				tos := v.Body[offset+12+i*4 : offset+16+i*4]
				a.v2RouterLink(v, &native, uint16(tos[0]&0x7f), uint32(binary.BigEndian.Uint16(tos[2:])), views)
			}
			offset += 12 + int(native.TOSCount)*4
		}
	case types.LSTypeNetwork:
		network, err := packet.DecodeNetworkLSA(v.Body)
		if err != nil {
			return
		}
		pseudo := a.nodeID(v.AdvertisingRouter, &v.LinkStateID, true)
		a.node(pseudo)
		for _, router := range network.AttachedRouters {
			link := a.link(&linkstateevents.Link{Local: pseudo, Remote: a.nodeID(router, nil, true)}, 0)
			bgplsMetric(&link.Attributes, 1095, 0)
		}
		if prefix, ok := bgplsV4Prefix(v.LinkStateID, network.NetworkMask); ok {
			bgplsMetric(&a.prefix(pseudo, prefix, 0, 1).Attributes, 1155, 0)
		}
	case types.LSTypeSummaryNetwork:
		summary, err := packet.DecodeSummaryLSA(v.Body)
		if err != nil {
			return
		}
		if prefix, ok := bgplsV4Prefix(v.LinkStateID, summary.NetworkMask); ok {
			bgplsMetric(&a.prefix(id, prefix, uint16(summary.TOS&0x7f), 2).Attributes, 1155, summary.Metric)
		}
	case types.LSTypeASExternal, types.LSTypeNSSA:
		external, err := packet.DecodeExternalLSA(v.Body)
		if err != nil {
			return
		}
		prefix, ok := bgplsV4Prefix(v.LinkStateID, external.NetworkMask)
		if !ok {
			return
		}
		route := bgplsExternalRoute(v.Type.NSSA(), external.ExternalType2)
		p := a.prefix(id, prefix, 0, route)
		bgplsMetric(&p.Attributes, 1155, external.Metric)
		bgplsMetric(&p.Attributes, 1153, external.ExternalRouteTag)
		if external.ForwardingAddr != [4]byte{} {
			bgplsAttribute(&p.Attributes, 1156, external.ForwardingAddr[:])
		}
	case types.LSTypeOpaqueArea, types.LSTypeOpaqueAS, types.LSTypeOpaqueLink:
		switch v.LinkStateID[0] {
		case packet.RIOpaqueType:
			a.routerInformation(v, id)
		case packet.TEOpaqueType, packet.InterAsTEOpaqueType:
			a.trafficEngineering(v, id, views)
		case packet.ExtPrefixOpaqueType:
			a.extendedV2Prefix(v, id, views)
		case packet.ExtLinkOpaqueType:
			a.extendedV2Link(v, views)
		}
	default:
		// Other LSA types carry no BGP-LS topology.
	}
}

func bgplsExternalRoute(nssa, type2 bool) uint8 {
	route := uint8(3)
	if nssa {
		route = 5
	}
	if type2 {
		route++
	}
	return route
}

func (a *bgplsArea) v2Link(v *ospflsdb.NativeLSAView, native *packet.RouterLink, views []ospflsdb.NativeLSAView) (linkstateevents.Link, bool) {
	a.indexV2(views)
	link := linkstateevents.Link{Local: a.nodeID(v.AdvertisingRouter, nil, true)}
	switch native.Type {
	case packet.RouterLinkTypeP2P:
		link.Remote = a.nodeID(types.RouterID(native.LinkID), nil, true)
	case packet.RouterLinkTypeTransit:
		network := a.v2Networks[native.LinkID]
		if network == nil {
			return linkstateevents.Link{}, false
		}
		link.Remote = a.nodeID(network.AdvertisingRouter, &network.LinkStateID, true)
	default:
		// RFC 9552 Section 5.7 leaves virtual-link export unspecified.
		return linkstateevents.Link{}, false
	}
	local := netip.AddrFrom4(native.LinkData)
	if native.LinkData[0] == 0 {
		link.HasLinkIDs = true
		link.LocalID = binary.BigEndian.Uint32(native.LinkData[:])
	} else {
		link.LocalAddresses = []netip.Addr{local}
	}
	if native.Type == packet.RouterLinkTypeP2P {
		var key [8]byte
		copy(key[:4], native.LinkID[:])
		copy(key[4:], v.AdvertisingRouter[:])
		matches := a.v2Reverse[key]
		// Parallel reverse links need TE interface addresses to disambiguate.
		if len(matches) == 1 {
			if link.HasLinkIDs {
				link.RemoteID = binary.BigEndian.Uint32(matches[0][:])
			} else {
				link.RemoteAddresses = []netip.Addr{netip.AddrFrom4(matches[0])}
			}
		}
	}
	return link, true
}

func (a *bgplsArea) v2RouterLink(v *ospflsdb.NativeLSAView, native *packet.RouterLink, topology uint16, metric uint32, views []ospflsdb.NativeLSAView) {
	if native.Type == packet.RouterLinkTypeStub {
		if prefix, ok := bgplsV4Prefix(native.LinkID, native.LinkData); ok {
			bgplsMetric(&a.prefix(a.nodeID(v.AdvertisingRouter, nil, true), prefix, topology, 1).Attributes, 1155, metric)
		}
		return
	}
	link, ok := a.v2Link(v, native, views)
	if !ok {
		return
	}
	bgplsMetric(&a.link(&link, topology).Attributes, 1095, metric)
}

func (a *bgplsArea) v3(v *ospflsdb.NativeLSAView) {
	id := a.nodeID(v.AdvertisingRouter, nil, !bgplsASWide(v.Type))
	lsa := v3packet.LSA{Body: v.Body}
	switch v3types.LSType(v.Type) {
	case v3types.LSTypeRouter:
		router, err := lsa.DecodeRouter()
		if err != nil {
			return
		}
		a.routerFlags(id, router.Flags)
		a.routerOptions(id, uint32(router.Options))
		for _, native := range router.Links {
			a.v3RouterLink(id, &native)
		}
	case v3types.LSTypeNetwork:
		network, err := lsa.DecodeNetwork()
		if err != nil {
			return
		}
		pseudo := a.nodeID(v.AdvertisingRouter, &v.LinkStateID, true)
		a.node(pseudo)
		for _, router := range network.AttachedRouters {
			bgplsMetric(&a.link(&linkstateevents.Link{Local: pseudo, Remote: a.nodeID(types.RouterID(router), nil, true)}, 0).Attributes, 1095, 0)
		}
	case v3types.LSTypeIntraAreaPrefix:
		intra, err := lsa.DecodeIntraAreaPrefix()
		if err != nil {
			return
		}
		id = a.nodeID(types.RouterID(intra.ReferencedAdvRouter), nil, true)
		if intra.ReferencedLSType == v3types.LSTypeNetwork {
			pseudo := types.LinkStateID(intra.ReferencedLinkStateID)
			id = a.nodeID(types.RouterID(intra.ReferencedAdvRouter), &pseudo, true)
		}
		for _, native := range intra.Prefixes {
			a.v3Prefix(id, native, 1, uint32(native.Field16))
		}
	case v3types.LSTypeInterAreaPrefix:
		inter, err := lsa.DecodeInterAreaPrefix()
		if err != nil {
			return
		}
		a.v3Prefix(id, inter.Prefix, 2, inter.Metric)
	case v3types.LSTypeASExternal, v3types.LSTypeNSSA:
		external, err := lsa.DecodeExternal()
		if err != nil {
			return
		}
		p := a.v3Prefix(id, external.Prefix, bgplsExternalRoute(v.Type.NSSA(), external.ExternalType2), external.Metric)
		if p == nil {
			return
		}
		if external.HasRouteTag {
			bgplsMetric(&p.Attributes, 1153, external.ExternalRouteTag)
		}
		if external.HasForwardingAddr {
			address := v6ForwardingAddr(external.ForwardingAddr, a.af)
			bgplsAttribute(&p.Attributes, 1156, address.AsSlice())
		}
	case v3types.LSTypeERouter, v3types.LSTypeENetwork, v3types.LSTypeEIntraAreaPrefix,
		v3types.LSTypeEInterAreaPrefix, v3types.LSTypeEASExternal, v3types.LSTypeEType7:
		a.extendedV3(v, id)
	default:
		if v3types.LSType(v.Type)&0x1FFF == v3types.RIFunctionCode {
			a.routerInformation(v, id)
		}
	}
}

func (a *bgplsArea) v3RouterLink(id linkstateevents.NodeID, native *v3packet.RouterLink) *linkstateevents.Link {
	if native.Type != v3packet.RouterLinkTypeP2P && native.Type != v3packet.RouterLinkTypeTransit {
		return nil
	}
	remote := a.nodeID(types.RouterID(native.NeighborRouterID), nil, true)
	if native.Type == v3packet.RouterLinkTypeTransit {
		var pseudo types.LinkStateID
		binary.BigEndian.PutUint32(pseudo[:], uint32(native.NeighborInterfaceID))
		remote = a.nodeID(types.RouterID(native.NeighborRouterID), &pseudo, true)
	}
	link := a.link(&linkstateevents.Link{Local: id, Remote: remote,
		LocalID: uint32(native.InterfaceID), RemoteID: uint32(native.NeighborInterfaceID), HasLinkIDs: true}, 0)
	bgplsMetric(&link.Attributes, 1095, uint32(native.Metric))
	return link
}

func (a *bgplsArea) v3Prefix(id linkstateevents.NodeID, native v3packet.Prefix, route uint8, metric uint32) *linkstateevents.Prefix {
	prefix, ok := v6PrefixToNetip(native, a.af)
	if !ok {
		return nil
	}
	p := a.prefix(id, prefix, 0, route)
	bgplsMetric(&p.Attributes, 1155, metric)
	var flags byte
	if native.Options&v3types.OptPrefixNU != 0 {
		flags |= 0x40
	}
	if native.Options&v3types.OptPrefixLA != 0 {
		flags |= 0x20
	}
	if native.Options&v3types.OptPrefixP != 0 {
		flags |= 0x10
	}
	bgplsAttribute(&p.Attributes, 1152, []byte{flags})
	bgplsAttribute(&p.Attributes, 1170, []byte{byte(native.Options)})
	return p
}

func (a *bgplsArea) routerInformation(v *ospflsdb.NativeLSAView, id linkstateevents.NodeID) {
	tlvs, err := packet.DecodeRITLVStream(v.Body)
	if err != nil {
		return
	}
	node := a.node(id)
	for _, tlv := range tlvs {
		switch tlv.Type {
		case sr.V4TypeSRAlgorithm:
			bgplsAttribute(&node.Attributes, 1035, tlv.Value)
		case sr.V4TypeSRGB, sr.V4TypeSRLB:
			r, err := sr.DecodeRangeValue(tlv.Value)
			if err != nil {
				continue
			}
			typ := uint16(1034)
			if tlv.Type == sr.V4TypeSRLB {
				typ = 1036
			}
			// RFC 9085 Sections 2.1.2/2.1.4: flags for OSPF "MUST be set to 0".
			// Value: flags@0, reserved@1, range@2..4, SID/Label TLV@5..11.
			value := []byte{0, 0, byte(r.Size >> 16), byte(r.Size >> 8), byte(r.Size),
				4, 0x89, 0, 3, byte(r.Base >> 16), byte(r.Base >> 8), byte(r.Base)}
			merged := false
			for i := range node.Attributes {
				if node.Attributes[i].Type == typ {
					node.Attributes[i].Value = append(node.Attributes[i].Value, value[2:]...)
					merged = true
					break
				}
			}
			if !merged {
				bgplsAttribute(&node.Attributes, typ, value)
			}
		case sr.V4TypeSRMS:
			preference, err := sr.DecodeSRMSValue(tlv.Value)
			if err == nil {
				bgplsAttribute(&node.Attributes, 1037, []byte{preference})
			}
		case 7: // RFC 5642 Dynamic Hostname.
			bgplsAttribute(&node.Attributes, 1026, tlv.Value)
		default:
			// RFC 9552 Section 5.3.1.5: "MUST NOT be used to advertise TLVs
			// other than those in the OSPF Router Information (RI) LSA".
			node.Opaque = append(node.Opaque, bgplsOpaque(linkstateevents.OSPFRouterInformation, tlv.Type, tlv.Value))
		}
	}
}

func bgplsOpaque(source linkstateevents.Provenance, typ uint16, value []byte) linkstateevents.Opaque {
	return linkstateevents.Opaque{Source: source, Value: packet.EncodeRITLVs([]packet.RITLV{{Type: typ, Value: value}})}
}

func (a *bgplsArea) trafficEngineering(v *ospflsdb.NativeLSAView, id linkstateevents.NodeID, views []ospflsdb.NativeLSAView) {
	te, ok := a.v2TE[v]
	if !ok {
		return
	}
	if te.IsRouterAddress {
		bgplsAddAttribute(&a.node(id).Attributes, 1028, te.RouterAddress[:])
		return
	}
	if !te.IsLink {
		return
	}
	var link linkstateevents.Link
	if te.Link.IsInterAS() {
		router := types.RouterID(te.Link.RemoteASBRv4)
		if !te.Link.HasRemoteASBRv4 {
			key := bgplsASIPv6{asn: te.Link.RemoteAS, address: te.Link.RemoteASBRv6}
			router = a.v2RemoteIPv6[key]
			if router == (types.RouterID{}) {
				a.unresolvedInterAS = append(a.unresolvedInterAS, bgplsUnresolvedInterAS{router: v.AdvertisingRouter, id: v.LinkStateID, remote: key})
				return
			}
		}
		link.Local = id
		link.Remote = a.nodeID(router, nil, false)
		link.Remote.ASN = te.Link.RemoteAS
		bgplsAddAttribute(&link.Attributes, 1030, router[:])
		if te.Link.HasRemoteASBRv6 {
			bgplsAddAttribute(&link.Attributes, 1031, te.Link.RemoteASBRv6[:])
		}
	} else {
		if !te.Link.HasLinkID || !te.Link.HasLinkType {
			return
		}
		native := packet.RouterLink{LinkID: types.LinkStateID(te.Link.LinkID), Type: te.Link.LinkType}
		if len(te.Link.LocalIPs) != 0 {
			native.LinkData = te.Link.LocalIPs[0]
		}
		var ok bool
		link, ok = a.v2Link(v, &native, views)
		if !ok {
			return
		}
	}
	if len(te.Link.LocalIPs) != 0 {
		link.LocalAddresses = make([]netip.Addr, len(te.Link.LocalIPs))
		for i, ip := range te.Link.LocalIPs {
			link.LocalAddresses[i] = netip.AddrFrom4(ip)
		}
	}
	if len(te.Link.RemoteIPs) != 0 {
		link.RemoteAddresses = make([]netip.Addr, len(te.Link.RemoteIPs))
		for i, ip := range te.Link.RemoteIPs {
			link.RemoteAddresses[i] = netip.AddrFrom4(ip)
		}
	}
	out := a.link(&link, 0)
	for _, attr := range link.Attributes {
		bgplsAddAttribute(&out.Attributes, attr.Type, attr.Value)
	}
	if len(link.LocalAddresses) != 0 {
		out.LocalAddresses = link.LocalAddresses
	}
	if len(link.RemoteAddresses) != 0 {
		out.RemoteAddresses = link.RemoteAddresses
	}
	tlvs, err := packet.DecodeRITLVStream(v.Body)
	if err != nil {
		return
	}
	for _, tlv := range tlvs {
		if tlv.Type != packet.TETLVLink {
			continue
		}
		subs, err := packet.DecodeRITLVStream(tlv.Value)
		if err != nil {
			continue
		}
		for _, sub := range subs {
			var typ uint16
			switch sub.Type {
			case packet.TESubTEMetric:
				typ = 1092
			case packet.TESubMaxBandwidth:
				typ = 1089
			case packet.TESubMaxReservableBW:
				typ = 1090
			case packet.TESubUnreservedBW:
				typ = 1091
			case packet.TESubAdminGroup:
				typ = 1088
			}
			if typ != 0 {
				bgplsAttribute(&out.Attributes, typ, sub.Value)
			}
			// Unknown TE TLVs cannot enter Opaque Link: RFC 9552 5.3.2.6
			// permits only the Extended Link Opaque LSA for OSPFv2.
		}
	}
}

func (a *bgplsArea) extendedV2Link(v *ospflsdb.NativeLSAView, views []ospflsdb.NativeLSAView) {
	ext, err := packet.DecodeExtLinkLSA(v.Body)
	if err != nil || !ext.HasLink {
		return
	}
	native := packet.RouterLink{Type: ext.Link.LinkType, LinkID: ext.Link.LinkID, LinkData: ext.Link.LinkData}
	link, ok := a.v2Link(v, &native, views)
	if !ok {
		return
	}
	topologies := []uint16{0}
	var opaque []linkstateevents.Opaque
	for _, sub := range ext.Link.SubTLVs {
		if sub.Type == sr.V4TypeAdjSID || sub.Type == sr.V4TypeLANAdjSID {
			sid, err := bgplsAdjSID(sub.Type, sub.Value, false)
			if err != nil {
				continue
			}
			topology := uint16(sid.MTID)
			target := a.link(&link, topology)
			target.Attributes = append(target.Attributes, bgplsAdjAttribute(&sid, sub.Value[0]))
			if !slices.Contains(topologies, topology) {
				topologies = append(topologies, topology)
			}
		} else {
			opaque = append(opaque, bgplsOpaque(linkstateevents.OSPFv2ExtendedLink, sub.Type, sub.Value))
		}
	}
	for _, topology := range topologies {
		target := a.link(&link, topology)
		target.Opaque = append(target.Opaque, opaque...)
	}
}

func bgplsAdjSID(typ uint16, value []byte, v3 bool) (sr.AdjSID, error) {
	if v3 {
		if typ == sr.V6TypeLANAdjSID {
			return sr.DecodeLANAdjSIDValueV6(value)
		}
		return sr.DecodeAdjSIDValueV6(value)
	}
	if typ == sr.V4TypeLANAdjSID {
		return sr.DecodeLANAdjSIDValue(value)
	}
	return sr.DecodeAdjSIDValue(value)
}

func bgplsAdjAttribute(sid *sr.AdjSID, flags byte) linkstateevents.TLV {
	width := 4
	if sid.IsLabel {
		width = 3
	}
	length := 4 + width
	typ := uint16(1099)
	if sid.IsLAN {
		length += 4
		typ = 1100
	}
	value := make([]byte, length)
	value[0], value[1] = flags, sid.Weight
	offset := 4
	if sid.IsLAN {
		copy(value[4:8], sid.NeighborID[:])
		offset = 8
	}
	bgplsSIDValue(value[offset:], sid.Index, sid.Label, sid.IsLabel)
	return linkstateevents.TLV{Type: typ, Value: value}
}

func bgplsSIDValue(value []byte, index, label uint32, isLabel bool) {
	if isLabel {
		value[0], value[1], value[2] = byte(label>>16), byte(label>>8), byte(label)
		return
	}
	binary.BigEndian.PutUint32(value, index)
}

func bgplsPrefixAttribute(sid *sr.PrefixSID, flags byte) linkstateevents.TLV {
	length := 8
	if sid.IsLabel {
		length = 7
	}
	value := make([]byte, length)
	value[0], value[1] = flags, sid.Algorithm
	bgplsSIDValue(value[4:], sid.Index, sid.Label, sid.IsLabel)
	return linkstateevents.TLV{Type: 1158, Value: value}
}

func (a *bgplsArea) extendedV2Prefix(v *ospflsdb.NativeLSAView, id linkstateevents.NodeID, views []ospflsdb.NativeLSAView) {
	ext, err := packet.DecodeExtPrefixLSA(v.Body)
	if err != nil {
		return
	}
	for _, native := range ext.Prefixes {
		if native.AF != packet.ExtPrefixAFIPv4Unicast || native.PrefixLength > 32 {
			continue
		}
		prefix := netip.PrefixFrom(netip.AddrFrom4(native.AddressPrefix), int(native.PrefixLength)).Masked()
		routes := a.v2PrefixRouteTypes(id, prefix, native.RouteType, views)
		var opaque []linkstateevents.Opaque
		for _, sub := range native.SubTLVs {
			if sub.Type != sr.V4TypePrefixSID {
				opaque = append(opaque, bgplsOpaque(linkstateevents.OSPFv2ExtendedPrefix, sub.Type, sub.Value))
			}
		}
		for _, route := range routes {
			a.prefix(id, prefix, 0, route)
			for _, sub := range native.SubTLVs {
				if sub.Type != sr.V4TypePrefixSID {
					continue
				}
				sid, err := sr.DecodePrefixSIDValue(sub.Value)
				if err != nil {
					continue
				}
				p := a.prefix(id, prefix, uint16(sid.MTID), route)
				p.Attributes = append(p.Attributes, bgplsPrefixAttribute(&sid, sub.Value[0]))
			}
			node := bgplsNodeIdentity(&id)
			for key, index := range a.prefixes {
				if key.node != node || key.prefix != prefix || key.route != route {
					continue
				}
				p := &a.snapshot.Prefixes[index]
				// RFC 9085 2.3.2: OSPF prefix attribute flags are carried unmodified.
				bgplsAttribute(&p.Attributes, 1170, []byte{native.Flags})
				p.Opaque = append(p.Opaque, opaque...)
			}
		}
	}
	// RFC 8665 range values have no exported decoder; the container decoder
	// above validates framing, then the existing Prefix-SID codec owns values.
	for _, r := range ext.Ranges {
		if len(r.Value) < 12 || r.Value[0] > 32 || r.Value[1] != 0 {
			continue
		}
		var address [4]byte
		copy(address[:], r.Value[8:12])
		prefix := netip.PrefixFrom(netip.AddrFrom4(address), int(r.Value[0])).Masked()
		subs, err := packet.DecodeRITLVStream(r.Value[12:])
		if err != nil {
			continue
		}
		for _, sub := range subs {
			if sub.Type != sr.V4TypePrefixSID {
				continue
			}
			sid, err := sr.DecodePrefixSIDValue(sub.Value)
			if err != nil {
				continue
			}
			a.prefixRange(id, prefix, &sid, sub.Value[0], r.Value[4], binary.BigEndian.Uint16(r.Value[2:4]))
		}
	}
}

func (a *bgplsArea) v2PrefixRouteTypes(id linkstateevents.NodeID, prefix netip.Prefix, native uint8, views []ospflsdb.NativeLSAView) []uint8 {
	if native == packet.ExtRouteTypeIntraArea {
		return []uint8{1}
	}
	if native == packet.ExtRouteTypeInterArea {
		return []uint8{2}
	}
	var routes []uint8
	for key := range a.prefixes {
		if key.route == 0 || key.prefix != prefix || key.node.router != bgplsNodeIdentity(&id).router {
			continue
		}
		if native == packet.ExtRouteTypeASExternal && key.route != 3 && key.route != 4 {
			continue
		}
		if native == packet.ExtRouteTypeNSSAExternal && key.route != 5 && key.route != 6 {
			continue
		}
		if !slices.Contains(routes, key.route) {
			routes = append(routes, key.route)
		}
	}
	// RFC 9552 Section 5.2.3.1: the Type-1/Type-2 distinction for an
	// Extended Prefix "can be determined by checking the OSPFv2 External
	// or NSSA External LSA for the prefix". The base NSSA LSA may be in
	// another area bucket than an AS-scoped Extended Prefix Opaque LSA.
	for i := range views {
		v := &views[i]
		if v.Age >= types.MaxAge || !bytes.Equal(v.AdvertisingRouter[:], id.RouterID) {
			continue
		}
		if v.Type != types.LSTypeASExternal && v.Type != types.LSTypeNSSA {
			continue
		}
		if native != 0 && types.LSType(native) != v.Type {
			continue
		}
		external, err := packet.DecodeExternalLSA(v.Body)
		if err != nil {
			continue
		}
		base, ok := bgplsV4Prefix(v.LinkStateID, external.NetworkMask)
		if !ok || base != prefix {
			continue
		}
		route := bgplsExternalRoute(v.Type.NSSA(), external.ExternalType2)
		if !slices.Contains(routes, route) {
			routes = append(routes, route)
		}
	}
	if len(routes) == 0 {
		return []uint8{0}
	}
	slices.Sort(routes)
	return routes
}

func (a *bgplsArea) prefixRange(id linkstateevents.NodeID, prefix netip.Prefix, sid *sr.PrefixSID, sidFlags, rangeFlags byte, size uint16) {
	sub := bgplsPrefixAttribute(sid, sidFlags)
	value := make([]byte, 8+len(sub.Value))
	value[0] = rangeFlags
	binary.BigEndian.PutUint16(value[2:4], size)
	binary.BigEndian.PutUint16(value[4:6], sub.Type)
	binary.BigEndian.PutUint16(value[6:8], uint16(len(sub.Value)))
	copy(value[8:], sub.Value)
	p := a.prefix(id, prefix, uint16(sid.MTID), 0)
	p.Attributes = append(p.Attributes, linkstateevents.TLV{Type: 1159, Value: value})
}

func (a *bgplsArea) extendedV3(v *ospflsdb.NativeLSAView, id linkstateevents.NodeID) {
	typ := v3types.LSType(v.Type)
	offset := 0
	switch typ {
	case v3types.LSTypeERouter, v3types.LSTypeENetwork:
		offset = 4
	case v3types.LSTypeEIntraAreaPrefix:
		offset = 12
	default:
		// Every other type decodes its TLV stream from offset zero.
	}
	if len(v.Body) < offset {
		return
	}
	ext, err := v3packet.DecodeExtendedLSABody(v.Body[offset:])
	if err != nil {
		return
	}
	if typ == v3types.LSTypeERouter {
		a.routerFlags(id, v.Body[0])
		a.routerOptions(id, binary.BigEndian.Uint32(v.Body[:4])&0xffffff)
	}
	if typ == v3types.LSTypeENetwork {
		id = a.nodeID(v.AdvertisingRouter, &v.LinkStateID, true)
		a.node(id)
	}
	if typ == v3types.LSTypeEIntraAreaPrefix {
		var router types.RouterID
		copy(router[:], v.Body[8:12])
		id = a.nodeID(router, nil, true)
		reference := v3types.LSType(binary.BigEndian.Uint16(v.Body[2:4]))
		if reference == v3types.LSTypeENetwork {
			var pseudo types.LinkStateID
			copy(pseudo[:], v.Body[4:8])
			id = a.nodeID(router, &pseudo, true)
		} else if reference != v3types.LSTypeERouter {
			return
		}
	}
	for _, tlv := range ext.TLVs {
		switch {
		case typ == v3types.LSTypeERouter && tlv.Type == 1:
			if len(tlv.Value) < 16 {
				continue
			}
			// RFC 8362 3.2 preserves the base 16-byte Router-Link layout.
			var body [20]byte
			copy(body[4:], tlv.Value[:16])
			router, err := v3packet.DecodeRouterLSA(body[:])
			if err != nil {
				continue
			}
			link := a.v3RouterLink(id, &router.Links[0])
			if link == nil {
				continue
			}
			subs, err := packet.DecodeRITLVStream(tlv.Value[16:])
			if err != nil {
				continue
			}
			for _, sub := range subs {
				if sub.Type == sr.V6TypeAdjSID || sub.Type == sr.V6TypeLANAdjSID {
					sid, err := bgplsAdjSID(sub.Type, sub.Value, true)
					if err == nil {
						link.Attributes = append(link.Attributes, bgplsAdjAttribute(&sid, sub.Value[0]))
					}
				} else {
					link.Opaque = append(link.Opaque, bgplsOpaque(linkstateevents.OSPFv3ExtendedRouter, sub.Type, sub.Value))
				}
			}
		case typ == v3types.LSTypeENetwork && tlv.Type == 2:
			if len(tlv.Value)%4 != 0 {
				continue
			}
			for offset := 0; offset < len(tlv.Value); offset += 4 {
				var router types.RouterID
				copy(router[:], tlv.Value[offset:offset+4])
				bgplsMetric(&a.link(&linkstateevents.Link{Local: id, Remote: a.nodeID(router, nil, true)}, 0).Attributes, 1095, 0)
			}
		case typ == v3types.LSTypeEIntraAreaPrefix && tlv.Type == 6:
			a.extendedV3Prefix(id, &tlv, 1, linkstateevents.OSPFv3ExtendedIntraAreaPrefix)
		case typ == v3types.LSTypeEInterAreaPrefix && tlv.Type == 3:
			a.extendedV3Prefix(id, &tlv, 2, linkstateevents.OSPFv3ExtendedInterAreaPrefix)
		case (typ == v3types.LSTypeEASExternal || typ == v3types.LSTypeEType7) && tlv.Type == 5:
			if len(tlv.Value) == 0 {
				continue
			}
			provenance := linkstateevents.OSPFv3ExtendedASExternal
			if typ == v3types.LSTypeEType7 {
				provenance = linkstateevents.OSPFv3ExtendedNSSA
			}
			a.extendedV3Prefix(id, &tlv, bgplsExternalRoute(typ == v3types.LSTypeEType7, tlv.Value[0]&4 != 0), provenance)
		case tlv.Type == sr.V6TypeExtPrefixRange:
			r, err := sr.DecodeExtPrefixRangeValueV6(tlv.Value)
			if err != nil {
				continue
			}
			prefix, ok := v6PrefixToNetip(v3packet.Prefix{Length: v3types.PrefixLength(r.PrefixLength), Address: r.AddressV6}, a.af)
			if !ok {
				continue
			}
			for i := range r.PrefixSIDs {
				flags := byte(0)
				if r.IAFlag {
					flags = 0x80
				}
				value := sr.EncodePrefixSIDValueV6(r.PrefixSIDs[i])
				a.prefixRange(id, prefix, &r.PrefixSIDs[i], value[0], flags, r.RangeSize)
			}
		}
	}
}

func (a *bgplsArea) extendedV3Prefix(id linkstateevents.NodeID, tlv *v3packet.ExtendedTLV, route uint8, provenance linkstateevents.Provenance) {
	v := tlv.Value
	if len(v) < 8 || v[4] > 128 {
		return
	}
	width := ((int(v[4]) + 31) / 32) * 4
	if len(v) < 8+width {
		return
	}
	native := v3packet.Prefix{Length: v3types.PrefixLength(v[4]), Options: v3types.PrefixOptions(v[5]), Address: v[8 : 8+width]}
	subs, err := packet.DecodeRITLVStream(v[8+width:])
	if err != nil {
		return
	}
	p := a.v3Prefix(id, native, route, binary.BigEndian.Uint32(v[:4])&0xffffff)
	if p == nil {
		return
	}
	for _, sub := range subs {
		switch sub.Type {
		case sr.V6TypePrefixSID:
			sid, err := sr.DecodePrefixSIDValueV6(sub.Value)
			if err == nil {
				p.Attributes = append(p.Attributes, bgplsPrefixAttribute(&sid, sub.Value[0]))
			}
		case 1:
			if len(sub.Value) == 16 {
				bgplsAttribute(&p.Attributes, 1156, sub.Value)
			}
		case 2:
			if len(sub.Value) == 4 {
				bgplsAttribute(&p.Attributes, 1156, sub.Value)
			}
		case 3:
			if len(sub.Value) == 4 {
				bgplsAttribute(&p.Attributes, 1153, sub.Value)
			}
		default:
			// RFC 9552 5.3.3.6 restricts the opaque prefix envelope to
			// E-Inter/Intra-Area-Prefix, E-AS-External and E-NSSA LSAs.
			p.Opaque = append(p.Opaque, bgplsOpaque(provenance, sub.Type, sub.Value))
		}
	}
}

func (a *bgplsArea) resolveV3Links(views []ospflsdb.NativeLSAView) {
	if a.snapshot.Domain.Protocol != linkstateevents.OSPFv3 {
		return
	}
	type interfaceKey struct {
		router [4]byte
		id     uint32
	}
	type interfaceData struct {
		addresses []netip.Addr
		opaque    []linkstateevents.Opaque
	}
	interfaces := make(map[interfaceKey]interfaceData)
	for i := range views {
		v := &views[i]
		if v.Age >= types.MaxAge || binary.BigEndian.Uint32(v.Area[:]) != a.snapshot.Domain.Area {
			continue
		}
		key := interfaceKey{router: v.AdvertisingRouter, id: binary.BigEndian.Uint32(v.LinkStateID[:])}
		switch v3types.LSType(v.Type) {
		case v3types.LSTypeLink:
			lsa := v3packet.LSA{Body: v.Body}
			link, err := lsa.DecodeLink()
			if err == nil {
				data := interfaces[key]
				if len(data.addresses) == 0 {
					data.addresses = []netip.Addr{v6ForwardingAddr(link.LinkLocalAddr, a.af)}
				}
				interfaces[key] = data
			}
		case v3types.LSTypeELink:
			if len(v.Body) < 4 {
				continue
			}
			tlvs, err := packet.DecodeRITLVStream(v.Body[4:])
			if err != nil {
				continue
			}
			var data interfaceData
			for _, tlv := range tlvs {
				width := 0
				if tlv.Type == 7 && !a.af.isIPv4() {
					width = 16
				}
				if tlv.Type == 8 && a.af.isIPv4() {
					width = 4
				}
				if width != 0 && len(tlv.Value) >= width {
					if len(data.addresses) != 0 {
						continue
					}
					subs, err := packet.DecodeRITLVStream(tlv.Value[width:])
					if err != nil {
						continue
					}
					address, _ := netip.AddrFromSlice(tlv.Value[:width])
					data.addresses = []netip.Addr{address}
					for _, sub := range subs {
						data.opaque = append(data.opaque, bgplsOpaque(linkstateevents.OSPFv3ExtendedLink, sub.Type, sub.Value))
					}
				} else if tlv.Type != 6 && tlv.Type != 7 && tlv.Type != 8 {
					data.opaque = append(data.opaque, bgplsOpaque(linkstateevents.OSPFv3ExtendedLink, tlv.Type, tlv.Value))
				}
			}
			if len(data.addresses) == 0 {
				data.addresses = interfaces[key].addresses
			}
			interfaces[key] = data
		default:
			// Other LSA types carry no interface address or TLV data.
		}
	}
	for i := range a.snapshot.Links {
		link := &a.snapshot.Links[i]
		if len(link.Local.RouterID) != 4 {
			continue
		}
		var local, remote [4]byte
		copy(local[:], link.Local.RouterID)
		copy(remote[:], link.Remote.RouterID)
		data := interfaces[interfaceKey{router: local, id: link.LocalID}]
		link.LocalAddresses = data.addresses
		link.Opaque = append(link.Opaque, data.opaque...)
		link.RemoteAddresses = interfaces[interfaceKey{router: remote, id: link.RemoteID}].addresses
	}
}

func bgplsAddAttribute(attrs *[]linkstateevents.TLV, typ uint16, value []byte) {
	for _, attr := range *attrs {
		if attr.Type == typ && bytes.Equal(attr.Value, value) {
			return
		}
	}
	*attrs = append(*attrs, linkstateevents.TLV{Type: typ, Value: value})
}

func (a *bgplsArea) linkAttributes(routerIDs map[[4]byte][]linkstateevents.TLV) {
	for i := range a.snapshot.Links {
		link := &a.snapshot.Links[i]
		// RFC 9552 Section 5.2.2: "IPv4/IPv6 link-local addresses MUST NOT
		// be carried in the IPv4/IPv6 interface/neighbor address TLVs".
		link.LocalAddresses = bgplsGlobalAddresses(link.LocalAddresses)
		link.RemoteAddresses = bgplsGlobalAddresses(link.RemoteAddresses)
		// RFC 9552 Section 5.3.2.1: "All auxiliary Router-IDs of both the
		// local and the remote node MUST be included in the link attribute".
		if len(link.Local.RouterID) == 4 {
			var router [4]byte
			copy(router[:], link.Local.RouterID)
			for _, attr := range routerIDs[router] {
				bgplsAddAttribute(&link.Attributes, attr.Type, attr.Value)
			}
		}
		if len(link.Remote.RouterID) == 4 && link.Remote.ASN == 0 {
			var router [4]byte
			copy(router[:], link.Remote.RouterID)
			for _, attr := range routerIDs[router] {
				bgplsAddAttribute(&link.Attributes, attr.Type+2, attr.Value)
			}
		}
	}
}

func bgplsLinkLocal(address netip.Addr) bool {
	return address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast()
}

func bgplsGlobalAddresses(addresses []netip.Addr) []netip.Addr {
	kept := 0
	for _, address := range addresses {
		if !bgplsLinkLocal(address) {
			kept++
		}
	}
	if kept == len(addresses) {
		return addresses
	}
	if kept == 0 {
		return nil
	}
	// Interface records can be shared by several directed links. Only a mixed
	// address list needs a copy before filtering; never mutate those aliases.
	return slices.DeleteFunc(slices.Clone(addresses), bgplsLinkLocal)
}

// RFC 9552 Section 5.9: a producer SHOULD withdraw objects "when the node
// that originated its corresponding LSPs/LSAs is determined to have become
// unreachable in the IGP" and MUST re-advertise when it becomes reachable.
// Keep the complete native objects for other consumers; only name identities
// to suppress in BGP. One immutable completed SPF snapshot covers the event.
func (a *bgplsArea) markUnreachable(reachability ospfspf.ReachabilitySnapshot, previous map[bgplsNodeKey]struct{}) map[bgplsNodeKey]struct{} {
	ready := reachability.Ready()
	if !ready && len(previous) == 0 {
		return nil
	}
	var unreachable map[bgplsNodeKey]struct{}
	seen := make(map[bgplsNodeKey]struct{}, len(a.snapshot.Nodes))
	consider := func(id linkstateevents.NodeID) {
		key := bgplsNodeIdentity(&id)
		if _, found := seen[key]; found {
			return
		}
		seen[key] = struct{}{}
		if len(id.RouterID) != 4 && len(id.RouterID) != 8 {
			return
		}
		var router types.RouterID
		copy(router[:], id.RouterID[:4])
		var area types.AreaID
		binary.BigEndian.PutUint32(area[:], id.Area)
		var known, reachable bool
		switch {
		case ready && len(id.RouterID) == 8:
			var network types.LinkStateID
			copy(network[:], id.RouterID[4:])
			v3 := a.snapshot.Domain.Protocol == linkstateevents.OSPFv3
			known, reachable = bgplsPseudonodeReachability(
				reachability.RouterKnown(area, router), reachability.RouterReachable(area, router),
				reachability.NetworkKnown(area, router, network, v3), reachability.NetworkReachable(area, router, network, v3))
		case ready && id.HasArea:
			known = reachability.RouterKnown(area, router)
			reachable = known && reachability.RouterReachable(area, router)
		case ready:
			// ASBRs need not have a Router-LSA in an attached area: native
			// inter-area ASBR paths also establish AS-scope reachability.
			known = reachability.RouterKnownAny(router)
			reachable = known && reachability.RouterReachableAny(router)
		}
		if !known {
			// A new LSA not yet seen by SPF is unknown, not unreachable.
			// Likewise, invalidating a tree is not evidence of recovery.
			_, failed := previous[key]
			reachable = !failed
		}
		if !reachable {
			a.snapshot.Unreachable = append(a.snapshot.Unreachable, id)
			if unreachable == nil {
				unreachable = make(map[bgplsNodeKey]struct{})
			}
			unreachable[key] = struct{}{}
		}
	}
	for i := range a.snapshot.Nodes {
		consider(a.snapshot.Nodes[i].ID)
	}
	for i := range a.snapshot.Links {
		consider(a.snapshot.Links[i].Local)
	}
	for i := range a.snapshot.Prefixes {
		consider(a.snapshot.Prefixes[i].Node)
	}
	for _, sid := range a.snapshot.SIDs {
		consider(sid.Node)
	}
	return unreachable
}

func bgplsPseudonodeReachability(routerKnown, routerReached, networkKnown, networkReached bool) (known, reachable bool) {
	if (routerKnown && !routerReached) || (networkKnown && !networkReached) {
		return true, false
	}
	if routerKnown && networkKnown {
		return true, true
	}
	return false, false
}
