// Design: docs/architecture/plugin/rib-storage-design.md -- received source leases.
// Related: storage/source_reference.go -- last release only wakes this owner.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type ribSourceLease struct {
	peer         netip.Addr
	peerOwner    uint64
	connectionID uint64
	ref          *storage.SourceReference
	busy         chan struct{}
	failure      error
	outputHolds  uint32                 // Active source operations, not route history.
	outputs      []*selectedDestination // Coalesced live worker obligations after a refused operation.
	sentOutputs  []*ribSourceSentOutput // Exact live auxiliary batches, never reconstructed from current metadata.
}

// ribSourceLeases owns current original-source leases, not routes or history.
// mu serializes registry mutations and each receipt's busy fence, never engine
// I/O. Other sources proceed while one receipt's final write is being joined.
// Storage Release never acquires it; that callback only sends a coalesced wake.
// No caller may hold a peer, storage, election or pool lock across a method.
// Nil denotes passive storage with no dissemination authority.
type ribSourceLeases struct {
	manager     *RIBManager
	mu          sync.Mutex
	leases      map[uint64]*ribSourceLease
	admission   map[uint64]error
	reportMu    sync.Mutex
	reported    map[uint64]string
	reportAfter map[uint64]time.Time
	wake        chan struct{}
	cancel      context.CancelFunc
	done        chan struct{}
	stopped     bool // under mu; closes intake before terminal local cleanup
}

func newRIBSourceLeases(manager *RIBManager) *ribSourceLeases {
	return &ribSourceLeases{manager: manager, leases: make(map[uint64]*ribSourceLease),
		admission: make(map[uint64]error), reported: make(map[uint64]string),
		reportAfter: make(map[uint64]time.Time), wake: make(chan struct{}, 1)}
}

func (s *ribSourceLeases) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *ribSourceLeases) acquire(ctx context.Context, peer netip.Addr, peerOwner, receipt, connectionID uint64) (*storage.SourceReference, error) {
	if s == nil {
		// Passive storage has no lease owner and intentionally grants no
		// authority; its callers retain the route without a SourceReference.
		return nil, nil //nolint:nilnil // Nil denotes passive storage, not a failed lease acquisition.
	}
	if connectionID == 0 {
		err := s.operationError(errors.New("the received UPDATE has no original physical connection"), peer, receipt, rpc.SourceLeaseAcquire)
		s.setAdmission(peerOwner, err)
		return nil, err
	}
	if receipt == 0 {
		err := s.operationError(errors.New("the received UPDATE has no original source receipt or peer owner"), peer, receipt, rpc.SourceLeaseAcquire)
		s.setAdmission(peerOwner, err)
		return nil, err
	}
	if peerOwner == 0 {
		err := s.operationError(errors.New("the received UPDATE has no original source receipt or peer owner"), peer, receipt, rpc.SourceLeaseAcquire)
		s.setAdmission(peerOwner, err)
		return nil, err
	}
	for {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return nil, s.operationError(context.Canceled, peer, receipt, rpc.SourceLeaseAcquire)
		}
		if lease, exists := s.leases[receipt]; exists {
			if lease.peer != peer {
				s.mu.Unlock()
				err := s.operationError(errors.New("the receipt belongs to another peer"), peer, receipt, rpc.SourceLeaseAcquire)
				s.setAdmission(peerOwner, err)
				return nil, err
			}
			if lease.peerOwner != peerOwner {
				s.mu.Unlock()
				err := s.operationError(errors.New("the receipt belongs to another peer"), peer, receipt, rpc.SourceLeaseAcquire)
				s.setAdmission(peerOwner, err)
				return nil, err
			}
			if lease.connectionID != connectionID {
				s.mu.Unlock()
				err := s.operationError(errors.New("the receipt belongs to another physical connection"), peer, receipt, rpc.SourceLeaseAcquire)
				s.setAdmission(peerOwner, err)
				return nil, err
			}
			if lease.busy != nil {
				busy := lease.busy
				s.mu.Unlock()
				select {
				case <-busy:
					continue
				case <-ctx.Done():
					return nil, s.operationError(ctx.Err(), peer, receipt, rpc.SourceLeaseAcquire)
				}
			}
			if lease.failure != nil {
				err := lease.failure
				s.mu.Unlock()
				return nil, err // A revoked lease awaiting settlement cannot be resurrected.
			}
			lease.ref.AddRef()
			_, cleared := s.admission[peerOwner]
			delete(s.admission, peerOwner)
			s.mu.Unlock()
			if cleared {
				s.notify()
			}
			return lease.ref, nil
		}
		lease := &ribSourceLease{peer: peer, peerOwner: peerOwner, connectionID: connectionID, busy: make(chan struct{})}
		s.leases[receipt] = lease
		s.mu.Unlock()
		_, err := s.manager.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, SourceReceipt: receipt, PeerOwner: peerOwner})
		s.mu.Lock()
		_, changed := s.admission[peerOwner]
		if s.stopped {
			delete(s.leases, receipt)
			close(lease.busy)
			lease.busy = nil
			s.mu.Unlock()
			return nil, s.operationError(context.Canceled, peer, receipt, rpc.SourceLeaseAcquire)
		}
		if err != nil {
			err = s.operationError(err, peer, receipt, rpc.SourceLeaseAcquire)
			previous := s.admission[peerOwner]
			changed = previous == nil || previous.Error() != err.Error()
			delete(s.leases, receipt)
			s.admission[peerOwner] = err
		} else {
			lease.ref = storage.NewSourceReference(lease.peerOwner, lease.connectionID, s.notify)
			delete(s.admission, peerOwner)
		}
		close(lease.busy)
		lease.busy = nil
		s.mu.Unlock()
		if changed {
			s.notify()
		}
		return lease.ref, err
	}
}

// retain is the explicit RIB GR/LLGR decision, never inferred from attributes
// or source DOWN. The reference registry includes alternate current candidates.
func (s *ribSourceLeases) retain(ctx context.Context, peer netip.Addr, peerOwner uint64) error {
	return s.retainReceipts(ctx, peer, peerOwner, s.receiptsForPeer(peerOwner))
}

// retainReceipts performs the real SDK decision on an already captured scope.
// No peer or storage lock may be held; it serializes each existing lease with
// the acquire/release worker. It never scans an address or mints a receipt.
func (s *ribSourceLeases) retainReceipts(ctx context.Context, peer netip.Addr, peerOwner uint64, receipts []uint64) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return s.operationError(context.Canceled, peer, 0, rpc.SourceLeaseRetain)
	}
	for _, receipt := range receipts {
		for {
			s.mu.Lock()
			if s.stopped {
				s.mu.Unlock()
				return s.operationError(context.Canceled, peer, receipt, rpc.SourceLeaseRetain)
			}
			lease := s.leases[receipt]
			if lease == nil {
				s.mu.Unlock()
				break
			}
			if lease.busy != nil {
				busy := lease.busy
				s.mu.Unlock()
				select {
				case <-busy:
					continue
				case <-ctx.Done():
					return s.operationError(ctx.Err(), peer, receipt, rpc.SourceLeaseRetain)
				}
			}
			if lease.ref.References() == 0 {
				s.mu.Unlock()
				break
			}
			lease.busy = make(chan struct{})
			s.mu.Unlock()
			_, err := s.manager.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRetain, SourceReceipt: receipt, PeerOwner: lease.peerOwner})
			s.mu.Lock()
			if err != nil {
				err = s.operationError(err, peer, receipt, rpc.SourceLeaseRetain)
			}
			close(lease.busy)
			lease.busy = nil
			s.mu.Unlock()
			s.notify()
			if err != nil {
				s.setAdmission(peerOwner, err)
				return err
			}
			break
		}
	}
	s.forgetAdmission(peerOwner)
	return nil
}

// settle removes only references with no candidate, temporary intake or owned
// snapshot left. A failed join stays pending for a subsequent settlement; it
// never becomes a falsely completed release. The scan is over original source
// leases, not the route table, and runs only after a last-reference wake.
func (s *ribSourceLeases) settle(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return context.Canceled
	}
	var receipts []uint64
	for receipt, lease := range s.leases {
		if lease.ref.References() == 0 {
			receipts = append(receipts, receipt)
		}
	}
	s.mu.Unlock()
	return s.settleReceipts(ctx, receipts)
}

// receiptsForPeer snapshots this source's current and pending original-source
// receipts. It matches the immutable original Peer owner, so one address alias
// cannot settle another's receipts. Settlement still uses exact receipt IDs.
func (s *ribSourceLeases) receiptsForPeer(peer uint64) []uint64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var receipts []uint64
	for receipt, lease := range s.leases {
		if lease.peerOwner == peer {
			receipts = append(receipts, receipt)
		}
	}
	return receipts
}

// settleReceipts joins only the supplied original-source receipts whose local
// references have ended. An empty list settles nothing. Synchronous source
// operations must not inherit another source's pending release; the background
// worker alone retries the full registry. No caller may hold a RIB lock.
func (s *ribSourceLeases) settleReceipts(ctx context.Context, receipts []uint64) error {
	if s == nil {
		return nil
	}
	var refused error
	for _, receipt := range receipts {
		for {
			s.mu.Lock()
			if s.stopped {
				s.mu.Unlock()
				return errors.Join(refused, context.Canceled)
			}
			lease := s.leases[receipt]
			if lease == nil {
				s.mu.Unlock()
				break
			}
			if lease.busy != nil {
				busy := lease.busy
				s.mu.Unlock()
				select {
				case <-busy:
					continue
				case <-ctx.Done():
					return errors.Join(refused, s.operationError(ctx.Err(), lease.peer, receipt, rpc.SourceLeaseRelease))
				}
			}
			if lease.ref.References() != 0 || lease.outputHolds != 0 {
				s.mu.Unlock()
				break
			}
			lease.busy = make(chan struct{})
			kept := lease.outputs[:0]
			for _, destination := range lease.outputs {
				select {
				case <-destination.done:
				default:
					kept = append(kept, destination)
				}
			}
			clear(lease.outputs[len(kept):])
			lease.outputs = kept
			active := append([]*selectedDestination(nil), kept...)
			auxiliary := append([]*ribSourceSentOutput(nil), lease.sentOutputs...)
			s.mu.Unlock()
			// The running retry worker takes this same nonblocking guard. It
			// MUST NOT release a source before publication or final output.
			err := s.manager.selectedDispatch.sourceReleaseReady(active)
			if err == nil {
				err = joinPhysicalSentOutputs(ctx, s.manager, auxiliary)
			}
			if err == nil {
				_, err = s.manager.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRelease, SourceReceipt: receipt, PeerOwner: lease.peerOwner})
			}
			s.mu.Lock()
			if err != nil {
				lease.failure = s.operationError(err, lease.peer, receipt, rpc.SourceLeaseRelease)
				refused = errors.Join(refused, lease.failure)
			} else {
				delete(s.leases, receipt)
			}
			close(lease.busy)
			lease.busy = nil
			s.mu.Unlock()
			break
		}
	}
	return refused
}

func (s *ribSourceLeases) start(ctx context.Context) {
	work, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		timer := time.NewTimer(sourceLeaseRetry)
		if !timer.Stop() {
			<-timer.C
		}
		defer timer.Stop()
		var retry <-chan time.Time
		for {
			select {
			case <-work.Done():
				return
			case <-s.wake:
			case <-retry:
			}
			pending := s.settle(work)
			reportPending := s.reportFailures(work)
			retry = nil
			if pending != nil {
				reportPending = true
			}
			if reportPending {
				timer.Reset(sourceLeaseRetry)
				retry = timer.C
			}
		}
	}()
}

func (s *ribSourceLeases) stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopped = true
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	// A direct report admitted before stopping also finishes before local
	// teardown; later callers observe stopped under this reporting fence.
	s.reportMu.Lock()
	s.reportMu.Unlock() //nolint:gocritic,staticcheck // badLock, SA2001: acquiring this fence joins reports admitted before stop.
}

// retainSourceOnDown runs while the ordered Session delivery reference is still
// open. It repeats the explicit retention decision so a later received UPDATE
// cannot leave a fresh lease outside the earlier retain-routes command.
func (r *RIBManager) retainSourceOnDown(peer netip.Addr, peerOwner uint64) {
	r.peerMu.RLock()
	retained := r.retainedPeers[peerOwner]
	state := r.grState[peerOwner]
	var source *ribPhysicalSource
	if state != nil {
		source = state.source
	}
	if source != nil {
		metadata := r.peerMeta[peerOwner]
		if metadata == nil {
			retained = false
		} else if metadata.ConnectionID != source.connection {
			retained = false
		}
	}
	r.peerMu.RUnlock()
	if !retained {
		return
	}
	var err error
	if source != nil {
		err = r.sourceLeases.retainPhysical(context.Background(), *source)
	} else {
		err = r.sourceLeases.retain(context.Background(), peer, peerOwner)
	}
	if err != nil {
		r.peerMu.Lock()
		if r.grState[peerOwner] == state {
			delete(r.retainedPeers, peerOwner)
		}
		r.peerMu.Unlock()
		r.sourceLeases.reportFailures(context.Background())
	}
}

// settleSourceLeases completes a best-effort lifecycle operation's sources.
// The background worker alone scans the entire pending registry.
func (r *RIBManager) settleSourceLeases(peers ...uint64) {
	if r.sourceLeases != nil {
		r.sourceLeases.mu.Lock()
		stopped := r.sourceLeases.stopped
		r.sourceLeases.mu.Unlock()
		if stopped {
			return // Engine generation cleanup owns the ended connection's rights.
		}
	}
	var receipts []uint64
	for _, peer := range peers {
		receipts = append(receipts, r.sourceLeases.receiptsForPeer(peer)...)
	}
	if err := r.sourceLeases.settleReceipts(context.Background(), receipts); err != nil {
		r.sourceLeases.notify()
	}
	r.sourceLeases.reportFailures(context.Background())
}

// holdSourceOutput MUST precede source mutation; its caller MUST release the
// hold after publication and the attempted output join. It stores no history:
// every existing lease keeps only a coalesced set of unjoined workers.
// A worker is pruned only when done closes, never merely on replacement UP.
// The worker done channel needs no dispatcher mutex; leaf locks never nest.
func (s *ribSourceLeases) holdSourceOutput(receipts []uint64, active []*selectedDestination) func() {
	if s == nil {
		return func() {}
	}
	s.mu.Lock()
	var held []*ribSourceLease
	seen := make(map[uint64]struct{}, len(receipts))
	for _, receipt := range receipts {
		if _, duplicate := seen[receipt]; duplicate {
			continue
		}
		seen[receipt] = struct{}{}
		lease := s.leases[receipt]
		if lease == nil {
			continue
		}
		lease.outputHolds++
		held = append(held, lease)
		kept := lease.outputs[:0]
		for _, destination := range lease.outputs {
			select {
			case <-destination.done:
			default:
				kept = append(kept, destination)
			}
		}
		clear(lease.outputs[len(kept):])
		lease.outputs = kept
		for _, destination := range active {
			select {
			case <-destination.done:
				continue
			default:
			}
			duplicate := false
			for _, previous := range lease.outputs {
				if previous == destination {
					duplicate = true
					break
				}
			}
			if !duplicate {
				lease.outputs = append(lease.outputs, destination)
			}
		}
	}
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			for _, lease := range held {
				lease.outputHolds--
			}
			s.mu.Unlock()
			s.notify()
		})
	}
}

// receiptsAtAddress captures current retained rights for an explicit
// address-wide operator purge. It reads the original lease's address without
// peerMu, so capture MUST precede the mutation lock and final joins MUST follow
// its unlock. It grants no source or output authority and stores no history.
func (s *ribSourceLeases) receiptsAtAddress(peer netip.Addr) []uint64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var receipts []uint64
	for receipt, lease := range s.leases {
		if lease.peer == peer {
			receipts = append(receipts, receipt)
		}
	}
	return receipts
}
