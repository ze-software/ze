//go:build linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- RFC 2328 section 8.2 receive
// checks, the source-network clause of case (1).
// Related: receive_source.go -- sourceOnInterfaceNetworkLocked, the check under test.
//
// VALIDATES: RFC 2328 Section 8.2 case (1): "the packet's IP source address is required to
// be on the same network as the receiving interface", and "This comparison should not be
// performed on point-to-point networks".
// PREVENTS: a packet from off the receiving interface's network being processed as a
// single-hop packet on a broadcast interface.
package ospf

import (
	"net/netip"
	"testing"

	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // the product iface backend, so lo's real address and mask reach the engine

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// loopbackSourceEngine runs OSPF on the host's loopback interface as networkType, so the
// engine reads lo's real address and mask (127.0.0.1/8) through interfaceRuntimeConfigLocked,
// the path production config takes. It returns the engine and lo's transport handle.
func loopbackSourceEngine(t *testing.T, networkType string) (*engine, *fakeHandle) {
	t.Helper()
	if got := interfaceNetworkMask("lo"); got != [4]byte{255, 0, 0, 0} {
		t.Fatalf("precondition: lo network mask %v, want 255.0.0.0 (127.0.0.1/8)", got)
	}
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lo":{"area":"0","network-type":"`+networkType+`"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	t.Cleanup(eng.shutdown)
	fb.mu.Lock()
	handle := fb.handles["lo"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("lo transport handle missing")
	}
	return eng, handle
}

// dispatchLoopbackHello dispatches a backbone Hello from peer, sourced at source and
// carrying lo's /8 mask (so the RFC 2328 Section 10.5 mask check cannot be what refuses
// it), and returns how many packets the dispatcher dropped for it.
func dispatchLoopbackHello(eng *engine, handle *fakeHandle, peer types.RouterID, source netip.Addr) uint64 {
	// The mask is set before encoding, so the OSPF checksum covers it and a checksum
	// failure cannot be what drops the packet.
	hello := packet.Hello{NetworkMask: [4]byte{255, 0, 0, 0}, HelloInterval: DefaultHelloInterval, Options: types.OptionE, Priority: 1, DeadInterval: uint32(DefaultDeadInterval)}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: peer, AreaID: types.BackboneArea}, Hello: &hello}
	payload := make([]byte, p.EncodedLen())
	p.WriteTo(payload, 0)
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: source, Payload: payload})
	return eng.dispatch.dropped() - before
}

// RFC requirement: RFC2328-8.2-2 negative — on a broadcast interface (lo, 127.0.0.1/8) a
// Hello whose Area ID matches the interface's area but whose IP source 10.0.0.2 is off the
// interface's network is dropped before any handler runs: the drop counter rises by one and
// no neighbor forms (sourceOnInterfaceNetworkLocked, acceptsArea).
func TestOSPFReceiveDropsOffNetworkSourceOnBroadcast(t *testing.T) {
	eng, handle := loopbackSourceEngine(t, "broadcast")
	peer := ridOf("10.0.0.2")
	if got := dispatchLoopbackHello(eng, handle, peer, netip.AddrFrom4([4]byte(peer))); got != 1 {
		t.Fatalf("dropped %d, want 1: a Hello from 10.0.0.2 is off lo's 127.0.0.0/8 network", got)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows = %d, want 0 after an off-network Hello", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-2 positive — on a broadcast interface (lo, 127.0.0.1/8) a
// Hello whose Area ID matches and whose IP source 127.0.0.2 is on the interface's network
// after masking both with 255.0.0.0 is not dropped and its sender becomes a neighbor.
func TestOSPFReceiveAcceptsOnNetworkSourceOnBroadcast(t *testing.T) {
	eng, handle := loopbackSourceEngine(t, "broadcast")
	peer := ridOf("10.0.0.2")
	if got := dispatchLoopbackHello(eng, handle, peer, netip.MustParseAddr("127.0.0.2")); got != 0 {
		t.Fatalf("dropped %d, want 0: 127.0.0.2 is on lo's 127.0.0.0/8 network", got)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1 after an on-network Hello", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-2 positive — on a point-to-point interface (lo, 127.0.0.1/8)
// the source comparison is not performed: a Hello from 10.0.0.2, off the interface's
// network, is not dropped and its sender becomes a neighbor.
func TestOSPFReceiveAcceptsOffNetworkSourceOnPointToPoint(t *testing.T) {
	eng, handle := loopbackSourceEngine(t, "point-to-point")
	peer := ridOf("10.0.0.2")
	if got := dispatchLoopbackHello(eng, handle, peer, netip.AddrFrom4([4]byte(peer))); got != 0 {
		t.Fatalf("dropped %d, want 0: point-to-point skips the source-network comparison", got)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1 after a point-to-point Hello", len(rows))
	}
}
