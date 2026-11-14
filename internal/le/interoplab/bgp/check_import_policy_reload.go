// Design: docs/architecture/testing/interop.md -- the IGP import-policy scenarios.
// Related: check_import_policy.go -- waitRIBCandidates and the prefixes this reuses.
// Related: check_import_policy_placement.go -- the placement and FRR identity checks.
// Related: check_rpki_reload.go -- applyZeReload, the SIGHUP reload this checker drives.
// Related: register_import_policy.go -- the registration of both reload checkers.
package bgp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	isisImportReloadScenario       = "isis-import-policy-reload-frr"
	isisImportReloadForkedScenario = "isis-import-policy-reload-frr-forked"
	// isisSettleReads identical reads of Ze's IS-IS state, isisSettleInterval
	// apart, make one settled reading. Settling waits out the LSPs both routers
	// regenerate once the adjacency comes up, so the baseline is the state the
	// reloads are judged against, not a state still converging.
	isisSettleReads    = 3
	isisSettleInterval = 10 * time.Second
	isisSettleWait     = 150 * time.Second
	// zeISISAdjacencyUp is adjacency.StateUp's name in Ze's `show isis neighbor`
	// answer (internal/plugins/isis/adjacency/adjacency.go, State.String).
	zeISISAdjacencyUp = "up"
)

func checkISISImportReloadInternal(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkImportPolicyPlacement(ctx, check, "internal"); err != nil {
		return err
	}
	if err := checkISISFRRIdentity(ctx, check, peerFRR); err != nil {
		return err
	}
	return checkISISImportReload(ctx, check)
}

func checkISISImportReloadForked(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkImportPolicyPlacement(ctx, check, "forked"); err != nil {
		return err
	}
	if err := checkISISFRRIdentity(ctx, check, peerFRR); err != nil {
		return err
	}
	return checkISISImportReload(ctx, check)
}

// checkISISImportReload proves that a reload re-judges the IS-IS paths the
// Loc-RIB already holds, with no IS-IS event behind it. Under ze.conf the
// chain hides 10.77.1.0/24 (assertions 1 and 2), and Ze's adjacencies and LSDB
// are read once settled (assertion 3). The reload into ze-reload.conf, which
// drops the chain, installs the hidden path (assertions 4 and 5); the reload
// back into ze.conf hides it again (assertions 7 and 8). After each reload the
// IS-IS state settles to exactly the first reading (assertions 6 and 9).
//
// That equality is the proof the re-admission came from the reload. FRR's
// config never changes, so a fresh advertisement of the prefix would be a new
// LSP, and an LSP's sequence number only grows from one version to the next, so no new
// version of any LSP, Ze's own or FRR's, was originated across the reload. An
// adjacency that went down would have changed the IS reachability of both
// routers' LSPs, so the same equality covers a flap that recovered between
// two reads.
func checkISISImportReload(ctx context.Context, check *interoplab.CheckContext) error {
	hidden := netip.MustParsePrefix(isisImportHiddenPrefix)
	control := netip.MustParsePrefix(isisImportControlPrefix)
	adjacency := operation{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowISISNeighbor}, contains: []string{"Up"}, timeout: 60 * time.Second}
	if err := runOperation(ctx, check.Network, check.Lab, &adjacency); err != nil {
		return fmt.Errorf("assertion 1: %w", err)
	}
	if err := waitISISImportHidden(ctx, check, hidden, control); err != nil {
		return fmt.Errorf("assertion 2: %w", err)
	}
	before, err := waitISISSettledState(ctx, check.Lab)
	if err != nil {
		return fmt.Errorf("assertion 3: %w", err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return fmt.Errorf("assertion 4: reload without the import chain: %w", err)
	}
	if err := waitISISImportAdmitted(ctx, check, hidden, control); err != nil {
		return fmt.Errorf("assertion 5: %w", err)
	}
	if err := requireISISStateKept(ctx, check.Lab, before); err != nil {
		return fmt.Errorf("assertion 6: %w", err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedConfig); err != nil {
		return fmt.Errorf("assertion 7: reload with the import chain restored: %w", err)
	}
	if err := waitISISImportHidden(ctx, check, hidden, control); err != nil {
		return fmt.Errorf("assertion 8: %w", err)
	}
	if err := requireISISStateKept(ctx, check.Lab, before); err != nil {
		return fmt.Errorf("assertion 9: %w", err)
	}
	return nil
}

// waitISISImportHidden waits until the chain's rejection of hidden shows at
// every layer: its one IS-IS path hidden by the `isis import` chain, the
// control path selected, hidden absent from `show rib` and the kernel FIB
// beside the control, and both still in IS-IS's own route table.
func waitISISImportHidden(ctx context.Context, check *interoplab.CheckContext, hidden, control netip.Prefix) error {
	if err := waitRIBCandidates(ctx, check.Lab, hidden, []ribCandidateWant{
		{source: sourceISIS, hidden: true, hiddenBy: hiddenBySource, chain: isisImportAttachment},
	}); err != nil {
		return err
	}
	if err := waitRIBCandidates(ctx, check.Lab, control, []ribCandidateWant{
		{source: sourceISIS, selected: true},
	}); err != nil {
		return err
	}
	operations := []operation{
		{kind: opWaitAbsent, peer: "ze", command: zeCommand("show rib"), absent: []string{isisImportHiddenPrefix}, proof: []string{isisImportControlPrefix}, timeout: importPolicyWait},
		{kind: opWaitAbsent, peer: "ze", command: []string{"ip", "-4", ipObjectRoute, ipActionShow}, absent: []string{isisImportHiddenPrefix}, proof: []string{isisImportControlPrefix}, timeout: importPolicyWait},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show isis route"), contains: []string{isisImportHiddenPrefix, isisImportControlPrefix}},
	}
	return runISISImportOperations(ctx, check, operations)
}

// waitISISImportAdmitted waits until hidden is installed like the control: its
// one IS-IS path selected and not hidden, and both prefixes in `show rib`, the
// kernel FIB and IS-IS's own route table.
func waitISISImportAdmitted(ctx context.Context, check *interoplab.CheckContext, hidden, control netip.Prefix) error {
	for _, prefix := range []netip.Prefix{hidden, control} {
		if err := waitRIBCandidates(ctx, check.Lab, prefix, []ribCandidateWant{
			{source: sourceISIS, selected: true},
		}); err != nil {
			return err
		}
	}
	both := []string{isisImportHiddenPrefix, isisImportControlPrefix}
	operations := []operation{
		{kind: opWaitContains, peer: "ze", command: zeCommand("show rib"), contains: both, timeout: importPolicyWait},
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-4", ipObjectRoute, ipActionShow}, contains: both, timeout: importPolicyWait},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show isis route"), contains: both},
	}
	return runISISImportOperations(ctx, check, operations)
}

// runISISImportOperations runs operations in order and names the first that
// fails by its position.
func runISISImportOperations(ctx context.Context, check *interoplab.CheckContext, operations []operation) error {
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return fmt.Errorf("step %d: %w", index+1, err)
		}
	}
	return nil
}

// requireISISStateKept waits until Ze's IS-IS state settles after a reload and
// refuses any difference from before.
func requireISISStateKept(ctx context.Context, lab interoplab.CheckerLab, before isisRouterState) error {
	after, err := waitISISSettledState(ctx, lab)
	if err != nil {
		return err
	}
	return isisRouterStateMoved(before, after)
}

// waitISISSettledState reads Ze's IS-IS adjacencies and LSDB until
// isisSettleReads consecutive reads agree, and returns that reading.
func waitISISSettledState(ctx context.Context, lab interoplab.CheckerLab) (isisRouterState, error) {
	var previous isisRouterState
	var readErr error
	agreeing := 0
	state, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: isisSettleWait, Interval: isisSettleInterval, Description: "Ze's IS-IS adjacencies and LSDB to settle",
	}, func(probeCtx context.Context) (isisRouterState, error) {
		current, probeErr := readISISRouterState(probeCtx, lab)
		readErr = probeErr
		return current, probeErr
	}, func(current isisRouterState) bool {
		if agreeing > 0 && isisRouterStateMoved(previous, current) == nil {
			agreeing++
		} else {
			agreeing = 1
		}
		previous = current
		return agreeing >= isisSettleReads
	})
	if err != nil {
		return isisRouterState{}, errors.Join(err, readErr)
	}
	return state, nil
}

// readISISRouterState reads Ze's `show isis database` and `show isis neighbor`
// answers once.
func readISISRouterState(ctx context.Context, lab interoplab.CheckerLab) (isisRouterState, error) {
	databaseCommand := zeCommand("show isis database")
	database, err := lab.Query(ctx, "ze", databaseCommand, queryEnvironment("ze", databaseCommand))
	if err != nil {
		return isisRouterState{}, err
	}
	neighborCommand := zeCommand("show isis neighbor")
	neighbors, err := lab.Query(ctx, "ze", neighborCommand, queryEnvironment("ze", neighborCommand))
	if err != nil {
		return isisRouterState{}, err
	}
	return parseISISRouterState(database, neighbors)
}

// isisLSPIdentity names one version of one LSP in Ze's `show isis database`
// answer (databaseSnapshot, internal/plugins/isis/lsdb_wiring.go). The
// remaining lifetime is left out: it counts down between two reads of the
// same version.
type isisLSPIdentity struct {
	Level    string `json:"level"`
	LSPID    string `json:"lsp-id"`
	Sequence uint32 `json:"sequence"`
	Checksum uint16 `json:"checksum"`
	Own      bool   `json:"own"`
	Purged   bool   `json:"purged"`
}

// isisAdjacencyIdentity is one row of Ze's `show isis neighbor` answer
// (neighborSnapshot, internal/plugins/isis/circuits.go), without the hold
// expiry, which moves with every Hello.
type isisAdjacencyIdentity struct {
	Interface string `json:"interface"`
	SystemID  string `json:"system-id"`
	Level     string `json:"level"`
	State     string `json:"state"`
}

// isisRouterState is Ze's IS-IS state as one reading: every LSP version in its
// LSDB and every adjacency, each sorted so two readings compare in order.
type isisRouterState struct {
	lsps        []isisLSPIdentity
	adjacencies []isisAdjacencyIdentity
}

// parseISISRouterState decodes both answers and refuses a reading that cannot
// show an adjacency or an LSP moving: no adjacency, an adjacency not up, no
// live LSP of Ze's own, or no live LSP from a neighbor.
func parseISISRouterState(database, neighbors string) (isisRouterState, error) {
	var state isisRouterState
	if err := json.Unmarshal([]byte(database), &state.lsps); err != nil {
		return isisRouterState{}, fmt.Errorf("decode Ze's show isis database answer: %w; output: %s", err, database)
	}
	if err := json.Unmarshal([]byte(neighbors), &state.adjacencies); err != nil {
		return isisRouterState{}, fmt.Errorf("decode Ze's show isis neighbor answer: %w; output: %s", err, neighbors)
	}
	if len(state.adjacencies) == 0 {
		return isisRouterState{}, fmt.Errorf("ze lists no IS-IS adjacency: %s", neighbors)
	}
	for _, adjacency := range state.adjacencies {
		if adjacency.State != zeISISAdjacencyUp {
			return isisRouterState{}, fmt.Errorf("ze's IS-IS adjacency to %s on %s at %s is %q, want %q", adjacency.SystemID, adjacency.Interface, adjacency.Level, adjacency.State, zeISISAdjacencyUp)
		}
	}
	own, foreign := 0, 0
	for _, lsp := range state.lsps {
		if lsp.Purged {
			continue
		}
		if lsp.Own {
			own++
		} else {
			foreign++
		}
	}
	if own == 0 {
		return isisRouterState{}, fmt.Errorf("ze's LSDB holds no live LSP of its own: %s", database)
	}
	if foreign == 0 {
		return isisRouterState{}, fmt.Errorf("ze's LSDB holds no live LSP from a neighbor: %s", database)
	}
	slices.SortFunc(state.lsps, func(a, b isisLSPIdentity) int {
		return cmp.Or(cmp.Compare(a.Level, b.Level), cmp.Compare(a.LSPID, b.LSPID))
	})
	slices.SortFunc(state.adjacencies, func(a, b isisAdjacencyIdentity) int {
		return cmp.Or(cmp.Compare(a.Interface, b.Interface), cmp.Compare(a.SystemID, b.SystemID), cmp.Compare(a.Level, b.Level))
	})
	return state, nil
}

// isisRouterStateMoved answers nil when after is exactly before, and otherwise
// names both readings of what moved.
func isisRouterStateMoved(before, after isisRouterState) error {
	if !slices.Equal(before.adjacencies, after.adjacencies) {
		return fmt.Errorf("ze's IS-IS adjacencies moved from %+v to %+v", before.adjacencies, after.adjacencies)
	}
	if !slices.Equal(before.lsps, after.lsps) {
		return fmt.Errorf("ze's IS-IS LSDB moved from %+v to %+v: an LSP was originated again, so the change was not the reload's alone", before.lsps, after.lsps)
	}
	return nil
}
