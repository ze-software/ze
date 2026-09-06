// Design: docs/architecture/testing/interop.md -- IS-IS interop assertions that
// read a Ze routing decision back through the CLI.
// Related: check_special.go -- specialCheckers binds these to their scenario.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

// Per-level Hello timers, read back off the wire by FRR
// (spec-isis-per-level-hello-timers). isis-per-level-hello-frr/ze.conf gives one
// broadcast circuit three distinct holding times: 30 seconds circuit-wide, 9
// seconds under level-1 and 120 seconds under level-2. FRR decodes the Holding
// Time field of each LAN IIH and keeps the two adjacencies apart, so the number
// it reports for each level names the pair Ze applied there.
const (
	// isisPerLevelL1HoldMax bounds the Level-1 holding time FRR may report. Ze
	// advertises 9 seconds at Level-1 and counts down from it, so any reading is
	// at most 9. The circuit-wide 30 is above this bound, which is what makes the
	// assertion a proof rather than a coincidence.
	isisPerLevelL1HoldMax = 15
	// isisPerLevelL2HoldMin bounds the Level-2 holding time from below. Ze
	// advertises 120 seconds and refreshes every 30, so a reading is never under
	// 90. The circuit-wide 30 is below this bound.
	isisPerLevelL2HoldMin = 60
)

// isisNeighborRow is one adjacency row of FRR's `show isis neighbor` table: the
// level it formed at, its state, and the holding time FRR is counting down from
// the last IIH it accepted at that level.
type isisNeighborRow struct {
	level    int
	state    string
	holdtime int
}

// parseISISNeighbors reads FRR's `show isis neighbor` table. The columns are
// System Id, Interface, L, State, Holdtime, SNPA; the header row and the area
// heading carry no level number in the third column and are skipped by the
// parse rather than by matching their text.
func parseISISNeighbors(table string) []isisNeighborRow {
	var rows []isisNeighborRow
	const (
		levelField    = 2
		stateField    = 3
		holdtimeField = 4
		minFields     = holdtimeField + 1
	)
	for line := range strings.SplitSeq(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) < minFields {
			continue
		}
		level, err := strconv.Atoi(fields[levelField])
		if err != nil || (level != 1 && level != 2) {
			continue
		}
		holdtime, err := strconv.Atoi(fields[holdtimeField])
		if err != nil {
			continue
		}
		rows = append(rows, isisNeighborRow{level: level, state: fields[stateField], holdtime: holdtime})
	}
	return rows
}

// upAtLevel returns the Up adjacency FRR holds at level.
func upAtLevel(rows []isisNeighborRow, level int) (isisNeighborRow, bool) {
	for _, row := range rows {
		if row.level == level && row.state == "Up" {
			return row, true
		}
	}
	return isisNeighborRow{}, false
}

// isisBothLevelsUp reports whether FRR holds an Up adjacency at each level, which
// is what gives the checker one holding time per level to read.
func isisBothLevelsUp(neighbors string) bool {
	rows := parseISISNeighbors(neighbors)
	_, level1 := upAtLevel(rows, 1)
	_, level2 := upAtLevel(rows, 2)
	return level1 && level2
}

// checkISISPerLevelHelloTimers proves that a per-level `hello-interval` and
// `hold-multiplier` reach the wire, against an independent implementation.
//
// The circuit runs one Hello timer for each level, so the Level-1 and the
// Level-2 IIH leave at periods of their own and each advertises the holding time
// of its own level. FRR decodes both, holds the two adjacencies separately, and
// reports one holding time for each. Before the per-level leaves were read, both
// numbers were the circuit-wide 30 seconds, which fails both bounds below.
//
// The period is proven with the holding time rather than beside it. Ze
// advertises 9 seconds at Level-1, so FRR drops that adjacency unless a Level-1
// IIH arrives inside every 9-second window. The circuit-wide period is 10
// seconds, so a circuit still sending on it cannot hold this adjacency Up.
func checkISISPerLevelHelloTimers(ctx context.Context, check *interoplab.CheckContext) error {
	const name = "isis-per-level-hello-frr"
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, name, assertion, cause)
	}
	neighbors := func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, []string{cmdVtysh, "-c", frrShowISISNeighbor}, nil)
	}

	// Assertion 1. One adjacency reaches Up at each level. Two adjacencies over
	// one circuit is what gives each level a holding time of its own to carry.
	table, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     120 * time.Second,
		Interval:    2 * time.Second,
		Description: "FRR IS-IS adjacencies Up at Level-1 and Level-2",
	}, neighbors, isisBothLevelsUp)
	if err != nil {
		return fail(1, err)
	}

	// Assertion 2. The two holding times differ, each on the side of the
	// circuit-wide 30 seconds its own level asked for. A run that read the
	// circuit-wide pair at both levels reports 30 twice and fails here.
	rows := parseISISNeighbors(table)
	level1, ok := upAtLevel(rows, 1)
	if !ok {
		return fail(2, fmt.Errorf("no Up Level-1 adjacency in the FRR neighbor table: %s", strings.TrimSpace(table)))
	}
	level2, ok := upAtLevel(rows, 2)
	if !ok {
		return fail(2, fmt.Errorf("no Up Level-2 adjacency in the FRR neighbor table: %s", strings.TrimSpace(table)))
	}
	if level1.holdtime > isisPerLevelL1HoldMax {
		return fail(2, fmt.Errorf("FRR read a Level-1 holding time of %ds, over the %ds bound the level-1 override (3 * 3) puts it under: %s",
			level1.holdtime, isisPerLevelL1HoldMax, strings.TrimSpace(table)))
	}
	if level2.holdtime < isisPerLevelL2HoldMin {
		return fail(2, fmt.Errorf("FRR read a Level-2 holding time of %ds, under the %ds bound the level-2 override (30 * 4) puts it over: %s",
			level2.holdtime, isisPerLevelL2HoldMin, strings.TrimSpace(table)))
	}

	// Assertion 3. The Level-1 adjacency is still Up after a settle longer than
	// the 9-second holding time it advertised, so Ze really sends the Level-1 IIH
	// at the Level-1 period. A circuit sending at the circuit-wide 10 seconds
	// against a 9-second holding time loses this adjacency.
	settle := time.NewTimer(20 * time.Second)
	defer settle.Stop()
	select {
	case <-ctx.Done():
		return fail(3, ctx.Err())
	case <-settle.C:
	}
	table, err = neighbors(ctx)
	if err != nil {
		return fail(3, err)
	}
	settled, ok := upAtLevel(parseISISNeighbors(table), 1)
	if !ok {
		return fail(3, fmt.Errorf("the Level-1 adjacency did not stay Up over a window longer than the holding time Ze advertised for it: %s",
			strings.TrimSpace(table)))
	}
	if settled.holdtime > isisPerLevelL1HoldMax {
		return fail(3, fmt.Errorf("FRR read a Level-1 holding time of %ds after the settle, over the %ds bound: %s",
			settled.holdtime, isisPerLevelL1HoldMax, strings.TrimSpace(table)))
	}
	return nil
}
