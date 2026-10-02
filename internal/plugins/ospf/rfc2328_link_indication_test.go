// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- link indications from the transport to OSPF.
// Related: instance.go -- newEngineWithCodecAF registers onInterfaceDown/onInterfaceUp with the transport.

package ospf

import (
	"testing"
	"time"

	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: RFC 2328 Section 4.4, link up/down indications reach the OSPF engine and change it.
// PREVENTS: an engine that no longer registers the transport's link callbacks, so a dead link
// keeps its interface and neighbors.

// linkIndicationInterface returns eth0's running OSPF interface, or nil.
func linkIndicationInterface(e *engine) *ospfiface.Interface {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.interfaces["eth0"]
}

// linkIndicationNeighborState returns neighbor 10.0.0.2's state on eth0, or "" when absent.
func linkIndicationNeighborState(e *engine) string {
	for _, row := range e.neighborSnapshot() {
		snap, ok := row.(ospfneighbor.Snapshot)
		if ok && snap.Interface == "eth0" && snap.RouterID == "10.0.0.2" {
			return snap.State
		}
	}
	return ""
}

// RFC requirement: RFC2328-4.4-3 positive -- indications are passed from the lower-level protocol to OSPF as the network interface goes up and down: on an engine built over the real transport, the transport's link-down for eth0 puts the OSPF interface in state Down and its neighbor in state Down, and the following link-up brings eth0 back to Point-to-point (newEngineWithCodecAF registers onInterfaceDown/onInterfaceUp; Transport.HandleLinkDown/HandleLinkUp call them).
func TestRFC2328LinkIndicationsReachTheOSPFEngine(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	tr := transport.New(&fakeBackend{})
	eng := newEngine(tr)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	defer eng.shutdown()

	ifc := linkIndicationInterface(eng)
	if ifc == nil {
		t.Fatal("eth0 runtime interface missing")
	}
	if got := ifc.State(); got != ospfiface.StatePointToPoint {
		t.Fatalf("eth0 before the link change: state %v, want Point-to-point; the setup is void", got)
	}
	peer := mustRouterID(t, "10.0.0.2")
	hello := packet.Hello{HelloInterval: DefaultHelloInterval, Options: types.OptionE, Priority: 1,
		DeadInterval: uint32(DefaultDeadInterval), Neighbors: []types.RouterID{cfg.RouterID}}
	if reason := ifc.ReceiveHello(peer, hello, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if got := linkIndicationNeighborState(eng); got == "" || got == "down" {
		t.Fatalf("neighbor 10.0.0.2 before the link change: state %q, want an up state; the setup is void", got)
	}

	if err := tr.HandleLinkDown("eth0"); err != nil {
		t.Fatalf("HandleLinkDown: %v", err)
	}
	if got := ifc.State(); got != ospfiface.StateDown {
		t.Fatalf("eth0 after link down: state %v, want Down (the indication did not reach OSPF)", got)
	}
	if got := linkIndicationNeighborState(eng); got != "down" {
		t.Fatalf("neighbor 10.0.0.2 after link down: state %q, want down (the indication did not reach OSPF)", got)
	}

	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}
	up := linkIndicationInterface(eng)
	if up == nil {
		t.Fatal("eth0 after link up: no running OSPF interface (the indication did not reach OSPF)")
	}
	if got := up.State(); got != ospfiface.StatePointToPoint {
		t.Fatalf("eth0 after link up: state %v, want Point-to-point (the indication did not reach OSPF)", got)
	}
}
