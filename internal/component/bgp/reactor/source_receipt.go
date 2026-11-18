// Design: docs/architecture/api/process-protocol.md -- retained selected-source authority.
// Related: reactor_api_relay.go -- the immutable source facts reused here.
// Related: forward_pool.go -- live-source and original-policy admission fences.
package reactor

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin"
)

// sourceReceiptFacts is an immutable engine snapshot, never caller authority.
// Its nil and zero values are invalid. Only newSourceReceiptFacts constructs it;
// the Peer pointer supplies revocation fences, never replacement session facts.
type sourceReceiptFacts struct {
	source       relaySource
	peerOwner    uint64
	connectionID uint64 // immutable original physical Session from the issuing factory
}

func newSourceReceiptFacts(source relaySource, connectionID uint64) (*sourceReceiptFacts, error) {
	if connectionID == 0 {
		return nil, errors.New("source receipt: original physical connection is missing")
	}

	if !source.ok {
		return nil, errors.New("source receipt: forwarding facts are unresolved")
	}
	if !source.info.resolved {
		return nil, errors.New("source receipt: source policy facts are unresolved")
	}
	if source.info.peer == nil {
		return nil, errors.New("source receipt: original peer is missing")
	}
	if source.info.peer.sourceOwner == 0 {
		return nil, errors.New("source receipt: original peer owner is unresolved")
	}
	if !source.addr.IsValid() {
		return nil, errors.New("source receipt: original peer address is missing")
	}
	if source.generation == 0 {
		return nil, errors.New("source receipt: original session generation is missing")
	}
	if source.ctxID == 0 {
		return nil, errors.New("source receipt: original receive encoding is missing")
	}
	// A forwardSourceInfo also carries one operation's sender, recovery and
	// replay state. None is a source-session fact or transferable authority.
	facts := &sourceReceiptFacts{connectionID: connectionID, peerOwner: source.info.peer.sourceOwner, source: relaySource{addr: source.addr, ctxID: source.ctxID,
		srcID: source.srcID, strAdr: source.strAdr, generation: source.generation, ok: true}}
	source.info.filterInfo.ConnectionID = connectionID
	facts.source.info = forwardSourceInfo{isIBGP: source.info.isIBGP, isRRClient: source.info.isRRClient,
		remoteRouterID: source.info.remoteRouterID, globalLocalAS: source.info.globalLocalAS,
		filterInfo: source.info.filterInfo, resolved: true, peer: source.info.peer}
	return facts, nil
}

// recoverySource returns the original immutable Session facts, never permission
// to send. queueRecovery still requires the exact round's Peer/generation cut
// and the normal source/export admission. No current Session facts are borrowed.
func (store *sourceReceiptStore) recoverySource(id, owner uint64) relaySource {
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.entries[id]
	if id == 0 || owner == 0 || record == nil || record.facts.peerOwner != owner {
		return relaySource{}
	}
	source := record.facts.source
	source.receipt = id
	return source
}

// sourceReceiptStore holds session facts, not routes or GR policy. Its zero
// value is usable. Entries are bounded by open session-delivery references,
// current owner leases, actual grant transactions and admitted final writes.
// Completed references leave no tombstones. Receipt identifiers never wrap or
// depend on peer addresses.
// Safe for concurrent use. mu is a leaf and is never held across I/O or waits.
type sourceReceiptStore struct {
	mu      sync.Mutex
	next    uint64
	entries map[uint64]*sourceReceiptRecord
	grants  map[sourceGrantKey]*sourceReceiptGrant
}

type sourceReceiptRecord struct {
	id           uint64
	facts        sourceReceiptFacts
	deliveryOpen bool
	liveLifetime context.Context
	endLive      context.CancelFunc
	leases       map[plugin.SelectedOwner]*sourceReceiptLease
	writes       uint64
	pins         uint64
}

// liveSourceReceipt holds the source session's admitted event-delivery lifetime,
// which can outlive its physical connection. The session MUST call close only
// after every admitted receive callback and its ordered DOWN have settled.
// close MUST release that reference on success, publication failure and stop.
// It never grants live forwarding: the original generation is checked again.
type liveSourceReceipt struct {
	store  *sourceReceiptStore
	record *sourceReceiptRecord
}

func (store *sourceReceiptStore) issue(facts *sourceReceiptFacts) (*liveSourceReceipt, error) {
	if facts == nil {
		return nil, errors.New("source receipt: original source facts are missing")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.next == math.MaxUint64 {
		return nil, errors.New("source receipt: identity exhausted; restart Ze with ze request restart before admitting a new source session")
	}
	store.next++
	liveLifetime, endLive := context.WithCancel(context.Background())
	record := &sourceReceiptRecord{id: store.next, facts: *facts, deliveryOpen: true, liveLifetime: liveLifetime, endLive: endLive,
		leases: make(map[plugin.SelectedOwner]*sourceReceiptLease)}
	if store.entries == nil {
		store.entries = make(map[uint64]*sourceReceiptRecord)
	}
	store.entries[record.id] = record
	return &liveSourceReceipt{store: store, record: record}, nil
}

func (live *liveSourceReceipt) ID() uint64 { return live.record.id }

// close releases the delivery reference after its applied DOWN boundary.
// It MUST follow issue, even when no plugin acquired a lease. Repeated cleanup
// is safe; no caller may publish another event with this receipt afterward.
func (live *liveSourceReceipt) close() {
	live.store.mu.Lock()
	defer live.store.mu.Unlock()
	live.record.endLive()
	live.record.deliveryOpen = false
	live.store.dropUnused(live.record)
}

// sourceReceiptLease belongs to one authenticated selecting process incarnation.
// Its zero and nil values grant nothing. The engine MUST check effective role
// and source grants before acquire; possession of a receipt alone grants neither.
// The owner MUST revoke the lease when its last candidate is purged or its
// retention expires. revoke MUST join every begin/complete pair before success.
type sourceReceiptLease struct {
	store     *sourceReceiptStore
	record    *sourceReceiptRecord
	owner     plugin.SelectedOwner
	retained  bool
	transient bool // Only actual live writes own this lease until explicit acquisition.
	revoked   chan struct{}
	// lifetime is canceled with revoked so a final writer blocked in a socket
	// flush can use the existing request cancellation and joined cleanup path.
	lifetime context.Context
	cancel   context.CancelFunc
	// writes contains only currently admitted guards. Their monotonically
	// increasing identities make copied completion tokens idempotent without
	// allocating a refcount object for every route. Its population is bounded
	// by the callers' bounded in-flight selected batches.
	writes map[uint64]struct{}
	next   uint64
	idle   chan struct{}
}

func (store *sourceReceiptStore) acquire(id uint64, owner plugin.SelectedOwner) (*sourceReceiptLease, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	// Admission and owner cleanup share this lock: an earlier liveness read
	// cannot authorize a lease inserted after cleanup has already completed.
	if err := owner.Current(); err != nil {
		return nil, err
	}
	record := store.entries[id]
	if record == nil {
		return nil, errors.New("source receipt: source session is unknown or released")
	}
	if record.facts.source.info.peer.stopping.Load() {
		return nil, errors.New("source receipt: original peer has been removed or stopped")
	}
	if lease := record.leases[owner]; lease != nil {
		if err := lease.current(); err != nil {
			return nil, err
		}
		if lease.transient {
			if !record.deliveryOpen {
				return nil, errors.New("source receipt: live write cannot acquire retention after original delivery ended")
			}
			lease.transient = false
		}
		return lease, nil
	}
	if !record.deliveryOpen {
		return nil, errors.New("source receipt: original event-delivery lifetime has ended")
	}
	lease := newSourceReceiptLease(store, record, owner)
	record.leases[owner] = lease
	return lease, nil
}

// retain records the owning plugin's retention decision on an existing lease.
// The engine computes no GR duration. Public acquisition cannot replace a lease
// after event delivery ends; only a real grant transaction's authenticated
// rollback may restore its pinned existing authority. Safe for concurrent use.
func (lease *sourceReceiptLease) retain() error {
	lease.store.mu.Lock()
	defer lease.store.mu.Unlock()
	if err := lease.current(); err != nil {
		return err
	}
	if lease.record.facts.source.info.peer.stopping.Load() {
		return errors.New("source receipt: original peer has been removed or stopped")
	}
	if lease.transient {
		return errors.New("source receipt: acquire the original source before retaining it")
	}
	lease.retained = true
	return nil
}

// current requires store.mu and checks only the process lease. A route's source
// generation and original policy epoch are separate checks in acceptsWrite.
func (lease *sourceReceiptLease) current() error {
	select {
	case <-lease.revoked:
		return errors.New("source receipt: selecting owner's lease was revoked")
	default:
		return lease.owner.Current()
	}
}

// sourceReceiptWrite is a validated final-write token. Its zero value is invalid.
// begin returns it only after admission; the writer MUST call current under its
// final socket lock, and MUST call complete after the write/flush or cancellation
// has ended. It MUST NOT reacquire a newer lease for an older queued operation.
// Copies name the same token, so complete is idempotent without pointer reuse.
type sourceReceiptWrite struct {
	lease *sourceReceiptLease
	id    uint64
	epoch uint64
	live  bool // Captured per write; a later shared lease retention cannot upgrade it.
}

func (lease *sourceReceiptLease) begin(epoch uint64) (sourceReceiptWrite, error) {
	lease.store.mu.Lock()
	defer lease.store.mu.Unlock()
	return lease.beginLocked(epoch)
}

// beginLocked shares write enrollment between explicit leases and live calls.
// Caller MUST hold store.mu; it never acquires a peer lock or performs I/O.
func (lease *sourceReceiptLease) beginLocked(epoch uint64) (sourceReceiptWrite, error) {
	if err := lease.acceptsWrite(epoch); err != nil {
		return sourceReceiptWrite{}, err
	}
	if lease.next == math.MaxUint64 {
		return sourceReceiptWrite{}, errors.New("source receipt: write identity exhausted; release the selecting owner's lease before retrying")
	}
	if lease.writes == nil {
		lease.writes = make(map[uint64]struct{})
	}
	lease.next++
	lease.writes[lease.next] = struct{}{}
	lease.record.writes++
	return sourceReceiptWrite{lease: lease, id: lease.next, epoch: epoch}, nil
}

func (lease *sourceReceiptLease) acceptsWrite(epoch uint64) error {
	if err := lease.current(); err != nil {
		return err
	}
	source := &lease.record.facts.source
	if source.info.peer.stopping.Load() {
		return errors.New("source receipt: original peer has been removed or stopped")
	}
	if !sourcePolicyCurrent(source.info.peer, epoch) {
		return errors.New("source receipt: original route policy admission is no longer current")
	}
	if forwardSourceCurrent(source.info.peer, source.generation) {
		return nil
	}
	if lease.retained {
		return nil
	}
	return errors.New("source receipt: original source session is down without retained authority")
}

func (write sourceReceiptWrite) current() error {
	if write.lease == nil {
		return errors.New("source receipt: final writer has no admitted lease")
	}
	write.lease.store.mu.Lock()
	defer write.lease.store.mu.Unlock()
	if _, pending := write.lease.writes[write.id]; !pending {
		return errors.New("source receipt: final write has already completed")
	}
	if write.live {
		source := &write.lease.record.facts.source
		if !forwardSourceCurrent(source.info.peer, source.generation) {
			return errForwardNoSource
		}
	}
	return write.lease.acceptsWrite(write.epoch)
}

// source returns immutable original facts, not permission to write them.
// The writer MUST still call current at its final socket admission boundary.
func (write sourceReceiptWrite) source() relaySource {
	source := write.lease.record.facts.source
	source.receipt = write.lease.record.id
	return source
}

func (write sourceReceiptWrite) done() <-chan struct{} { return write.lease.revoked }

// complete MUST follow begin after final write/flush or cancellation. It is safe
// for error unwinding to complete a copied token again; another write is untouched.
func (write sourceReceiptWrite) complete() {
	if write.lease == nil {
		return
	}
	lease := write.lease
	lease.store.mu.Lock()
	defer lease.store.mu.Unlock()
	if _, pending := lease.writes[write.id]; !pending {
		return
	}
	delete(lease.writes, write.id)
	lease.record.writes--
	if len(lease.writes) == 0 {
		if lease.transient {
			if lease.record.leases[lease.owner] == lease {
				delete(lease.record.leases, lease.owner)
			}
		}
		if lease.idle != nil {
			close(lease.idle)
			lease.idle = nil
		}
		select {
		case <-lease.revoked:
			if lease.record.leases[lease.owner] == lease {
				delete(lease.record.leases, lease.owner)
			}
		default:
		}
	}
	lease.store.dropUnused(lease.record)
}

// revoke first prevents admission and cancels every captured writer, then joins
// their final completion. The owner MUST call it on purge/expiry/removal/stop;
// a canceled wait leaves authority revoked and memory owned by pending writes.
func (lease *sourceReceiptLease) revoke(ctx context.Context) error {
	lease.store.mu.Lock()
	lease.invalidate()
	idle := lease.idle
	lease.store.mu.Unlock()
	return waitSourceReceiptWrites(ctx, idle)
}

// invalidate requires store.mu. A revoked lease with outstanding writes remains
// discoverable by process cleanup until its last complete; it grants no work.
func (lease *sourceReceiptLease) invalidate() {
	select {
	case <-lease.revoked:
		return
	default:
		close(lease.revoked)
		lease.cancel()
	}
	// Allocate a waiter only on revocation of actual outstanding writes,
	// never for each route's ordinary begin/complete pair.
	if len(lease.writes) != 0 {
		lease.idle = make(chan struct{})
		return
	}
	if lease.record.leases[lease.owner] == lease {
		delete(lease.record.leases, lease.owner)
	}
	lease.store.dropUnused(lease.record)
}

// revokeOwner is called by the engine's existing process-stop cleanup, never a
// watcher. It revokes all captured leases before waiting on any one of them.
// The caller MUST invoke it for the old incarnation before replacement is ready.
func (store *sourceReceiptStore) revokeOwner(ctx context.Context, owner plugin.SelectedOwner) error {
	store.mu.Lock()
	var pending []<-chan struct{}
	// The scan is bounded by current source-session and retained-lease records.
	for _, record := range store.entries {
		if lease := record.leases[owner]; lease != nil {
			lease.invalidate()
			pending = append(pending, lease.idle)
		}
	}
	store.mu.Unlock()
	for _, idle := range pending {
		if err := waitSourceReceiptWrites(ctx, idle); err != nil {
			return err
		}
	}
	return nil
}

func waitSourceReceiptWrites(ctx context.Context, idle <-chan struct{}) error {
	// A nil waiter explicitly means no final write was outstanding at revoke.
	if idle == nil {
		return nil
	}
	// Completed work needs no deadline budget, even if its caller was canceled.
	select {
	case <-idle:
		return nil
	default:
	}
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// dropUnused requires store.mu. It deletes only completed ownership, never
// historical keys: there is no candidate or route storage in this table.
func (store *sourceReceiptStore) dropUnused(record *sourceReceiptRecord) {
	if record.pins != 0 {
		return
	}
	if record.deliveryOpen {
		return
	}
	if len(record.leases) != 0 {
		return
	}
	if record.writes != 0 {
		return
	}
	delete(store.entries, record.id)
}
