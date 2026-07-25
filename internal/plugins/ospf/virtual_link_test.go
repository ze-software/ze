// VALIDATES: spec-ospf-ext-7 -- the shared engine-side virtual-link manager: config drives
// the SPF virtual-link requests, the transit-area SPF resolution drives each link's runtime
// state, and a reachable virtual link surfaces as a synthetic backbone point-to-point
// interface in the origination topology (keyed by transit area + neighbor, reserved name).
package ospf

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func vlArea(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatalf("ParseAreaID(%q): %v", s, err)
	}
	return id
}

func vlRID(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

// VALIDATES: spec-ospf-ext-7 AC-1 / R-10 -- configureVirtualLinks builds one runtime per
// configured virtual link keyed by (transit area, neighbor), with a reserved synthetic
// interface name that cannot collide with a real interface.
func TestVirtualInterfaceNameReserved(t *testing.T) {
	e := newEngine(nil)
	cfg := defaultOSPFConfig()
	cfg.present = true
	cfg.RouterID = vlRID(t, "1.1.1.1")
	cfg.VirtualLinks = []virtualLinkConfig{
		{TransitArea: vlArea(t, "0.0.0.1"), RemoteRouterID: vlRID(t, "9.9.9.9")},
	}
	e.configureVirtualLinks(cfg)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.virtualLinks) != 1 {
		t.Fatalf("virtualLinks = %d, want 1", len(e.virtualLinks))
	}
	for _, rt := range e.virtualLinks {
		if !strings.HasPrefix(rt.name, "*") {
			t.Fatalf("synthetic name %q must be reserved (start with '*') so it cannot clash with a real interface", rt.name)
		}
	}
}

// VALIDATES: spec-ospf-ext-7 R-10 -- two virtual links sharing a transit area do not
// collide; they are keyed by (transit area, neighbor).
func TestTwoVirtualLinksSameTransit(t *testing.T) {
	e := newEngine(nil)
	cfg := defaultOSPFConfig()
	cfg.present = true
	cfg.RouterID = vlRID(t, "1.1.1.1")
	transit := vlArea(t, "0.0.0.1")
	cfg.VirtualLinks = []virtualLinkConfig{
		{TransitArea: transit, RemoteRouterID: vlRID(t, "9.9.9.9")},
		{TransitArea: transit, RemoteRouterID: vlRID(t, "8.8.8.8")},
	}
	e.configureVirtualLinks(cfg)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.virtualLinks) != 2 {
		t.Fatalf("virtualLinks = %d, want 2 distinct links", len(e.virtualLinks))
	}
	names := map[string]struct{}{}
	for _, rt := range e.virtualLinks {
		names[rt.name] = struct{}{}
	}
	if len(names) != 2 {
		t.Fatalf("two virtual links share a synthetic name: %v", names)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-4 / AC-5 -- the transit-area SPF resolution drives the
// runtime up (reachable, cost) and down (unreachable).
func TestVirtualLinkResolutionDrivesRuntime(t *testing.T) {
	e := newEngine(nil)
	// onVirtualLinksResolved re-triggers SPF (RFC 2328 sec 16.1); the live computer's 50ms
	// back-off timer would then re-enter the callback on its own goroutine and write the
	// runtime fields this test reads (a data race), and, with no transit topology configured,
	// resolve the link back down (a latent flake). This test drives the callback directly to
	// assert its synchronous effect, so stop the computer up front: triggerSPF becomes a
	// no-op and the callback runs inline, leaving the runtime owned by this goroutine.
	e.spf.Stop()
	cfg := defaultOSPFConfig()
	cfg.present = true
	cfg.RouterID = vlRID(t, "1.1.1.1")
	transit := vlArea(t, "0.0.0.1")
	neighbor := vlRID(t, "9.9.9.9")
	cfg.VirtualLinks = []virtualLinkConfig{{TransitArea: transit, RemoteRouterID: neighbor}}
	e.configureVirtualLinks(cfg)

	e.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: transit, Neighbor: neighbor, Reachable: true, Cost: 42,
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("10.1.0.2"), Interface: "eth1"}},
	}})
	e.mu.Lock()
	rt := e.virtualLinks[virtualLinkKey{transit: transit, neighbor: neighbor}]
	e.mu.Unlock()
	if rt == nil || !rt.reachable || rt.cost != 42 {
		t.Fatalf("runtime not driven up: %+v", rt)
	}

	e.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{TransitArea: transit, Neighbor: neighbor, Reachable: false}})
	e.mu.Lock()
	rt = e.virtualLinks[virtualLinkKey{transit: transit, neighbor: neighbor}]
	e.mu.Unlock()
	if rt == nil || rt.reachable {
		t.Fatalf("runtime not driven down when the neighbor became unreachable: %+v", rt)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-8 (backbone-only) -- a reachable virtual link surfaces as a
// synthetic NetworkVirtual interface in the BACKBONE origination topology, carrying its
// transit area and computed cost; the record it drives is thus backbone-only.
func TestVirtualLinkTopologyEmitsBackboneInterface(t *testing.T) {
	e := newEngine(nil)
	transit := vlArea(t, "0.0.0.2")
	neighbor := vlRID(t, "9.9.9.9")
	key := virtualLinkKey{transit: transit, neighbor: neighbor}
	e.virtualLinks = map[virtualLinkKey]*virtualLinkRuntime{
		key: {
			cfg:       virtualLinkConfig{TransitArea: transit, RemoteRouterID: neighbor},
			name:      virtualLinkName(key),
			reachable: true,
			cost:      42,
			localAddr: netip.MustParseAddr("172.16.0.1"),
		},
	}
	e.cfg.RouterID = vlRID(t, "1.1.1.1")
	got := e.virtualLinkTopology()
	if len(got) != 1 {
		t.Fatalf("virtualLinkTopology = %d entries, want 1", len(got))
	}
	iface := got[0]
	if iface.NetworkType != ospflsdb.NetworkVirtual {
		t.Fatalf("network type = %q, want %q", iface.NetworkType, ospflsdb.NetworkVirtual)
	}
	if iface.AreaID != types.BackboneArea {
		t.Fatalf("area = %s, want backbone (a virtual link belongs to Area 0)", iface.AreaID)
	}
	if iface.VirtualTransitArea != transit {
		t.Fatalf("transit area = %s, want %s", iface.VirtualTransitArea, transit)
	}
	if iface.Cost != 42 {
		t.Fatalf("cost = %d, want the transit cost 42", iface.Cost)
	}
	if iface.Address != [4]byte{172, 16, 0, 1} {
		t.Fatalf("local address = %v, want the local transit address", iface.Address)
	}
}

// vlEngine builds an engine with a fake transport for the given OSPF config and opens its
// interfaces, returning the engine and the transit interface's fake ifindex.
func vlEngine(t *testing.T, cfgJSON string) (*engine, *fakeBackend) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(cfgJSON), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	return eng, fb
}

func vlIfindex(t *testing.T, fb *fakeBackend, name string) int {
	t.Helper()
	fb.mu.Lock()
	defer fb.mu.Unlock()
	h := fb.handles[name]
	if h == nil {
		t.Fatalf("interface %q not opened", name)
	}
	return h.ifindex
}

func dispatchVirtualHello(t *testing.T, eng *engine, ifindex int, router types.RouterID, area types.AreaID, src netip.Addr, self types.RouterID) {
	t.Helper()
	hello := packet.Hello{
		HelloInterval: DefaultHelloInterval,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  uint32(DefaultDeadInterval),
		Neighbors:     []types.RouterID{self},
	}
	p := packet.Packet{Header: packet.Header{RouterID: router, AreaID: area}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: ifindex, Src: src, Payload: buf})
}

func vlDecodeRouter(t *testing.T, db *ospflsdb.LSDB, area types.AreaID, rid types.RouterID) packet.RouterLSA {
	t.Helper()
	lsa, ok := db.LookupLSA(area, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid), AdvertisingRouter: rid})
	if !ok {
		t.Fatalf("self Router-LSA missing in area %s", area)
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter area %s: %v", area, err)
	}
	return body
}

// VALIDATES: spec-ospf-ext-7 AC-6 -- an OSPFv2 virtual link reaches Full over routed IP: a
// backbone Hello + DD exchange from the virtual neighbor, arriving on the TRANSIT ifindex,
// is demuxed to the synthetic virtual interface and drives its NSM to Full, after which the
// backbone Router-LSA carries the Type-4 virtual record and the transit-area Router-LSA
// carries the V-bit.
func TestVirtualLinkAdjacencyReachesFull(t *testing.T) {
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{
			"0.0.0.0":{"area-id":"0.0.0.0"},
			"0.0.0.1":{"area-id":"0.0.0.1","virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
			"lo":{"area":"0.0.0.0","network-type":"loopback"}}}}}`
	eng, fb := vlEngine(t, j)
	defer eng.shutdown()
	ifindex := vlIfindex(t, fb, "eth0")

	self := vlRID(t, "10.0.0.1")
	peer := vlRID(t, "10.0.0.2")
	transit := vlArea(t, "0.0.0.1")
	backbone := types.BackboneArea

	// The transit-area SPF resolved the virtual neighbor reachable: bring the link up.
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: transit, Neighbor: peer, Reachable: true, Cost: 10,
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Interface: "eth0"}},
	}})
	vlname := virtualLinkName(virtualLinkKey{transit: transit, neighbor: peer})

	// A backbone Hello + DD from the virtual neighbor, arriving on the transit ifindex.
	dispatchVirtualHello(t, eng, ifindex, peer, backbone, netip.MustParseAddr("192.0.2.2"), self)
	dispatchDBDesc(t, eng, ifindex, peer, backbone, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7})
	dispatchDBDesc(t, eng, ifindex, peer, backbone, packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: packet.DDFlagMaster, DDSequence: 8})

	full := false
	for _, row := range eng.neighborSnapshot() {
		snap, ok := row.(ospfneighbor.Snapshot)
		if ok && snap.Interface == vlname && snap.RouterID == "10.0.0.2" && snap.State == ospflsdb.NeighborStateFull {
			full = true
		}
	}
	if !full {
		t.Fatalf("virtual neighbor did not reach Full on %s: %+v", vlname, eng.neighborSnapshot())
	}

	// The FSM reached Full. Feed the engine's live topology (now carrying the Full virtual
	// neighbor) through a fresh, unthrottled LSDB to assert origination content -- the
	// engine LSDB's RFC 2328 MinLSInterval reorigination throttle (fixed 5s) would otherwise
	// defer the update past the sub-second test.
	topo := eng.lsdbTopology()
	db := ospflsdb.New(func() time.Time { return time.Unix(0, 0) })
	db.SetTopology(func() []ospflsdb.InterfaceInfo { return topo })
	db.OriginateFromTopology(self, false)

	bb := vlDecodeRouter(t, db, backbone, self)
	if bb.Flags&packet.RouterFlagV != 0 {
		t.Fatalf("backbone Router-LSA must NOT carry the V-bit (it belongs to the transit area): flags = %#x", bb.Flags)
	}
	found := false
	for _, l := range bb.Links {
		if l.Type == packet.RouterLinkTypeVirtual && l.LinkID == types.LinkStateID(peer) {
			found = true
		}
	}
	if !found {
		t.Fatalf("backbone Router-LSA missing the Type-4 virtual record: %+v", bb.Links)
	}
	tr := vlDecodeRouter(t, db, transit, self)
	if tr.Flags&packet.RouterFlagV == 0 {
		t.Fatalf("transit-area Router-LSA must carry the V-bit: flags = %#x", tr.Flags)
	}
}

// VALIDATES: spec-ospf-ext-7 R-9 -- a backbone packet from the virtual neighbor on the
// transit ifindex demuxes to the synthetic virtual interface, while a packet on a real
// backbone interface, or from any other router, does NOT (base demux undisturbed).
func TestVirtualLinkPacketDemux(t *testing.T) {
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{
			"0.0.0.0":{"area-id":"0.0.0.0"},
			"0.0.0.1":{"area-id":"0.0.0.1","virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
			"eth1":{"area":"0.0.0.0","network-type":"point-to-point"}}}}}`
	eng, fb := vlEngine(t, j)
	defer eng.shutdown()
	transitIf := vlIfindex(t, fb, "eth0")
	backboneIf := vlIfindex(t, fb, "eth1")
	peer := vlRID(t, "10.0.0.2")
	transit := vlArea(t, "0.0.0.1")
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: transit, Neighbor: peer, Reachable: true, Cost: 10,
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Interface: "eth0"}},
	}})
	vlname := virtualLinkName(virtualLinkKey{transit: transit, neighbor: peer})

	eng.mu.Lock()
	defer eng.mu.Unlock()
	// Backbone packet from the virtual neighbor on the transit ifindex -> the virtual iface.
	if name, ifc, ok := eng.receiveTargetLocked(transitIf, Header{RouterID: peer, AreaID: types.BackboneArea}); !ok || name != vlname || ifc == nil {
		t.Fatalf("virtual-neighbor backbone packet not demuxed to %s: name=%q ok=%v", vlname, name, ok)
	}
	// Backbone packet from a DIFFERENT router on the transit ifindex -> not virtual.
	if name, _, _ := eng.receiveTargetLocked(transitIf, Header{RouterID: vlRID(t, "9.9.9.9"), AreaID: types.BackboneArea}); name == vlname {
		t.Fatalf("backbone packet from a non-virtual router wrongly demuxed to the virtual interface")
	}
	// A backbone packet on the REAL backbone interface (eth1) is a normal backbone packet.
	if name, _, ok := eng.receiveTargetLocked(backboneIf, Header{RouterID: peer, AreaID: types.BackboneArea}); !ok || name != "eth1" {
		t.Fatalf("packet on a real backbone interface not routed to eth1: name=%q ok=%v", name, ok)
	}
	// A backbone packet from the virtual neighbor on an UNKNOWN (non-enrolled) ifindex must
	// NOT resolve to the virtual link (the tightened predicate requires a real transit
	// interface).
	if _, _, ok := eng.receiveTargetLocked(987654, Header{RouterID: peer, AreaID: types.BackboneArea}); ok {
		t.Fatalf("backbone packet on an unknown ifindex wrongly resolved to an interface")
	}
	if eng.virtualLinkTargetLocked(987654, Header{RouterID: peer, AreaID: types.BackboneArea}) != nil {
		t.Fatalf("virtual demux accepted an unknown, non-enrolled ifindex")
	}
}

// VALIDATES: spec-ospf-ext-7 AC-18 -- a virtual link inherits the TRANSIT area's
// authentication via the REAL path production takes: its routed packets are sent over the
// transit egress interface (signed against that interface's transit-area key chain) and
// received on the transit ifindex (verified against the same chain). There is no synthetic
// interface key registration, so this asserts the transit-egress sign/verify round-trip.
func TestVirtualLinkUsesTransitAreaAuth(t *testing.T) {
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"key-chains":{"kc1":{"key":{"1":{"algorithm":"md5","secret":"transitsecret"}}}},
		"areas":{"area":{
			"0.0.0.0":{"area-id":"0.0.0.0"},
			"0.0.0.1":{"area-id":"0.0.0.1","authentication":{"key-chain":"kc1"},"virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
			"lo":{"area":"0.0.0.0","network-type":"loopback"}}}}}`
	eng, _ := vlEngine(t, j)
	defer eng.shutdown()
	transit := vlArea(t, "0.0.0.1")
	peer := vlRID(t, "10.0.0.2")

	// The resolved virtual link sends over the transit egress (eth0), so its packets are
	// signed with eth0's chain -- the transit area's kc1.
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: transit, Neighbor: peer, Reachable: true, Cost: 10,
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Interface: "eth0"}},
	}})
	eng.mu.Lock()
	egress := eng.virtualLinks[virtualLinkKey{transit: transit, neighbor: peer}].transitNextHop.Interface
	eng.mu.Unlock()
	if egress != "eth0" {
		t.Fatalf("virtual link egress = %q, want the transit interface eth0", egress)
	}
	if _, au, _, _, ok := eng.auth.signKey(egress); !ok || au == packet.AuTypeNull {
		t.Fatalf("transit egress %q does not sign with the transit-area key chain: au=%v ok=%v", egress, au, ok)
	}

	// Round-trip: a packet signed as the transit egress verifies on the transit ifindex,
	// so a virtual-link packet is authenticated end-to-end with the transit area's keys.
	hello := packet.Hello{HelloInterval: DefaultHelloInterval, Options: types.OptionE, Priority: 1, DeadInterval: uint32(DefaultDeadInterval)}
	p := packet.Packet{Header: packet.Header{RouterID: peer, AreaID: types.BackboneArea}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	signed := eng.signPacket("eth0", buf)
	if reason, pass := eng.auth.verify("eth0", peer, [4]byte{}, signed); !pass {
		t.Fatalf("transit-egress-signed virtual-link packet failed verification on the transit interface: %s", reason)
	}
}

// installV6IntraPrefix installs a Router-referencing OSPFv3 Intra-Area-Prefix-LSA advertising
// one global prefix for router in area, so the v6 endpoint resolver can learn the router's
// global address from the transit-area LSDB.
func installV6IntraPrefix(t *testing.T, db *ospflsdb.LSDB, area types.AreaID, router types.RouterID, cidr string) {
	t.Helper()
	p, ok := netipToV6Prefix(netip.MustParsePrefix(cidr), 0)
	if !ok {
		t.Fatalf("netipToV6Prefix(%s)", cidr)
	}
	body := ospfv3packet.IntraAreaPrefixLSA{
		ReferencedLSType:    ospfv3types.LSTypeRouter,
		ReferencedAdvRouter: ospfv3types.RouterID(router),
		Prefixes:            []ospfv3packet.Prefix{p},
	}
	lsa := v6SelfLSA(ospfv3packet.LSA{
		Header:       v6OriginHeader(ospfv3types.LSTypeIntraAreaPrefix, ospfv3types.LinkStateID{}, router, types.InitialSequenceNumber, false),
		IntraAreaPfx: &body,
	})
	if !db.Install(area, lsa) {
		t.Fatalf("Install v6 Intra-Area-Prefix-LSA for %s failed", router)
	}
}

func v6DecodeAreaRouter(t *testing.T, eng *engine, area types.AreaID, rid types.RouterID) ospfv3packet.RouterLSA {
	t.Helper()
	lsa, ok := eng.lsdb.LookupLSA(area, v6RouterKey(rid))
	if !ok {
		t.Fatalf("v6 Router-LSA missing in area %s", area)
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA area %s: %v", area, err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter area %s: %v", area, err)
	}
	return body
}

// VALIDATES: spec-ospf-ext-7 AC-7 / A-5 -- the OSPFv3 virtual link resolves both a local
// GLOBAL source and the neighbor's GLOBAL destination from the transit area's
// Intra-Area-Prefix-LSAs (RFC 5340 sec 2.9), not the transit link-locals.
// RFC requirement: RFC5838-2.8-1 positive -- the OSPFv3 virtual-link endpoint resolves to a global IPv6 address for both ends, so control packets are forwarded correctly by the intermediate hops.
// RFC requirement: RFC5340-2.5-1 positive -- the source address used for OSPF protocol packets on
// a virtual link is a GLOBAL scope IPv6 address: v6ResolveVirtualEndpointLocked returns this
// router's global 2001:db8:1::1 (v6RouterGlobalAddr IsGlobalUnicast filter, virtuallink_v6.go:69),
// and the routed send binds exactly that source into both the IPv6 header and the checksum
// pseudo-header (SendPacketRouted, v3/transport/transport.go:528-548).
// RFC requirement: RFC5340-4.1.2-2 positive -- the address the virtual link uses as its IP
// interface address is one of THIS router's own global-scope IPv6 addresses, looked up by this
// router's own Router ID in the transit area's Intra-Area-Prefix-LSAs, not a link-local
// (v6ResolveVirtualEndpointLocked, virtuallink_v6.go:32).
func TestV6VirtualEndpointResolvesGlobalAddress(t *testing.T) {
	e := newV6OriginEngine()
	transit := vlArea(t, "0.0.0.1")
	self := vlRID(t, "10.0.0.1")
	neighbor := vlRID(t, "10.0.0.2")
	e.cfg.RouterID = self
	installV6IntraPrefix(t, e.lsdb, transit, self, "2001:db8:1::1/128")
	installV6IntraPrefix(t, e.lsdb, transit, neighbor, "2001:db8:2::2/128")

	rt := &virtualLinkRuntime{cfg: virtualLinkConfig{TransitArea: transit, RemoteRouterID: neighbor}}
	src, dst, ok := e.v6ResolveVirtualEndpointLocked(rt)
	if !ok {
		t.Fatalf("v6 endpoint resolution failed")
	}
	if !src.Is6() || src.IsUnspecified() || src != netip.MustParseAddr("2001:db8:1::1") {
		t.Fatalf("local global source = %v, want 2001:db8:1::1", src)
	}
	if !dst.Is6() || dst.IsUnspecified() || dst != netip.MustParseAddr("2001:db8:2::2") {
		t.Fatalf("neighbor global destination = %v, want 2001:db8:2::2", dst)
	}
}

// TestV6VirtualEndpointRequiresGlobalAddress pins RFC 5838 §2.8: an OSPFv3 virtual-link
// endpoint resolves only from a GLOBAL IPv6 address. When the neighbor advertises only a
// link-local prefix in the transit area, no endpoint is resolved and the virtual link cannot
// carry control packets (fail-closed), so a missing global IPv6 address is never masked.
// RFC requirement: RFC5838-2.8-1 negative -- when the neighbor advertises no global IPv6 address (only a link-local), the virtual-link endpoint does not resolve, so no virtual link is formed without the required global address.
// RFC requirement: RFC5340-2.5-1 negative -- a link-local address is never promoted to the
// virtual link's packet source/destination: with only fe80::2 advertised for the far end the
// endpoint fails to resolve and the link sends nothing, rather than falling back to link-local
// (v6RouterGlobalAddr rejects a non-global-unicast prefix, virtuallink_v6.go:69).
func TestV6VirtualEndpointRequiresGlobalAddress(t *testing.T) {
	e := newV6OriginEngine()
	transit := vlArea(t, "0.0.0.1")
	self := vlRID(t, "10.0.0.1")
	neighbor := vlRID(t, "10.0.0.2")
	e.cfg.RouterID = self
	// This router advertises a global address, but the neighbor advertises only a link-local
	// prefix -- there is no global IPv6 address associated with the virtual link's far end.
	installV6IntraPrefix(t, e.lsdb, transit, self, "2001:db8:1::1/128")
	installV6IntraPrefix(t, e.lsdb, transit, neighbor, "fe80::2/128")

	rt := &virtualLinkRuntime{cfg: virtualLinkConfig{TransitArea: transit, RemoteRouterID: neighbor}}
	if _, _, ok := e.v6ResolveVirtualEndpointLocked(rt); ok {
		t.Fatal("virtual-link endpoint resolved without a global IPv6 address for the neighbor (RFC 5838 §2.8)")
	}
}

// VALIDATES: spec-ospf-ext-7 AC-7 -- an OSPFv3 virtual link reaches Full over routed IP: the
// endpoint resolves a global src/dst, a backbone v6 Hello + DD from the virtual neighbor on
// the transit ifindex demuxes to the synthetic interface and drives its NSM to Full, after
// which the backbone Router-LSA carries the RouterLinkTypeVirtual record and the transit-area
// Router-LSA sets the V-bit (RFC 5340 App A.4.3).
func TestV6VirtualAdjacencyReachesFull(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{"0":{"area-id":"0"},"0.0.0.1":{"area-id":"0.0.0.1","virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{"eth0":{"area":"0.0.0.1","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	t.Cleanup(eng.shutdown)
	// Shrink MinLSInterval so the post-Full reorigination (the 1s refresh ticker in
	// production) is not rate-limited in this microsecond-scale test.
	eng.lsdb.SetTimers(ospflsdb.TimerConfig{MinLSInterval: time.Nanosecond})

	self := vlRID(t, "10.0.0.1")
	peer := vlRID(t, "10.0.0.2")
	transit := vlArea(t, "0.0.0.1")
	backbone := types.BackboneArea
	ifindex := vlIfindex(t, fb, "eth0")
	selfGlobal := netip.MustParseAddr("2001:db8:1::1")
	nbrGlobal := netip.MustParseAddr("2001:db8:2::2")

	installV6IntraPrefix(t, eng.lsdb, transit, self, "2001:db8:1::1/128")
	installV6IntraPrefix(t, eng.lsdb, transit, peer, "2001:db8:2::2/128")

	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: transit, Neighbor: peer, Reachable: true, Cost: 10,
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("fe80::2"), Interface: "eth0"}},
	}})
	eng.mu.Lock()
	rt := eng.virtualLinks[virtualLinkKey{transit: transit, neighbor: peer}]
	gotSrc, gotDst := rt.localAddr, rt.neighborAddr
	eng.mu.Unlock()
	if gotSrc != selfGlobal || gotDst != nbrGlobal {
		t.Fatalf("resolved v6 endpoint = src %v dst %v, want %v / %v", gotSrc, gotDst, selfGlobal, nbrGlobal)
	}

	vlname := virtualLinkName(virtualLinkKey{transit: transit, neighbor: peer})
	ddOpts := ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR
	dispatchHelloV6(t, eng, ifindex, peer, backbone, nbrGlobal, selfGlobal, ospfv3packet.Hello{
		InterfaceID: 2, Priority: 1, Options: ddOpts,
		HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval,
		Neighbors: []ospfv3types.RouterID{ospfv3types.RouterID(self)},
	})
	dispatchDBDescV6(t, eng, ifindex, peer, backbone, nbrGlobal, selfGlobal, ospfv3packet.DBDesc{
		InterfaceMTU: 1500, Options: ddOpts,
		Flags: ospfv3packet.DDFlagInit | ospfv3packet.DDFlagMore | ospfv3packet.DDFlagMaster, DDSequence: 7,
	})
	dispatchDBDescV6(t, eng, ifindex, peer, backbone, nbrGlobal, selfGlobal, ospfv3packet.DBDesc{
		InterfaceMTU: 1500, Options: ddOpts, Flags: ospfv3packet.DDFlagMaster, DDSequence: 8,
	})

	full := false
	for _, row := range eng.neighborSnapshot() {
		snap, ok := row.(ospfneighbor.Snapshot)
		if ok && snap.Interface == vlname && snap.RouterID == "10.0.0.2" && snap.State == ospflsdb.NeighborStateFull {
			full = true
		}
	}
	if !full {
		t.Fatalf("v6 virtual neighbor did not reach Full on %s: %+v", vlname, eng.neighborSnapshot())
	}

	eng.originateSelfLSAs()
	bb := v6DecodeAreaRouter(t, eng, backbone, self)
	if bb.Flags&ospfv3packet.RouterFlagV != 0 {
		t.Fatalf("backbone Router-LSA must NOT carry the V-bit: flags = %#x", bb.Flags)
	}
	foundV := false
	for _, l := range bb.Links {
		if l.Type == ospfv3packet.RouterLinkTypeVirtual && l.NeighborRouterID == ospfv3types.RouterID(peer) {
			foundV = true
		}
	}
	if !foundV {
		t.Fatalf("backbone Router-LSA missing the RouterLinkTypeVirtual record: %+v", bb.Links)
	}
	tr := v6DecodeAreaRouter(t, eng, transit, self)
	if tr.Flags&ospfv3packet.RouterFlagV == 0 {
		t.Fatalf("transit-area Router-LSA must carry the V-bit: flags = %#x", tr.Flags)
	}
}

// VALIDATES: spec-ospf-ext-7 AC-17 -- with no virtual link configured, the manager emits no
// backbone virtual interface, so OSPF origination is byte-for-byte unchanged.
func TestNoVirtualLinkBehaviorUnchanged(t *testing.T) {
	e := newEngine(nil)
	if got := e.virtualLinkTopology(); len(got) != 0 {
		t.Fatalf("virtualLinkTopology with no config = %+v, want empty", got)
	}
	cfg := defaultOSPFConfig()
	cfg.present = true
	cfg.RouterID = vlRID(t, "1.1.1.1")
	e.configureVirtualLinks(cfg)
	if got := e.virtualLinkTopology(); len(got) != 0 {
		t.Fatalf("virtualLinkTopology after empty config = %+v, want empty", got)
	}
}
