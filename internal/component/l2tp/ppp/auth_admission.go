// Design: docs/architecture/l2tp/subscriber-session-model.md -- original authentication to accounting transfer.
package ppp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// ErrAuthAdmissionAborted proves this original authentication round did not
// acknowledge a first network assignment. It says nothing about a successor.
var ErrAuthAdmissionAborted = errors.New("ppp: original authentication ended without a network assignment")

// ErrAuthAssignmentClaimed refuses a second owner for one original request.
// The first claimant retains its obligation; a refusal never releases it.
var ErrAuthAssignmentClaimed = errors.New("ppp: original accounting assignment already has an owner")

// AuthNetworkAssignment is copied from the original PPP producer. Preparation
// precedes publication; successful Assignment follows its acknowledgment. The
// counter receipt retains that same network even after the worker disappears.
type AuthNetworkAssignment struct {
	Birth            subscriberid.L2TPBirth
	NetworkLifetime  l2tpevents.NetworkLifetime
	BaselineCounters l2tpevents.TrafficSample
	CounterReceipt   NetworkCounterReceipt
}

type authAdmissionBinding struct {
	mu       sync.Mutex
	birth    subscriberid.L2TPBirth
	done     chan struct{}
	accepted bool
	prepared *AuthNetworkAssignment
	ended    bool
	assigned bool
	approval *authAssignmentApproval
}

// AuthAdmissionReceipt is a readonly observation of one original emitted
// initial L2TP authentication round. It grants no answer or cleanup authority,
// owns no goroutine and carries no credential payload. Copies compare equal
// only for the same producer binding. The zero receipt is invalid.
type AuthAdmissionReceipt struct{ binding *authAdmissionBinding }

func (receipt AuthAdmissionReceipt) IsValid() bool { return receipt.binding != nil }

// AuthAssignmentApproval is the one local accounting owner's capability for
// this initial L2TP request. It MUST remain with that owner's reserved task,
// never on EventBus. Approve MUST follow queuing its exact immutable header.
// Copies retain the same original; zero is invalid. Safe for concurrent use.
type AuthAssignmentApproval struct{ binding *authAdmissionBinding }

type authAssignmentApproval struct {
	ready    chan struct{}
	deadline time.Time
	approved bool
}

// ClaimAccountingAssignment MUST precede answering Accept. Its owner MUST
// reserve its existing finite accounting capacity before answering Accept,
// retain the returned token in that task and reject if capacity is refused. Only a genuine
// unanswered initial L2TP request can claim, exactly once; duplicates refuse.
// No-claim, PPPoE and reauthentication paths acquire no local approval wait.
func (request EventAuthRequest) ClaimAccountingAssignment() (AuthAssignmentApproval, error) {
	if request.pending == nil {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	request.pending.mu.Lock()
	defer request.pending.mu.Unlock()
	if !request.validLocked() {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	if request.pending.answered {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	if !request.Control.Active() {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	binding := request.pending.admission
	if binding == nil {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.ended {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	if binding.accepted {
		return AuthAssignmentApproval{}, ErrRequestExpired
	}
	if binding.approval != nil {
		return AuthAssignmentApproval{}, ErrAuthAssignmentClaimed
	}
	binding.approval = &authAssignmentApproval{ready: make(chan struct{})}
	return AuthAssignmentApproval{binding: binding}, nil
}

// Approve accepts only the claimed original's actually prepared Birth and
// network lifetime. Refusing a wrong, expired or aborted value changes no
// original or sibling. Repeating the same successful approval is idempotent.
// This is local admission only; it never proves a RADIUS exchange completed.
func (approval AuthAssignmentApproval) Approve(birth subscriberid.L2TPBirth, lifetime l2tpevents.NetworkLifetime) bool {
	if approval.binding == nil {
		return false
	}
	binding := approval.binding
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.approval == nil {
		return false
	}
	if binding.prepared == nil {
		return false
	}
	if binding.prepared.Birth != birth {
		return false
	}
	if binding.prepared.NetworkLifetime != lifetime {
		return false
	}
	if binding.ended {
		return binding.assigned
	}
	if binding.approval.deadline.IsZero() {
		return false
	}
	if !time.Now().Before(binding.approval.deadline) {
		return false
	}
	if binding.approval.approved {
		return true
	}
	binding.approval.approved = true
	close(binding.approval.ready)
	return true
}

// armAssignmentApproval runs before publication only for a claimed initial
// assignment. The existing immutable IP timeout bounds this local ownership
// transfer; no new config, timer worker or unclaimed-path wait is introduced.
func (receipt AuthAdmissionReceipt) armAssignmentApproval(timeout time.Duration) (<-chan struct{}, time.Time, error) {
	if receipt.binding == nil {
		return nil, time.Time{}, nil
	}
	binding := receipt.binding
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.assigned {
		return nil, time.Time{}, nil
	}
	if binding.ended {
		return nil, time.Time{}, ErrAuthAdmissionAborted
	}
	if binding.approval == nil {
		return nil, time.Time{}, nil
	}
	if binding.approval.deadline.IsZero() {
		binding.approval.deadline = time.Now().Add(timeout)
	}
	return binding.approval.ready, binding.approval.deadline, nil
}

// AccountingAdmission captures only the actual request's immutable receipt.
// It deliberately does not use Active: consuming Accept retires answer
// authority while the provisional accounting reservation remains necessary.
// PPPoE, reauthentication in an existing network and synthetic requests have
// no receipt. Callers MUST reserve the accounting owner before sending Accept.
func (request EventAuthRequest) AccountingAdmission() AuthAdmissionReceipt {
	if request.pending == nil {
		return AuthAdmissionReceipt{}
	}
	if request.Control != request.pending.control {
		return AuthAdmissionReceipt{}
	}
	if request.SubscriberID != request.pending.subscriberID {
		return AuthAdmissionReceipt{}
	}
	return AuthAdmissionReceipt{binding: request.pending.admission}
}

// PreparedAssignment is a nonblocking copied binding, established before
// assignment publication. It is NOT permission to Start accounting. A callback
// may use it to match BOTH actual typed Birth and NetworkLifetime against an
// already-reserved owner and enqueue its first immutable header. The binding
// remains readable after abort; Assignment alone decides acknowledgment.
func (receipt AuthAdmissionReceipt) PreparedAssignment() (AuthNetworkAssignment, bool) {
	if receipt.binding == nil {
		return AuthNetworkAssignment{}, false
	}
	binding := receipt.binding
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.prepared == nil {
		return AuthNetworkAssignment{}, false
	}
	return *binding.prepared, true
}

// Assignment waits on the existing accounting owner, never on a publication
// callback. A context error leaves ownership unresolved; a later wait on this
// SAME receipt remains useful. Only ErrAuthAdmissionAborted proves no Start
// was authorized by an acknowledged first assignment.
func (receipt AuthAdmissionReceipt) Assignment(ctx context.Context) (AuthNetworkAssignment, error) {
	if receipt.binding == nil {
		return AuthNetworkAssignment{}, ErrRequestExpired
	}
	if ctx == nil {
		return AuthNetworkAssignment{}, errors.New("ppp: authentication admission wait requires a context")
	}
	if err := ctx.Err(); err != nil {
		return AuthNetworkAssignment{}, err
	}
	select {
	case <-ctx.Done():
		return AuthNetworkAssignment{}, ctx.Err()
	case <-receipt.binding.done:
	}
	binding := receipt.binding
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if !binding.assigned {
		return AuthNetworkAssignment{}, ErrAuthAdmissionAborted
	}
	return *binding.prepared, nil
}

func (binding *authAdmissionBinding) accept() bool {
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.ended {
		return false
	}
	binding.accepted = true
	return true
}

// Retiring answer authority does not abort a consumed Accept. Every other
// exit from the real wait proves that this original request cannot assign.
func (binding *authAdmissionBinding) retireWait() {
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.accepted {
		return
	}
	if binding.ended {
		return
	}
	binding.ended = true
	close(binding.done)
}

func (binding *authAdmissionBinding) prepare(lifetime l2tpevents.NetworkLifetime, baseline l2tpevents.TrafficSample, counters NetworkCounterReceipt) bool {
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if !binding.accepted {
		return false
	}
	if binding.ended {
		if !binding.assigned {
			return false
		}
	}
	if !lifetime.IsValid() {
		return false
	}
	if !counters.IsValid() {
		return false
	}
	if binding.prepared != nil {
		if binding.prepared.NetworkLifetime != lifetime {
			return false
		}
		return binding.prepared.CounterReceipt == counters
	}
	binding.prepared = &AuthNetworkAssignment{Birth: binding.birth,
		NetworkLifetime: lifetime, BaselineCounters: baseline, CounterReceipt: counters}
	return true
}

func (binding *authAdmissionBinding) finish(assigned bool) bool {
	binding.mu.Lock()
	defer binding.mu.Unlock()
	if binding.ended {
		return binding.assigned
	}
	if assigned {
		if !binding.accepted {
			return false
		}
		if binding.prepared == nil {
			return false
		}
		if binding.approval != nil {
			if !binding.approval.approved {
				return false
			}
			if !time.Now().Before(binding.approval.deadline) {
				return false
			}
		}
	}
	binding.ended = true
	binding.assigned = assigned
	close(binding.done)
	return assigned
}

// The worker alone opens a new round. The small metadata mutex also fences
// canceled-owner Close against inserting a late round. No native I/O, answer,
// callback or goroutine join occurs under either admission mutex.
func (session *pppSession) beginAuthAdmission() (*authAdmissionBinding, error) {
	if session.pppoe {
		return nil, nil
	}
	if !session.l2tpBirth.IsValid() {
		return nil, nil
	}
	if session.ncpStarted {
		return nil, nil
	}
	session.authAdmissionMu.Lock()
	defer session.authAdmissionMu.Unlock()
	if session.authAdmissionClosed {
		return nil, ErrSessionNotFound
	}
	if session.authAdmission != nil {
		session.authAdmission.finish(false)
	}
	binding := &authAdmissionBinding{birth: session.l2tpBirth, done: make(chan struct{})}
	session.authAdmission = binding
	return binding, nil
}

// reset aborts only the current unassigned round and permits the next LCP
// authentication. A retained old receipt cannot mutate that next round.
func (session *pppSession) abortAuthAdmission() {
	session.authAdmissionMu.Lock()
	defer session.authAdmissionMu.Unlock()
	if session.authAdmission != nil {
		session.authAdmission.finish(false)
	}
	session.authAdmission = nil
}

// close permanently prevents late preparation before actual native Close or
// final publication can block. An already acknowledged assignment is unchanged.
func (session *pppSession) closeAuthAdmission() {
	session.authAdmissionMu.Lock()
	defer session.authAdmissionMu.Unlock()
	session.authAdmissionClosed = true
	if session.authAdmission != nil {
		session.authAdmission.finish(false)
	}
}

// Called after successful prepareNetworkCounters and before event send.
// Return the exact receipt so a concurrent reset cannot make the later ACK
// commit a successor, or accidentally succeed because the current cell is nil.
func (session *pppSession) prepareAuthNetworkAssignment(lifetime l2tpevents.NetworkLifetime, baseline l2tpevents.TrafficSample) (AuthAdmissionReceipt, error) {
	if session.pppoe {
		return AuthAdmissionReceipt{}, nil
	}
	if !session.l2tpBirth.IsValid() {
		return AuthAdmissionReceipt{}, nil
	}
	session.authAdmissionMu.Lock()
	binding := session.authAdmission
	closed := session.authAdmissionClosed
	session.authAdmissionMu.Unlock()
	if closed {
		return AuthAdmissionReceipt{}, ErrSessionNotFound
	}
	if binding == nil {
		return AuthAdmissionReceipt{}, ErrRequestExpired
	}
	counters, err := session.control.CaptureNetworkCounters(lifetime)
	if err != nil {
		return AuthAdmissionReceipt{}, err
	}
	if !binding.prepare(lifetime, baseline, counters) {
		return AuthAdmissionReceipt{}, ErrRequestExpired
	}
	return AuthAdmissionReceipt{binding: binding}, nil
}

// The exact provisional outcome is completed before fail can publish Down or
// wait for native service teardown. A deadline is not remote cleanup evidence.
func (session *pppSession) authAssignmentTimedOut(receipt AuthAdmissionReceipt) bool {
	if receipt.binding != nil {
		receipt.binding.finish(false)
	}
	session.fail(opererr.New(
		"The subscriber accounting approval deadline expired before forwarding began",
		"The reserved accounting owner did not accept this original network assignment",
		"show doctor",
	).Error())
	return false
}
