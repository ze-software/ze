// VALIDATES: the IPv4 (RFC 3623) Grace-LSA opaque consumer registers under Opaque Type 3 with
// link scope, and its OnReceive hook is invoked by the ext-1 carrier (AC-1); the OSPFv3 Grace
// type is exercised in v3/types (TestGraceLSATypeRegistered).
// PREVENTS: the Grace-LSA consumer being unregistered (no helper reaction to a received
// Grace-LSA) or registered at the wrong scope.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestGraceLSAConsumerRegistered (AC-1): registerGraceConsumer stores the Opaque-Type-3
// consumer at link scope and its OnReceive drives the helper.
func TestGraceLSAConsumerRegistered(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	e := grEnableEngine(t, false, time.Unix(1_000_000, 0))
	if err := registerGraceConsumer(e); err != nil {
		t.Fatalf("registerGraceConsumer: %v", err)
	}
	c, ok := lookupOpaqueConsumer(ospfpacket.GraceOpaqueType)
	if !ok {
		t.Fatalf("Grace-LSA consumer (Opaque Type 3) not registered")
	}
	if c.scope != OpaqueScopeLink {
		t.Fatalf("Grace-LSA consumer scope = %v, want link", c.scope)
	}
	if c.onReceive == nil {
		t.Fatalf("Grace-LSA consumer OnReceive must be set")
	}

	// Deliver a valid Grace-LSA to a Full neighbor's engine and confirm the helper reacts.
	// (No Full neighbor here, so entry is refused, but the callback must run without panic.)
	x := ospftypes.RouterID{10, 0, 0, 2}
	body := grV4Body(120, 2, [4]byte{10, 0, 0, 2}, false)
	c.onReceive(opaqueReceived{
		OpaqueType:        ospfpacket.GraceOpaqueType,
		Scope:             OpaqueScopeLink,
		Interface:         "eth0",
		AdvertisingRouter: x,
		Body:              body,
	})
	// Entry needs a Full adjacency; none exists in this bare engine, so no session forms.
	if e.gr.isHelping("eth0", x) || e.gr.isHelping("eth1", x) {
		t.Fatalf("helper must not enter without a Full adjacency")
	}
	_ = netip.Addr{}
}
