// VALIDATES: spec-ospf-ext-4 AC-1/A-1 -- the Extended Prefix (Opaque Type 7) and Extended
// Link (Opaque Type 8) consumers register with the ext-1 carrier at the correct scopes, and
// the engine discovers both at startup.
// PREVENTS: a silently unregistered consumer whose OnOriginate/OnReceive is never invoked.
package ospf

import (
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// extEngineWithTopology builds an OSPFv2 engine (opaque + extended-prefix/link enabled),
// installs the self LSAs for a canned live topology, and returns it ready for an Extended
// Prefix/Link origination pass.
func extEngineWithTopology(t *testing.T, topo []ospflsdb.InterfaceInfo) (*engine, types.RouterID) {
	t.Helper()
	eng, router := newRedistEngine(t, extOrigCfg)
	// Drop the RFC 2328 sec 9.5 MinLSInterval floor so a same-test re-origination after a
	// topology change is not rate-limited (tests shorten timers instead of sleeping).
	eng.lsdb.SetTimers(ospflsdb.TimerConfig{MinLSInterval: time.Nanosecond})
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return topo })
	eng.lsdb.OriginateFromTopology(router, false)
	return eng, router
}

// extStubIface returns a backbone interface that advertises one connected stub prefix
// (network+mask), so the self Router-LSA carries a stub link for that prefix.
func extStubIface(name string, addr, mask [4]byte) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{Name: name, AreaID: types.BackboneArea, Address: addr, NetworkMask: mask, Passive: true}
}

func TestExtPrefixRegistersAsOpaqueConsumer(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng := newEngine(transport.New(&fakeBackend{}))
	if err := registerExtConsumers(eng); err != nil {
		t.Fatalf("registerExtConsumers: %v", err)
	}
	c, ok := lookupOpaqueConsumer(packet.ExtPrefixOpaqueType)
	if !ok {
		t.Fatalf("Opaque Type 7 not registered")
	}
	if c.scope != OpaqueScopeArea {
		t.Fatalf("Opaque Type 7 registered scope = %v, want area (AS via per-origination override)", c.scope)
	}
	if c.onOriginate == nil || c.onReceive == nil {
		t.Fatalf("Opaque Type 7 callbacks nil: %+v", c)
	}
}

func TestExtLinkRegistersAsOpaqueConsumer(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng := newEngine(transport.New(&fakeBackend{}))
	if err := registerExtConsumers(eng); err != nil {
		t.Fatalf("registerExtConsumers: %v", err)
	}
	c, ok := lookupOpaqueConsumer(packet.ExtLinkOpaqueType)
	if !ok {
		t.Fatalf("Opaque Type 8 not registered")
	}
	if c.scope != OpaqueScopeArea {
		t.Fatalf("Opaque Type 8 registered scope = %v, want area only", c.scope)
	}
	if c.onOriginate == nil || c.onReceive == nil {
		t.Fatalf("Opaque Type 8 callbacks nil: %+v", c)
	}
	// Both types are discoverable in the process-global registry.
	if len(opaqueConsumerSnapshot()) != 2 {
		t.Fatalf("consumer snapshot = %d, want 2 (types 7 and 8)", len(opaqueConsumerSnapshot()))
	}
}
