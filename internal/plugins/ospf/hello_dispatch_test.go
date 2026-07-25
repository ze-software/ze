// VALIDATES: spec-ospf-af-unify -- the Hello body decode is routed through the engine Codec
// (not the interface). A real Hello dispatched through the engine flows dispatch ->
// handleHello -> codec.DecodeHello -> ReceiveDecodedHello -> neighbor, and the neighbor
// appears in the snapshot. PREVENTS: the rewired Hello seam silently failing to wire (the
// codec method exists but the engine no longer reaches the neighbor FSM).
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFHelloDispatchFormsNeighbor(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	peer := ridOf("10.0.0.2")
	hello := packet.Hello{
		HelloInterval: DefaultHelloInterval,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  uint32(DefaultDeadInterval),
	}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: peer, AreaID: cfg.Areas[0].AreaID}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}

	// Dispatch the Hello through the engine's real handler (not a stub), exercising the codec
	// body-decode seam end to end.
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: buf})

	rows := eng.neighborSnapshot()
	if len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1 (Hello decoded through the codec seam)", len(rows))
	}
	snap, ok := rows[0].(ospfneighbor.Snapshot)
	if !ok {
		t.Fatalf("snapshot row type = %T, want neighbor.Snapshot", rows[0])
	}
	if snap.RouterID != "10.0.0.2" || snap.Interface != "eth0" {
		t.Fatalf("neighbor snapshot = %+v, want peer 10.0.0.2 on eth0", snap)
	}
}

// TestNeighborOnlyFormsWithinInstance proves AC-4 / R-6 (RFC 6549 §2/§3.1): an engine
// configured for Instance ID 5 forms no neighbor from a Hello carrying a different Instance
// ID (it is discarded at the demux before the FSM runs), and forms one from a matching Hello.
func TestNeighborOnlyFormsWithinInstance(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point","instance-id":"5"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg.forInstance(5))
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()
	if eng.dispatch.instanceID != 5 {
		t.Fatalf("engine dispatch.instanceID = %d, want 5", eng.dispatch.instanceID)
	}

	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}

	peer := ridOf("10.0.0.2")
	helloFor := func(instanceID uint8) []byte {
		hello := packet.Hello{HelloInterval: DefaultHelloInterval, Options: types.OptionE, Priority: 1, DeadInterval: uint32(DefaultDeadInterval)}
		p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: peer, AreaID: cfg.Areas[0].AreaID, InstanceID: instanceID}, Hello: &hello}
		buf := make([]byte, p.EncodedLen())
		p.WriteTo(buf, 0)
		return buf
	}

	// A Hello for Instance 0 must be dropped by the demux: no neighbor forms.
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: helloFor(0)})
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows after mismatched-instance Hello = %d, want 0 (no cross-instance adjacency)", len(rows))
	}

	// A Hello for the engine's Instance ID 5 forms the neighbor.
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: helloFor(5)})
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("neighbor rows after matching-instance Hello = %d, want 1", len(rows))
	}
}

// TestOSPFEngineIPv6HelloFormsNeighbor proves the unified engine forms a Hello adjacency over
// the OSPFv3 codec: a v6 engine instance (newEngineWithCodecAF with v6Codec) receives a real
// OSPFv3 Hello (16-byte header, IPv6 upper-layer checksum, no Network Mask) and the shared
// FSM creates the neighbor. The transport here is plumbing (it delivers the v6 payload +
// ifindex + IPv6 src/dst); the OSPFv3 raw socket / multicast is covered by ospfv3/transport's
// own tests. This is the "shared FSM reaches adjacency over the v6 codec" proof (AC-4 core)
// at the unit level; the address-family config + register.go runtime wiring is the follow-up.
func TestOSPFEngineIPv6HelloFormsNeighbor(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	peer := ridOf("10.0.0.2")
	src := netip.MustParseAddr("fe80::2")
	dst := netip.MustParseAddr("ff02::5")
	h := ospfv3packet.Hello{
		InterfaceID:        2,
		Priority:           1,
		Options:            ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR,
		HelloInterval:      DefaultHelloInterval,
		RouterDeadInterval: DefaultDeadInterval,
	}
	p := ospfv3packet.Packet{
		Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeHello, RouterID: ospfv3types.RouterID(peer), AreaID: ospfv3types.AreaID(cfg.Areas[0].AreaID)},
		Hello:  &h,
	}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	ospfv3packet.FinalizePacketChecksum(src, dst, buf) // bind the IPv6 upper-layer checksum

	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: src, Dst: dst, Payload: buf})

	rows := eng.neighborSnapshot()
	if len(rows) != 1 {
		t.Fatalf("v6 neighbor rows = %d, want 1 (shared FSM formed a neighbor from the v6 Hello)", len(rows))
	}
	snap, ok := rows[0].(ospfneighbor.Snapshot)
	if !ok {
		t.Fatalf("snapshot row type = %T, want neighbor.Snapshot", rows[0])
	}
	if snap.RouterID != "10.0.0.2" || snap.Interface != "eth0" {
		t.Fatalf("v6 neighbor snapshot = %+v, want peer 10.0.0.2 on eth0", snap)
	}
}

// dispatchHelloV6 encodes an OSPFv3 Hello, binds its IPv6 upper-layer checksum, and feeds it
// through the engine's real receive path (the v6 codec), as a neighbor on the wire would.
func dispatchHelloV6(t *testing.T, eng *engine, ifindex int, router types.RouterID, area types.AreaID, src, dst netip.Addr, h ospfv3packet.Hello) {
	t.Helper()
	p := ospfv3packet.Packet{
		Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeHello, RouterID: ospfv3types.RouterID(router), AreaID: ospfv3types.AreaID(area)},
		Hello:  &h,
	}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	ospfv3packet.FinalizePacketChecksum(src, dst, buf)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: ifindex, Src: src, Dst: dst, Payload: buf})
}

// dispatchHelloV6Instance encodes an OSPFv3 Hello with an explicit Instance ID in the common
// header and feeds it through the engine's real receive path (used to prove the RFC 5340 sec
// 4.2.2 Instance ID demux).
func dispatchHelloV6Instance(t *testing.T, eng *engine, ifindex int, router types.RouterID, area types.AreaID, src, dst netip.Addr, instance uint8, h ospfv3packet.Hello) {
	t.Helper()
	p := ospfv3packet.Packet{
		Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeHello, RouterID: ospfv3types.RouterID(router), AreaID: ospfv3types.AreaID(area), InstanceID: ospfv3types.InstanceID(instance)},
		Hello:  &h,
	}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	ospfv3packet.FinalizePacketChecksum(src, dst, buf)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: ifindex, Src: src, Dst: dst, Payload: buf})
}

// TestOSPFEngineIPv6InstanceIDMismatchDropped proves the RFC 5340 sec 4.2.2 Instance ID demux:
// the engine runs the default Instance 0, so an OSPFv3 Hello carrying a different Instance ID
// is discarded (forms no neighbor) while a matching Instance-0 Hello forms the neighbor.
func TestOSPFEngineIPv6InstanceIDMismatchDropped(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	peer := ridOf("10.0.0.2")
	src := netip.MustParseAddr("fe80::2")
	dst := netip.MustParseAddr("ff02::5")
	area := cfg.Areas[0].AreaID
	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}
	hello := ospfv3packet.Hello{InterfaceID: 2, Priority: 1, Options: ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR, HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval}

	// Instance 7 != the engine's Instance 0 -> dropped, no neighbor.
	dispatchHelloV6Instance(t, eng, handle.ifindex, peer, area, src, dst, 7, hello)
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("Instance-7 Hello formed %d neighbor(s); want 0 (RFC 5340 sec 4.2.2 demux)", len(rows))
	}

	// Instance 0 matches the engine -> neighbor forms.
	dispatchHelloV6Instance(t, eng, handle.ifindex, peer, area, src, dst, 0, hello)
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("Instance-0 Hello formed %d neighbor(s); want 1", len(rows))
	}
}

// dispatchDBDescV6 encodes an OSPFv3 Database Description and feeds it through the engine's
// real receive path (the v6 codec), so the shared neighbor FSM advances on v6 wire bytes.
func dispatchDBDescV6(t *testing.T, eng *engine, ifindex int, router types.RouterID, area types.AreaID, src, dst netip.Addr, dd ospfv3packet.DBDesc) {
	t.Helper()
	p := ospfv3packet.Packet{
		Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeDBDesc, RouterID: ospfv3types.RouterID(router), AreaID: ospfv3types.AreaID(area)},
		DBDesc: &dd,
	}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	ospfv3packet.FinalizePacketChecksum(src, dst, buf)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: ifindex, Src: src, Dst: dst, Payload: buf})
}

// bringV6NeighborFull stands up a v6 engine (real dispatch + v6 codec) and drives a
// point-to-point neighbor to Full via a dispatched two-way OSPFv3 Hello and Database
// Description exchange, returning the engine and the peer's identity/source for follow-on
// packet injection. MinLSInterval is shrunk so a re-origination after Full (which the 1s
// refresh ticker performs in production) is not rate-limited in this microsecond-scale test.
func bringV6NeighborFull(t *testing.T) (eng *engine, ifindex int, peer types.RouterID, src netip.Addr, area types.AreaID) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng = newEngineWithCodecAF(transport.New(fb), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	t.Cleanup(eng.shutdown)
	eng.lsdb.SetTimers(ospflsdb.TimerConfig{MinLSInterval: time.Nanosecond})

	peer = ridOf("10.0.0.2")
	src = netip.MustParseAddr("fe80::2")
	dst := netip.MustParseAddr("ff02::5")
	area = cfg.Areas[0].AreaID
	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}
	ifindex = handle.ifindex
	ddOpts := ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR

	// Two-way Hello (it lists our Router ID) drives the point-to-point neighbor to ExStart.
	dispatchHelloV6(t, eng, ifindex, peer, area, src, dst, ospfv3packet.Hello{
		InterfaceID:        2,
		Priority:           1,
		Options:            ddOpts,
		HelloInterval:      DefaultHelloInterval,
		RouterDeadInterval: DefaultDeadInterval,
		Neighbors:          []ospfv3types.RouterID{ospfv3types.RouterID(cfg.RouterID)},
	})
	// DD exchange: the peer (higher Router ID) is master; with empty databases the slave
	// reaches Full after the master's initial and final DDs.
	dispatchDBDescV6(t, eng, ifindex, peer, area, src, dst, ospfv3packet.DBDesc{
		InterfaceMTU: 1500, Options: ddOpts,
		Flags:      ospfv3packet.DDFlagInit | ospfv3packet.DDFlagMore | ospfv3packet.DDFlagMaster,
		DDSequence: 7,
	})
	dispatchDBDescV6(t, eng, ifindex, peer, area, src, dst, ospfv3packet.DBDesc{
		InterfaceMTU: 1500, Options: ddOpts,
		Flags:      ospfv3packet.DDFlagMaster,
		DDSequence: 8,
	})
	return eng, ifindex, peer, src, area
}

// TestOSPFEngineIPv6AdjacencyFull drives the whole OSPFv3 receive + FSM + LSDB + origination
// stack in-process on any platform: the shared neighbor FSM reaches Full over the v6 codec,
// and reaching Full triggers v6 self-origination of the address-free Router-LSA (RFC 5340 App
// A.4.3) carrying a point-to-point link to the peer. It also exercises the AF-correct LSDB
// lookup (the stored LSA's 16-bit LS Type survives LookupLSA). The end-to-end FRR interop is
// the QEMU lab; this is the strongest single-host proof of the v6 origination path.
func TestOSPFEngineIPv6AdjacencyFull(t *testing.T) {
	eng, _, peer, _, area := bringV6NeighborFull(t)
	local := ridOf("10.0.0.1")

	rows := eng.neighborSnapshot()
	if len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1", len(rows))
	}
	snap, ok := rows[0].(ospfneighbor.Snapshot)
	if !ok {
		t.Fatalf("snapshot row type = %T, want neighbor.Snapshot", rows[0])
	}
	if snap.State != ospflsdb.NeighborStateFull || snap.RouterID != "10.0.0.2" {
		t.Fatalf("neighbor snapshot = %+v, want full peer 10.0.0.2 over the v6 codec", snap)
	}

	// The self-origination produced this router's address-free Router-LSA with the adjacency.
	lsa, found := eng.lsdb.LookupLSA(area, v6RouterKey(local))
	if !found {
		t.Fatalf("v6 Router-LSA not self-originated after reaching Full")
	}
	if lsa.Header.Type != types.LSType(ospfv3types.LSTypeRouter) {
		t.Fatalf("self Router-LSA neutral type = %#x, want 0x2001 (LookupLSA must keep the v6 LS Type)", uint16(lsa.Header.Type))
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatalf("self-originated v6 Router-LSA has an invalid Fletcher checksum")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if len(body.Links) != 1 || body.Links[0].Type != ospfv3packet.RouterLinkTypeP2P || body.Links[0].NeighborRouterID != ospfv3types.RouterID(peer) {
		t.Fatalf("self Router-LSA links = %+v, want one point-to-point link to the peer", body.Links)
	}
}

// dispatchLSUpdateV6 floods one or more OSPFv3 LSAs to the engine through its real receive
// path (the v6 codec), as a neighbor's Link State Update would.
func dispatchLSUpdateV6(t *testing.T, eng *engine, ifindex int, router types.RouterID, area types.AreaID, src, dst netip.Addr, lsas ...ospfv3packet.LSA) {
	t.Helper()
	up := ospfv3packet.LSUpdate{LSAs: lsas}
	p := ospfv3packet.Packet{
		Header:   ospfv3packet.Header{Type: ospfv3packet.PacketTypeLSUpdate, RouterID: ospfv3types.RouterID(router), AreaID: ospfv3types.AreaID(area)},
		LSUpdate: &up,
	}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	ospfv3packet.FinalizePacketChecksum(src, dst, buf)
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: ifindex, Src: src, Dst: dst, Payload: buf})
}

// TestOSPFEngineIPv6InstallsRoute is the receive-side counterpart to the origination test: a
// Full v6 neighbor floods its address-free Router-LSA (link back to us) and an
// Intra-Area-Prefix-LSA (RFC 5340 App A.4.10) for 2001:db8:2::/64. SPF over the received
// topology (BuildGraph two-way + BuildRoutes) installs an IPv6 route to that prefix with the
// next-hop resolved to the neighbor's IPv6 link-local from the adjacency table. This validates
// the real LSDB -> SPF -> route-install integration on a single host (the route computation was
// otherwise only unit-tested with a stand-in source).
func TestOSPFEngineIPv6InstallsRoute(t *testing.T) {
	eng, ifindex, peer, src, area := bringV6NeighborFull(t)
	local := ridOf("10.0.0.1")
	dst := netip.MustParseAddr("ff02::5")

	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:2::/64"), 5)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	routerLSA := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeRouter, AdvertisingRouter: ospfv3types.RouterID(peer), Sequence: ospfv3types.InitialSequenceNumber},
		Router: &ospfv3packet.RouterLSA{
			Options: ospfv3types.OptV6 | ospfv3types.OptR,
			Links:   []ospfv3packet.RouterLink{{Type: ospfv3packet.RouterLinkTypeP2P, Metric: 1, NeighborRouterID: ospfv3types.RouterID(local)}},
		},
	}
	intraLSA := ospfv3packet.LSA{
		Header:       ospfv3packet.LSAHeader{Age: 1, Type: ospfv3types.LSTypeIntraAreaPrefix, LinkStateID: ospfv3types.LinkStateID{0, 0, 0, 1}, AdvertisingRouter: ospfv3types.RouterID(peer), Sequence: ospfv3types.InitialSequenceNumber},
		IntraAreaPfx: &ospfv3packet.IntraAreaPrefixLSA{ReferencedLSType: ospfv3types.LSTypeRouter, ReferencedAdvRouter: ospfv3types.RouterID(peer), Prefixes: []ospfv3packet.Prefix{prefix}},
	}
	dispatchLSUpdateV6(t, eng, ifindex, peer, area, src, dst, routerLSA, intraLSA)

	eng.spf.Run()

	want := netip.MustParsePrefix("2001:db8:2::/64")
	var got bool
	for _, r := range eng.spf.Routes() {
		if r.Prefix != want {
			continue
		}
		got = true
		if len(r.NextHops) != 1 || r.NextHops[0].Addr != src {
			t.Fatalf("route %s next-hops = %v, want the peer's link-local [%s]", r.Prefix, r.NextHops, src)
		}
	}
	if !got {
		t.Fatalf("IPv6 route %s not installed from the peer's Intra-Area-Prefix-LSA; routes = %v", want, eng.spf.Routes())
	}
}
