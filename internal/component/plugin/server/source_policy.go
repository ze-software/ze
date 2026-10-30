// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle.
package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SourcePolicyOwner captures one immutable process and connection identity.
// A coordinator MUST retain these handles through prepare/apply/abort/finalize,
// never recapture successors to complete work owned by an earlier lifetime.
// Safe for concurrent calls; transaction sequencing belongs to the coordinator.
type SourcePolicyOwner struct {
	name   string
	server *Server
	proc   *process.Process
	conn   *ipc.PluginConn
}

// CaptureSourcePolicyOwners preflights the selected processes' Stage 1 roles.
// Observers are excluded. A declared owner without a handler refuses the whole
// capture before any callback runs, with an operator remedy that needs no
// lifecycle support: take the peer down around the change. Callers MUST capture before policy publication.
func (s *Server) CaptureSourcePolicyOwners(names []string) ([]*SourcePolicyOwner, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, fmt.Errorf("source-policy: plugin process manager is unavailable")
	}
	owners := make([]*SourcePolicyOwner, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		proc := pm.GetProcess(name)
		if proc == nil {
			return nil, fmt.Errorf("source-policy owner %s: selected process is unavailable", name)
		}
		reg := proc.Registration()
		if reg == nil {
			return nil, fmt.Errorf("source-policy owner %s: Stage 1 registration is unavailable", name)
		}
		if !reg.SourcePolicyOwner {
			if reg.SourcePolicy {
				return nil, fmt.Errorf("source-policy owner %s: handler requires source-policy-owner declaration", name)
			}
			continue
		}
		if !reg.SourcePolicy {
			return nil, fmt.Errorf("plugin %s holds routes received from this peer but cannot remove them when the "+
				"peer's import policy is withdrawn (it declares source-policy-owner without a source-policy handler), "+
				"so the change is refused before anything is published; to change the policy, disable the peer, "+
				"commit, set the new policy, re-enable the peer and commit again, or stop using %s for this peer", name, name)
		}
		owner := &SourcePolicyOwner{name: name, server: s, proc: proc, conn: proc.Conn()}
		if err := owner.current(); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return orderSourcePolicyOwners(owners)
}

// orderSourcePolicyOwners puts every captured owner after the captured owners
// it depends on, by the edges startup orders the same processes with
// (selectedPluginGraph: Dependencies, OptionalDependencies, StartAfter), and
// breaks ties by process name. A dependent owner (bgp-rs, which reads what
// bgp-rib and bgp-adj-rib-in hold) is then called after them, so its own settle
// barrier covers the work their callbacks published. Abort and finalize use the
// same order: each owner restores or drops only its own state, and a
// dependent's settle still has to follow the work its prerequisites publish.
func orderSourcePolicyOwners(owners []*SourcePolicyOwner) ([]*SourcePolicyOwner, error) {
	configs := make([]plugin.PluginConfig, 0, len(owners))
	byName := make(map[string]*SourcePolicyOwner, len(owners))
	for _, owner := range owners {
		configs = append(configs, owner.proc.Config())
		byName[owner.name] = owner
	}
	tiers, err := registry.TopologicalGraphTiers(selectedPluginGraph(configs))
	if err != nil {
		return nil, fmt.Errorf("source-policy owners cannot be ordered by their plugin dependencies: %w", err)
	}
	ordered := make([]*SourcePolicyOwner, 0, len(owners))
	for _, tier := range tiers {
		for _, name := range tier {
			if owner, captured := byName[name]; captured {
				ordered = append(ordered, owner)
			}
		}
	}
	if len(ordered) != len(owners) {
		// A process whose config names another label would leave the order
		// without its owner, and its state would never be invalidated.
		return nil, fmt.Errorf("source-policy owners: ordered %d of %d captured owners; a process config name differs from its process name", len(ordered), len(owners))
	}
	return ordered, nil
}

// DrainEventSubscribers waits until every plugin process subscribed to the
// (namespace, eventType) event has applied each such event queued for it
// before the call, so work a subscriber issues in answer (a destination write)
// has been issued when it returns. The processes are the ones deliverEvent
// feeds for an event carrying no peer address. Engine-side subscribers need no
// drain: they run synchronously inside the emit. A coordinator calls it after
// an owner callback that publishes the event and before it settles the writers
// that work reaches. ctx bounds every wait. Callers MUST NOT hold locks the
// subscribers' handlers need. Safe for concurrent use.
func (s *Server) DrainEventSubscribers(ctx context.Context, namespace, eventType string) error {
	if !events.IsValidEvent(namespace, eventType) {
		return fmt.Errorf("drain subscribers of %s/%s: event is not registered", namespace, eventType)
	}
	// refuse-by-default: guards Server.subscriptions -- with no subscription manager deliverEvent queues the event to no process, so none has work to apply
	if s.subscriptions == nil {
		return nil
	}
	procs := s.subscriptions.getMatching(events.LookupNamespaceID(namespace), events.LookupEventTypeID(eventType),
		events.DirUnspecified, "", "")
	var result error
	for _, proc := range procs {
		if err := proc.DrainEventsApplied(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("subscriber of %s/%s: %w", namespace, eventType, err))
		}
	}
	return result
}

// Name returns the selected process name, not its implementation alias.
func (owner *SourcePolicyOwner) Name() string { return owner.name }

// Call waits for actual application on the captured owner, never a successor.
// Before prepare the coordinator MUST settle its receive-publication frontier;
// this method then strictly drains event application. Callers MUST NOT hold any
// locks handlers need. Owners MUST settle asynchronous and downstream work before
// acknowledging; the process FIFO alone cannot establish that settlement.
func (owner *SourcePolicyOwner) Call(ctx context.Context, input rpc.SourcePolicyInput) error {
	if err := input.Validate(time.Now()); err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := owner.current(); err != nil {
		return err
	}
	if input.Operation == rpc.SourcePolicyPrepare {
		if err := owner.proc.DrainEventsApplied(ctx); err != nil {
			return err
		}
		if err := owner.current(); err != nil {
			return err
		}
	}
	if err := owner.conn.SendSourcePolicy(ctx, input); err != nil {
		return fmt.Errorf("source-policy owner %s: %w", owner.name, err)
	}
	if err := owner.current(); err != nil {
		return err
	}
	return ctx.Err()
}

func (owner *SourcePolicyOwner) current() error {
	if owner == nil {
		return fmt.Errorf("source-policy owner is uninitialized")
	}
	if owner.server == nil {
		return fmt.Errorf("source-policy owner is uninitialized")
	}
	pm := owner.server.procManager.Load()
	if pm == nil {
		return fmt.Errorf("source-policy owner %s: captured process lifetime ended", owner.name)
	}
	if owner.proc == nil {
		return fmt.Errorf("source-policy owner %s: captured process is missing", owner.name)
	}
	if pm.GetProcess(owner.name) != owner.proc {
		return fmt.Errorf("source-policy owner %s: captured process lifetime ended", owner.name)
	}
	if owner.conn == nil {
		return fmt.Errorf("source-policy owner %s: captured connection is missing", owner.name)
	}
	if owner.proc.Conn() != owner.conn {
		return fmt.Errorf("source-policy owner %s: captured connection lifetime ended", owner.name)
	}
	if owner.proc.Done() == nil {
		return fmt.Errorf("source-policy owner %s: process has not started", owner.name)
	}
	select {
	case <-owner.proc.Done():
		return fmt.Errorf("source-policy owner %s: process stopped", owner.name)
	default:
	}
	if err := owner.conn.Err(); err != nil {
		return fmt.Errorf("source-policy owner %s: %w", owner.name, err)
	}
	return nil
}
