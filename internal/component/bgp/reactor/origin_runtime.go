// Design: docs/architecture/api/process-protocol.md -- current original data and runtime authority.
// Related: origin_receipt.go -- bounded issuer; origin_intake.go -- current provider callback.
package reactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/report"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errOriginOriginalGone = errors.New("origin retained original was replaced or withdrawn")

// originIntakeCall is one actual joined operation, never a historical scope ban.
// Composite offers MUST retain it through postvalidation and compensation.
// The store leaf mutex protects membership; cancel and done also fence teardown.
type originIntakeCall struct {
	scope    originScope
	receipt  uint64
	original *originReceiptRecord
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// bindLifetime keeps the issuer pointer stable across a stopped Reactor restart.
// IDs never reset, and sealed old intakes cannot commit in a renewed lifetime.
func (store *originReceiptStore) bindLifetime(parent context.Context) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if parent == nil {
		return errors.New("origin reactor lifetime is missing")
	}
	if parent.Done() == nil {
		return errors.New("origin reactor lifetime cannot be canceled")
	}
	if err := parent.Err(); err != nil {
		return err
	}
	if store.closed {
		if len(store.receipts) != 0 {
			return errors.New("origin reactor restart has unjoined final writers")
		}
		if len(store.calls) != 0 {
			return errors.New("origin reactor restart has unjoined predecessor operations")
		}
		store.lifetime, store.cancel = context.WithCancel(parent)
		store.closed = false
	}
	if store.cancel == nil {
		return errors.New("origin reactor issuer has no owned cancellation")
	}
	if store.parentStop != nil {
		store.parentStop()
	}
	store.parentStop = context.AfterFunc(parent, store.cancel)
	return nil
}

// issueProducerCurrent requires store.mu. Only an exact, still-current original
// supplied by engine lifecycle code may retain an ended producer's existing data.
func (store *originReceiptStore) issueProducerCurrent(intake *originIssue) error {
	original := intake.original
	if original == nil {
		return intake.key.producer.current()
	}
	if original.store != store {
		return errors.New("origin retained original belongs to another issuer")
	}
	if store.current[original.key] != original {
		return errOriginOriginalGone
	}
	if original.key != intake.key {
		return errors.New("origin retained original belongs to another source")
	}
	if !original.revoked {
		return errors.New("origin retained original has not joined its preceding authority")
	}
	if len(original.writes) != 0 {
		return errors.New("origin retained original still has preceding final writers")
	}
	for call := range store.calls {
		if call.scope == original.originScope {
			return errors.New("origin retained original still has a preceding intake callback")
		}
	}
	if original.retained {
		return nil
	}
	return original.key.producer.current()
}

// producerCurrent requires store.mu. A kept receipt remains bound to its exact
// source data, original Peer and current provider; it confers no new ingress.
func (record *originReceiptRecord) producerCurrent() error {
	if record.store.current[record.key] != record {
		return errors.New("origin contribution is no longer current")
	}
	if record.retained {
		return nil
	}
	return record.key.producer.current()
}

func sameOriginSource(a, b originContributionKey) bool {
	return a.producer.sender() == b.producer.sender() && a.producer.kind == b.producer.kind &&
		a.source == b.source && a.path == b.path && a.pathID == b.pathID && a.hasPathID == b.hasPathID && a.peerOwner == b.peerOwner
}

// park requires store.mu and preserves only current original data. Old receipt
// authority, callbacks and final writes are canceled; no new arbitrary data enters.
func (store *originReceiptStore) park(record *originReceiptRecord) {
	current := store.current[record.key] == record
	store.invalidate(record)
	if current {
		store.current[record.key] = record
	}
}

func scopeOriginOwner(scope originScope, owner plugin.SelectedOwner) bool {
	if scope.provider.SameIncarnation(owner) {
		return true
	}
	return scope.key.producer.kind == originProducerProcess && scope.key.producer.owner.SameIncarnation(owner)
}

// endOwner seals and joins an exited owner's intakes, callbacks, and writes
// using the default withdraw-on-exit policy (E-2).
func (store *originReceiptStore) endOwner(ctx context.Context, owner plugin.SelectedOwner) ([]*originReceiptRecord, error) {
	return store.endOwnerWithPolicy(ctx, owner, false)
}

// endOwnerWithPolicy first seals every actual old intake/callback/write, then
// joins them. When keep is false (default), routes produced by owner are
// invalidated so the deciding RIB retires them; when keep is true (external
// on-exit keep), local BGP originals stay parked as retained while
// redistributed inputs are still invalidated. Exited incarnations can never
// issue a new contribution.
func (store *originReceiptStore) endOwnerWithPolicy(ctx context.Context, owner plugin.SelectedOwner, keep bool) ([]*originReceiptRecord, error) {
	if owner.Done() == nil {
		return nil, errors.New("origin cleanup has no original process incarnation")
	}
	store.mu.Lock()
	if store.lifetime == nil {
		store.mu.Unlock()
		return nil, errors.New("origin issuer is uninitialized")
	}
	for intake := range store.intakes {
		if scopeOriginOwner(intake.originScope, owner) {
			intake.revoked = true
		}
	}
	var records []*originReceiptRecord
	for _, record := range store.current {
		if !scopeOriginOwner(record.originScope, owner) {
			continue
		}
		producing := record.key.producer.kind == originProducerProcess
		if producing {
			producing = record.key.producer.owner.SameIncarnation(owner)
		}
		if producing {
			if record.source.identity.kind == originSourceRedistributed {
				store.invalidate(record)
			} else if keep {
				record.retained = true
				store.park(record)
			} else {
				store.invalidate(record)
			}
		} else {
			store.park(record)
		}
		records = append(records, record)
	}
	var pending []<-chan struct{}
	for call := range store.calls {
		if scopeOriginOwner(call.scope, owner) {
			call.cancel()
			pending = append(pending, call.done)
		}
	}
	for _, record := range store.receipts {
		if !scopeOriginOwner(record.originScope, owner) {
			continue
		}
		store.park(record)
		pending = append(pending, record.idle)
	}
	store.mu.Unlock()
	for _, idle := range pending {
		if err := waitSourceReceiptWrites(ctx, idle); err != nil {
			return records, err
		}
	}
	return records, nil
}

func (store *originReceiptStore) reissue(ctx context.Context, original *originReceiptRecord, owner plugin.SelectedOwner) (uint64, error) {
	if original == nil {
		return 0, errors.New("origin replay original is missing")
	}
	return store.issueOriginal(ctx, original.key.producer, original.source, original.build, original.peer, owner, original)
}

// originWithdrawal accepts a same-name successor only through its live captured
// token and current actual send grant, and only finds an engine-marked kept value.
// No matching original returns errOriginOriginalGone. The withdrawal adapter
// MUST consume that outcome as the existing successful no-op, not a failure.
func (store *originReceiptStore) originWithdrawal(ctx context.Context, producer originProducer, source *originSource, identity *originIdentity, peer *Peer, owner plugin.SelectedOwner) (*originReceiptRecord, error) {
	retirement, retiring := plugin.OriginRetirementFromContext(ctx)
	if err := producer.current(); err != nil {
		if !retiring {
			return nil, err
		}
		if !retirement.OwnsProducer(producer.owner) {
			return nil, err
		}
	}
	if source == nil {
		return nil, errors.New("origin withdrawal source is missing")
	}
	if identity == nil {
		return nil, errors.New("origin withdrawal native identity is missing")
	}
	if peer == nil {
		return nil, errors.New("origin withdrawal Peer is missing")
	}
	peer.mu.RLock()
	allowed := producer.sender().IsOperator() || peer.maySend(announceOrigin(producer.sender()))
	peer.mu.RUnlock()
	if !allowed {
		return nil, errors.New("origin withdrawal has no current original Peer grant")
	}
	key := originKey(producer, source, identity, peer.sourceOwner)
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.live(); err != nil {
		return nil, err
	}
	if !retiring {
		if err := producer.current(); err != nil {
			return nil, err
		}
	}
	record := store.current[key]
	if record == nil {
		if !retiring {
			for _, candidate := range store.current {
				if !candidate.retained {
					continue
				}
				if sameOriginSource(candidate.key, key) {
					record = candidate
					break
				}
			}
		}
	}
	if record == nil {
		return nil, errOriginOriginalGone
	}
	if retiring {
		if !record.matchesRetirement(retirement) {
			return nil, errOriginOriginalGone
		}
	}
	if record.peer != peer {
		return nil, errors.New("origin withdrawal concrete Peer changed")
	}
	if !record.provider.SameIncarnation(owner) {
		return nil, errors.New("origin withdrawal deciding process changed")
	}
	return record, nil
}

// beginOriginCall requires no socket, Peer or reactor lock. Engine cleanup may
// send an exact original withdrawal after revocation; ordinary offers may not.
func (store *originReceiptStore) beginOriginCall(ctx context.Context, record *originReceiptRecord, owner plugin.SelectedOwner, retirement bool) (*originIntakeCall, func(), error) {
	store.mu.Lock()
	if err := store.live(); err != nil {
		store.mu.Unlock()
		return nil, nil, err
	}
	if record == nil {
		store.mu.Unlock()
		return nil, nil, errors.New("origin callback has no issued original scope")
	}
	if record.store != store {
		store.mu.Unlock()
		return nil, nil, errors.New("origin callback belongs to another issuer")
	}
	if !record.provider.SameIncarnation(owner) {
		store.mu.Unlock()
		return nil, nil, errors.New("origin callback belongs to another provider")
	}
	if !retirement {
		if record.revoked {
			store.mu.Unlock()
			return nil, nil, errors.New("origin callback receipt was revoked")
		}
		if store.current[record.key] != record {
			store.mu.Unlock()
			return nil, nil, errors.New("origin callback receipt is no longer current")
		}
	}
	if err := owner.Current(); err != nil {
		store.mu.Unlock()
		return nil, nil, err
	}
	work, cancel := context.WithCancel(ctx)
	call := &originIntakeCall{scope: record.originScope, receipt: record.id, original: record, ctx: work, cancel: cancel, done: make(chan struct{})}
	store.calls[call] = struct{}{}
	stop := context.AfterFunc(store.lifetime, cancel)
	if !retirement { // Normal replacement also revokes callbacks of this receipt.
		// record.ctx is immutable and owned by this exact receipt generation.
		previous := stop
		sourceStop := context.AfterFunc(record.ctx, cancel)
		stop = func() bool { a := previous(); b := sourceStop(); return a || b }
	}
	store.mu.Unlock()
	release := func() {
		stop()
		cancel()
		store.mu.Lock()
		if _, active := store.calls[call]; active {
			delete(store.calls, call)
			close(call.done)
		}
		store.mu.Unlock()
	}
	return call, release, nil
}

// replayOrigin re-offers original engine-owned bytes, never plugin-echoed bytes.
// A failed replay stays parked for an explicit later retry; it is reported as a
// failure and never answered accepted or routed around the configured provider.
func (a *reactorAPIAdapter) replayOrigin(ctx context.Context, original *originReceiptRecord, owner plugin.SelectedOwner) error {
	receipt, err := a.r.originReceipts.reissue(ctx, original, owner)
	if errors.Is(err, errOriginOriginalGone) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = a.offerIssuedOrigin(ctx, receipt, original.peer, owner, true)
	return err
}

// EndOriginOwner is the typed generic process-generation cleanup extension.
// No replacement is acknowledged before old callbacks and final writes join.
// Uses the default withdraw-on-exit policy (E-2).
func (a *reactorAPIAdapter) EndOriginOwner(ctx context.Context, owner plugin.SelectedOwner) error {
	return a.EndOriginOwnerWithPolicy(ctx, owner, false)
}

// EndOriginOwnerWithPolicy cleans up originated routes owned by an exited
// process incarnation on both the direct (no-bgp-rib) path and the selected-RIB
// (originReceiptStore) path. When keep is false (default), routes originated by
// owner are retired or withdrawn unless another participant still holds them.
// When keep is true (external { on-exit keep; }), local BGP routes stay
// advertised across process exit.
func (a *reactorAPIAdapter) EndOriginOwnerWithPolicy(ctx context.Context, owner plugin.SelectedOwner, keep bool) error {
	var failures []error
	if err := a.endDirectOriginOwner(ctx, owner, keep); err != nil {
		failures = append(failures, err)
	}
	store := a.r.originReceipts
	if store == nil {
		return errors.Join(failures...)
	}
	records, err := store.endOwnerWithPolicy(ctx, owner, keep)
	if err != nil {
		// The server's cleanup callback has no error channel to defer its barrier.
		// Rights are already canceled; keep joining outside every component lock.
		_, joined := store.endOwnerWithPolicy(context.Background(), owner, keep)
		failures = append(failures, err, joined)
		return errors.Join(failures...)
	}
	for _, record := range records {
		producing := record.key.producer.kind == originProducerProcess && record.key.producer.owner.SameIncarnation(owner)
		retire := producing && (record.source.identity.kind == originSourceRedistributed || !keep)
		if retire {
			if !record.provider.SameIncarnation(owner) {
				if record.provider.Current() == nil {
					if err := a.retireOriginCandidate(ctx, record); err != nil {
						failures = append(failures, err)
					} else if record.source.identity.kind == originSourceLocalBGP {
						record.peer.restoreConfiguredOrigin(record.build)
					}
				}
			}
			continue
		}
		if record.provider.SameIncarnation(owner) {
			continue
		}
		if err := record.provider.Current(); err != nil {
			continue
		}
		if err := a.replayOrigin(ctx, record, record.provider); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// ReplayOriginOwner runs after the actual new generation's complete handshake.
// It only rebinds current engine-owned data for that configured destination owner.
func (a *reactorAPIAdapter) ReplayOriginOwner(ctx context.Context, owner plugin.SelectedOwner) error {
	if err := owner.Current(); err != nil {
		return err
	}
	store := a.r.originReceipts
	if store == nil {
		return nil
	}
	name, named := owner.Sender().Process()
	if !named {
		return errors.New("origin replay provider is not a named process")
	}
	store.mu.Lock()
	var records []*originReceiptRecord
	for _, record := range store.current {
		if !record.revoked {
			continue
		}
		if record.provider.Sender().String() != name {
			continue
		}
		records = append(records, record)
	}
	store.mu.Unlock()
	var failures []error
	for _, record := range records {
		if err := a.replayOrigin(ctx, record, owner); err != nil {
			failures = append(failures, fmt.Errorf("replay origin to %s: %w", record.peer.addrString, err))
		}
	}
	return errors.Join(failures...)
}

func (r *Reactor) completeOriginClose() {
	r.directOrigins.clear()
	if r.originReceipts == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	err := r.originReceipts.close(ctx)
	cancel()
	if err == nil {
		return
	}
	message := report.FitMessage("Origin shutdown has revoked authority but is still joining canceled work: " + err.Error())
	report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, "origin-shutdown", message, "ze request restart")
	peerLogger().Error(message, "code", codeSourceLeaseCleanup, "fix", "ze request restart")
	if err := r.originReceipts.close(context.Background()); err != nil {
		panic("BUG: origin shutdown join failed: " + err.Error())
	}
}

func (r *Reactor) completeOriginPeerRemoval(peer *Peer) {
	r.directOrigins.removePeer(peer)
	if r.originReceipts == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	err := r.originReceipts.revokePeer(ctx, peer)
	cancel()
	if err == nil {
		return
	}
	message := report.FitMessage("Origin removal for Peer " + peer.addrString + " has revoked authority but is still joining canceled work: " + err.Error())
	report.RaiseErrorFix(reportSourceBGP, codeSourceLeaseCleanup, peer.addrString, message, "ze request restart")
	peerLogger().Error(message, "peer", peer.addrString, "code", codeSourceLeaseCleanup, "fix", "ze request restart")
	if err := r.originReceipts.revokePeer(context.Background(), peer); err != nil {
		panic("BUG: origin Peer removal join failed: " + err.Error())
	}
}
