// Design: docs/architecture/api/process-protocol.md -- exact original scope removal.
// Related: dissemination_revoke.go -- published configuration precedes this barrier.
package reactor

import (
	"context"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (p *Peer) revokeRemovedOriginGrants(r *Reactor, before, after *PeerSettings, beforeGraph, afterGraph *pluginserver.DeliveryGraph) error {
	if r == nil {
		return nil
	}
	if r.originReceipts == nil {
		return nil
	}
	// Resolve destination ownership from this exact published settings snapshot.
	// A stopped producer needs no manager lookup to lose its original send grant.
	nextName := ""
	explicit := false
	if r.api != nil {
		next := deliveryPeerFromSettings(after)
		owner, err := r.api.ResolveDisseminationOwnerForPeer(next, SelectedDisseminationRole)
		if err == nil {
			nextName, explicit = owner.Name(), owner.Explicit()
		}
	}
	match := func(scope originScope) bool {
		if scope.peer != p {
			return false
		}
		if explicit {
			return true
		}
		if nextName != scope.provider.Sender().String() {
			return true
		}
		sender := scope.key.producer.sender()
		if sender.IsOperator() {
			return false
		}
		procName, ok := sender.Process()
		if !ok {
			return false
		}
		return beforeGraph.MaySend(p.addrString, procName, bgpevents.SendUpdate) && !afterGraph.MaySend(p.addrString, procName, bgpevents.SendUpdate)
	}
	store := r.originReceipts
	store.mu.Lock()
	var removed []*originReceiptRecord
	for _, record := range store.current {
		if match(record.originScope) {
			removed = append(removed, record)
		}
	}
	store.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	var failures []error
	if err := store.revoke(ctx, match); err != nil {
		failures = append(failures, err)
	}
	adapter := &reactorAPIAdapter{r: r}
	retired := make(map[plugin.SelectedOwner]bool)
	for _, record := range removed {
		destinationLost := explicit || nextName != record.provider.Sender().String()
		if destinationLost {
			if !retired[record.provider] {
				retired[record.provider] = true
				if session := p.currentSession(); session != nil {
					if err := session.retireSelectedOwner(ctx, p, record.provider, nil); err != nil {
						session.seal()
						session.closeConnWithin(0)
						failures = append(failures, err)
					}
				}
				name, named := record.provider.Sender().Process()
				if named {
					p.unbindProcesses(r, []string{name})
				}
			}
		}
		if err := adapter.retireOriginCandidate(ctx, record); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// retireOriginCandidate is engine-only cleanup of an actual removed original,
// not producer ingress and not a new receipt. Provider mutation is joined.
func (a *reactorAPIAdapter) retireOriginCandidate(ctx context.Context, record *originReceiptRecord) error {
	input := originIntakeValue(record.key.producer, record.source, record.build, record.peer, record.id, rpc.OriginIntakeWithdraw)
	call, release, err := a.r.originReceipts.beginOriginCall(ctx, record, record.provider, true)
	if err != nil {
		return err
	}
	defer release()
	return withdrawOriginIntake(call.ctx, record.provider, input)
}

// originRetirementOwner recognizes only a trusted, engine registry context.
// It is never reconstructed from a sender name or an external input field.
func originRetirementOwner(ctx context.Context, owner plugin.SelectedOwner) bool {
	retirement, ok := plugin.OriginRetirementFromContext(ctx)
	return ok && retirement.OwnsProducer(owner)
}

// parkReboundOrigins MUST precede terminal DOWN. It retains exact current data,
// joins old intake/write obligations and retires this provider's actual frontier.
func (p *Peer) parkReboundOrigins(r *Reactor, names []string) error {
	if r == nil {
		return nil
	}
	store := r.originReceipts
	if store == nil {
		return nil
	}
	store.mu.Lock()
	var owners []plugin.SelectedOwner
	for _, record := range store.current {
		if record.peer != p {
			continue
		}
		name, named := record.provider.Sender().Process()
		if !named {
			continue
		}
		if !slices.Contains(names, name) {
			continue
		}
		if !slices.Contains(owners, record.provider) {
			owners = append(owners, record.provider)
		}
		store.park(record)
	}
	var pending []<-chan struct{}
	for call := range store.calls {
		if call.scope.peer != p {
			continue
		}
		if !slices.Contains(owners, call.scope.provider) {
			continue
		}
		call.cancel()
		pending = append(pending, call.done)
	}
	for _, record := range store.receipts {
		if record.peer != p {
			continue
		}
		if !slices.Contains(owners, record.provider) {
			continue
		}
		pending = append(pending, record.idle)
	}
	store.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	var failures []error
	for _, done := range pending {
		if err := waitSourceReceiptWrites(ctx, done); err != nil {
			failures = append(failures, err)
		}
	}
	for _, owner := range owners {
		if session := p.currentSession(); session != nil {
			if err := session.retireSelectedOwner(ctx, p, owner, nil); err != nil {
				session.seal()
				session.closeConnWithin(0)
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}

// replayPeerOrigins follows the binding reconciliation and fresh publication.
// No sender name supplies original input; every offer uses its immutable record.
func (p *Peer) replayPeerOrigins(r *Reactor) error {
	if r == nil {
		return nil
	}
	store := r.originReceipts
	if store == nil {
		return nil
	}
	store.mu.Lock()
	var records []*originReceiptRecord
	for _, record := range store.current {
		if record.peer != p {
			continue
		}
		if !record.revoked {
			continue
		}
		records = append(records, record)
	}
	store.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	var failures []error
	for _, record := range records {
		adapter := &reactorAPIAdapter{r: r}
		if err := adapter.replayOrigin(ctx, record, record.provider); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
