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
	peer      netip.Addr
	peerOwner uint64
	ref       *storage.SourceReference
	busy      chan struct{}
	failure   error
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
	admission   map[netip.Addr]error
	reportMu    sync.Mutex
	reported    map[netip.Addr]string
	reportAfter map[netip.Addr]time.Time
	wake        chan struct{}
	cancel      context.CancelFunc
	done        chan struct{}
	stopped     bool // under mu; closes intake before terminal local cleanup
}

func newRIBSourceLeases(manager *RIBManager) *ribSourceLeases {
	return &ribSourceLeases{manager: manager, leases: make(map[uint64]*ribSourceLease),
		admission: make(map[netip.Addr]error), reported: make(map[netip.Addr]string),
		reportAfter: make(map[netip.Addr]time.Time), wake: make(chan struct{}, 1)}
}

func (s *ribSourceLeases) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *ribSourceLeases) acquire(ctx context.Context, peer netip.Addr, peerOwner, receipt uint64) (*storage.SourceReference, error) {
	if s == nil {
		// Passive storage has no lease owner and intentionally grants no
		// authority; its callers retain the route without a SourceReference.
		return nil, nil //nolint:nilnil // Nil denotes passive storage, not a failed lease acquisition.
	}
	if receipt == 0 {
		err := s.operationError(errors.New("the received UPDATE has no original source receipt or peer owner"), peer, receipt, rpc.SourceLeaseAcquire)
		s.setAdmission(peer, err)
		return nil, err
	}
	if peerOwner == 0 {
		err := s.operationError(errors.New("the received UPDATE has no original source receipt or peer owner"), peer, receipt, rpc.SourceLeaseAcquire)
		s.setAdmission(peer, err)
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
				s.setAdmission(peer, err)
				return nil, err
			}
			if lease.peerOwner != peerOwner {
				s.mu.Unlock()
				err := s.operationError(errors.New("the receipt belongs to another peer"), peer, receipt, rpc.SourceLeaseAcquire)
				s.setAdmission(peer, err)
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
			_, cleared := s.admission[peer]
			delete(s.admission, peer)
			s.mu.Unlock()
			if cleared {
				s.notify()
			}
			return lease.ref, nil
		}
		lease := &ribSourceLease{peer: peer, peerOwner: peerOwner, busy: make(chan struct{})}
		s.leases[receipt] = lease
		s.mu.Unlock()
		_, err := s.manager.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseAcquire, SourceReceipt: receipt, PeerOwner: peerOwner})
		s.mu.Lock()
		_, changed := s.admission[peer]
		if s.stopped {
			delete(s.leases, receipt)
			close(lease.busy)
			lease.busy = nil
			s.mu.Unlock()
			return nil, s.operationError(context.Canceled, peer, receipt, rpc.SourceLeaseAcquire)
		}
		if err != nil {
			err = s.operationError(err, peer, receipt, rpc.SourceLeaseAcquire)
			previous := s.admission[peer]
			changed = previous == nil || previous.Error() != err.Error()
			delete(s.leases, receipt)
			s.admission[peer] = err
		} else {
			lease.ref = storage.NewSourceReference(lease.peerOwner, s.notify)
			delete(s.admission, peer)
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
func (s *ribSourceLeases) retain(ctx context.Context, peer netip.Addr) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return s.operationError(context.Canceled, peer, 0, rpc.SourceLeaseRetain)
	}
	var receipts []uint64
	for receipt, lease := range s.leases {
		if lease.peer == peer {
			receipts = append(receipts, receipt)
		}
	}
	s.mu.Unlock()
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
				s.setAdmission(peer, err)
				return err
			}
			break
		}
	}
	s.forgetAdmission(peer)
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
// receipts. The address is the existing RIB source selector, not release
// authority: settlement operates on the returned receipt IDs only.
func (s *ribSourceLeases) receiptsForPeer(peer netip.Addr) []uint64 {
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
			if lease.ref.References() != 0 {
				s.mu.Unlock()
				break
			}
			lease.busy = make(chan struct{})
			s.mu.Unlock()
			_, err := s.manager.plugin.SourceLease(ctx, rpc.SourceLeaseInput{Action: rpc.SourceLeaseRelease, SourceReceipt: receipt, PeerOwner: lease.peerOwner})
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
	defer s.reportMu.Unlock()
}

// retainSourceOnDown runs while the ordered Session delivery reference is still
// open. It repeats the explicit retention decision so a later received UPDATE
// cannot leave a fresh lease outside the earlier retain-routes command.
func (r *RIBManager) retainSourceOnDown(peer netip.Addr) {
	r.peerMu.RLock()
	retained := r.retainedPeers[peer]
	r.peerMu.RUnlock()
	if !retained {
		return
	}
	if err := r.sourceLeases.retain(context.Background(), peer); err != nil {
		r.peerMu.Lock()
		delete(r.retainedPeers, peer)
		r.peerMu.Unlock()
		r.sourceLeases.reportFailures(context.Background())
	}
}

// settleSourceLeases completes a best-effort lifecycle operation's sources.
// The background worker alone scans the entire pending registry.
func (r *RIBManager) settleSourceLeases(peers ...netip.Addr) {
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
