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

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

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

	gateway := newConfigEventGateway(s)
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

// operationPlannerFromTrees installs the planner the orchestrator calls once
// per transaction. It decomposes the roots that have a diff, computes the
// addresses that plan takes off the host, and decomposes again with that set
// where there is one, so every binder answers for the addresses it holds.
//
// The second pass is what phase 2 of the requirement needs
// (docs/architecture/config/apply-ordering.md). A binder's stop is decided by
// what ANOTHER root does: an address that moves between interfaces changes the
// `interface` root and leaves the binder's own config identical. The core
// cannot know which addresses move until the roots that own them have
// decomposed, and the binders cannot answer until it does, so the two happen
// in that order.
//
// There is no second pass where no address moves, which is every commit that
// edits an MTU, a description or a service. A decomposer whose root has no
// diff and no disturbed address has nothing to answer for: its active and
// candidate subtrees are the same bytes.
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

		plan := &operationPlan{
			gateway:       gateway,
			txID:          req.TransactionID,
			runningTree:   runningTree,
			candidateTree: candidateTree,
			participants:  participants,
			diffs:         req.Diffs,
			okCh:          decomposeOKCh,
			failedCh:      decomposeFailedCh,
		}

		changedRoots := sortedDiffRoots(req.Diffs)
		operations, err := plan.decomposeRoots(ctx, changedRoots, nil)
		if err != nil {
			return nil, err
		}
		disturbed := transaction.DisturbedAddresses(operations)
		if len(disturbed) > 0 {
			operations, err = plan.decomposeRoots(ctx, bindingRoots(changedRoots, participants), disturbed)
			if err != nil {
				return nil, err
			}
			if err := checkDisturbanceSettled(disturbed, operations); err != nil {
				return nil, err
			}
		}
		if err := validateOperationDeclarations(participants, operations); err != nil {
			return nil, err
		}
		return operations, nil
	}
}

// operationPlan holds what every decompose call in one transaction shares, so
// the call itself carries only what changes between roots.
type operationPlan struct {
	gateway       transaction.EventGateway
	txID          string
	runningTree   map[string]any
	candidateTree map[string]any
	participants  []transaction.Participant
	diffs         map[string][]transaction.DiffSection
	okCh          <-chan transaction.ConfigOperationDecomposeAck
	failedCh      <-chan transaction.ConfigOperationDecomposeAck
}

// decomposeRoots asks each root in turn for the operations it owns, and
// returns them in root order.
//
// A root with no diff is asked once, with an empty diff section. That is the
// binder whose own config did not change while the address it binds moves, and
// the caller passes a root of that shape only once an address is disturbed.
func (p *operationPlan) decomposeRoots(ctx context.Context, roots, disturbed []string) ([]transaction.ConfigOperation, error) {
	var operations []transaction.ConfigOperation
	for _, root := range roots {
		decomposer, _ := transaction.OperationDecomposerFor(root)
		activeRoot, err := marshalOperationRoot(p.runningTree, root)
		if err != nil {
			return nil, err
		}
		candidateRoot, err := marshalOperationRoot(p.candidateTree, root)
		if err != nil {
			return nil, err
		}
		sections := p.diffs[root]
		if len(sections) == 0 {
			sections = []transaction.DiffSection{{Root: root}}
		}
		for _, diff := range sections {
			ops, err := p.decomposeRoot(ctx, root, activeRoot, candidateRoot, diff, disturbed, decomposer)
			if err != nil {
				return nil, err
			}
			operations = append(operations, ops...)
		}
	}
	return operations, nil
}

// decomposeRoot asks one root for its operations, through its in-process
// decomposer where it registered one and over the plugin event where the
// declaration is all the engine has.
func (p *operationPlan) decomposeRoot(ctx context.Context, root, activeRoot, candidateRoot string, diff transaction.DiffSection, disturbed []string, decomposer transaction.OperationDecomposer) ([]transaction.ConfigOperation, error) {
	if decomposer != nil {
		return decomposer(ctx, transaction.DecomposeRequest{
			TransactionID:      p.txID,
			Root:               diff.Root,
			ActiveRoot:         activeRoot,
			CandidateRoot:      candidateRoot,
			Diff:               diff,
			DisturbedAddresses: disturbed,
		})
	}

	participant, decl, ok := operationDeclForRoot(p.participants, root)
	if !ok {
		return nil, nil
	}
	if !decl.Decompose {
		return nil, fmt.Errorf("plugin %s declares config operations for root %s without operation decomposition", participant.Name, root)
	}
	if p.gateway == nil {
		return nil, fmt.Errorf("plugin %s declares operation decomposition for root %s but no event gateway is available", participant.Name, root)
	}
	payload, err := json.Marshal(transaction.ConfigOperationDecomposeEvent{
		TransactionID:      p.txID,
		Root:               root,
		ActiveRoot:         activeRoot,
		CandidateRoot:      candidateRoot,
		Diff:               diff,
		DisturbedAddresses: disturbed,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal operation decompose %s: %w", root, err)
	}
	if _, err := p.gateway.EmitConfigEvent(transaction.EventOperationDecomposeFor(participant.Name), payload); err != nil {
		return nil, fmt.Errorf("emit operation decompose %s: %w", root, err)
	}
	return waitDecomposeAck(ctx, participant.Name, root, p.okCh, p.failedCh)
}

// bindingRoots returns the roots the second pass asks: the roots with a diff,
// plus every root that decomposes and has none.
//
// A root that decomposes is asked whether it binds a disturbed address,
// whether or not its own config changed. That is what makes phase 2 reach a
// binder at all: the commit that moves an address touches the `interface` root
// alone, and the peer bound to that address is declared in a root with no diff.
//
// The answer comes from what each component REGISTERED, in process or through
// its plugin declaration, so the core names no root and no binder
// (ai/rules/principles.md).
func bindingRoots(changedRoots []string, participants []transaction.Participant) []string {
	seen := make(map[string]struct{}, len(changedRoots))
	roots := make([]string, 0, len(changedRoots))
	add := func(root string) {
		if root == "" {
			return
		}
		if _, exists := seen[root]; exists {
			return
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	for _, root := range changedRoots {
		add(root)
	}
	for _, root := range transaction.OperationDecomposerRoots() {
		add(root)
	}
	for _, participant := range participants {
		for _, decl := range participant.ConfigOperations {
			if decl.Decompose {
				add(decl.Root)
			}
		}
	}
	slices.Sort(roots)
	return roots
}

// ErrDisturbanceUnsettled reports a second decomposition pass that takes a
// different set of addresses off the host from the set every binder was told
// about.
//
// It fails the transaction closed. The binders answered for the first set, so
// a plan that removes an address outside it leaves a binder holding a binding
// the core said nothing about, which is the failure phase 2 exists to prevent
// (ai/rules/principles.md). No first-party root reaches it: only `interface`
// produces addresses, and its decomposition reads the diff and the two trees
// and never the disturbed set.
var ErrDisturbanceUnsettled = errors.New("config operation planning disturbs a different address set on the second pass")

// checkDisturbanceSettled reports whether the second pass took the same
// addresses off the host as the first.
func checkDisturbanceSettled(disturbed []string, operations []transaction.ConfigOperation) error {
	settled := transaction.DisturbedAddresses(operations)
	if slices.Equal(settled, disturbed) {
		return nil
	}
	return fmt.Errorf("%w: first pass %v, second pass %v", ErrDisturbanceUnsettled, disturbed, settled)
}

// sortedDiffRoots returns the config roots this transaction has a diff on, in
// a fixed order so two runs plan the same operations in the same sequence.
func sortedDiffRoots(diffs map[string][]transaction.DiffSection) []string {
	roots := make([]string, 0, len(diffs))
	for root := range diffs {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	return roots
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

// validateOperationDeclarations refuses a planned operation the emitting
// plugin did not declare, one that declares no verb, and one carrying the
// section-apply label.
//
// A missing verb and a resource entry naming nothing are both refused here, at
// the planner, so a plugin process that sends either is named in the error and
// nothing it emitted reaches the graph. The orchestrator refuses the same
// operation again when the plan comes back, because it accepts a plan from any
// installed planner and a value that cannot be ordered must not be reachable
// through either door (ai/rules/principles.md).
//
// The section-apply label belongs to the coarse node the orchestrator
// synthesizes for a participant with no operations, and the executor routes it
// to the section apply instead of the per-operation callback. A plugin that
// declared and emitted it would have its whole section applied under an
// operation's name, in the position the graph gave that operation.
func validateOperationDeclarations(participants []transaction.Participant, operations []transaction.ConfigOperation) error {
	if err := transaction.ValidateOperations(operations); err != nil {
		return err
	}
	for i := range operations {
		op := &operations[i]
		if op.Type == transaction.OperationSectionApply {
			return fmt.Errorf("plugin %s returned operation %s for root %s with the reserved type %s", op.Owner, op.ID, op.Root, op.Type)
		}
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
	slices.Sort(allRoots)

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

	// The participants come back in the order buildTxInputs read them, and
	// nothing here moves one. Which participant applies last is decided by
	// the operation graph: the solver places a coarse section-apply node
	// before the decomposed starts that bind what it configures
	// (placeSectionNodes in internal/component/config/transaction/solver.go).
	// A sort that moved the participant called "bgp" to the tail used to
	// stand in for that, and it was a central enumeration of one component's
	// name in a core file (ai/rules/principles.md).
	return participants, diffMap, verifySections, nil
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

// restartPlugin restarts a broken plugin. Called by the orchestrator when a
// rollback ack reports CodeBroken. The spawner owns the respawn policy (limits,
// the disabled set); this function owns everything the engine must do around it.
//
// The respawn is only half of a restart. It replaces the PROCESS and runs no
// startup handshake, so the replacement holds no registration, no delivered
// config, no subscriptions, no commands and no exclusive-role claim set. The
// three steps below are ordered and none is optional:
//
//  1. Respawn, which stops the old process and spawns its replacement.
//  2. Release the old process's name-keyed registrations, so Stage 1 of the
//     replacement's handshake is not refused its own plugin's name.
//  3. Run the handshake on the replacement and start its runtime handler.
//
// Step 2 runs AFTER step 1 on purpose: a respawn that is refused (limit
// exceeded, plugin disabled, no such plugin) must leave the running plugin
// exactly as it found it, and a teardown before the refusal would unregister a
// plugin that is still serving.
func (s *Server) restartPlugin(pluginName string) error {
	if s.spawner == nil {
		return fmt.Errorf("no plugin spawner available to restart %s", pluginName)
	}
	pm := s.procManager.Load()
	if pm == nil {
		return fmt.Errorf("no process manager available to restart %s", pluginName)
	}

	old := pm.GetProcess(pluginName)
	newProc, err := pm.Respawn(pluginName)
	if err != nil {
		return fmt.Errorf("respawn %s: %w", pluginName, err)
	}

	if old != nil {
		s.releasePluginRegistrations(old)
	}
	if err := s.restartHandshake(newProc); err != nil {
		return fmt.Errorf("restart handshake %s: %w", pluginName, err)
	}
	return nil
}
