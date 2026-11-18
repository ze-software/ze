// Design: docs/architecture/api/process-protocol.md -- issued origin final-use leases.
// Related: origin_receipt.go -- current source/scoped records, not sent history.
package reactor

import (
	"context"
	"errors"
	"math"

	"github.com/ze-software/ze/internal/component/plugin"
)

// originWrite borrows one issued immutable contribution through a final write.
// A successful admit MUST be followed by complete on every return path. The
// writer MUST call current under its final socket lock and honor done while
// writing/flushing. Zero is invalid. Copied tokens share an idempotent completion
// identity, as sourceReceiptWrite does. Safe for concurrent checks/completion.
type originWrite struct {
	id           uint64
	record       *originReceiptRecord
	snapshot     *disseminationSnapshot
	sessionToken uint64
}

func (store *originReceiptStore) admit(id uint64, provider plugin.SelectedOwner, peer *Peer, snapshot *disseminationSnapshot, key fwdPathKey) (originWrite, error) {
	if store == nil {
		return originWrite{}, errors.New("origin issuer is missing")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.live(); err != nil {
		return originWrite{}, err
	}
	record := store.receipts[id]
	if record == nil {
		return originWrite{}, errors.New("origin receipt is unknown or released")
	}
	if record.build.key != key {
		return originWrite{}, errors.New("origin receipt belongs to another native route key")
	}
	if !record.provider.SameIncarnation(provider) {
		return originWrite{}, errors.New("origin receipt belongs to another deciding process incarnation")
	}
	if peer != record.peer {
		return originWrite{}, errors.New("origin receipt belongs to another original Peer")
	}
	if snapshot == nil {
		return originWrite{}, errors.New("origin output has no destination Session publication")
	}
	if snapshot.session == nil {
		return originWrite{}, errors.New("origin output has no captured destination Session")
	}
	write := originWrite{record: record, snapshot: snapshot, sessionToken: snapshot.session.initialReplay.Load()}
	if err := write.accepts(); err != nil {
		return originWrite{}, err
	}
	if record.nextWrite == math.MaxUint64 {
		return originWrite{}, errors.New("origin final-write identities exhausted; replace this contribution before issuing another write")
	}
	record.nextWrite++
	write.id = record.nextWrite
	if record.writes == nil {
		record.writes = make(map[uint64]struct{})
	}
	record.writes[write.id] = struct{}{}
	return write, nil
}

// accepts requires store.mu and takes no peer/manager lock or callback.
func (write originWrite) accepts() error {
	record := write.record
	if record.revoked {
		return errors.New("origin receipt has been revoked")
	}
	if err := record.store.live(); err != nil {
		return err
	}
	if err := record.producerCurrent(); err != nil {
		return err
	}
	if err := record.provider.Current(); err != nil {
		return err
	}
	if record.peer.sourceOwner != record.key.peerOwner {
		return errors.New("origin original Peer identity changed")
	}
	if record.peer.stopping.Load() {
		return errors.New("origin original Peer has been removed")
	}
	if write.snapshot.peer != record.peer {
		return errors.New("origin output names another original Peer")
	}
	if write.snapshot.session == nil {
		return errors.New("origin output has no captured destination Session")
	}
	if write.sessionToken == 0 {
		return errors.New("origin output has an unrecorded destination Session token")
	}
	if write.snapshot.session.initialReplay.Load() != write.sessionToken {
		return errors.New("origin destination Session token changed")
	}
	if record.peer.selectedOwnerSnapshot() != write.snapshot {
		return errors.New("origin destination Session or ownership publication is no longer current")
	}
	if _, err := write.snapshot.ExportKind(record.provider, false); err != nil {
		return err
	}
	return nil
}

func (write originWrite) current() error {
	if write.record == nil {
		return errors.New("origin final write is uninitialized")
	}
	store := write.record.store
	store.mu.Lock()
	if _, active := write.record.writes[write.id]; !active {
		store.mu.Unlock()
		return errors.New("origin final write has completed")
	}
	err := write.accepts()
	store.mu.Unlock()
	if err != nil {
		return err
	}
	if err := write.record.source.acceptedCurrent(write.record.ctx, write.record.peer); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, active := write.record.writes[write.id]; !active {
		return errors.New("origin final write completed during accepted-source lookup")
	}
	return write.accepts()
}

// build returns only original local build choices. It grants no received-source
// authority, output policy verdict or later lifetime; current remains mandatory.
func (write originWrite) build() *originBuildValue { return write.record.build }
func (write originWrite) done() <-chan struct{}    { return write.record.ctx.Done() }

func (write originWrite) complete() {
	if write.record == nil {
		return
	}
	record := write.record
	store := record.store
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, active := record.writes[write.id]; !active {
		return
	}
	delete(record.writes, write.id)
	if len(record.writes) != 0 {
		return
	}
	if record.idle != nil {
		close(record.idle)
		record.idle = nil
	}
	if record.revoked {
		delete(store.receipts, record.id)
	}
}

// revoke seals every matching enrolled issue and invalidates issued records
// under the same leaf mutex as issue commit, before waiting on any final write.
// Canceled waits keep sealed actual intake calls and genuine write obligations;
// completion/endIssue remove them, never a historical producer/scope deny map.
func (store *originReceiptStore) revoke(ctx context.Context, match func(originScope) bool) error {
	if store == nil {
		return errors.New("origin issuer is missing")
	}
	store.mu.Lock()
	if store.lifetime == nil {
		store.mu.Unlock()
		return errors.New("origin issuer is uninitialized")
	}
	for intake := range store.intakes {
		if match(intake.originScope) {
			intake.revoked = true
		}
	}
	var pending []<-chan struct{}
	for call := range store.calls {
		if match(call.scope) {
			call.cancel()
			pending = append(pending, call.done)
		}
	}
	for _, record := range store.current {
		if match(record.originScope) {
			store.invalidate(record)
		}
	}
	for _, record := range store.receipts {
		if !match(record.originScope) {
			continue
		}
		store.invalidate(record)
		pending = append(pending, record.idle)
	}
	store.mu.Unlock()
	for _, idle := range pending {
		if err := waitSourceReceiptWrites(ctx, idle); err != nil {
			return err
		}
	}
	return nil
}

// withdraw removes the exact source, including old admitted writes replaced
// under that same source key. A never-issued identity creates no record or ID.
func (store *originReceiptStore) withdraw(ctx context.Context, producer originProducer, source *originSource, identity *originIdentity, peer *Peer) error {
	if err := producer.current(); err != nil {
		return err
	}
	if source == nil {
		return errors.New("origin withdrawal source is missing")
	}
	if identity == nil {
		return errors.New("origin withdrawal native identity is missing")
	}
	if peer == nil {
		return errors.New("origin withdrawal Peer is missing")
	}
	key := originKey(producer, source, identity, peer.sourceOwner)
	return store.revoke(ctx, func(scope originScope) bool { return scope.key == key })
}

// revokeOwner MUST use the original captured process token, even after it ends.
// It revokes both production and deciding lifetimes; a same-name successor is
// distinct. Explicit keep-on-exit retention needs its own admitted transition
// in the ingress/lifecycle adapter and MUST NOT be inferred here.
func (store *originReceiptStore) revokeOwner(ctx context.Context, owner plugin.SelectedOwner) error {
	if owner.Done() == nil {
		return errors.New("origin cleanup process incarnation is unrecorded")
	}
	return store.revoke(ctx, func(scope originScope) bool {
		if scope.provider.SameIncarnation(owner) {
			return true
		}
		if scope.key.producer.kind == originProducerProcess {
			return scope.key.producer.owner.SameIncarnation(owner)
		}
		return false
	})
}

func (store *originReceiptStore) revokePeer(ctx context.Context, peer *Peer) error {
	return store.revoke(ctx, func(scope originScope) bool { return scope.peer == peer })
}

// revokeProducerPeer is the explicit sender-grant removal barrier. Lifecycle
// callers MUST publish removal before this call and await its successful join
// before acknowledging it. issue MUST remain enrolled until commit or refusal;
// no preflighted predecessor may escape this barrier after a later regrant.
func (store *originReceiptStore) revokeProducerPeer(ctx context.Context, producer originProducer, peer *Peer) error {
	return store.revoke(ctx, func(scope originScope) bool { return scope.peer == peer && scope.key.producer == producer })
}

func (store *originReceiptStore) close(ctx context.Context) error {
	if store == nil {
		return errors.New("origin issuer is missing")
	}
	store.mu.Lock()
	if store.lifetime == nil {
		store.mu.Unlock()
		return errors.New("origin issuer is uninitialized")
	}
	store.closed = true
	if store.parentStop != nil {
		store.parentStop()
		store.parentStop = nil
	}
	if store.cancel != nil {
		store.cancel()
	}
	store.mu.Unlock()
	return store.revoke(ctx, func(originScope) bool { return true })
}
