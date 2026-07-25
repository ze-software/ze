// Design: docs/architecture/config/transaction-protocol.md -- reload to TxCoordinator wiring
// Related: reload.go -- hub flow (lock, diff, auto-load/stop, commit)
// Related: config_tx_bridge.go -- engine-side RPC bridge for per-plugin verify/apply events
// Related: engine_event_gateway.go -- gateway the orchestrator publishes on

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// bgpParticipantName is the plugin name whose apply must run last, matching
// the legacy reload.go semantic where BGP peer reconciliation saw every other
// plugin's committed state. Sorted to the tail of the participant list so the
// orchestrator's publish loop emits verify/apply for "bgp" after every other
// participant has acked.
const bgpParticipantName = "bgp"

// runTxCoordinator runs the transaction orchestrator for a reload once the
// caller has computed the affected plugins and the raw diff. It builds
// participants from the affected plugin registrations, converts the diff
// into per-root DiffSection slices the orchestrator's filterDiffs walks,
// starts an RPC bridge so the stream-based orchestrator can reach the
// plugin SDK callbacks that still speak RPC, and blocks on
// TxCoordinator.Execute.
//
// Returns nil on commit. Returns a non-nil error on abort (verify failed),
// rollback (apply failed), or gateway misconfiguration. Callers use the
// returned error verbatim as the reload error so operators see the same
// message they did on the legacy RPC loop.
func (s *Server) runTxCoordinator(ctx context.Context, affected []affectedPlugin, diff *config.ConfigDiff, runningTree, candidateTree map[string]any) error {
	if len(affected) == 0 {
		return nil
	}

	participants, diffs, verifySections, err := buildTxInputs(affected, diff)
	if err != nil {
		return fmt.Errorf("build transaction inputs: %w", err)
	}

	gateway := NewConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gateway, participantNames(participants), verifySections)
	if err := bridge.Subscribe(ctx); err != nil {
		return fmt.Errorf("config tx bridge subscribe: %w", err)
	}
	defer bridge.Close()

	coordinator, err := transaction.NewTxCoordinator(gateway, participants, s.restartPluginFn())
	if err != nil {
		return fmt.Errorf("create transaction coordinator: %w", err)
	}
	coordinator.SetOperationPlanner(operationPlannerFromTrees(gateway, runningTree, candidateTree, participants))

	result := coordinator.Execute(ctx, diffs)
	return txResultToError(result)
}

func operationPlannerFromTrees(gateway transaction.EventGateway, runningTree, candidateTree map[string]any, participants []transaction.Participant) transaction.OperationPlanner {
	return func(ctx context.Context, req transaction.OperationPlanRequest) ([]transaction.ConfigOperation, error) {
		decomposeOKCh := make(chan transaction.ConfigOperationDecomposeAck, len(req.Diffs))
		decomposeFailedCh := make(chan transaction.ConfigOperationDecomposeAck, len(req.Diffs))
		var unsubs []func()
		if gateway != nil {
			unsubs = append(unsubs,
				gateway.SubscribeConfigEvent(transaction.EventOperationDecomposeOK, func(payload []byte) {
					var ack transaction.ConfigOperationDecomposeAck
					if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == req.TransactionID {
						decomposeOKCh <- ack
					}
				}),
				gateway.SubscribeConfigEvent(transaction.EventOperationDecomposeFailed, func(payload []byte) {
					var ack transaction.ConfigOperationDecomposeAck
					if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == req.TransactionID {
						decomposeFailedCh <- ack
					}
				}),
			)
			defer closeUnsubs(unsubs)
		}

		roots := make([]string, 0, len(req.Diffs))
		for root := range req.Diffs {
			roots = append(roots, root)
		}
		sort.Strings(roots)

		var operations []transaction.ConfigOperation
		for _, root := range roots {
			decomposer, _ := transaction.OperationDecomposerFor(root)
			activeRoot, err := marshalOperationRoot(runningTree, root)
			if err != nil {
				return nil, err
			}
			candidateRoot, err := marshalOperationRoot(candidateTree, root)
			if err != nil {
				return nil, err
			}
			for _, diff := range req.Diffs[root] {
				ops, err := decomposeRootOperations(ctx, gateway, req.TransactionID, root, activeRoot, candidateRoot, diff, decomposer, participants, decomposeOKCh, decomposeFailedCh)
				if err != nil {
					return nil, err
				}
				operations = append(operations, ops...)
			}
		}
		if err := validateOperationDeclarations(participants, operations); err != nil {
			return nil, err
		}
		return operations, nil
	}
}

func decomposeRootOperations(ctx context.Context, gateway transaction.EventGateway, txID, root, activeRoot, candidateRoot string, diff transaction.DiffSection, decomposer transaction.OperationDecomposer, participants []transaction.Participant, okCh, failedCh <-chan transaction.ConfigOperationDecomposeAck) ([]transaction.ConfigOperation, error) {
	if decomposer != nil {
		return decomposer(ctx, transaction.DecomposeRequest{
			TransactionID: txID,
			Root:          diff.Root,
			ActiveRoot:    activeRoot,
			CandidateRoot: candidateRoot,
			Diff:          diff,
		})
	}

	participant, decl, ok := operationDeclForRoot(participants, root)
	if !ok {
		return nil, nil
	}
	if !decl.Decompose {
		return nil, fmt.Errorf("plugin %s declares config operations for root %s without operation decomposition", participant.Name, root)
	}
	if gateway == nil {
		return nil, fmt.Errorf("plugin %s declares operation decomposition for root %s but no event gateway is available", participant.Name, root)
	}
	payload, err := json.Marshal(transaction.ConfigOperationDecomposeEvent{
		TransactionID: txID,
		Root:          root,
		ActiveRoot:    activeRoot,
		CandidateRoot: candidateRoot,
		Diff:          diff,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal operation decompose %s: %w", root, err)
	}
	if _, err := gateway.EmitConfigEvent(transaction.EventOperationDecomposeFor(participant.Name), payload); err != nil {
		return nil, fmt.Errorf("emit operation decompose %s: %w", root, err)
	}
	return waitDecomposeAck(ctx, participant.Name, root, okCh, failedCh)
}

func operationDeclForRoot(participants []transaction.Participant, root string) (transaction.Participant, transaction.ConfigOperationDecl, bool) {
	for _, participant := range participants {
		for _, decl := range participant.ConfigOperations {
			if decl.Root == root {
				return participant, decl, true
			}
		}
	}
	return transaction.Participant{}, transaction.ConfigOperationDecl{}, false
}

func waitDecomposeAck(ctx context.Context, pluginName, root string, okCh, failedCh <-chan transaction.ConfigOperationDecomposeAck) ([]transaction.ConfigOperation, error) {
	for {
		select {
		case ack := <-okCh:
			if ack.Plugin != pluginName || ack.Root != root {
				continue
			}
			if ack.Status != transaction.CodeOK {
				return nil, fmt.Errorf("operation decompose for %s/%s failed: %s", pluginName, root, ack.Error)
			}
			return ack.Operations, nil
		case ack := <-failedCh:
			if ack.Plugin != pluginName || ack.Root != root {
				continue
			}
			return nil, fmt.Errorf("operation decompose for %s/%s failed: %s", pluginName, root, ack.Error)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func validateOperationDeclarations(participants []transaction.Participant, operations []transaction.ConfigOperation) error {
	for i := range operations {
		op := &operations[i]
		if !declaresOperation(participants, op) {
			return fmt.Errorf("plugin %s does not declare config operation %s for root %s", op.Owner, op.Type, op.Root)
		}
	}
	return nil
}

func declaresOperation(participants []transaction.Participant, op *transaction.ConfigOperation) bool {
	if op == nil || op.Owner == "" || op.Root == "" || op.Type == "" {
		return false
	}
	for _, participant := range participants {
		if participant.Name != op.Owner {
			continue
		}
		for _, decl := range participant.ConfigOperations {
			if decl.Root == op.Root && slices.Contains(decl.Operations, op.Type) {
				return true
			}
		}
	}
	return false
}

func closeUnsubs(unsubs []func()) {
	for _, unsub := range unsubs {
		unsub()
	}
}

func marshalOperationRoot(tree map[string]any, root string) (string, error) {
	subtree := ExtractConfigSubtree(tree, root)
	if subtree == nil {
		return "{}", nil
	}
	data, err := json.Marshal(subtree)
	if err != nil {
		return "", fmt.Errorf("marshal operation root %s: %w", root, err)
	}
	return string(data), nil
}

// buildTxInputs turns the affected plugin list into the typed participant
// slice, the diff map the orchestrator expects, and the per-plugin verify
// sections the RPC bridge hands back to SendConfigVerify.
//
// Participants are derived from the affected plugin registrations and
// sorted so the "bgp" participant (if present) comes last. The orchestrator
// emits events in participant order, and the bridge's synchronous dispatch
// loop then applies to plugins in that order -- matching the legacy
// reload.go semantic where BGP's peer reconciliation ran after every other
// plugin committed.
//
// The diff map is taken straight from the same buildDiffSections helper
// the legacy reload path used, so the shape of Added/Removed/Changed is
// identical between the two paths.
//
// verifySections carries the per-plugin candidate subtree sections (built
// by reload.go via ExtractConfigSubtree + WantsConfigRoots) straight
// through to the bridge. The orchestrator's VerifyEvent payload carries
// only the neutral diff representation, which is not the candidate shape
// the SDK's OnConfigVerify contract expects; forwarding the sections
// out-of-band preserves the contract without changing the orchestrator.
//
// Wildcard config roots (["*"]) are expanded to the concrete (sorted)
// list of roots present in the diff, because the orchestrator's
// filterDiffs does exact match lookups and has no wildcard awareness.
func buildTxInputs(affected []affectedPlugin, diff *config.ConfigDiff) ([]transaction.Participant, map[string][]transaction.DiffSection, map[string][]rpc.ConfigSection, error) {
	// Group the diff by the roots the participants actually declared, so the
	// orchestrator's exact-match filterDiffs can find a nested root's section
	// (see buildDiffSections). Collected before grouping because the grouping
	// decides the section roots, which allRoots (and therefore wildcard
	// expansion) is derived from below.
	declaredRoots := make([]string, 0, len(affected))
	for _, ap := range affected {
		if reg := ap.proc.Registration(); reg != nil {
			declaredRoots = append(declaredRoots, reg.WantsConfigRoots...)
		}
	}

	diffMap := make(map[string][]transaction.DiffSection)
	for _, section := range buildDiffSections(diff, declaredRoots) {
		diffMap[section.Root] = append(diffMap[section.Root], transaction.DiffSection{
			Root:    section.Root,
			Added:   section.Added,
			Removed: section.Removed,
			Changed: section.Changed,
		})
	}
	allRoots := make([]string, 0, len(diffMap))
	for root := range diffMap {
		allRoots = append(allRoots, root)
	}
	sort.Strings(allRoots)

	participants := make([]transaction.Participant, 0, len(affected))
	verifySections := make(map[string][]rpc.ConfigSection, len(affected))
	for _, ap := range affected {
		reg := ap.proc.Registration()
		if reg == nil {
			return nil, nil, nil, fmt.Errorf("plugin %q has no registration", ap.proc.Name())
		}
		if err := transaction.ValidatePluginName(ap.proc.Name()); err != nil {
			return nil, nil, nil, fmt.Errorf("plugin %q: %w", ap.proc.Name(), err)
		}
		roots := expandWildcardRoots(reg.WantsConfigRoots, allRoots)
		participants = append(participants, transaction.Participant{
			Name:             ap.proc.Name(),
			ConfigRoots:      roots,
			ConfigOperations: reg.ConfigOperations,
			VerifyBudget:     reg.VerifyBudget,
			ApplyBudget:      reg.ApplyBudget,
		})
		copied := make([]rpc.ConfigSection, len(ap.sections))
		copy(copied, ap.sections)
		verifySections[ap.proc.Name()] = copied
	}

	sortParticipantsBGPLast(participants)

	return participants, diffMap, verifySections, nil
}

// sortParticipantsBGPLast places the "bgp" participant at the tail of the
// slice, preserving the relative order of every other participant. Used
// by buildTxInputs so the orchestrator's serialized publish loop applies
// bgp after sysrib/interface/gr/etc. have finished, matching the legacy
// reload.go ordering.
func sortParticipantsBGPLast(participants []transaction.Participant) {
	sort.SliceStable(participants, func(i, j int) bool {
		if participants[i].Name == bgpParticipantName {
			return false
		}
		if participants[j].Name == bgpParticipantName {
			return true
		}
		return false
	})
}

// expandWildcardRoots replaces a "*" entry in the plugin's declared roots
// with the concrete list of roots that actually changed this transaction.
// Plugins with explicit roots are copied verbatim so the orchestrator sees
// exactly the roots the plugin registered interest in. The wildcard list
// is sorted by the caller, so the resulting slice is deterministic.
func expandWildcardRoots(declared, allRoots []string) []string {
	if slices.Contains(declared, "*") {
		out := make([]string, len(allRoots))
		copy(out, allRoots)
		return out
	}
	out := make([]string, len(declared))
	copy(out, declared)
	return out
}

// participantNames projects participant names for the RPC bridge. Kept as
// a helper so the caller does not hand the bridge the full participant slice
// (it only needs names; decoupling the two keeps the bridge simple).
func participantNames(participants []transaction.Participant) []string {
	names := make([]string, len(participants))
	for i, p := range participants {
		names[i] = p.Name
	}
	return names
}

// txResultToError converts a TxResult into the error shape reload.go's
// callers expect. StateCommitted maps to nil; StateAborted and
// StateRolledBack wrap the coordinator's error with the legacy prefixes
// so test assertions on error substrings ("config verify failed",
// "config apply") keep working.
func txResultToError(result *transaction.TxResult) error {
	if result == nil {
		return errors.New("transaction coordinator returned nil result")
	}
	if result.State == transaction.StateCommitted {
		return nil
	}
	if result.State == transaction.StateAborted {
		return fmt.Errorf("config verify failed: %w", result.Err)
	}
	if result.State == transaction.StateRolledBack {
		return fmt.Errorf("config apply partial failure: %w", result.Err)
	}
	if result.Err != nil {
		return fmt.Errorf("config transaction %s: %w", result.State, result.Err)
	}
	return fmt.Errorf("config transaction ended in unexpected state %q", result.State)
}

// restartPluginFn returns a RestartFunc that delegates plugin restart to the
// Server's spawner, or nil if no spawner is wired (tests). Nil is acceptable
// to NewTxCoordinator; the orchestrator skips the restart step when the
// function is nil.
func (s *Server) restartPluginFn() transaction.RestartFunc {
	if s.spawner == nil {
		return nil
	}
	return func(pluginName string) error {
		return s.restartPlugin(pluginName)
	}
}

// restartPlugin performs a best-effort restart of a broken plugin. Called by
// the orchestrator when a rollback ack reports CodeBroken. The spawner owns
// the respawn logic; the bridge only decides WHEN to restart.
func (s *Server) restartPlugin(pluginName string) error {
	if s.spawner == nil {
		return fmt.Errorf("no plugin spawner available to restart %s", pluginName)
	}
	pm := s.procManager.Load()
	if pm == nil {
		return fmt.Errorf("no process manager available to restart %s", pluginName)
	}
	if err := pm.Respawn(pluginName); err != nil {
		return fmt.Errorf("respawn %s: %w", pluginName, err)
	}
	return nil
}
