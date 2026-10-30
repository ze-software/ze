// Design: docs/architecture/testing/interop.md -- the IS-IS narrow-metric interop
// assertions against a narrow-only FRR.
// Related: register_isis_narrow.go -- binds checkISISNarrowMetric to its scenario.
// Related: check_isis.go -- the sibling IS-IS checkers this one mirrors.
package bgp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// isisNarrowScenario is the scenario directory under test/interop/scenarios.
	isisNarrowScenario = "isis-narrow-metric-frr"
	// isisNarrowZePrefix is the static prefix isis-narrow-metric-frr/ze.conf
	// redistributes into IS-IS, and isisNarrowFRRPrefix the loopback only FRR
	// advertises.
	isisNarrowZePrefix  = "10.78.2.0/24"
	isisNarrowFRRPrefix = "10.78.7.7"
	// isisNarrowZeReachability is the line FRR prints for a TLV 128 entry
	// (format_item_oldstyle_ip_reach). A redistributed route carries the fixed
	// metric 100, which a 6-bit narrow metric writes as 63.
	isisNarrowZeReachability = "IP Reachability: " + isisNarrowZePrefix + " (Metric: 63)"
)

// isisNarrowReachabilityLine reports whether FRR's database detail holds Ze's
// prefix as a narrow TLV 128 entry. The line must START with the narrow form:
// FRR prints a TLV 135 entry as "Extended IP Reachability: ...", which carries
// the narrow text as a substring.
func isisNarrowReachabilityLine(database string) bool {
	for line := range strings.SplitSeq(database, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), isisNarrowZeReachability) {
			return true
		}
	}
	return false
}

// RFC requirement: RFC3787-5-1 positive -- against FRR running
// `metric-style narrow`, a Ze configured `metric-style narrow` forms the
// adjacency, FRR's database shows Ze's redistributed 10.78.2.0/24 as a narrow
// TLV 128 entry at metric 63, FRR's narrow-only SPF installs that prefix, and Ze
// installs FRR's narrow-advertised 10.78.7.7/32.
//
// RFC 3787 Section 5: "If not all devices in the IS-IS domain support wide
// metrics, narrow metrics MUST continue to be used." FRR with
// `metric-style narrow` skips every wide TLV in process_lsp, so assertion 3
// passes only when Ze's own LSP carries TLV 2 back to FRR and TLV 128 for the
// prefix.
func checkISISNarrowMetric(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, isisNarrowScenario, assertion, cause)
	}
	frr := func(command string) func(context.Context) (string, error) {
		return func(probeCtx context.Context) (string, error) {
			return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", command}, nil)
		}
	}

	// Assertion 1. The point-to-point adjacency reaches Up.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR IS-IS adjacency Up",
	}, frr(frrShowISISNeighbor), isisAdjacencyUp); err != nil {
		return fail(1, err)
	}

	// Assertion 2. FRR decoded Ze's prefix out of a TLV 128 entry, at the 6-bit
	// ceiling the fixed redistribution metric is written as.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "ze narrow IP reachability in the FRR IS-IS database",
	}, frr(frrShowISISDatabase+" detail"), isisNarrowReachabilityLine); err != nil {
		return fail(2, err)
	}

	// Assertion 3. The requirement: FRR's narrow-only SPF reached Ze and
	// installed the prefix behind it.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "ze prefix in the FRR IS-IS route table",
	}, frr("show isis route"), func(output string) bool {
		return strings.Contains(output, isisNarrowZePrefix)
	}); err != nil {
		return fail(3, err)
	}

	// Assertion 4. Ze routes the other way, through FRR's narrow LSP.
	routes := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, "ze", zeCommand("show isis route"), queryEnvironment("ze", zeCommand("show isis route")))
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR prefix in the ze IS-IS route table",
	}, routes, func(output string) bool {
		return strings.Contains(output, isisNarrowFRRPrefix)
	}); err != nil {
		return fail(4, err)
	}

	// The adjacency is still Up, so the routes above were read off a live session.
	adjacencies, err := frr(frrShowISISNeighbor)(ctx)
	if err != nil {
		return fail(5, err)
	}
	if !isisAdjacencyUp(adjacencies) {
		return fail(5, errors.New("the IS-IS adjacency dropped after the routes converged"))
	}
	return nil
}
