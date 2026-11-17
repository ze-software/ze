// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact lifecycle cancellation

package ppp

import (
	"testing"
	"testing/synctest"
)

// This preserves the held exact-cancellation prototype's actual NCP barrier
// proof. The direct protocol fixture supplies a worker control locally; minting
// at real admission is covered separately above. Cancel sends no ACK or Connect.
func TestIPAssignmentConsumerCancelsExactWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.control = SessionControl{worker: session}
		connects := 0
		testHandleFor(session).ops.connect = func(_, _ int) error { connects++; return nil }
		finish, exited, _ := startAssignmentNCP(t, session, events, AddressFamilyIPv4)
		finish()
		synctest.Wait()
		assigned, ok := (<-events).(EventSessionIPAssigned)
		if !ok {
			t.Fatal("NCP did not publish an assignment")
		}
		if assigned.Control != session.control {
			t.Fatal("assignment lost its exact control")
		}
		if !assigned.Cancel() {
			t.Fatal("production assignment has no exact-worker cancellation")
		}
		if !assigned.Cancel() {
			t.Fatal("exact-worker cancellation is not idempotent")
		}
		synctest.Wait()
		select {
		case <-exited:
		default:
			t.Fatal("canceled worker remained at the assignment barrier")
		}
		if connects != 0 {
			t.Fatalf("failed publication enabled forwarding %d times", connects)
		}
		select {
		case <-assigned.done:
			t.Fatal("cancellation sent a success acknowledgment")
		default:
		}
		select {
		case event := <-events:
			if _, ok := event.(EventSessionDown); !ok {
				t.Fatalf("canceled worker published %T", event)
			}
		default:
			t.Fatal("cancellation omitted the final teardown")
		}
	})
}

// The actual LCP withdrawal barrier must return failure after its consumer
// cancels the exact worker, without receiving a success acknowledgment.
func TestLCPWithdrawalConsumerCancelsExactWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateOpened)
		session.control = SessionControl{worker: session}
		session.ncpStarted = true
		stop := make(chan struct{})
		session.stopCh = stop
		exited := make(chan bool, 1)
		gone := make(chan struct{})
		go func() { defer close(gone); exited <- session.performAction(LCPActTLD, LCPPacket{}) }()
		t.Cleanup(func() { close(stop); <-gone })
		synctest.Wait()
		withdrawal, ok := (<-events).(EventLCPDown)
		if !ok {
			t.Fatal("LCP did not publish a withdrawal")
		}
		if withdrawal.Control != session.control {
			t.Fatal("withdrawal lost its exact control")
		}
		if !withdrawal.NetworkPhase {
			t.Fatal("withdrawal lost its network barrier")
		}
		if !withdrawal.Cancel() {
			t.Fatal("withdrawal has no exact-worker cancellation")
		}
		synctest.Wait()
		select {
		case success := <-exited:
			if success {
				t.Fatal("canceled network withdrawal permitted replacement authentication")
			}
		default:
			t.Fatal("canceled worker remained at the withdrawal barrier")
		}
		select {
		case <-withdrawal.done:
			t.Fatal("cancellation acknowledged successful cleanup")
		default:
		}
	})
}

// A captured withdrawal and assignment both cancel only their original worker;
// neither ACKs its barrier or removes an admission key.
func TestStaleLifecycleCancellationCannotReachReplacement(t *testing.T) {
	original := &pppSession{tunnelID: 1, sessionID: 2, sessStop: make(chan struct{})}
	replacement := &pppSession{tunnelID: 1, sessionID: 2, sessStop: make(chan struct{})}
	control := SessionControl{worker: original}
	assignment := EventSessionIPAssigned{Control: control, done: make(chan struct{})}
	withdrawal := EventLCPDown{Control: control, done: make(chan struct{})}
	if !assignment.Cancel() {
		t.Fatal("original assignment cancellation is absent")
	}
	if !withdrawal.Cancel() {
		t.Fatal("original withdrawal cancellation is absent")
	}
	select {
	case <-original.sessStop:
	default:
		t.Fatal("original worker was not canceled")
	}
	select {
	case <-replacement.sessStop:
		t.Fatal("stale event canceled replacement")
	default:
	}
	select {
	case <-assignment.done:
		t.Fatal("assignment cancellation acknowledged success")
	default:
	}
	select {
	case <-withdrawal.done:
		t.Fatal("withdrawal cancellation acknowledged success")
	default:
	}
}
