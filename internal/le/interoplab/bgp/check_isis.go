// Design: docs/architecture/testing/interop.md -- IS-IS interop assertions that
// read a Ze routing decision back through the CLI.
// Related: check_special.go -- specialCheckers binds these to their scenario.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// isisMaxLinkMetric is the RFC 5305 section 3 maximum LINK metric, 2^24 - 1.
	// isis-max-metric-frr/ze.conf configures eth0 with it.
	isisMaxLinkMetric = "16777215"
	// isisMaxMetricZeHostname and isisMaxMetricFRRPrefix are the two identities
	// the scenario configures: the name Ze advertises in TLV 137, and the prefix
	// only FRR originates.
	isisMaxMetricZeHostname = "ze-maxmetric"
	isisMaxMetricFRRPrefix  = "10.77.7.7"
)

// checkISISMaxLinkMetric proves the RFC 5305 section 3 maximum-link-metric
// exclusion against FRR.
//
// RFC 5305 section 3: "If a link is advertised with the maximum link metric
// (2^24 - 1), this link MUST NOT be considered during the normal SPF
// computation."
//
// The scenario gives Ze's eth0 that metric. The adjacency still forms, Ze still
// originates the link, and FRR still parses it: assertions 1 and 2 are what
// makes this an interop proof rather than a unit test, because a foreign
// implementation confirms the max-metric neighbor entry reached the wire intact.
// Assertion 3 shows Ze holds FRR's LSP, so SPF had the input it would need to
// install FRR's prefix. Assertion 4 is the requirement: with all of that true,
// 10.77.7.7/32 must not be in Ze's IS-IS route table, because the only link to
// it is one the normal SPF computation must not consider.
func checkISISMaxLinkMetric(ctx context.Context, check *interoplab.CheckContext) error {
	const name = "isis-max-metric-frr"
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}

	// Assertion 1. The adjacency reaches Up. A link excluded from SPF is still a
	// live link: the exclusion is a routing decision, never an adjacency one.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR IS-IS adjacency Up",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", frrShowISISNeighbor}, nil)
	}, isisAdjacencyUp); err != nil {
		return fail(1, err)
	}

	// Assertion 2. FRR renders Ze's LSP carrying the maximum link metric, so an
	// independent implementation read 16777215 out of Ze's TLV 22 neighbor entry.
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "ze maximum-metric link in the FRR IS-IS database",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", frrShowISISDatabase + " detail"}, nil)
	}, func(output string) bool {
		return strings.Contains(output, isisMaxMetricZeHostname) && strings.Contains(output, isisMaxLinkMetric)
	}); err != nil {
		return fail(2, err)
	}

	// Assertion 3. Ze holds FRR's LSP, which carries the 10.77.7.7/32
	// reachability. Without this the absence below would also pass against a Ze
	// that never learned the prefix at all.
	database := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, "ze", zeCommand("show isis database"), queryEnvironment("ze", zeCommand("show isis database")))
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     90 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR LSP in the ze IS-IS database",
	}, database, func(output string) bool {
		return strings.Contains(output, "0000.0000.0000.0003")
	}); err != nil {
		return fail(3, err)
	}

	// Assertion 4. The requirement. SPF has had every input it needs and settles
	// before the read, so the absence is a decision rather than a race.
	settle := time.NewTimer(10 * time.Second)
	defer settle.Stop()
	select {
	case <-ctx.Done():
		return fail(4, ctx.Err())
	case <-settle.C:
	}
	routes, err := check.Lab.Query(ctx, "ze", zeCommand("show isis route"), queryEnvironment("ze", zeCommand("show isis route")))
	if err != nil {
		return fail(4, err)
	}
	if strings.Contains(routes, isisMaxMetricFRRPrefix) {
		return fail(4, fmt.Errorf("ze installed %s over a link advertised at the maximum link metric %s, which RFC 5305 section 3 excludes from the normal SPF computation: %s",
			isisMaxMetricFRRPrefix, isisMaxLinkMetric, strings.TrimSpace(routes)))
	}

	// The adjacency is still Up, so the absence above was read against a live
	// session rather than one that dropped.
	adjacencies, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", frrShowISISNeighbor}, nil)
	if err != nil {
		return fail(4, err)
	}
	if !isisAdjacencyUp(adjacencies) {
		return fail(4, errors.New("the IS-IS adjacency dropped, so the excluded route was absent for the wrong reason"))
	}
	return nil
}
