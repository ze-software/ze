// RFC 3101 Appendix D: the metric of the default LSA a router advertises into its attached
// NSSAs is configurable. The area's default-cost leaf flows through areaConfig.DefaultCost
// into the Type-7 default that applyNSSADefaults originates.

package ospf

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// nssaDefaultMetric originates the NSSA default on a single-NSSA router configured with
// defaultCost (an empty string leaves the leaf unset) and returns the metric of the Type-7
// default LSA it installed.
func nssaDefaultMetric(t *testing.T, defaultCost string) uint32 {
	t.Helper()
	leaf := ""
	if defaultCost != "" {
		leaf = `,"default-cost":"` + defaultCost + `"`
	}
	cfgJSON := `{"ospf":{"router-id":"10.0.5.1","areas":{"area":{"0.0.0.5":{"area-id":"0.0.0.5",` +
		`"area-type":"nssa","nssa":{"default-originate":"true"}` + leaf + `}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.5"}}}}}`
	nssa := types.AreaID{0, 0, 0, 5}
	eng, rid := newRedistEngine(t, cfgJSON)
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}
	eng.forwardingAddress = func(string) (netip.Addr, bool) { return netip.MustParseAddr("192.0.2.1"), true }
	eng.applyNSSADefaults()
	lsa, ok := eng.lsdb.LookupLSA(nssa, types.LSAKey{Type: types.LSTypeNSSA, AdvertisingRouter: rid})
	if !ok {
		t.Fatal("no Type-7 default LSA originated")
	}
	body, err := packet.DecodeExternalLSA(lsa.Body)
	if err != nil {
		t.Fatalf("DecodeExternalLSA: %v", err)
	}
	return body.Metric
}

// RFC requirement: RFC3101-x-5 positive — the area's configured default-cost is the metric of
// the Type-7 default LSA the router originates into that NSSA: 77 configured, 77 advertised
// (parseArea reads default-cost into areaConfig.DefaultCost; applyNSSADefaults originates
// with it).
func TestOSPFNSSADefaultCarriesConfiguredMetric(t *testing.T) {
	const want = uint32(77)
	if got := nssaDefaultMetric(t, strconv.Itoa(int(want))); got != want {
		t.Fatalf("Type-7 default metric = %d, want the configured default-cost %d", got, want)
	}
}

// RFC requirement: RFC3101-x-5 negative — the configured metric is not replaced by a fixed
// value: two areas configured with different default-costs advertise two different metrics,
// and an area with no default-cost advertises DefaultAreaCost rather than either of them
// (applyNSSADefaults passes areaConfig.DefaultCost through unchanged).
func TestOSPFNSSADefaultMetricIsNotFixed(t *testing.T) {
	low := nssaDefaultMetric(t, "5")
	high := nssaDefaultMetric(t, "5000")
	if low != 5 || high != 5000 {
		t.Fatalf("Type-7 default metrics = %d and %d, want 5 and 5000", low, high)
	}
	if got := nssaDefaultMetric(t, ""); got != DefaultAreaCost {
		t.Fatalf("Type-7 default metric with no default-cost = %d, want DefaultAreaCost %d", got, DefaultAreaCost)
	}
}
