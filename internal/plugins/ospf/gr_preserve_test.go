// VALIDATES: the RFC 5187 sec 3.1 / sec 3.2 OSPFv3 preservation across a restart -- the
// LSA-ID -> prefix correspondence for redistributed External LSAs (AC-10) and the OSPFv3
// Interface ID per interface (AC-9) are captured, persisted, and restored so re-originated
// LSAs carry the same identifiers as before the restart.
// PREVENTS: network churn (a prefix re-originated under a different LSA-ID) or a silently
// terminated restart (a renumbered Interface ID mismatching neighbor adjacency state).
package ospf

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// TestLSAIDPrefixCorrespondencePreserved (AC-10, A-12, R-7): a redistributed IPv6 prefix's
// arbitrary 32-bit LSA ID survives capture -> persist form -> restore into a fresh engine.
func TestLSAIDPrefixCorrespondencePreserved(t *testing.T) {
	src := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	pfx := netip.MustParsePrefix("2001:db8:1::/48")
	src.redistV6[pfx] = v6SummaryLSID(77)

	captured := src.capturePrefixLSIDs()
	if captured[pfx.String()] != 77 {
		t.Fatalf("capturePrefixLSIDs: got %v want lsid 77 for %s", captured, pfx)
	}

	// Simulate the restart: a fresh engine restores the map.
	dst := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	dst.restorePrefixLSIDs(captured)
	if got := dst.redistV6[pfx]; got != v6SummaryLSID(77) {
		t.Fatalf("restorePrefixLSIDs: prefix %s got LSID %v want %v", pfx, got, v6SummaryLSID(77))
	}
}

type grHelloTransport struct {
	Transport
	hellos chan []byte
}

func (r *grHelloTransport) SendPacket(name string, dst netip.Addr, payload []byte) error {
	if header, err := (v6Codec{}).DecodeHeader(payload); err == nil && header.Type == PacketTypeHello {
		select {
		case r.hellos <- append([]byte(nil), payload...):
		default:
		}
	}
	return r.Transport.SendPacket(name, dst, payload)
}

// Restored protocol identity must reach the wire and native SPF even when the
// transport uses another kernel index, including after restart suppression ends.
func TestInterfaceIDPreservedAcrossRestart(t *testing.T) {
	const iface = "gr-test"
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"gr-test":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := &fakeV6Backend{nextIdx: 40}
	recorder := &grHelloTransport{Transport: ospfv3transport.New(backend), hellos: make(chan []byte, 2)}
	eng := newEngineWithCodecAF(recorder, v6Codec{}, afIPv6Unicast)
	t.Cleanup(eng.shutdown)
	// Keep installation local to this fixture; the production engine still drives
	// enrollment, packet dispatch, neighbor events and native LSA origination.
	eng.spf.Stop()
	eng.setConfig(cfg)
	eng.state = newFakeGRStore()
	protocolID := interfaceIndex(iface) + 1000
	peer := ospftypes.RouterID{10, 0, 0, 2}
	fact := restartFact{
		Restarting: true, GraceEndUnix: time.Now().Add(5 * time.Minute).Unix(),
		Reason: grReasonReload, Expected: []string{peer.String()},
		InterfaceIDs: map[string]uint32{iface: protocolID},
	}
	if err := writeRestartFact(context.Background(), eng.state, eng.grFactKey(), fact); err != nil {
		t.Fatal(err)
	}
	if err := eng.openInterfaces(); err != nil {
		t.Fatal(err)
	}
	if !eng.gr.inRestart() {
		t.Fatal("active restart fact did not suppress origination")
	}
	eng.mu.Lock()
	runtime := eng.interfaces[iface]
	eng.mu.Unlock()
	checkHello := func() {
		t.Helper()
		if runtime == nil {
			t.Fatal("configured interface was not enrolled")
		}
		if err := runtime.SendHello(); err != nil {
			t.Fatal(err)
		}
		select {
		case payload := <-recorder.hellos:
			hello, err := (v6Codec{}).DecodeHello(payload)
			if err != nil || hello.InterfaceID != protocolID {
				t.Fatalf("emitted Hello identity = %d, want restored %d: %v", hello.InterfaceID, protocolID, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no native Hello emitted")
		}
	}
	checkHello()
	backend.mu.Lock()
	kernelID := backend.handles[iface].ifindex
	backend.mu.Unlock()
	if uint32(kernelID) == protocolID {
		t.Fatal("fixture must use distinct kernel and protocol identifiers")
	}
	area := ospftypes.BackboneArea
	src, dst := netip.MustParseAddr("fe80::2"), netip.MustParseAddr("ff02::5")
	opts := ospfv3types.OptE | ospfv3types.OptV6 | ospfv3types.OptR
	dispatchHelloV6(t, eng, kernelID, peer, area, src, dst, ospfv3packet.Hello{
		InterfaceID: 22, Priority: 1, Options: opts,
		HelloInterval: DefaultHelloInterval, RouterDeadInterval: DefaultDeadInterval,
		Neighbors: []ospfv3types.RouterID{ospfv3types.RouterID(cfg.RouterID)},
	})
	dispatchDBDescV6(t, eng, kernelID, peer, area, src, dst, ospfv3packet.DBDesc{
		InterfaceMTU: 1500, Options: opts, DDSequence: 7,
		Flags: ospfv3packet.DDFlagInit | ospfv3packet.DDFlagMore | ospfv3packet.DDFlagMaster,
	})
	dispatchDBDescV6(t, eng, kernelID, peer, area, src, dst, ospfv3packet.DBDesc{
		InterfaceMTU: 1500, Options: opts, DDSequence: 8, Flags: ospfv3packet.DDFlagMaster,
	})
	if row, ok := eng.neighbors.Lookup(iface, peer); !ok || row.State != neighborStateFull {
		t.Fatalf("neighbor through kernel index did not reach Full: %+v", row)
	}
	if eng.gr.inRestart() {
		t.Fatal("Full adjacency did not end restart suppression")
	}
	// The local Router-LSA comes from the production exit/origination path, not
	// a fixture whose Interface ID could accidentally match the neighbor config.
	remote := v6SelfLSA(ospfv3packet.LSA{
		Header: v6OriginHeader(ospfv3types.LSTypeRouter, ospfv3types.LinkStateID{}, peer, ospftypes.InitialSequenceNumber, false),
		Router: &ospfv3packet.RouterLSA{Options: opts, Links: []ospfv3packet.RouterLink{{
			Type: ospfv3packet.RouterLinkTypeP2P, Metric: 10, InterfaceID: 22,
			NeighborInterfaceID: ospfv3types.InterfaceID(protocolID), NeighborRouterID: ospfv3types.RouterID(cfg.RouterID),
		}}},
	})
	if !eng.lsdb.Install(area, remote) {
		t.Fatal("install remote Router-LSA")
	}
	prefix := netip.MustParsePrefix("2001:db8:42::/64")
	installV6IntraPrefix(t, eng.lsdb, area, peer, prefix.String())
	loc := locrib.NewRIB()
	computer := ospfspf.NewComputer(ospfspf.Config{
		Source: eng.lsdb, Root: cfg.RouterID, Areas: []ospftypes.AreaID{area},
		Strategy: v6Strategy{eng: eng}, Installer: ospfspf.NewInstallerFamily(loc, family.IPv6Unicast),
	})
	t.Cleanup(computer.Stop)
	computer.Run()
	if route, ok := loc.Best(family.IPv6Unicast, prefix); !ok || route.NextHop != src || route.Interface != iface || !route.OnLink {
		t.Fatalf("native route lost restored adjacency scope after restart: %+v", route)
	}
	checkHello()
}

// TestLSIDRoundTrip guards the uint32 <-> LinkStateID conversion the preservation maps use.
func TestLSIDRoundTrip(t *testing.T) {
	for _, v := range []uint32{0, 1, 77, 0xFFFFFFFF} {
		if got := lsidToUint32(v6SummaryLSID(v)); got != v {
			t.Fatalf("lsidToUint32(v6SummaryLSID(%d)) = %d", v, got)
		}
	}
	_ = ospftypes.LinkStateID{}
}

func TestGRHelperContentChangeReleasesManagerLock(t *testing.T) {
	eng := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	eng.gr.configure(gracefulRestartConfig{StrictLSAChecking: true})
	eng.running["gr-test"] = interfaceConfig{Name: "gr-test", AreaID: ospftypes.BackboneArea}
	key := helperKey{iface: "gr-test", router: ospftypes.RouterID{10, 0, 0, 2}}
	eng.gr.helperEnter(key, graceReceived{gracePeriod: 120}, false, netip.MustParseAddr("fe80::2"), 22)
	done := make(chan struct{})
	go func() {
		eng.gr.onContentChange(ospftypes.BackboneArea, ospftypes.LSType(ospfv3types.LSTypeRouter))
		close(done)
	}()
	select {
	case <-done:
		t.Cleanup(eng.shutdown)
	case <-time.After(5 * time.Second):
		eng.cancel()
		t.Fatal("helper content-change callback deadlocked during topology lookup")
	}
	if eng.gr.isHelping(key.iface, key.router) {
		t.Fatal("topology change did not terminate the affected helper session")
	}
}

func TestGRHelperStaleExitPreservesReplacement(t *testing.T) {
	eng := newEngineWithCodecAF(nil, v6Codec{}, afIPv6Unicast)
	t.Cleanup(eng.shutdown)
	key := helperKey{iface: "gr-test", router: ospftypes.RouterID{10, 0, 0, 2}}
	eng.gr.helperEnter(key, graceReceived{gracePeriod: 120}, false, netip.MustParseAddr("fe80::2"), 22)
	eng.gr.mu.Lock()
	old := eng.gr.helping[key]
	eng.gr.mu.Unlock()
	eng.gr.helperExit(key, old, grExitFlushed)
	eng.gr.helperEnter(key, graceReceived{gracePeriod: 120}, false, netip.MustParseAddr("fe80::2"), 22)
	// An already-computed topology-change decision belongs to the old session.
	eng.gr.helperExit(key, old, grExitTopologyChange)
	if !eng.gr.isHelping(key.iface, key.router) {
		t.Fatal("stale topology-change exit removed a new helper session")
	}
}
