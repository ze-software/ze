// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting snapshots precede forwarding and owner release.
package ppp

import (
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

type accountingCloseTrace struct {
	trace  *sessionStopTrace
	closed *atomic.Bool
}

func (closer accountingCloseTrace) Close() error {
	closer.closed.Store(true)
	closer.trace.note("owner-close")
	return nil
}

// The actual session loop completes a real IPCP exchange. Its first original
// counter read must precede assignment acknowledgment/forwarding; terminal
// counters must follow forwarding disconnect but precede the same owner's Close.
// MUTATION: omit the first-assignment baseline or central pre-Close sample.
func TestPPPOriginalCounterCaptureBeforeForwardingAndClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		owner := testHandleFor(session)
		trace := &sessionStopTrace{}
		var closed atomic.Bool
		var raw atomic.Uint64
		raw.Store(100)
		owner.readCounters = func() (SessionCounters, error) {
			if closed.Load() {
				return SessionCounters{}, errors.New("original unit is closed")
			}
			if !session.mu.TryLock() {
				return SessionCounters{}, errors.New("counter read held the worker metadata lock")
			}
			session.mu.Unlock()
			trace.note("sample")
			return SessionCounters{RxBytes: raw.Load(), TxBytes: 200, RxPackets: 3, TxPackets: 5}, nil
		}
		owner.ops.connect = func(_, _ int) error { trace.note("connect"); return nil }
		owner.ops.disconnect = func(_ int) error { trace.note("disconnect"); return nil }
		owner.owned = append(owner.owned, accountingCloseTrace{trace: trace, closed: &closed})
		finish, exited, stop := startAssignmentNCP(t, session, events, AddressFamilyIPv4)
		finish()
		synctest.Wait()
		event := <-events
		assigned, ok := event.(EventSessionIPAssigned)
		if !ok {
			t.Fatalf("completed IPCP published %T, want assignment", event)
		}
		if got := trace.snapshot(); !reflect.DeepEqual(got, []string{"sample"}) {
			t.Fatalf("assignment baseline/forwarding order=%v, want one original sample before forwarding", got)
		}
		assigned.Acknowledge()
		awaitLifecycleUp(t, events)
		synctest.Wait()
		raw.Store(173)
		close(stop)
		synctest.Wait()
		select {
		case <-exited:
		default:
			t.Fatal("the actual PPP worker did not finish")
		}
		want := []string{"sample", "connect", "disconnect", "sample", "owner-close"}
		if got := trace.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("original lifetime sample/release order=%v, want %v", got, want)
		}
	})
}
