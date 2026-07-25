// VALIDATES: spec-ospf-ext-10 AC-2b, AC-13, A-6, A-8, A-9, A-11, R-4 (IPv6 family) -- the v6
// request builder produces the link-local pair (never an IPv4 address, zone preserved); a v6
// engine opens a single-hop session on Full; a v6 BFD Down drives the NSM down; a v4 and a v6
// session on the same link have distinct keys; the per-engine client map is isolated so a v6
// down never touches a v4 neighbor.
// PREVENTS: a v6 engine opening an IPv4 session, a lost IPv6 zone, key collision across
// families, or a cross-engine map cross-fire.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const bfdTestPeerLL = "fe80::2%eth0"

// bfdTestEngineV6 mirrors bfdTestEngine but drives the IPv6 (OSPFv3) family (v6 codec, no
// transport). interfaceIPv6LinkLocal resolves via the OS, which returns the zero Addr in a
// unit test; the engine tolerates a zero Local (kernel source selection), so the request's
// Peer (the neighbor link-local) is what the assertions pin.
func bfdTestEngineV6(t *testing.T, bfd bfdInterfaceConfig, svc api.Service) (*engine, *bfdMetricRegistry) {
	t.Helper()
	e := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	reg := newBFDMetricRegistry()
	e.setBFDMetrics(reg)
	e.mu.Lock()
	e.running[bfdTestIface] = interfaceConfig{Name: bfdTestIface, AreaID: types.BackboneArea, Enabled: true, NetworkType: networkPointToPoint, BFD: bfd}
	e.mu.Unlock()
	e.neighbors.ConfigureInterface(ospfneighbor.InterfaceConfig{
		Name: bfdTestIface, AreaID: types.BackboneArea, RouterID: ridMust(t, bfdTestLocalRID),
		NetworkType: networkPointToPoint, Options: types.OptionE, DeadInterval: 40, InterfaceMTU: 1500,
	})
	api.SetService(svc)
	t.Cleanup(func() { api.SetService(nil); e.shutdown() })
	return e, reg
}

// ---- pure v6 request builder (AC-13, R-2, A-8) ----

func TestBFDRequestForNeighborV6(t *testing.T) {
	peerLL := netip.MustParseAddr(bfdTestPeerLL)   // link-local with zone
	ifaceLL := netip.MustParseAddr("fe80::1%eth0") // interface link-local
	cfg := bfdInterfaceConfig{Enabled: true, MinTxUs: 50000, MinRxUs: 50000, Multiplier: 3}
	req := bfdRequestForNeighborV6(peerLL, ifaceLL, "eth0", cfg)
	if req.Peer != peerLL {
		t.Fatalf("Peer = %v, want %v (link-local incl. zone preserved)", req.Peer, peerLL)
	}
	if req.Local != ifaceLL {
		t.Fatalf("Local = %v, want %v (interface link-local)", req.Local, ifaceLL)
	}
	if !req.Peer.Is6() || req.Peer.Is4() {
		t.Fatalf("Peer %v is not an IPv6 address", req.Peer)
	}
	if req.Mode != api.SingleHop {
		t.Fatalf("Mode = %v, want SingleHop", req.Mode)
	}
	if req.Peer.Zone() != "eth0" {
		t.Fatalf("Peer zone = %q, want eth0", req.Peer.Zone())
	}
}

// ---- v6 session opened on Full uses the link-local pair (AC-2b, A-6) ----

func TestOSPFv3BFDSessionOpenedOnFull(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngineV6(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerLL))

	e.bfdNeighborFull(fullSnap(id))

	if svc.ensure.Load() != 1 {
		t.Fatalf("EnsureSession calls = %d, want 1", svc.ensure.Load())
	}
	req := svc.firstRequest(t)
	if !req.Peer.Is6() || req.Peer.Is4() {
		t.Fatalf("v6 request Peer = %v, want an IPv6 link-local (never IPv4)", req.Peer)
	}
	if req.Mode != api.SingleHop {
		t.Fatalf("Mode = %v, want SingleHop", req.Mode)
	}
	if reg.get("ze_ospf_bfd_sessions") != 1 {
		t.Fatalf("sessions gauge = %d, want 1", reg.get("ze_ospf_bfd_sessions"))
	}
}

// TestOSPFv3BFDUsesLinkLocalPair asserts the v6 dispatch never emits an IPv4 Peer even when
// the family gate is engaged (A-6).
func TestOSPFv3BFDUsesLinkLocalPair(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngineV6(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerLL))
	e.bfdNeighborFull(fullSnap(id))
	req := svc.firstRequest(t)
	if req.Peer.Is4() {
		t.Fatalf("v6 engine produced an IPv4 Peer %v", req.Peer)
	}
}

// ---- v6 only at Full ----

func TestOSPFv3BFDOnlyAtFull(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngineV6(t, enabledBFD(), svc)
	e.bfdNeighborFull(fullSnap(ridMust(t, "10.0.0.9")))
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession = %d, want 0 (no Full neighbor address)", svc.ensure.Load())
	}
}

// ---- v6 graceful degradation ----

func TestOSPFv3BFDGracefulWhenPluginAbsent(t *testing.T) {
	e, reg := bfdTestEngineV6(t, enabledBFD(), nil)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerLL))
	e.bfdNeighborFull(fullSnap(id))
	if reg.get("ze_ospf_bfd_register_failures_total") != 1 {
		t.Fatalf("register failures = %d, want 1", reg.get("ze_ospf_bfd_register_failures_total"))
	}
	if reg.get("ze_ospf_bfd_sessions") != 0 {
		t.Fatal("a session was opened despite the missing plugin")
	}
}

// ---- v6 BFD Down drives NeighborDown (AC-5) ----

func TestOSPFv3BFDDownDrivesNeighborDown(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngineV6(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerLL))
	e.bfdNeighborFull(fullSnap(id))
	svc.handleFor(t, svc.firstRequest(t).Key()).emit(t, packet.StateDown, packet.DiagControlDetectExpired)
	waitFor(t, func() bool {
		snap, ok := e.neighbors.Lookup(bfdTestIface, id)
		return ok && snap.State == "down"
	})
	if reg.get("ze_ospf_bfd_session_down_total") != 1 {
		t.Fatalf("session_down_total = %d, want 1", reg.get("ze_ospf_bfd_session_down_total"))
	}
}

// ---- v6 reload enable opens for already-Full neighbors (AC-11, A-10) ----

func TestOSPFv3BFDReloadEnablesSessionForFullNeighbor(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngineV6(t, bfdInterfaceConfig{Enabled: false}, svc)
	id := ridMust(t, bfdTestPeerRID)
	driveNeighborFull(t, e, id, netip.MustParseAddr(bfdTestPeerLL))
	e.bfdNeighborFull(fullSnap(id))
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession = %d before enable, want 0", svc.ensure.Load())
	}
	desired := map[string]interfaceConfig{
		bfdTestIface: {Name: bfdTestIface, AreaID: types.BackboneArea, Enabled: true, NetworkType: networkPointToPoint, BFD: enabledBFD()},
	}
	e.reconcileBFD(desired)
	waitFor(t, func() bool { return svc.ensure.Load() == 1 })
	if reg.get("ze_ospf_bfd_sessions") != 1 {
		t.Fatalf("sessions gauge = %d after reload-enable, want 1", reg.get("ze_ospf_bfd_sessions"))
	}
}

// ---- a v4 and a v6 session on the same link are distinct; per-engine map isolation
// (AC-12, A-9, R-4) ----

func TestOSPFv3BFDDistinctFromV2OnSameLink(t *testing.T) {
	svc := newFakeBFDService()
	// One shared BFD service; two engine instances (v4 + v6) for the same interface + router.
	e4, reg4 := bfdTestEngine(t, enabledBFD(), svc)
	e6, reg6 := bfdTestEngineV6(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e4, id, netip.MustParseAddr(bfdTestPeerAddr)) // IPv4 pair
	seedNeighbor(t, e6, id, netip.MustParseAddr(bfdTestPeerLL))   // link-local pair

	e4.bfdNeighborFull(fullSnap(id))
	e6.bfdNeighborFull(fullSnap(id))

	if svc.ensure.Load() != 2 {
		t.Fatalf("EnsureSession calls = %d, want 2 (distinct v4 + v6 keys)", svc.ensure.Load())
	}
	svc.mu.Lock()
	k0, k1 := svc.requests[0].Key(), svc.requests[1].Key()
	svc.mu.Unlock()
	if k0 == k1 {
		t.Fatal("v4 and v6 sessions on the same link share a Key")
	}

	// A v6 BFD Down must release only the v6 session; the v4 engine's map is untouched (R-4).
	svc.handleFor(t, svc.requests[1].Key()).emit(t, packet.StateDown, packet.DiagControlDetectExpired)
	waitFor(t, func() bool { return reg6.get("ze_ospf_bfd_sessions") == 0 })
	if reg4.get("ze_ospf_bfd_sessions") != 1 {
		t.Fatalf("v4 sessions gauge = %d after a v6 down, want 1 (isolated)", reg4.get("ze_ospf_bfd_sessions"))
	}
	if snap, ok := e4.neighbors.Lookup(bfdTestIface, id); !ok || snap.State == "down" {
		t.Fatalf("v4 neighbor state = %v (ok=%v), want unaffected by the v6 down", snap.State, ok)
	}
}
