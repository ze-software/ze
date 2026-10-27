// Design: docs/architecture/ospf/ospf-4-component-config.md -- engine receive worker
//
// VALIDATES: a panic raised while the receive worker holds the engine mutex does not leave
// the mutex held, so the engine keeps serving every other packet after the worker's
// boundary contains the panic.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestOSPFReceivePanicReleasesEngineLock drives handleHello into a panic inside its
// mu-held interface lookup: a nil virtual-link runtime makes virtualLinkTargetLocked
// dereference nil for a backbone Hello from that neighbor on the transit interface. The
// test recovers the panic the way the worker boundary does, then requires mu to be free
// and a Hello on another interface to be processed. Before the lookup released mu by
// defer, TryLock failed here and every later packet of the engine blocked forever.
func TestOSPFReceivePanicReleasesEngineLock(t *testing.T) {
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{
			"0.0.0.0":{"area-id":"0.0.0.0"},
			"0.0.0.1":{"area-id":"0.0.0.1","virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
			"eth1":{"area":"0.0.0.1","network-type":"point-to-point"},
			"lo":{"area":"0.0.0.0","network-type":"loopback"}}}}}`
	eng, fb := vlEngine(t, j)
	defer eng.shutdown()
	transit := vlArea(t, "0.0.0.1")
	peer := vlRID(t, "10.0.0.2")

	eng.mu.Lock()
	eng.virtualLinks[virtualLinkKey{transit: transit, neighbor: peer}] = nil
	eng.mu.Unlock()

	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		eng.handleHello(
			transport.RawPacket{IfIndex: vlIfindex(t, fb, "eth0"), Src: netip.MustParseAddr("192.0.2.2")},
			Header{AreaID: types.BackboneArea, RouterID: peer},
		)
		return nil
	}()
	if panicked == nil {
		t.Fatal("handleHello did not panic on the nil virtual-link runtime: the seam no longer reaches the mu-held lookup")
	}

	if !eng.mu.TryLock() {
		t.Fatal("engine mutex still held after a contained receive panic: every later packet of this engine would block")
	}
	delete(eng.virtualLinks, virtualLinkKey{transit: transit, neighbor: peer})
	eng.mu.Unlock()

	name, _, ok := eng.receiveTarget(vlIfindex(t, fb, "eth1"), Header{AreaID: transit, RouterID: vlRID(t, "10.0.0.3")})
	if !ok || name != "eth1" {
		t.Fatalf("receiveTarget(eth1) after the panic = %q, %v; want eth1, true", name, ok)
	}
}
