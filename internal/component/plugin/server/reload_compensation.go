// Design: docs/architecture/config/transaction-protocol.md -- rejected reload compensation
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Keep the first tree and last applied tree of a whole reload. The hub may
// already have reversed its candidate before it rejects the outer scope.
type reloadCompensation struct {
	before map[string]any
	// created is the reactor's created peers when before was recorded. A
	// reload whose tree declares one takes it over, and restoring before does
	// not mark it again, nor rebuild one it renamed (RestoreCreatedPeers,
	// plugin.ReactorConfigurator).
	created         map[netip.Addr]string
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
	// Lock order: the reactor's lock is never taken under pending.mu, so the
	// snapshot is read before pending.mu. Only the first record keeps it.
	created := pending.server.reactor.CreatedPeers()
	pending.mu.Lock()
	defer pending.mu.Unlock()
	if pending.compensation == nil {
		pending.compensation = &reloadCompensation{
			before:   before,
			created:  created,
			affected: make(map[string]affectedPlugin),
		}
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
		return s.reactor.RestoreCreatedPeers(undo.before, undo.created)
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
			sections, err := reloadConfigSections(undo.before, diff, current.Registration())
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
		// Before the reconcile, which removes a running peer the file lacks
		// unless it is marked created.
		if err := s.reactor.RestoreCreatedPeers(undo.before, undo.created); err != nil {
			return err
		}
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
	// And after SetConfigTree, which drops every created peer the tree declares.
	// A created peer the reconcile freed the address of, because the reload
	// had declared that address under another name, is rebuilt here.
	if err := s.reactor.RestoreCreatedPeers(undo.before, undo.created); err != nil {
		return err
	}
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

// reloadConfigSections answers the sections a reload delivers to one plugin.
// A plugin receives the roots of its registration that changed, and an empty
// section for a changed root the tree no longer holds, which is its deletion.
//
// A plugin that reads another root (reg.ConfigReads) receives every root of its
// registration whole as soon as one of them changed. Its verifier checks a
// relation across roots, as static and BGP check a named bfd-profile against
// the bfd section: a bgp edit must carry the bfd section to check against, and
// a bfd edit must reach BGP, with the bgp section, to be checked at all. A root
// absent from the tree that did not change is not delivered, so a reader is
// never told its own root was deleted.
func reloadConfigSections(tree map[string]any, diff *config.ConfigDiff, reg *plugin.PluginRegistration) ([]rpc.ConfigSection, error) {
	roots := reg.WantsConfigRoots
	whole := false
	if len(reg.ConfigReads) > 0 {
		whole = slices.ContainsFunc(roots, func(root string) bool { return rootHasChanges(diff, root) })
	}
	var sections []rpc.ConfigSection
	for _, root := range roots {
		changed := rootHasChanges(diff, root)
		if !changed && !whole {
			continue
		}
		subtree := ExtractConfigSubtree(tree, root)
		if subtree == nil {
			if changed {
				sections = append(sections, rpc.ConfigSection{Root: root, Data: "{}"})
			}
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
