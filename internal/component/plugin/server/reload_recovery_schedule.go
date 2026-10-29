// Design: docs/architecture/config/transaction-protocol.md -- rejected reload compensation
// Related: reload_compensation.go -- restoreReload, which runs the schedule
// Related: startup_autoload.go -- selectAutoLoad and startAutoLoaded
package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config/transaction"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// inverseKey names one retained original inverse by its transaction, the owner
// lifetime captured when the operation applied, and the operation's position
// in that transaction. A name alone would also match a replacement process or
// another transaction's operation carrying the same identifier.
type inverseKey struct {
	txID     string
	owner    *process.Process
	position int
}

// inverseOutcome records an inverse that reached its owner. Sending consumes
// the owner's journal entry whether the owner acknowledged success or not, so
// a recorded inverse is never sent again: a later attempt reports err instead.
// An inverse that was never sent has no outcome and stays owed.
type inverseOutcome struct {
	err error
}

// inverseAction is one retained original inverse, in dispatch order.
type inverseAction struct {
	batch       *reloadOperationBatch
	position    int
	participant reloadParticipant
}

func (a inverseAction) key() inverseKey {
	return inverseKey{txID: a.batch.txID, owner: a.participant.proc, position: a.position}
}

func (a inverseAction) operation() *transaction.ConfigOperation {
	return &a.batch.operations[a.position]
}

// recoverySchedule interleaves the restarts of removed owners with the
// retained owners' original inverses. starts[k] runs before chain[k], and
// starts[len(chain)] after the last inverse.
type recoverySchedule struct {
	chain  []inverseAction
	starts [][]plugin.PluginConfig
}

// restoreOriginalJournals restores the owners a rejected reload removed and
// replays the retained owners' original journals, as one schedule under one
// rollback deadline. Selection and ordering finish before anything executes:
// a cycle or an order the retained journals cannot satisfy is refused with
// nothing started and nothing undone. addedRoots are the roots the rollback
// adds back to tree; nil restores journals only.
func (s *Server) restoreOriginalJournals(pending *reloadAcceptance, undo *reloadCompensation, tree map[string]any, addedRoots []string) error {
	ctx, cancel := context.WithTimeout(pending.lifetime, reloadCompletionBudget)
	defer cancel()
	if err := pending.acquireCompletion(ctx); err != nil {
		return err
	}
	defer func() { <-pending.completionGate }()

	chain, consumed := pending.inverseChain(undo)
	var recovery []plugin.PluginConfig
	if len(addedRoots) != 0 {
		var err error
		recovery, err = s.selectAutoLoad(tree, addedRoots)
		if err != nil {
			return errors.Join(consumed, err)
		}
	}
	schedule, err := planRecoverySchedule(chain, recovery)
	if err != nil {
		return errors.Join(consumed, err)
	}
	return errors.Join(consumed, s.runRecoverySchedule(ctx, pending, undo, schedule, tree))
}

// inverseChain answers the retained inverses still owed, in exact reverse
// batch and operation order, and the joined failures of those already
// consumed. Removed owners are excluded: their lifetime ended, and their
// restart is the recovery set's business.
func (a *reloadAcceptance) inverseChain(undo *reloadCompensation) ([]inverseAction, error) {
	var chain []inverseAction
	var consumed error
	for _, batch := range slices.Backward(a.operationBatches()) {
		if batch.undoDone {
			consumed = errors.Join(consumed, batch.undoErr)
			continue
		}
		for position := len(batch.operations) - 1; position >= 0; position-- {
			op := &batch.operations[position]
			participant := batch.participants[op.Owner]
			if !a.ownsInverse(participant, op) {
				continue
			}
			action := inverseAction{batch: batch, position: position, participant: participant}
			if outcome, recorded := undo.inverses[action.key()]; recorded {
				consumed = errors.Join(consumed, outcome.err)
				continue
			}
			chain = append(chain, action)
		}
	}
	return chain, consumed
}

// ownsInverse reports whether op has an original journal its owner undoes: a
// declared operation, not a coarse section, of a completion owner whose
// captured lifetime the reload did not end by acknowledged removal.
func (a *reloadAcceptance) ownsInverse(participant reloadParticipant, op *transaction.ConfigOperation) bool {
	// refuse-by-default: guards inverseChain -- an owner the batch never captured holds no journal to undo
	if participant.proc == nil {
		return false
	}
	if a.removedParticipant(participant) {
		return false
	}
	if transaction.IsSectionApply(op) {
		return false
	}
	return participant.proc.Registration().ConfigCompletion
}

// planRecoverySchedule places each removed owner between the retained inverses.
//
// The retained chain keeps its exact order. The dependency graph is the one
// startup uses (selectedPluginGraph: Dependencies, OptionalDependencies,
// StartAfter, aliases and external programs) over removed and retained
// participants together, and removedCycle refuses a cycle that passes through
// a removed participant. A cycle of retained participants alone is left
// alone: they are not started, and the chain already fixes their order.
// A removed owner starts after the last inverse of every retained owner it
// reaches through the graph, and before the first inverse of every retained
// owner that reaches it. Reach runs through removed owners and stops at a
// retained one, whose own order the chain already fixes. Between those bounds
// the owner takes the coarse placement (transaction.InverseSectionPosition):
// after the last inverse that can restore addressing, before the first binder
// an inverse brings back. A declared edge outranks that placement, which only
// guesses for participants that declared nothing.
//
// Removed owners sharing a slot start in one phase, which tiers them; the
// bounds and the placement only grow along a dependency, so a removed
// prerequisite never takes a later slot than its removed dependent.
func planRecoverySchedule(chain []inverseAction, recovery []plugin.PluginConfig) (*recoverySchedule, error) {
	schedule := &recoverySchedule{chain: chain, starts: make([][]plugin.PluginConfig, len(chain)+1)}
	if len(recovery) == 0 {
		return schedule, nil
	}
	first := make(map[string]int)
	last := make(map[string]int)
	var retained []plugin.PluginConfig
	originals := make([]transaction.ConfigOperation, len(chain))
	for k, action := range chain {
		name := action.participant.proc.Name()
		if _, seen := first[name]; !seen {
			first[name] = k
			retained = append(retained, action.participant.proc.Config())
		}
		last[name] = k
		originals[k] = *action.operation()
	}
	graph := selectedPluginGraph(recovery, retained)
	if cycle := removedCycle(graph, recovery); cycle != nil {
		return nil, fmt.Errorf("reload compensation cannot order the removed and retained plugins, so nothing was restored: %w: "+
			"plugins %s each declare that they start after another of them, so no restart order exists.\n"+
			"Restart the ze daemon so every plugin starts from the committed configuration, under systemd:\n"+
			"  systemctl restart ze", registry.ErrCircularDependency, strings.Join(cycle, ", "))
	}
	removed := make(map[string]bool, len(recovery))
	for _, cfg := range recovery {
		removed[cfg.Name] = true
	}
	dependents := reversePluginGraph(graph)
	placement := transaction.InverseSectionPosition(originals)
	for _, cfg := range recovery {
		earliest, after := 0, ""
		for _, owner := range retainedReach(graph, removed, cfg.Name) {
			if last[owner]+1 > earliest {
				earliest, after = last[owner]+1, owner
			}
		}
		latest, before := len(chain), ""
		for _, owner := range retainedReach(dependents, removed, cfg.Name) {
			if first[owner] < latest {
				latest, before = first[owner], owner
			}
		}
		if earliest > latest {
			// No cycle exists: the declared edges are satisfiable, but not
			// inside the fixed undo order, which is never reordered.
			return nil, fmt.Errorf("reload compensation cannot restart removed plugin %s, so nothing was restored: "+
				"%s depends on retained plugin %s, and retained plugin %s depends on %s, "+
				"so %s must start after %s is undone and before %s is undone; "+
				"undo runs in reverse apply order, which undoes %s before %s, so no slot is left for %s.\n"+
				"Restart the ze daemon so every plugin starts from the committed configuration, under systemd:\n"+
				"  systemctl restart ze",
				cfg.Name, cfg.Name, after, before, cfg.Name, cfg.Name, after, before, before, after, cfg.Name)
		}
		slot := min(max(placement, earliest), latest)
		schedule.starts[slot] = append(schedule.starts[slot], cfg)
	}
	return schedule, nil
}

// removedCycle answers, sorted, the members of the first dependency cycle that
// passes through a removed participant, or nil when none does. A participant
// is in such a cycle when it reaches itself; the cycle's members are those it
// reaches that also reach it. Bounded by the graph's size: one forward and one
// backward walk per removed participant.
func removedCycle(graph map[string][]string, recovery []plugin.PluginConfig) []string {
	dependents := reversePluginGraph(graph)
	for _, cfg := range recovery {
		forward := pluginReach(graph, cfg.Name)
		if !forward[cfg.Name] {
			continue
		}
		backward := pluginReach(dependents, cfg.Name)
		var cycle []string
		for name := range forward {
			if backward[name] {
				cycle = append(cycle, name)
			}
		}
		slices.Sort(cycle)
		return cycle
	}
	return nil
}

// pluginReach answers every participant start reaches along graph, start
// itself only when a path returns to it. Bounded by the graph's size: each
// participant is visited once.
func pluginReach(graph map[string][]string, start string) map[string]bool {
	reached := make(map[string]bool)
	queue := []string{start}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, next := range graph[name] {
			if reached[next] {
				continue
			}
			reached[next] = true
			queue = append(queue, next)
		}
	}
	return reached
}

// retainedReach answers the retained participants start reaches along graph,
// going through removed participants only. Bounded by the graph's size:
// each participant is visited once.
func retainedReach(graph map[string][]string, removed map[string]bool, start string) []string {
	var reached []string
	visited := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, next := range graph[name] {
			if visited[next] {
				continue
			}
			visited[next] = true
			if removed[next] {
				queue = append(queue, next)
				continue
			}
			reached = append(reached, next)
		}
	}
	return reached
}

// reversePluginGraph turns dependent-to-prerequisite edges around.
func reversePluginGraph(graph map[string][]string) map[string][]string {
	reversed := make(map[string][]string, len(graph))
	for _, name := range slices.Sorted(maps.Keys(graph)) {
		for _, dependency := range graph[name] {
			reversed[dependency] = append(reversed[dependency], name)
		}
	}
	return reversed
}

// runRecoverySchedule executes the schedule under ctx, the one rollback
// context of the whole restoration: recovery startups and inverse
// acknowledgements share its deadline. An inverse that reached its owner is
// recorded whatever its result and the schedule continues, as a rollback
// does. A recovery startup that fails, a deadline that ends before an inverse
// is sent, or an inverse that could not be sent stops the schedule: the unsent
// inverses stay owed for a retry, which keeps them in reverse apply order.
func (s *Server) runRecoverySchedule(ctx context.Context, pending *reloadAcceptance, undo *reloadCompensation, schedule *recoverySchedule, tree map[string]any) error {
	var result error
	var open *inverseDispatch
	defer func() { open.close() }()
	for k := 0; k <= len(schedule.chain); k++ {
		if _, err := s.startAutoLoaded(ctx, schedule.starts[k], tree, true); err != nil {
			return errors.Join(result, err)
		}
		if k == len(schedule.chain) {
			break
		}
		action := schedule.chain[k]
		op := action.operation()
		if err := ctx.Err(); err != nil {
			return errors.Join(result, fmt.Errorf("reload compensation deadline ended before undo %s of plugin %s was sent: %w", op.ID, op.Owner, err))
		}
		if open == nil || open.batch != action.batch {
			open.close()
			var err error
			open, err = s.openInverseDispatch(ctx, action.batch)
			if err != nil {
				return errors.Join(result, err)
			}
		}
		sent, err := open.driver.Undo(ctx, op)
		if !sent {
			return errors.Join(result, fmt.Errorf("reload compensation stopped before undo %s of plugin %s was sent, so it is still owed and the next reload retries it: %w", op.ID, op.Owner, err))
		}
		undo.inverses[action.key()] = inverseOutcome{err: err}
		pending.settleInverseBatch(undo, action.batch)
		result = errors.Join(result, err)
	}
	return result
}

// settleInverseBatch marks a batch undone once every inverse it owes has been
// dispatched, carrying their joined failures.
func (a *reloadAcceptance) settleInverseBatch(undo *reloadCompensation, batch *reloadOperationBatch) {
	var failed error
	for position := range batch.operations {
		op := &batch.operations[position]
		participant := batch.participants[op.Owner]
		if !a.ownsInverse(participant, op) {
			continue
		}
		outcome, recorded := undo.inverses[inverseKey{txID: batch.txID, owner: participant.proc, position: position}]
		if !recorded {
			return
		}
		failed = errors.Join(failed, outcome.err)
	}
	batch.undoDone, batch.undoErr = true, failed
}

// inverseDispatch holds one transaction's acknowledgement subscriptions while
// its inverses are dispatched. close MUST run after the last Undo.
type inverseDispatch struct {
	batch  *reloadOperationBatch
	bridge *configTxBridge
	driver *transaction.RollbackDriver
}

func (s *Server) openInverseDispatch(ctx context.Context, batch *reloadOperationBatch) (*inverseDispatch, error) {
	gateway := newConfigEventGateway(s)
	bridge := newConfigTxBridge(s, gateway, slices.Sorted(maps.Keys(batch.participants)), nil)
	bridge.participants = batch.participants
	if err := bridge.Subscribe(ctx); err != nil {
		return nil, fmt.Errorf("subscribe to undo acknowledgements of transaction %s: %w", batch.txID, err)
	}
	executor := transaction.NewOperationExecutor(gateway, batch.txID)
	return &inverseDispatch{batch: batch, bridge: bridge, driver: executor.NewRollbackDriver(len(batch.operations))}, nil
}

func (d *inverseDispatch) close() {
	// refuse-by-default: guards runRecoverySchedule -- nil is the state before the first batch opens
	if d == nil {
		return
	}
	d.driver.Close()
	d.bridge.Close()
}
