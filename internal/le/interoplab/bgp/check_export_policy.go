// Design: docs/architecture/testing/interop.md -- the IGP export-policy scenarios.
// Related: check_export_policy_predicate.go -- the pure predicates these bodies call.
// Related: register_export_policy.go -- the registration of both checkers.
package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	isisExportPolicyScenario = "isis-export-policy-frr"
	ospfExportPolicyScenario = "ospf-export-policy-frr"
	exportPolicyWait         = 90 * time.Second

	// isisExportSetPrefix is the tagged /32 whose metric the export chain's
	// modify sets to isisExportSetMetric (test/interop/scenarios/isis-export-policy-frr/ze.conf).
	isisExportSetPrefix = "10.66.0.1/32"
	isisExportSetMetric = 77
	// isisExportDefaultPrefix is the untagged /32 the chain accepts and sets
	// nothing on, so Ze advertises it at DefaultRedistMetric
	// (internal/plugins/isis/redistribute/redistribute.go, applied by exportVerdict in export.go).
	isisExportDefaultPrefix = "10.66.0.2/32"
	isisExportDefaultMetric = 100
	// isisExportRefusedPrefix is the /24 the `loopbacks` prefix-list rejects.
	isisExportRefusedPrefix = "10.66.1.0/24"
	// isisExportFRRLinkMetric is FRR's own interface metric toward Ze
	// (`isis metric 10` in frr.conf): FRR's SPF cost to each of Ze's prefixes
	// is this plus the metric Ze advertised.
	isisExportFRRLinkMetric = 10

	// ospfExportSetPrefix is the static route tagged 100, on which the modify
	// sets metric 50, type-1 and tag 200 (test/interop/scenarios/ospf-export-policy-frr/ze.conf).
	ospfExportSetPrefix = "10.67.1.0/24"
	// ospfExportDefaultPrefix is the untagged route no filter sets anything on:
	// DefaultExternalMetric 20, type-2, and its own tag 0
	// (internal/plugins/ospf/redistribute/redistribute.go, applied by exportVerdict in export.go).
	ospfExportDefaultPrefix = "10.67.2.0/24"
	// ospfExportRefusedPrefix is the route `ospf-export-refused` rejects.
	ospfExportRefusedPrefix = "10.67.3.0/24"
	// ospfExportRouterID is Ze's OSPF router ID in the scenario, outside the
	// lab subnet so the runner's address rewrite leaves it as written.
	ospfExportRouterID = "10.255.0.2"
)

// checkISISExportPolicy proves that `isis filter export` decides what Ze
// advertises into IS-IS and at what metric. Assertion 1 is the adjacency.
// Assertion 2 reads one FRR answer, `show ip route isis json`: the tagged /32
// installed through Ze at FRR's link metric plus the 77 the modify set, the
// untagged /32 at the link metric plus Ze's default 100, and no route for the
// /24 the prefix-list rejects. All three come from Ze's one LSP, so the /24's
// absence is read beside the presence of two prefixes that rode with it.
func checkISISExportPolicy(ctx context.Context, check *interoplab.CheckContext) error {
	adjacency := operation{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 60 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &adjacency); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	zeAddress, err := netip.ParseAddr(labIPv4(check.Network, zeHostNumber))
	if err != nil {
		return fmt.Errorf("assertion 2: Ze's lab address: %w", err)
	}
	wants := []frrISISExportWant{
		{prefix: netip.MustParsePrefix(isisExportSetPrefix), nextHop: zeAddress, metric: isisExportFRRLinkMetric + isisExportSetMetric},
		{prefix: netip.MustParsePrefix(isisExportDefaultPrefix), nextHop: zeAddress, metric: isisExportFRRLinkMetric + isisExportDefaultMetric},
	}
	refused := netip.MustParsePrefix(isisExportRefusedPrefix)
	if err := waitFRRVerdict(ctx, check.Lab, "show ip route isis json", "FRR's IS-IS routes from Ze's export chain", func(output string) error {
		return isisExportPolicyVerdict(output, wants, refused)
	}); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	return nil
}

// checkOSPFExportPolicy proves that `ospf filter export` decides which
// AS-External-LSAs Ze originates and what they carry. Assertion 1 is the
// adjacency. Assertion 2 reads one FRR answer, `show ip ospf database external
// json`: the tagged route's LSA from Ze at metric 50, E1 and tag 200, which the
// modify set; the untagged route's LSA at the defaults (metric 20, E2, tag 0);
// and no LSA for the route the match rejects, read beside the other two.
func checkOSPFExportPolicy(ctx context.Context, check *interoplab.CheckContext) error {
	adjacency := operation{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &adjacency); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	router := netip.MustParseAddr(ospfExportRouterID)
	wants := []ospfExternalWant{
		{prefix: netip.MustParsePrefix(ospfExportSetPrefix), metric: 50, metricType: ospfExternalE1, tag: 200},
		{prefix: netip.MustParsePrefix(ospfExportDefaultPrefix), metric: 20, metricType: ospfExternalE2, tag: 0},
	}
	refused := netip.MustParsePrefix(ospfExportRefusedPrefix)
	if err := waitFRRVerdict(ctx, check.Lab, "show ip ospf database external json", "FRR's AS-External-LSAs from Ze's export chain", func(output string) error {
		return ospfExportPolicyVerdict(output, router, wants, refused)
	}); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	return nil
}

// waitFRRVerdict waits until FRR's answer to the vtysh command passes verdict,
// and reports the last verdict and reply when it never does.
func waitFRRVerdict(ctx context.Context, lab interoplab.CheckerLab, vtysh, description string, verdict func(string) error) error {
	command := []string{cmdVtysh, "-c", vtysh}
	var last string
	var judged error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: exportPolicyWait, Interval: time.Second, Description: description,
	}, func(probeCtx context.Context) (string, error) {
		output, queryErr := lab.Query(probeCtx, peerFRR, command, queryEnvironment(peerFRR, command))
		last = output
		return output, queryErr
	}, func(output string) bool {
		judged = verdict(output)
		return judged == nil
	})
	if err != nil {
		return fmt.Errorf("%w (last verdict: %w); last reply: %s", err, judged, last)
	}
	return nil
}
