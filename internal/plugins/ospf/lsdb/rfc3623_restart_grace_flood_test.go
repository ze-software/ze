// VALIDATES: RFC 3623 Section 5 on the restarting router: a Grace-LSA originated while in
// restart is sent in a Link State Update on its interface even when no neighbor is known, and
// on a broadcast network it goes to AllSPFRouters although the router is neither DR nor Backup.
// PREVENTS: floodLink dropping the Grace-LSA because no neighbor qualified for the retransmit
// list, or addressing it to AllDRouters, which only the DR and Backup receive.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// graceFloodTargets originates a Grace-LSA on a broadcast interface in state DROther with no
// neighbors, with the router in restart or not, and returns every destination it was sent to.
func graceFloodTargets(t *testing.T, restarting bool) []netip.Addr {
	t.Helper()
	db := newTestDB(&fakeClock{now: time.Unix(0, 0)})
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{Name: "eth0", AreaID: types.BackboneArea, NetworkType: types.NetworkBroadcast, State: "drother", Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1")}}
	})
	var targets []netip.Addr
	db.SetTx(func(_ string, dst netip.Addr, _ []byte) error {
		targets = append(targets, dst)
		return nil
	})
	db.SetSelfFlushSuppress(func() bool { return restarting })
	_, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: packet.GraceOpaqueType, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Options: types.OptionO, Body: []byte{0, 1, 0, 4, 0, 0, 0, 120},
	})
	if !ok {
		t.Fatalf("Grace-LSA origination refused")
	}
	return targets
}

func TestRFC3623RestartGraceLSAFloodedToAllSPFRouters(t *testing.T) {
	// RFC requirement: RFC3623-5-3 positive -- in restart, with no adjacency on the
	// interface, the Grace-LSA is still sent in a Link State Update.
	// RFC requirement: RFC3623-5-2 positive -- on a broadcast network it is sent to
	// AllSPFRouters (224.0.0.5), not AllDRouters, although the router is neither DR nor Backup.
	targets := graceFloodTargets(t, true)
	if len(targets) != 1 || targets[0] != transport.AllSPFRouters {
		t.Fatalf("restarting router sent the Grace-LSA to %v, want exactly [%v]", targets, transport.AllSPFRouters)
	}
	// Outside restart the ordinary flooding rule holds: no eligible neighbor, nothing sent.
	if targets := graceFloodTargets(t, false); len(targets) != 0 {
		t.Fatalf("router not in restart sent a Grace-LSA with no eligible neighbor: %v", targets)
	}
}
