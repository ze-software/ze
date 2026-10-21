// Design: docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md -- LDP-IGP synchronization at origination.
// Related: origination_v6.go -- v6RouterLSABody, the OSPFv3 Router-LSA link builder these tests drive.
// Related: rfc5443_origination_test.go -- the OSPFv2 tests over the same LDP-sync machine.
//
// VALIDATES: RFC 5443 section 2 on the OSPFv3 Router-LSA. Section 2 says "In the case of
// OSPF" with no version split, so the OSPFv3 point-to-point link is advertised at LSInfinity
// 0xFFFF while LDP is not fully operational, and at the configured cost once it is.
// Method: the production applyLDPSyncOverride runs over an OSPFv3 interface snapshot, the
// engine originates the OSPFv3 Router-LSA into a fresh LSDB, and the installed LSA is decoded.
// PREVENTS: an OSPFv3 Router-LSA that ignores the LDP-sync cost-out the OSPFv2 path honors.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// rfc5443V6LinkMetric builds eth1's OSPFv3 point-to-point snapshot at the configured cost,
// applies the production LDP-sync override for eng's current state, originates the OSPFv3
// Router-LSA, and returns the metric of its one point-to-point link.
func rfc5443V6LinkMetric(t *testing.T, eng *engine) uint16 {
	t.Helper()
	self := types.RouterID{1, 1, 1, 1}
	info := v6P2PInterface(types.BackboneArea, self, types.RouterID{2, 2, 2, 2})
	info.Name = "eth1"
	ic := eng.cfg.Interfaces[0]
	info.Cost = interfaceCost(ic, eng.cfg.ReferenceBandwidth)
	eng.applyLDPSyncOverride(&info, ic)

	origin := newV6OriginEngine()
	opts := ospfv3types.OptV6 | ospfv3types.OptR
	if _, ok := origin.v6OriginateRouter(types.BackboneArea, self, opts, []ospflsdb.InterfaceInfo{info}, false, false, false, false, false); !ok {
		t.Fatal("v6OriginateRouter returned false")
	}
	body := v6DecodeBackboneRouter(t, origin, types.BackboneArea, self)
	var metric uint16
	count := 0
	for _, l := range body.Links {
		if l.Type == ospfv3packet.RouterLinkTypeP2P {
			metric = l.Metric
			count++
		}
	}
	if count != 1 {
		t.Fatalf("point-to-point links = %d, want 1 (the cost-out raises the link, never withholds it): %+v", count, body.Links)
	}
	return metric
}

// TestRFC5443V6NotOperationalLinkAdvertisedAtMaxCost pins the OSPFv3 cost-out while LDP is not
// fully operational. Method: originate before LDP session-up and again during the hold-down.
//
// RFC requirement: RFC5443-2-1 positive -- while LDP is not fully operational (before session-up,
// and during the hold-down after it) the originated OSPFv3 Router-LSA advertises the
// point-to-point link at the maximum cost instead of the configured 100.
// RFC requirement: RFC5443-2-2 positive -- that maximum cost on the OSPFv3 link is exactly the
// 16-bit value 0xFFFF.
func TestRFC5443V6NotOperationalLinkAdvertisedAtMaxCost(t *testing.T) {
	eng, _ := rfc5443Engine(t, "100", "")
	if m := rfc5443V6LinkMetric(t, eng); m != 0xFFFF {
		t.Fatalf("not-synchronized OSPFv3 link metric = %#x, want 0xFFFF", m)
	}
	eng.ldpSync.onSessionUp("eth1")
	if m := rfc5443V6LinkMetric(t, eng); m != 0xFFFF {
		t.Fatalf("hold-down OSPFv3 link metric = %#x, want 0xFFFF until LDP is fully operational", m)
	}
}

// TestRFC5443V6OperationalLinkAdvertisedAtConfiguredCost pins the end of the OSPFv3 cost-out.
// Method: synchronize eth1 (session up, hold-down expired), originate, decode.
//
// RFC requirement: RFC5443-2-1 negative -- once LDP is fully operational the originated OSPFv3
// Router-LSA no longer advertises the maximum cost: the point-to-point link carries the
// configured 100.
// RFC requirement: RFC5443-2-2 negative -- a configured cost of 0xFFFE is not the cost-out
// value: while LDP is not synchronized the OSPFv3 link is advertised at 0xFFFF, and once
// synchronized it returns to 0xFFFE, so the two values are told apart.
func TestRFC5443V6OperationalLinkAdvertisedAtConfiguredCost(t *testing.T) {
	eng, ft := rfc5443Engine(t, "100", "")
	rfc5443Synchronize(t, eng, ft)
	if m := rfc5443V6LinkMetric(t, eng); m != 100 {
		t.Fatalf("synchronized OSPFv3 link metric = %d, want the configured 100", m)
	}

	near, nearTimers := rfc5443Engine(t, "65534", "")
	if m := rfc5443V6LinkMetric(t, near); m != 0xFFFF {
		t.Fatalf("not-synchronized OSPFv3 link metric = %#x, want 0xFFFF, not the configured 0xFFFE", m)
	}
	rfc5443Synchronize(t, near, nearTimers)
	if m := rfc5443V6LinkMetric(t, near); m != 0xFFFE {
		t.Fatalf("synchronized OSPFv3 link metric = %#x, want the configured 0xFFFE", m)
	}
}
