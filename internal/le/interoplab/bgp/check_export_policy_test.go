// Design: docs/architecture/testing/interop.md -- the IGP export-policy scenarios.
// Related: check_export_policy_predicate.go -- the predicates these subtests drive.
package bgp

import (
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

// isisExportPolicyCheckerBranches drives isisExportPolicyVerdict in both
// polarities.
// VALIDATES: FRR's IS-IS table passes only with both /32s through Ze at their
// own metrics and no route for the rejected /24.
// Method: FRR-shaped `show ip route isis json` answers, one correct and one per
// wrong reading.
// PREVENTS: an export chain that judges nothing (the /24 present), a set
// metric applied to every route or to none (the metrics swapped or equal), a
// route learned from another neighbor, and an empty table read as absence.
func isisExportPolicyCheckerBranches(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	wants := []frrISISExportWant{
		{prefix: netip.MustParsePrefix(isisExportSetPrefix), nextHop: ze, metric: isisExportFRRLinkMetric + isisExportSetMetric},
		{prefix: netip.MustParsePrefix(isisExportDefaultPrefix), nextHop: ze, metric: isisExportFRRLinkMetric + isisExportDefaultMetric},
	}
	refused := netip.MustParsePrefix(isisExportRefusedPrefix)
	route := func(prefix string, metric int, hop string) string {
		return `"` + prefix + `":[{"prefix":"` + prefix + `","protocol":"isis","selected":true,"metric":` +
			strconv.Itoa(metric) + `,"nexthops":[{"ip":"` + hop + `","interfaceName":"eth0"}]}]`
	}
	good := "{" + route(isisExportSetPrefix, 87, "172.30.0.2") + "," + route(isisExportDefaultPrefix, 110, "172.30.0.2") + "}"
	t.Run("both loopbacks at their metrics, no /24", func(t *testing.T) {
		if err := isisExportPolicyVerdict(good, wants, refused); err != nil {
			t.Fatalf("the correct table was refused: %v", err)
		}
	})
	for _, tc := range []struct {
		name   string
		output string
		reason string
	}{
		{"the rejected /24 installed", "{" + route(isisExportSetPrefix, 87, "172.30.0.2") + "," + route(isisExportDefaultPrefix, 110, "172.30.0.2") + "," +
			route(isisExportRefusedPrefix, 110, "172.30.0.2") + "}", "export chain rejects"},
		{"the set metric on neither route", "{" + route(isisExportSetPrefix, 110, "172.30.0.2") + "," + route(isisExportDefaultPrefix, 110, "172.30.0.2") + "}", "has metric 110, want 87"},
		{"the set metric on both routes", "{" + route(isisExportSetPrefix, 87, "172.30.0.2") + "," + route(isisExportDefaultPrefix, 87, "172.30.0.2") + "}", "has metric 87, want 110"},
		{"a route through another neighbor", "{" + route(isisExportSetPrefix, 87, "172.30.0.9") + "," + route(isisExportDefaultPrefix, 110, "172.30.0.2") + "}", "through 172.30.0.2"},
		{"an empty table", "{}", "no IS-IS route for"},
		{"not json", "% Unknown command", "decode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := isisExportPolicyVerdict(tc.output, wants, refused)
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.reason)
			}
		})
	}
}

// ospfExportPolicyCheckerBranches drives ospfExportPolicyVerdict in both
// polarities.
// VALIDATES: FRR's external LSDB passes only with the tagged route's LSA from
// Ze at metric 50, E1, tag 200, the untagged one at metric 20, E2, tag 0, and no
// LSA for the rejected route.
// Method: FRR-shaped `show ip ospf database external json` answers, nested the
// way FRR nests them, one correct and one per wrong reading.
// PREVENTS: an export chain that judges nothing (the rejected LSA present), a
// modify whose sets never reach the LSA (defaults on the tagged route), sets
// applied to every route, an LSA from another router, and an empty database
// read as absence.
func ospfExportPolicyCheckerBranches(t *testing.T) {
	router := netip.MustParseAddr(ospfExportRouterID)
	wants := []ospfExternalWant{
		{prefix: netip.MustParsePrefix(ospfExportSetPrefix), metric: 50, metricType: ospfExternalE1, tag: 200},
		{prefix: netip.MustParsePrefix(ospfExportDefaultPrefix), metric: 20, metricType: ospfExternalE2, tag: 0},
	}
	refused := netip.MustParsePrefix(ospfExportRefusedPrefix)
	lsa := func(id, adv, metricType string, metric, tag int) string {
		return `"` + id + `":{"lsaAge":5,"linkStateId":"` + id + `","advertisingRouter":"` + adv + `","networkMask":24,"metricType":"` +
			metricType + `","tos":0,"metric":` + strconv.Itoa(metric) + `,"forwardAddress":"0.0.0.0","externalRouteTag":` + strconv.Itoa(tag) + `}`
	}
	database := func(lsas ...string) string {
		return `{"routerId":"172.30.0.3","asExternalLinkStates":{` + strings.Join(lsas, ",") + `}}`
	}
	good := database(lsa("10.67.1.0", ospfExportRouterID, "E1", 50, 200), lsa("10.67.2.0", ospfExportRouterID, "E2", 20, 0))
	t.Run("set route carries the sets, default route the defaults", func(t *testing.T) {
		if err := ospfExportPolicyVerdict(good, router, wants, refused); err != nil {
			t.Fatalf("the correct database was refused: %v", err)
		}
	})
	for _, tc := range []struct {
		name   string
		output string
		reason string
	}{
		{"the rejected route originated", database(lsa("10.67.1.0", ospfExportRouterID, "E1", 50, 200), lsa("10.67.2.0", ospfExportRouterID, "E2", 20, 0),
			lsa("10.67.3.0", ospfExportRouterID, "E2", 20, 0)), "export chain rejects"},
		{"the sets never reached the LSA", database(lsa("10.67.1.0", ospfExportRouterID, "E2", 20, 100), lsa("10.67.2.0", ospfExportRouterID, "E2", 20, 0)), "has metric 20, want 50"},
		{"metric type left E2", database(lsa("10.67.1.0", ospfExportRouterID, "E2", 50, 200), lsa("10.67.2.0", ospfExportRouterID, "E2", 20, 0)), `metric type "E2", want E1`},
		{"route tag kept over the set tag", database(lsa("10.67.1.0", ospfExportRouterID, "E1", 50, 100), lsa("10.67.2.0", ospfExportRouterID, "E2", 20, 0)), "has tag 100, want 200"},
		{"sets applied to the unmatched route", database(lsa("10.67.1.0", ospfExportRouterID, "E1", 50, 200), lsa("10.67.2.0", ospfExportRouterID, "E1", 50, 200)), "has metric 50, want 20"},
		{"LSA from another router", database(lsa("10.67.1.0", "172.30.0.3", "E1", 50, 200), lsa("10.67.2.0", ospfExportRouterID, "E2", 20, 0)), "no AS-External-LSA for 10.67.1.0/24"},
		{"an empty database", `{"routerId":"172.30.0.3","asExternalLinkStates":{}}`, "holds no LSA"},
		{"not json", "% Unknown command", "decode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ospfExportPolicyVerdict(tc.output, router, wants, refused)
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.reason)
			}
		})
	}
}
