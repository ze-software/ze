// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA default origination.
// Related: nssa.go -- applyNSSADefaults, the border router's Type-7 default.
// Related: rfc3101_default_cost_test.go -- the same leaf on a router that is not a border router.
//
// VALIDATES: RFC 3101 Appendix D, "there must be a way of configuring the metric of the
// default LSA that a border router advertises into its directly attached NSSAs": on an
// area border router, each attached NSSA's default-cost is the metric of the Type-7
// default the router advertises into that NSSA.
// PREVENTS: a border router advertising a fixed or borrowed metric in place of the
// configured one.
package ospf

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// borderDefaultMetrics runs the NSSA default origination on an area border router attached
// to the backbone and to NSSAs 0.0.0.5 and 0.0.0.6, with areaLeaves appended inside each
// NSSA's config (index 0 for 0.0.0.5, index 1 for 0.0.0.6), and returns the metric of the
// Type-7 default the router advertised into each NSSA.
func borderDefaultMetrics(t *testing.T, areaLeaves [2]string) [2]uint32 {
	t.Helper()
	cfgJSON := `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},` +
		`"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"` + areaLeaves[0] + `},` +
		`"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"` + areaLeaves[1] + `}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"},"eth2":{"area":"0.0.0.6"}}}}}`
	eng, rid := newRedistEngine(t, cfgJSON)
	areas := [2]types.AreaID{{0, 0, 0, 5}, {0, 0, 0, 6}}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: areas[0]}
	eng.running["eth2"] = interfaceConfig{Name: "eth2", AreaID: areas[1]}
	eng.applyNSSADefaults()

	var metrics [2]uint32
	for index, area := range areas {
		lsa, ok := eng.lsdb.LookupLSA(area, types.LSAKey{Type: types.LSTypeNSSA, AdvertisingRouter: rid})
		if !ok {
			t.Fatalf("the border router advertised no Type-7 default into NSSA %v", area)
		}
		body, err := packet.DecodeExternalLSA(lsa.Body)
		if err != nil {
			t.Fatalf("DecodeExternalLSA: %v", err)
		}
		// The lookup key's zero Link State ID is 0.0.0.0; a zero mask makes it the default.
		if binary.BigEndian.Uint32(body.NetworkMask[:]) != 0 {
			t.Fatalf("NSSA %v: the self-originated Type-7 is not the default destination", area)
		}
		metrics[index] = body.Metric
	}
	return metrics
}

// TestRFC3101BorderRouterDefaultCarriesConfiguredMetric checks the configured case. Goal: a
// border router's default into each attached NSSA carries that NSSA's configured metric.
// Method: two NSSAs configured with default-cost 77 and 5000 on one area border router.
// RFC requirement: RFC3101-x-5 positive -- an area border router advertises its Type-7
// default into each directly attached NSSA with the metric configured for that NSSA: 77 and
// 5000.
func TestRFC3101BorderRouterDefaultCarriesConfiguredMetric(t *testing.T) {
	got := borderDefaultMetrics(t, [2]string{`,"default-cost":"77"`, `,"default-cost":"5000"`})
	if got != [2]uint32{77, 5000} {
		t.Fatalf("border router Type-7 default metrics = %v, want the configured [77 5000]", got)
	}
}

// TestRFC3101BorderRouterDefaultMetricIsNotBorrowed checks the unconfigured case. Goal: a
// metric configured for one NSSA is not advertised into another. Method: only 0.0.0.5 is
// configured (77); 0.0.0.6 must carry DefaultAreaCost, not 77.
// RFC requirement: RFC3101-x-5 negative -- the default a border router advertises into an
// NSSA with no configured metric carries DefaultAreaCost, never the metric configured for a
// sibling NSSA.
func TestRFC3101BorderRouterDefaultMetricIsNotBorrowed(t *testing.T) {
	got := borderDefaultMetrics(t, [2]string{`,"default-cost":"77"`, ``})
	if got != [2]uint32{77, DefaultAreaCost} {
		t.Fatalf("border router Type-7 default metrics = %v, want [77 %d]", got, DefaultAreaCost)
	}
}
