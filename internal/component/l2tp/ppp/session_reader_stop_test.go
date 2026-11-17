// Design: docs/research/l2tpv2-ze-integration.md -- session reader ownership at shutdown

package ppp

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// StopSession must join the control reader when its frame queue is full.
// A full authentication output blocks the real session before it can drain
// frames, and five acknowledged reads fill the four-slot queue plus its sender.
func TestDriverStopSessionJoinsFullControlQueue(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	stopDone := make(chan error, 1)
	stopStarted := false
	t.Cleanup(func() {
		// Whole-driver stop also releases a reader stranded by the old code,
		// so a regression fails this test without stranding its helper.
		driver.Stop()
		if stopStarted {
			<-stopDone
		}
	})
	for range cap(driver.authEventsOut) {
		driver.authEventsOut <- eventAuthSuccess{}
	}
	start, control, _ := fullControlQueueStart()
	driver.SessionsIn() <- start
	openedEvent, opened := waitForEventOfType[EventLCPUp](t, driver.EventsOut(), time.Second)
	if !opened {
		t.Fatal("session did not reach the blocked authentication publication")
	}
	control.fill(t)
	stopStarted = true
	go func() { stopDone <- driver.StopSession(openedEvent.Control) }()
	select {
	case err := <-stopDone:
		stopStarted = false
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("StopSession did not join its control reader with a full frame queue")
	}
}

// A natural activation failure must also join the full control queue, even
// though neither the per-session nor whole-driver stop signal was requested.
func TestDriverActivationFailureJoinsFullControlQueue(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, control, handle := fullControlQueueStart()
	failure := errors.New("activation failed while the control queue was full")
	owner := &fullControlQueueOwner{
		SessionHandle: handle,
		activation:    make(chan struct{}),
		release:       make(chan struct{}),
		failure:       failure,
	}
	start.Dataplane = owner
	testRebindSessionAttempt(t, &start)
	var release sync.Once
	unblock := func() { release.Do(func() { close(owner.release) }) }
	t.Cleanup(func() {
		unblock()
		driver.Stop()
	})
	driver.SessionsIn() <- start
	select {
	case <-owner.activation:
	case <-time.After(time.Second):
		t.Fatal("session did not reach its controlled activation")
	}
	openedEvent, opened := waitForEventOfType[EventLCPUp](t, driver.EventsOut(), time.Second)
	if !opened {
		t.Fatal("no observed LCP-up capability for the activating session")
	}
	driver.mu.Lock()
	session, _ := driver.sessionForControlLocked(openedEvent.Control)
	driver.mu.Unlock()
	if session == nil {
		t.Fatal("activated session has no admitted owner")
	}
	control.fill(t)
	unblock()
	ended, found := waitForEventOfType[EventSessionDown](t, driver.EventsOut(), time.Second)
	if !found {
		t.Fatal("activation failure did not publish session down")
	}
	if !strings.Contains(ended.Reason, failure.Error()) {
		t.Fatalf("session-down reason %q lost activation failure", ended.Reason)
	}
	select {
	case <-session.done:
	case <-time.After(time.Second):
		t.Fatal("natural session exit did not join its control reader with a full frame queue")
	}
	if _, present := driver.sessionForControl(openedEvent.Control); present {
		t.Fatal("natural exit retained the ended session admission")
	}
}

// The fixture supplies only a borrowed control stream, so shutdown must wake
// its queued sender rather than relying on a kernel unit or a second reader.
func fullControlQueueStart() (StartSession, *fullControlQueue, *testSessionHandle) {
	control := &fullControlQueue{
		frames: make(chan struct{}),
		read:   make(chan struct{}, 1),
		closed: make(chan struct{}),
	}
	operations, _, _ := newFakeOps()
	handle := newTestSessionHandle(&fakeBackend{}, "full-control-queue")
	handle.ops = operations
	handle.owned = []io.Closer{control}
	start := StartSession{
		LCP: testLCPConfig(), TunnelID: 1, SessionID: 1,
		Frames: &testSessionFrames{control: control}, Dataplane: handle,
		LNSMode: true, DisableIPCP: true, DisableIPv6CP: true,
	}
	start.SubscriberID = "fixture:full-control-queue"
	attempt, err := NewSessionAttempt(start.SubscriberID, handle)
	if err != nil {
		panic("BUG: invalid full queue fixture admission: " + err.Error())
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	return start, control, handle
}

type fullControlQueue struct {
	frames    chan struct{}
	read      chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func (q *fullControlQueue) Read(buffer []byte) (int, error) {
	select {
	case <-q.frames:
		copy(buffer, []byte{0xc0, 0x21})
		q.read <- struct{}{}
		return 2, nil
	case <-q.closed:
		return 0, io.ErrClosedPipe
	}
}

func (*fullControlQueue) Write(frame []byte) (int, error) { return len(frame), nil }

func (q *fullControlQueue) Close() error {
	q.closeOnce.Do(func() { close(q.closed) })
	return nil
}

// fill MUST run while the session cannot consume frames. Each acknowledgment
// proves an actual Read; the fifth cannot enqueue behind the four pending ones.
func (q *fullControlQueue) fill(t *testing.T) {
	t.Helper()
	for range 5 {
		select {
		case q.frames <- struct{}{}:
		case <-time.After(time.Second):
			t.Fatal("control reader stopped before its frame queue was full")
		}
		select {
		case <-q.read:
		case <-time.After(time.Second):
			t.Fatal("control reader did not acknowledge its queued frame")
		}
	}
}

type fullControlQueueOwner struct {
	SessionHandle
	activation chan struct{}
	release    chan struct{}
	failure    error
}

func (o *fullControlQueueOwner) SetMTU(int) error {
	close(o.activation)
	<-o.release
	return o.failure
}
