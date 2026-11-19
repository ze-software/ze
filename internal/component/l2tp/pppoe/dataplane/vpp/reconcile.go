// Design: docs/architecture/l2tp/bng-5-pppoe.md -- retained VPP ownership
// Related: reconcile_map.go -- control leases; reconcile_doctor.go -- reports
package vpp

import (
	"context"
	"errors"
	"fmt"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	"github.com/ze-software/ze/internal/core/opererr"
	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
)

// Reconciliation is a value snapshot, including reservations still unresolved.
// A successful read with a nonzero pending count is not a clean release.
type Reconciliation struct {
	PendingSessions uint32
	PendingAccesses uint32
}

type retainedSession struct {
	reservation *macReservation
	mutation    *pendingMutation
}
type nativeSessionKey struct {
	mac [6]byte
	sid uint16
}

func init() {
	if err := vppcomp.RegisterProcessExitObserver("pppoe-session-dataplane", retireSessionProcess); err != nil {
		panic("BUG: VPP PPPoE retirement registration failed: " + err.Error())
	}
}

// Reconcile retries only this provider's retained cleanup, on the original
// immutable connection. A failed or changed connection cannot authorize an
// old owner to use a new one. Unknown creations are released only by completed
// absence readback or an actual supervised process exit; presence is never
// converted to deletion authority. Safe for concurrent calls and admissions.
func Reconcile(ctx context.Context) (Reconciliation, error) {
	if ctx == nil {
		return Reconciliation{}, errors.New("pppoe-vpp: reconciliation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return Reconciliation{}, err
	}
	retireSessionProcess(nil)
	var failure error
	unknown := make(map[*binaryAPI][]retainedSession)
	for _, item := range retainedSessions() {
		if err := ctx.Err(); err != nil {
			failure = errors.Join(failure, err)
			break
		}
		pending := item.mutation
		if pending.api == nil {
			failure = errors.Join(failure, errors.New("retained session has no API owner"))
			continue
		}
		if pending.session == nil {
			unknown[pending.api] = append(unknown[pending.api], item)
			continue
		}
		err := pending.retrySession(ctx)
		if err == nil {
			releaseRetainedSession(item)
		} else {
			reportSessionReconciliation(pending, err)
			failure = errors.Join(failure, err)
		}
	}
	// At most 65,534 reservations and at most that many captured API owners
	// exist. One complete dump per owner avoids a dump for every subscriber.
	for client, items := range unknown {
		keys, err := client.readSessionKeys(ctx)
		for _, item := range items {
			pending := item.mutation
			if err != nil {
				reportSessionReconciliation(pending, err)
				continue
			}
			if _, present := keys[nativeSessionKey{pending.request.PeerMAC, pending.request.SessionID}]; present {
				reportSessionReconciliation(pending, errors.New("session creation is visible but its ownership remains unconfirmed"))
				continue
			}
			releaseRetainedSession(item)
		}
		failure = errors.Join(failure, err)
	}
	accesses, err := preparedVPPMaps.reconcileRetained(ctx)
	failure = errors.Join(failure, err)
	status := Reconciliation{PendingSessions: uint32(len(retainedSessions())), PendingAccesses: uint32(accesses)}
	if failure != nil {
		return status, opererr.Wrap(failure, "PPPoE VPP cleanup could not be confirmed",
			"Unresolved owners remain reserved; another subscriber cannot inherit their forwarding state.", "show warnings")
	}
	return status, nil
}

func (a *binaryAPI) processExited() bool {
	if a == nil {
		return false
	}
	return a.retired.Load()
}

// The observer performs bounded local bookkeeping only. Active owners remain
// admitted until the PPP lifetime stops them; their later Close sees the same
// connector's supervised exit fence and sends no native cleanup requests.
func retireSessionProcess(proof vppcomp.ProcessExit) {
	if proof != nil {
		preparedVPPMaps.retireProcess(proof)
		retireCoveredSessions(func(client *binaryAPI) bool {
			if client == nil {
				return false
			}
			if client.processExited() {
				return true
			}
			if client.connector == nil {
				return false
			}
			return proof.Covers(client.connector.Identity(), client.generation)
		})
		return
	}
	retireCoveredSessions(func(client *binaryAPI) bool {
		if client == nil {
			return false
		}
		return client.processExited()
	})
}

func retireCoveredSessions(covered func(*binaryAPI) bool) {
	// Admission and proof publication share this lock. Capture every API
	// before its first mutation, including active owners not yet pending.
	// A later retain sees the same retired API and cannot lose this proof.
	macReservations.Lock()
	for _, reservation := range macReservations.held {
		if reservation.api == nil {
			continue
		}
		if covered(reservation.api) {
			reservation.api.retired.Store(true)
		}
	}
	macReservations.Unlock()

	// Never hold the reservation mutex while joining an owner's state lock:
	// initial Close takes those locks in the opposite order when retaining.
	for _, item := range retainedSessions() {
		pending := item.mutation
		if !covered(pending.api) {
			continue
		}
		pending.api.retired.Store(true)
		if owner := pending.session; owner != nil {
			owner.mu.Lock()
			if owner.closed {
				clear(owner.routes)
				clear(owner.stagedRoutes)
				clear(owner.addresses)
				clear(owner.withdrawals)
				// This endpoint already completed its one Close before the
				// failed native cleanup was retained. Preserve that local error.
				owner.closeErr = owner.socket.Close()
			}
			owner.mu.Unlock()
		}
		releaseRetainedSession(item)
	}
}

func retainedSessions() []retainedSession {
	macReservations.Lock()
	defer macReservations.Unlock()
	items := make([]retainedSession, 0, len(macReservations.held))
	for _, reservation := range macReservations.held {
		if reservation.pending != nil {
			items = append(items, retainedSession{reservation, reservation.pending})
		}
	}
	return items
}

func releaseRetainedSession(item retainedSession) {
	macReservations.Lock()
	defer macReservations.Unlock()
	if macReservations.held[item.reservation.mac] != item.reservation {
		return
	}
	if item.reservation.pending != item.mutation {
		return
	}
	item.reservation.pending = nil
	delete(macReservations.held, item.reservation.mac)
	clearSessionReconciliation(item.mutation)
}

func (p *pendingMutation) retrySession(ctx context.Context) error {
	s := p.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		return errors.New("pppoe-vpp: active session cannot be reconciled as failed cleanup")
	}
	if !p.api.processExited() {
		present, err := p.readSessionIdentity(ctx)
		if err != nil {
			return err
		}
		if !present {
			if len(s.routes) != 0 {
				return errors.New("pppoe-vpp: session disappeared while owned forwarding paths remain")
			}
			s.closeErr = s.socket.Close()
			return s.closeErr
		}
	}
	cleanup := s.closeNative(ctx)
	s.closeErr = errors.Join(s.socket.Close(), cleanup)
	return s.closeErr
}

func (p *pendingMutation) readSessionIdentity(ctx context.Context) (bool, error) {
	found := false
	s := p.session
	err := p.api.dump(ctx, &pppoeapi.PppoeSessionDump{SwIfIndex: interface_types.InterfaceIndex(s.identity.NativeIndex)}, 1, func(message api.Message) error {
		details, ok := message.(*pppoeapi.PppoeSessionDetails)
		if !ok {
			return fmt.Errorf("pppoe-vpp: retained session readback %T", message)
		}
		if details == nil {
			return errors.New("pppoe-vpp: retained session readback is nil")
		}
		if found {
			return errors.New("pppoe-vpp: retained session readback is duplicated")
		}
		found = true
		if uint32(details.SwIfIndex) != s.identity.NativeIndex {
			return errors.New("pppoe-vpp: retained index belongs to another interface")
		}
		if details.SessionID != p.request.SessionID {
			return errors.New("pppoe-vpp: retained index belongs to another session ID")
		}
		if [6]byte(details.ClientMac) != p.request.PeerMAC {
			return errors.New("pppoe-vpp: retained index belongs to another subscriber")
		}
		if uint32(details.EncapIfIndex) != p.access {
			return errors.New("pppoe-vpp: retained session belongs to another access")
		}
		if [6]byte(details.LocalMac) != p.local {
			return errors.New("pppoe-vpp: retained session has another local MAC")
		}
		if details.DecapVrfID != p.table {
			return errors.New("pppoe-vpp: retained session belongs to another routing table")
		}
		return nil
	})
	return found, err
}

func (a *binaryAPI) readSessionKeys(ctx context.Context) (map[nativeSessionKey]struct{}, error) {
	keys := make(map[nativeSessionKey]struct{})
	const sessionMax = 65534
	err := a.dump(ctx, &pppoeapi.PppoeSessionDump{SwIfIndex: interface_types.InterfaceIndex(noInterface)}, sessionMax, func(message api.Message) error {
		details, ok := message.(*pppoeapi.PppoeSessionDetails)
		if !ok {
			return fmt.Errorf("pppoe-vpp: session absence readback %T", message)
		}
		if details == nil {
			return errors.New("pppoe-vpp: session absence readback is nil")
		}
		key := nativeSessionKey{[6]byte(details.ClientMac), details.SessionID}
		if _, duplicate := keys[key]; duplicate {
			return errors.New("pppoe-vpp: session absence readback has duplicate identities")
		}
		keys[key] = struct{}{}
		return nil
	})
	return keys, err
}
