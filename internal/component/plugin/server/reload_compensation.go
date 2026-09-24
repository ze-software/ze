// Design: docs/architecture/config/transaction-protocol.md -- rejected reload compensation
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Keep the first tree and last applied tree of a whole reload. The hub may
// already have reversed its candidate before it rejects the outer scope.
type reloadCompensation struct {
	before          map[string]any
	after           map[string]any
	affected        map[string]affectedPlugin
	attempted       bool
	err             error
	restored        *reloadAcceptance
	reactorRestored bool
}

// claimReloadOwnership runs under transaction exclusion. A no-diff reload also
// claims its tree: accepting it supersedes an older pending publication.
func (s *Server) claimReloadOwnership(pending *reloadAcceptance) {
	pending.mu.Lock()
	if !pending.claimed {
		pending.previous = s.txLock.configOwner
		pending.claimed = true
	}
	pending.mu.Unlock()
	s.txLock.configOwner = pending
}

func recordReloadCompensation(ctx context.Context, before, after map[string]any, affected []affectedPlugin) {
	pending, _ := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	if pending == nil {
		return
	}
	pending.server.claimReloadOwnership(pending)
	pending.mu.Lock()
	defer pending.mu.Unlock()
	if pending.compensation == nil {
		pending.compensation = &reloadCompensation{before: before, affected: make(map[string]affectedPlugin)}
	}
	undo := pending.compensation
	undo.after = after
	undo.attempted = false
	undo.err = nil
	undo.restored = nil
	undo.reactorRestored = false
	for _, participant := range affected {
		undo.affected[participant.proc.Name()] = participant
	}
}

// compensateReload holds transaction exclusion throughout restoration. A child
// rejection returns ownership to its predecessor, including a predecessor whose
// outer caller already rejected it while the child was still pending.
func (s *Server) compensateReload(ctx context.Context) error {
	pending, _ := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	if pending == nil || s.txLock.configOwner != pending {
		return nil
	}
	for pending != nil {
		pending.mu.Lock()
		undo, previous := pending.compensation, pending.previous
		finished := pending.finished
		pending.mu.Unlock()
		if undo != nil {
			if !undo.attempted {
				undo.attempted = true
				undo.err = s.restoreReload(pending, undo)
			}
			if undo.err != nil {
				s.txLock.pendingCompensation = pending
				return fmt.Errorf("restore committed configuration: %w", undo.err)
			}
		}
		if s.txLock.pendingCompensation == pending {
			s.txLock.pendingCompensation = nil
		}
		s.txLock.configOwner = previous
		if finished {
			pending.complete(false)
		}
		if previous == nil {
			return nil
		}
		previous.mu.Lock()
		rejected := previous.finished && !previous.accepted
		previous.mu.Unlock()
		if !rejected {
			return nil
		}
		pending = previous
	}
	return nil
}

func (s *Server) retryReloadCompensation() error {
	pending := s.txLock.pendingCompensation
	if pending == nil {
		return nil
	}
	pending.mu.Lock()
	pending.compensation.attempted = false
	pending.mu.Unlock()
	return s.compensateReload(context.WithValue(s.Context(), reloadAcceptanceKey{}, pending))
}

// A rejected ancestor waits for the newer pending scope to settle. Acceptance
// of that newer tree resolves the ancestor; rejection restores its ownership.
func (s *Server) reloadScopePendingBehind(scope *reloadAcceptance) bool {
	for current := s.txLock.configOwner; current != nil && current != scope; {
		current.mu.Lock()
		previous := current.previous
		current.mu.Unlock()
		if previous == scope {
			return true
		}
		current = previous
	}
	return false
}

func (s *Server) restoreReload(pending *reloadAcceptance, undo *reloadCompensation) error {
	diff := config.DiffMaps(undo.after, undo.before)
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		return nil
	}
	if undo.restored == nil {
		restoreCtx, accept := s.DeferReloadAcceptance(s.Context())
		s.seedReloadTransactions(restoreCtx)
		if len(diff.Added) != 0 {
			recoveryCtx := context.WithValue(restoreCtx, removalRecoveryContextKey{}, true)
			if _, err := s.autoLoadForNewConfigPaths(recoveryCtx, undo.before, slices.Sorted(maps.Keys(diff.Added))); err != nil {
				accept(false)
				return err
			}
		}
		var affected []affectedPlugin
		pm := s.procManager.Load()
		for _, participant := range undo.affected {
			current := participant.proc
			if pm != nil {
				if replacement := pm.GetProcess(current.Name()); replacement != nil {
					current = replacement
				}
			}
			if s.pluginRemovalPending(current) {
				continue
			}
			participant.proc = current
			sections, err := reloadConfigSections(undo.before, diff, current.Registration().WantsConfigRoots)
			if err != nil {
				accept(false)
				return err
			}
			participant.sections = sections
			affected = append(affected, participant)
		}
		if err := s.runTxCoordinator(restoreCtx, affected, diff, undo.after, undo.before); err != nil {
			accept(false)
			return err
		}
		undo.restored, _ = restoreCtx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	}
	if !undo.reactorRestored {
		if err := s.reactor.ApplyConfigDiff(undo.before); err != nil {
			return err
		}
		undo.reactorRestored = true
	}
	if len(diff.Removed) != 0 {
		removed := s.collectProcessesForRemovedConfigPaths(slices.Sorted(maps.Keys(diff.Removed)))
		if err := s.stopCollectedProcesses(removed); err != nil {
			return err
		}
	}
	s.reactor.SetConfigTree(undo.before)
	undo.restored.mu.Lock()
	s.txLock.configTransactions = undo.restored.transactions
	undo.restored.mu.Unlock()
	pending.mu.Lock()
	undo.after = undo.before
	pending.transactions = s.txLock.configTransactions
	pending.mu.Unlock()
	undo.restored.finish(true)
	return nil
}

func reloadConfigSections(tree map[string]any, diff *config.ConfigDiff, roots []string) ([]rpc.ConfigSection, error) {
	var sections []rpc.ConfigSection
	for _, root := range roots {
		if !rootHasChanges(diff, root) {
			continue
		}
		subtree := ExtractConfigSubtree(tree, root)
		if subtree == nil {
			sections = append(sections, rpc.ConfigSection{Root: root, Data: "{}"})
			continue
		}
		data, err := json.Marshal(subtree)
		if err != nil {
			return nil, fmt.Errorf("marshal %s config: %w", root, err)
		}
		sections = append(sections, rpc.ConfigSection{Root: root, Data: string(data)})
	}
	return sections, nil
}
