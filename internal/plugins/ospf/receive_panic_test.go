// Design: docs/architecture/ospf/ospf-4-component-config.md -- engine receive worker
//
// VALIDATES: a panic while the engine's receive worker handles one packet is contained
// by the engine guard (logged, counted in ze_task_panics_total{component="ospf",
// task="receive"}, crash-recorded), takes only that packet's neighbor Down (KillNbr),
// and the same worker goes on to handle the next packet, so every other neighbor
// stays up.
package ospf

import (
	"net/netip"
	"strconv"
	"testing"
	"time"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// receivePanicPacket encodes a header-only packet of type pt from router id in the
// backbone, which the dispatcher accepts on a point-to-point interface.
func receivePanicPacket(t *testing.T, pt packet.PacketType, id types.RouterID) []byte {
	t.Helper()
	p := packet.Packet{Header: packet.Header{Type: pt, RouterID: id, AreaID: types.BackboneArea}}
	buf := make([]byte, packet.CommonHeaderLen)
	p.WriteTo(buf, 0)
	return buf
}

// requireNeighborState fails unless the neighbor id on name is in state want.
func requireNeighborState(t *testing.T, eng *engine, name string, id types.RouterID, want string) {
	t.Helper()
	snap, ok := eng.neighbors.Lookup(name, id)
	if !ok {
		t.Fatalf("neighbor %s on %s not found", id, name)
	}
	if snap.State != want {
		t.Fatalf("neighbor %s on %s is %q, want %q", id, name, snap.State, want)
	}
}

// TestOSPFReceivePanicContained runs the real receive worker over two point-to-point
// interfaces, each with one neighbor brought to Init by a Hello. The LSAck handler is
// replaced through the dispatcher's registration seam with one that panics for eth0's
// packets and reports eth1's. One LSAck from eth0's neighbor panics: the guard counts
// and records it, that neighbor goes Down, and the next LSAck, from eth1's neighbor,
// reaches the handler on the same worker while eth1's neighbor keeps its state.
func TestOSPFReceivePanicContained(t *testing.T) {
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.0","network-type":"point-to-point"},
			"eth1":{"area":"0.0.0.0","network-type":"point-to-point"}}}}}`
	before := enginePanicBaseline(t, "receive")
	eng, fb := vlEngine(t, j)
	t.Cleanup(eng.shutdown)
	local := vlRID(t, "10.0.0.1")
	broken, healthy := vlRID(t, "10.0.0.2"), vlRID(t, "10.0.0.3")

	for _, n := range []struct {
		name string
		id   types.RouterID
		addr string
	}{{"eth0", broken, "192.0.2.2"}, {"eth1", healthy, "198.51.100.3"}} {
		reason := eng.neighbors.Hello(ospfneighbor.HelloInput{
			InterfaceName: n.name, AreaID: types.BackboneArea, LocalRouterID: local,
			NeighborID: n.id, Address: netip.MustParseAddr(n.addr),
			NetworkType: "point-to-point", DeadInterval: 40,
		})
		if reason != "" {
			t.Fatalf("Hello for %s on %s refused: %s", n.id, n.name, reason)
		}
	}
	requireNeighborState(t, eng, "eth0", broken, "init")
	requireNeighborState(t, eng, "eth1", healthy, "init")

	eth0, eth1 := vlIfindex(t, fb, "eth0"), vlIfindex(t, fb, "eth1")
	handled := make(chan int, 1)
	eng.dispatch.register(PacketTypeLSAck, func(rp transport.RawPacket, _ Header) {
		if rp.IfIndex == eth0 {
			panic("test: LSAck handler seam panics on eth0")
		}
		handled <- rp.IfIndex
	})

	fb.mu.Lock()
	h0, h1 := fb.handles["eth0"], fb.handles["eth1"]
	fb.mu.Unlock()
	// Each interface has its own fan-in, so eth1's packet is sent only once eth0's
	// panic was counted: the order the worker sees them is then fixed.
	h0.recv <- transport.RawPacket{IfIndex: eth0, Payload: receivePanicPacket(t, packet.PacketTypeLSAck, broken)}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		if n, _ := enginePanics().guard.Panics("receive"); n > before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("eth0's packet never panicked the receive worker: the seam is not reached")
		}
	}
	h1.recv <- transport.RawPacket{IfIndex: eth1, Payload: receivePanicPacket(t, packet.PacketTypeLSAck, healthy)}

	select {
	case got := <-handled:
		if got != eth1 {
			t.Fatalf("handled ifindex %d, want eth1's %d", got, eth1)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the receive worker never handled eth1's packet after eth0's panic")
	}
	requireEngineContained(t, before, "receive", "packet on ifindex "+strconv.Itoa(eth0)+" from router 10.0.0.2")
	requireNeighborState(t, eng, "eth0", broken, "down")
	requireNeighborState(t, eng, "eth1", healthy, "init")
}
