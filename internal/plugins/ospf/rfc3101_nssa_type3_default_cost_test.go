// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA default origination.
// Related: spf_wiring.go -- configureSPF, which hands each area's default-cost to the SPF computer.
// Related: spf/area_type.go -- applyAreaTypePolicy, the Type-3 default a no-summary NSSA receives.
// Related: rfc3101_border_default_cost_test.go -- the Type-7 default of the same border router.
//
// VALIDATES: RFC 3101 Appendix D, "there must be a way of configuring the metric of the
// default LSA that a border router advertises into its directly attached NSSAs", for the
// default LSA of RFC 3101 Section 2.7: "When OSPF's summary routes are not imported, the
// default LSA originated by an NSSA border router into the NSSA should be a Type-3
// summary-LSA." The default-cost leaf of a no-summary NSSA is the metric of that Type-3.
// PREVENTS: a border router advertising a fixed or borrowed metric in the Type-3 default,
// which the Type-7 tests cannot see because a no-summary NSSA carries no Type-7 default.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// type3DefaultMetrics configures an area border router attached to the backbone and to the
// no-summary NSSAs 0.0.0.5 and 0.0.0.6, with areaLeaves appended inside each NSSA's config
// (index 0 for 0.0.0.5, index 1 for 0.0.0.6), runs the SPF computer the config drives, and
// returns the metric of the Type-3 default the router advertised into each NSSA.
func type3DefaultMetrics(t *testing.T, areaLeaves [2]string) [2]uint32 {
	t.Helper()
	cfgJSON := `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},` +
		`"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa","no-summary":"true"` + areaLeaves[0] + `},` +
		`"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa","no-summary":"true"` + areaLeaves[1] + `}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"},"eth2":{"area":"0.0.0.6"}}}}}`
	eng, rid := newRedistEngine(t, cfgJSON)
	if eng.spf == nil {
		t.Fatal("the engine built no SPF computer")
	}
	areas := [2]types.AreaID{{0, 0, 0, 5}, {0, 0, 0, 6}}
	for index, area := range []types.AreaID{types.BackboneArea, areas[0], areas[1]} {
		stub := packet.RouterLink{Type: packet.RouterLinkTypeStub, LinkID: types.LinkStateID{10, 0, byte(index), 0},
			LinkData: [4]byte{255, 255, 255, 0}, Metric: 1}
		if !eng.lsdb.Install(area, bgplsReachabilityRouter(rid, stub)) {
			t.Fatalf("install the router-LSA of area %v", area)
		}
	}
	eng.spf.Run()

	var metrics [2]uint32
	key := types.LSAKey{Type: types.LSTypeSummaryNetwork, AdvertisingRouter: rid}
	for index, area := range areas {
		lsa, ok := eng.lsdb.LookupLSA(area, key)
		if !ok {
			t.Fatalf("the border router advertised no Type-3 default into no-summary NSSA %v", area)
		}
		body, err := packet.DecodeSummaryLSA(lsa.Body)
		if err != nil {
			t.Fatalf("DecodeSummaryLSA: %v", err)
		}
		// The lookup key's zero Link State ID is 0.0.0.0; a zero mask makes it the default.
		if body.NetworkMask != [4]byte{} {
			t.Fatalf("NSSA %v: the self-originated Type-3 is not the default destination", area)
		}
		metrics[index] = body.Metric
	}
	return metrics
}

// TestRFC3101BorderRouterType3DefaultCarriesConfiguredMetric checks the configured case.
// Goal: the Type-3 default a border router advertises into each no-summary NSSA carries that
// NSSA's configured metric. Method: two no-summary NSSAs configured with default-cost 77 and
// 5000 on one area border router; the config drives the SPF computer, whose run originates
// the defaults.
// RFC requirement: RFC3101-x-5 positive -- an area border router advertises its Type-3
// default into each directly attached no-summary NSSA with the metric configured for that
// NSSA: 77 and 5000.
func TestRFC3101BorderRouterType3DefaultCarriesConfiguredMetric(t *testing.T) {
	got := type3DefaultMetrics(t, [2]string{`,"default-cost":"77"`, `,"default-cost":"5000"`})
	if got != [2]uint32{77, 5000} {
		t.Fatalf("border router Type-3 default metrics = %v, want the configured [77 5000]", got)
	}
}

// TestRFC3101BorderRouterType3DefaultMetricIsNotBorrowed checks the unconfigured case. Goal:
// a metric configured for one no-summary NSSA is not advertised into another. Method: only
// 0.0.0.5 is configured (77); the Type-3 default into 0.0.0.6 must carry DefaultAreaCost.
// RFC requirement: RFC3101-x-5 negative -- the Type-3 default a border router advertises into
// a no-summary NSSA with no configured metric carries DefaultAreaCost, never the metric
// configured for a sibling NSSA.
func TestRFC3101BorderRouterType3DefaultMetricIsNotBorrowed(t *testing.T) {
	got := type3DefaultMetrics(t, [2]string{`,"default-cost":"77"`, ``})
	if got != [2]uint32{77, DefaultAreaCost} {
		t.Fatalf("border router Type-3 default metrics = %v, want [77 %d]", got, DefaultAreaCost)
	}
}
