// Design: docs/architecture/l2tp/subscriber-session-model.md -- original accounting assignment approval.
package ppp

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// A real admitted IPCP network needs BOTH its exact local owner approval and
// the transport ACK. Either arrival order still permits useful LCP traffic.
func TestAuthAssignmentApprovalWaitsForOwnerAndTransport(t *testing.T) {
	for _, approvalFirst := range []bool{true, false} {
		name := "ack-first"
		if approvalFirst {
			name = "approval-first"
		}
		t.Run(name, func(t *testing.T) {
			session, request := startAuthAdmissionDriver(t)
			receipt := request.AccountingAdmission()
			connects := authApprovalConnects(t, session)
			approval, err := request.ClaimAccountingAssignment()
			if err != nil {
				t.Fatal(err)
			}
			acceptAuthAdmissionRequest(t, session, request)
			assigned := prepareAuthAdmissionIPCP(t, session)
			if approvalFirst {
				if !approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
					t.Fatal("exact prepared original was refused")
				}
				requireAuthAdmissionPending(t, receipt)
				assigned.Acknowledge()
			} else {
				assigned.Acknowledge()
				requireAuthAdmissionPending(t, receipt)
				if !approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
					t.Fatal("exact prepared original was refused after ACK")
				}
			}
			session.requireUp(t)
			if connects.Load() != 1 {
				t.Fatalf("useful original physical Connect calls = %d", connects.Load())
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			actual, err := receipt.Assignment(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if actual.Birth != assigned.Birth {
				t.Fatal("approval changed the original birth")
			}
			if actual.NetworkLifetime != assigned.NetworkLifetime {
				t.Fatal("approval changed the original network")
			}
			if !approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
				t.Fatal("same original approval was not idempotent")
			}
			frameLeaseEcho(t, session.peer)
		})
	}
}

// An actual transport ACK without its claimed accounting owner must terminate
// before forwarding. Observing the real Down avoids a time-only no-Up check.
// MUTATION: ignoring the claimed binding lets the real ACK publish Up and call Connect.
func TestAuthAssignmentApprovalMissingOwnerEndsBeforeForwarding(t *testing.T) {
	session, request := startAuthAdmissionDriver(t, func(start *StartSession) { start.IPTimeout = 2 * time.Second })
	receipt := request.AccountingAdmission()
	connects := authApprovalConnects(t, session)
	approval, err := request.ClaimAccountingAssignment()
	if err != nil {
		t.Fatal(err)
	}
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	assigned.Acknowledge()
	event := waitForEvent(t, session.driver.EventsOut(), 4*time.Second)
	down, ok := event.(EventSessionDown)
	if !ok {
		t.Fatalf("unapproved assigned network published %T, want Down before Up", event)
	}
	if down.Control != session.control {
		t.Fatal("approval timeout ended another owner")
	}
	if connects.Load() != 0 {
		t.Fatal("unapproved original connected forwarding before timeout")
	}
	if !strings.Contains(down.Reason, "accounting approval deadline") {
		t.Fatalf("Down did not identify accounting approval deadline: %q", down.Reason)
	}
	if _, err := receipt.Assignment(context.Background()); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("expired original assignment = %v", err)
	}
	if approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
		t.Fatal("late local approval reopened the expired original")
	}
	if receipt.binding.finish(true) {
		t.Fatal("late ACK reopened the expired original")
	}
}

// Exact cancellation before ACK prevents a retained token from authorizing
// that ended original. The producer receipt, rather than Active, proves it.
// MUTATION: allowing approval of an ended binding accepts the canceled original.
func TestAuthAssignmentApprovalCancellationCannotReopen(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	receipt := request.AccountingAdmission()
	approval, err := request.ClaimAccountingAssignment()
	if err != nil {
		t.Fatal(err)
	}
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	if !assigned.Cancel() {
		t.Fatal("original assignment cancellation was refused")
	}
	assigned.Acknowledge()
	if approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
		t.Fatal("local approval reopened a canceled original")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := receipt.Assignment(ctx); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("canceled approved assignment = %v", err)
	}
}

// Only one genuine unanswered request can claim. Wrong typed birth/epoch
// approvals refuse without consuming the useful original's token.
// MUTATION: removing the typed Birth comparison accepts the foreign approval.
func TestAuthAssignmentApprovalClaimBindsExactOriginal(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	forged := request
	forged.SubscriberID = "another opaque subscriber"
	if _, err := forged.ClaimAccountingAssignment(); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("forged request claim = %v", err)
	}
	approval, err := request.ClaimAccountingAssignment()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request.ClaimAccountingAssignment(); !errors.Is(err, ErrAuthAssignmentClaimed) {
		t.Fatalf("second claimant = %v", err)
	}
	acceptAuthAdmissionRequest(t, session, request)
	if _, err := request.ClaimAccountingAssignment(); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("post-answer claimant = %v", err)
	}
	assigned := prepareAuthAdmissionIPCP(t, session)
	foreign, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	if approval.Approve(foreign, assigned.NetworkLifetime) {
		t.Fatal("foreign birth approved the original")
	}
	next, err := assigned.NetworkLifetime.Next()
	if err != nil {
		t.Fatal(err)
	}
	if approval.Approve(assigned.Birth, next) {
		t.Fatal("foreign network approved the original")
	}
	if (AuthAssignmentApproval{}).Approve(assigned.Birth, assigned.NetworkLifetime) {
		t.Fatal("zero token approved the original")
	}
	if !approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
		t.Fatal("wrong-input refusal consumed useful original approval")
	}
	assigned.Acknowledge()
	session.requireUp(t)
	frameLeaseEcho(t, session.peer)
}

// A capacity refusal after Claim still rejects the actual auth round. No
// assigned worker or Start is possible; its original unused lease may end.
func TestAuthAssignmentApprovalRejectedClaimNeverTransfers(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	receipt := request.AccountingAdmission()
	approval, err := request.ClaimAccountingAssignment()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.driver.AuthResponse(request, false, "accounting capacity exhausted", nil); err != nil {
		t.Fatal(err)
	}
	event := waitForEvent(t, session.driver.EventsOut(), time.Second)
	if _, ok := event.(EventSessionDown); !ok {
		t.Fatalf("rejected claimed admission published %T", event)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := receipt.Assignment(ctx); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("rejected claimed admission = %v", err)
	}
	if approval.Approve(session.birth, l2tpevents.NetworkLifetime{}) {
		t.Fatal("rejected original token authorized an assignment")
	}
	if _, prepared := receipt.PreparedAssignment(); prepared {
		t.Fatal("rejected original manufactured a prepared network")
	}
}

// An installation without an accounting claimant keeps the original actual
// IPCP ACK contract and permits useful traffic without acquiring a token.
func TestAuthAssignmentApprovalNoClaimKeepsUsefulIPCP(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	assigned.Acknowledge()
	session.requireUp(t)
	frameLeaseEcho(t, session.peer)
}

// PPPoE's actual producer never offers this L2TP-only claim. Its genuine IPCP
// assignment still reaches Up and answers Echo using the existing ACK alone.
func TestAuthAssignmentApprovalPPPoERemainsUnclaimed(t *testing.T) {
	session, request := startAuthAdmissionDriver(t, func(start *StartSession) {
		start.PPPoE = true
		start.L2TPBirth = subscriberid.L2TPBirth{}
	})
	if request.AccountingAdmission().IsValid() {
		t.Fatal("PPPoE acquired an L2TP accounting admission")
	}
	if _, err := request.ClaimAccountingAssignment(); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("PPPoE accounting claim = %v", err)
	}
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	assigned.Acknowledge()
	session.requireUp(t)
	frameLeaseEcho(t, session.peer)
}

// A controlled deadline at the real producer binding pins the final commit
// race: both useful signals are ready, but an expired binding must not commit.
// The separate missing-owner root proves the actual timer reaches this abort.
// MUTATION: removing finish's deadline fence commits the expired ready pair.
func TestAuthAssignmentApprovalExpiredBindingCannotCommitReadySignals(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	receipt := request.AccountingAdmission()
	connects := authApprovalConnects(t, session)
	approval, err := request.ClaimAccountingAssignment()
	if err != nil {
		t.Fatal(err)
	}
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	if !approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
		t.Fatal("useful original approval was refused")
	}
	receipt.binding.mu.Lock()
	receipt.binding.approval.deadline = time.Now().Add(-time.Nanosecond)
	receipt.binding.mu.Unlock()
	if approval.Approve(assigned.Birth, assigned.NetworkLifetime) {
		t.Fatal("expired original accepted another approval before ACK")
	}
	assigned.Acknowledge()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := receipt.Assignment(ctx); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("ready signals committed expired original: %v", err)
	}
	if connects.Load() != 0 {
		t.Fatal("expired original connected forwarding")
	}
}

// Install while actual auth is pending, before native Connect is permitted.
// The existing fake handle's connection mutex protects both hook and use.
func authApprovalConnects(t *testing.T, session *l2tpBirthProducerSession) *atomic.Int32 {
	t.Helper()
	counter, ok := session.control.worker.dataplane.(*sessionLeaseCounter)
	if !ok {
		t.Fatalf("session dataplane = %T, want *sessionLeaseCounter", session.control.worker.dataplane)
	}
	handle, ok := counter.SessionHandle.(*testSessionHandle)
	if !ok {
		t.Fatalf("session handle = %T, want *testSessionHandle", counter.SessionHandle)
	}
	connects := &atomic.Int32{}
	handle.connectionMu.Lock()
	connect := handle.ops.connect
	handle.ops.connect = func(channel, unit int) error {
		connects.Add(1)
		return connect(channel, unit)
	}
	handle.connectionMu.Unlock()
	return connects
}
