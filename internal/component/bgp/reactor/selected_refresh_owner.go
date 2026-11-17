// Design: docs/architecture/update-building.md -- unfinished refresh ownership.
// Related: selected_cleanup.go -- ended-owner retirement uses this Session.
package reactor

import (
	"context"
	"errors"
	"maps"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedRefreshPending is an immutable snapshot of actual unfinished Begin
// writes, bounded by this Session's negotiated families. Its zero value and a
// nil pointer both mean no pending markers. Only recordSelectedRefresh creates
// populated snapshots, after a validated marker actually reaches the wire.
// Marker writers MUST hold selectedMu and writeMu. Terminal cleanup clears
// only after sealing under selectedMu. Readers may load concurrently.
type selectedRefreshPending struct {
	connection *selectedRefreshConnection
	owners     map[family.Family]plugin.SelectedOwner
}

// selectedRefreshConnection retains the one socket claim even after ordinary
// bounded close clears Session.socketClosed. Its zero value and nil pointer
// are invalid; only an accepted Begin constructs it with the exact wire conn.
// The owners snapshot is immutable, while this completion link is published
// once by the existing claimSocket path or the marker's publication recheck.
type selectedRefreshConnection struct {
	conn  net.Conn
	claim atomic.Pointer[socketClaim]
}

// selectedRefreshOwner prevents a successor from overwriting or ending an
// earlier incarnation's actual wire obligation. The caller MUST hold both
// selectedMu and writeMu, after checking the captured Session's authority.
func (s *Session) selectedRefreshOwner(fam family.Family, owner plugin.SelectedOwner) error {
	pending := s.selectedRefresh.Load()
	if pending == nil {
		return nil
	}
	previous, found := pending.owners[fam]
	if !found {
		return nil
	}
	if previous.SameIncarnation(owner) {
		return nil
	}
	return errors.New("select-refresh family has an unfinished Begin from another selecting process incarnation; finish its cleanup before refreshing this session")
}

// recordSelectedRefresh follows a successful complete marker write while its
// caller still holds selectedMu and writeMu. The validated request supplies
// only negotiated families and Begin/End, and selectedRefreshOwner already
// proved any existing family obligation belongs to this exact incarnation.
// Copying this bounded control snapshot lets cleanup skip unrelated Sessions
// without acquiring their selectedMu or joining their blocked writers.
func (s *Session) recordSelectedRefresh(conn net.Conn, fam family.Family, action rpc.SelectedRefreshAction, owner plugin.SelectedOwner) {
	// Pair publication with claimSocket: either it observes our new pending
	// snapshot, or this post-publication load observes its matching claim.
	// writeMu prevents that claim being cleared before this recheck.
	defer func() { s.captureSelectedRefreshClose(s.socketClosed.Load()) }()
	previous := s.selectedRefresh.Load()
	if previous == nil {
		if action == rpc.SelectedRefreshEnd {
			return
		}
		s.selectedRefresh.Store(&selectedRefreshPending{connection: &selectedRefreshConnection{conn: conn},
			owners: map[family.Family]plugin.SelectedOwner{fam: owner}})
		return
	}
	if action == rpc.SelectedRefreshBegin {
		if current, found := previous.owners[fam]; found {
			if current.SameIncarnation(owner) {
				return
			}
		}
	}
	next := &selectedRefreshPending{connection: previous.connection, owners: maps.Clone(previous.owners)}
	if action == rpc.SelectedRefreshBegin {
		next.owners[fam] = owner
	} else {
		delete(next.owners, fam)
	}
	if len(next.owners) == 0 {
		s.selectedRefresh.Store(nil)
		return
	}
	s.selectedRefresh.Store(next)
}

// hasSelectedRefreshOwner is a lock-free cleanup prefilter. Cleanup MUST
// recheck under selectedMu before retiring the captured Session, because a
// completed End may have removed this obligation since the first load.
func (s *Session) hasSelectedRefreshOwner(owner plugin.SelectedOwner) bool {
	return s.pendingSelectedRefreshOwner(owner) != nil
}

// pendingSelectedRefreshOwner captures an immutable exact-owner obligation.
// A terminal cleanup retains it across an overdue selectedMu admission, so
// deadline expiry cannot discard the original socket completion boundary.
func (s *Session) pendingSelectedRefreshOwner(owner plugin.SelectedOwner) *selectedRefreshPending {
	pending := s.selectedRefresh.Load()
	if pending == nil {
		return nil
	}
	for _, pendingOwner := range pending.owners {
		if pendingOwner.SameIncarnation(owner) {
			return pending
		}
	}
	return nil
}

// captureSelectedRefreshClose links only the exact pending wire to its real
// first close completion. Safe for concurrent use; this retains no history
// beyond one Session and its unfinished negotiated-family obligations.
func (s *Session) captureSelectedRefreshClose(claim *socketClaim) {
	if claim == nil {
		return
	}
	pending := s.selectedRefresh.Load()
	if pending == nil {
		return
	}
	if pending.connection.conn != claim.conn {
		return
	}
	pending.connection.claim.CompareAndSwap(nil, claim)
}

// joinSelectedRefreshClose follows normal close, which may have bounded its
// wait for an existing claimant. A duplicate net.Conn.Close is not a join.
// Like source removal's final cancellation barrier, an overdue caller still
// joins the actual owner before it may report the cleanup as completed.
func (pending *selectedRefreshPending) joinSelectedRefreshClose(ctx context.Context) error {
	claim := pending.connection.claim.Load()
	if claim == nil {
		return errors.New("select-refresh original socket has no retirement completion claim; inspect the captured peer session")
	}
	select {
	case <-claim.done:
		return nil
	case <-ctx.Done():
		<-claim.done
		return ctx.Err()
	}
}

// joinSelectedRefreshWriter also joins the direct retirement Close from a
// failed or canceled local UPDATE. The writer holds writeMu through Close;
// an ordinary socket claimant may finish a duplicate Close earlier. A sealed
// Session admits no new output, so taking and releasing this existing path
// joins the last holder without installing another worker or a timeout.
func (s *Session) joinSelectedRefreshWriter(ctx context.Context) error {
	if err := s.writeMu.LockContext(ctx); err != nil {
		s.writeMu.Lock()
		s.writeMu.Unlock() //nolint:gocritic,staticcheck // badLock, SA2001: the empty critical section joins the sealed Session's last writer
		return err
	}
	s.writeMu.Unlock()
	return nil
}
