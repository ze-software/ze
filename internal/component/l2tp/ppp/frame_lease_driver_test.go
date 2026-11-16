// Design: docs/research/l2tpv2-ze-integration.md -- protocol sessions over borrowed frame channels

package ppp

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type sessionLeaseCounter struct {
	SessionHandle
	closes atomic.Int32
}

func (s *sessionLeaseCounter) Close() error {
	s.closes.Add(1)
	return s.SessionHandle.Close()
}

// The actual driver performs the complete LCP exchange through a borrowed
// stream, with no descriptor, kernel unit or second control reader supplied.
func TestDriverNegotiatesOnFrameChannelWithoutDescriptors(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, owner := frameLeaseStart(t, 1)
	peerDone := make(chan struct{})
	go scriptedPeer(t, peer, peerDone)
	t.Cleanup(func() {
		closeConn(peer)
		<-peerDone
	})
	driver.SessionsIn() <- start
	events := drainTwoEvents(t, driver.EventsOut(), time.Second)
	if _, opened := events[1].(EventSessionUp); !opened {
		t.Fatalf("events %v, want a completed PPP session", events)
	}
	if err := driver.StopSession(1, 1); err != nil {
		t.Fatal(err)
	}
	if owner.closes.Load() != 1 {
		t.Fatalf("session owner released %d times", owner.closes.Load())
	}
}

// A refused payload still transfers its valid owner. The driver must release
// that owner once and publish refusal before it ever reads or writes a frame.
func TestDriverRefusedSessionReleasesHandleOnce(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, owner := frameLeaseStart(t, 1)
	start.LCP = LCPConfig{}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	if _, refused := event.(EventSessionRejected); !refused {
		t.Fatalf("event %T, want rejected ownership transfer", event)
	}
	if owner.closes.Load() != 1 {
		t.Fatalf("refused owner released %d times", owner.closes.Load())
	}
	var frame [2]byte
	if _, err := peer.Read(frame[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("refused frame channel still live: %v", err)
	}
}

// A different incoming lease with a duplicate session key must close only that
// incoming lease. The admitted peer must still answer a fresh Echo-Request.
func TestDriverDuplicateLeaseKeepsAdmittedPeerAlive(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	first, peer, admitted := frameLeaseStart(t, 1)
	frameLeaseProxy(&first)
	driver.SessionsIn() <- first
	drainTwoEvents(t, driver.EventsOut(), time.Second)
	incoming, incomingPeer, refused := frameLeaseStart(t, 1)
	if err := incomingPeer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	driver.SessionsIn() <- incoming
	if event := waitForEvent(t, driver.EventsOut(), time.Second); event == nil {
		t.Fatal("no duplicate refusal")
	} else if _, rejected := event.(EventSessionRejected); !rejected {
		t.Fatalf("event %T, want duplicate refusal", event)
	}
	if refused.closes.Load() != 1 {
		t.Fatalf("incoming lease released %d times", refused.closes.Load())
	}
	if admitted.closes.Load() != 0 {
		t.Fatal("duplicate admission closed the existing session")
	}
	var frame [2]byte
	if _, err := incomingPeer.Read(frame[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("incoming duplicate remained open: %v", err)
	}
	frameLeaseEcho(t, peer)
}

// Requeueing the exact live owner is an invalid transfer, but refusing it must
// not close the already admitted session that still owns that same lease.
func TestDriverRequeuedLeaseKeepsAdmittedPeerAlive(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, owner := frameLeaseStart(t, 1)
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	drainTwoEvents(t, driver.EventsOut(), time.Second)
	driver.SessionsIn() <- start
	if event := waitForEvent(t, driver.EventsOut(), time.Second); event == nil {
		t.Fatal("no requeued-lease refusal")
	} else if _, rejected := event.(EventSessionRejected); !rejected {
		t.Fatalf("event %T, want requeued-lease refusal", event)
	}
	if owner.closes.Load() != 0 {
		t.Fatal("refusing a requeue closed the admitted lease")
	}
	frameLeaseEcho(t, peer)
}

// A requeued owner does not transfer ownership again, even when the payload's
// borrowed resources or negotiation parameters have become invalid. Refusal
// must preserve the original session's protocol progress.
func TestDriverMalformedRequeueKeepsAdmittedPeerAlive(t *testing.T) {
	cases := []struct {
		name   string
		change func(*StartSession)
	}{
		{"missing frames", func(start *StartSession) { start.Frames = nil }},
		{"missing control", func(start *StartSession) { start.Frames = &testSessionFrames{} }},
		{"invalid LCP", func(start *StartSession) { start.LCP = LCPConfig{} }},
		{"invalid MRU", func(start *StartSession) { start.MaxMRU = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driver := newSessionTestDriver()
			if err := driver.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Stop)
			start, peer, owner := frameLeaseStart(t, 1)
			frameLeaseProxy(&start)
			driver.SessionsIn() <- start
			drainTwoEvents(t, driver.EventsOut(), time.Second)
			tc.change(&start)
			driver.SessionsIn() <- start
			event := waitForEvent(t, driver.EventsOut(), time.Second)
			if _, rejected := event.(EventSessionRejected); !rejected {
				t.Fatalf("event %T, want malformed requeue refusal", event)
			}
			if owner.closes.Load() != 0 {
				t.Fatal("refusing malformed requeue closed the admitted lease")
			}
			frameLeaseEcho(t, peer)
		})
	}
}

// One live owner cannot back two protocol goroutines. Reusing it under a new
// key must refuse only the incoming request and preserve its admitted peer.
func TestDriverOwnerCannotBeAdmittedUnderAnotherKey(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, owner := frameLeaseStart(t, 1)
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	drainTwoEvents(t, driver.EventsOut(), time.Second)
	start.SessionID = 2
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	rejected, ok := event.(EventSessionRejected)
	if !ok || rejected.TunnelID != 1 || rejected.SessionID != 2 {
		t.Fatalf("event %+v, want refusal of incoming key (1,2)", event)
	}
	if owner.closes.Load() != 0 {
		t.Fatal("refusing another key closed the admitted lease")
	}
	frameLeaseEcho(t, peer)
}

// The transport may end only a refused key that was never admitted. Owner
// reuse alone must not grant an unrelated incoming key the active-key flag.
func TestDriverRejectionMarksOnlyAnActiveIncomingKey(t *testing.T) {
	cases := []struct {
		name      string
		sameKey   bool
		sameOwner bool
	}{
		{"same key same owner", true, true},
		{"same key distinct owner", true, false},
		{"different key same owner", false, true},
		{"different key distinct invalid owner", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driver := newSessionTestDriver()
			if err := driver.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Stop)
			start, peer, owner := frameLeaseStart(t, 1)
			frameLeaseProxy(&start)
			driver.SessionsIn() <- start
			drainTwoEvents(t, driver.EventsOut(), time.Second)
			incoming := start
			incomingOwner := owner
			if !tc.sameOwner {
				incoming, _, incomingOwner = frameLeaseStart(t, 1)
			}
			if !tc.sameKey {
				incoming.SessionID = 2
				if !tc.sameOwner {
					incoming.LCP = LCPConfig{}
				}
			}
			driver.SessionsIn() <- incoming
			event := waitForEvent(t, driver.EventsOut(), time.Second)
			rejected, ok := event.(EventSessionRejected)
			if !ok {
				t.Fatalf("event %T, want refused transfer", event)
			}
			if rejected.TunnelID != incoming.TunnelID || rejected.SessionID != incoming.SessionID || rejected.Reason == "" {
				t.Fatalf("refusal lost incoming identity or reason: %+v", rejected)
			}
			if rejected.ExistingSession != tc.sameKey {
				t.Fatalf("existing-session=%v, want active incoming key=%v", rejected.ExistingSession, tc.sameKey)
			}
			if owner.closes.Load() != 0 {
				t.Fatal("refusal closed the admitted owner")
			}
			if !tc.sameOwner && incomingOwner.closes.Load() != 1 {
				t.Fatalf("distinct incoming owner released %d times", incomingOwner.closes.Load())
			}
			frameLeaseEcho(t, peer)
		})
	}
}

// An old worker can finish after StopSession releases its key and the next
// session is admitted. That old exit must not delete its successor's entry.
func TestDriverOldWorkerExitKeepsReplacementSession(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, _ := frameLeaseStart(t, 1)
	blocked := &blockingSessionLease{
		SessionHandle: start.Dataplane,
		closeStarted:  make(chan struct{}),
		releaseClose:  make(chan struct{}),
	}
	var release sync.Once
	unblock := func() { release.Do(func() { close(blocked.releaseClose) }) }
	t.Cleanup(unblock)
	start.Dataplane = blocked
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	drainTwoEvents(t, driver.EventsOut(), time.Second)
	stopped := make(chan error, 1)
	stopDone := make(chan struct{})
	// This cleanup was registered after driver.Stop and therefore runs
	// first on every Fatal path. Release the controlled close and join its
	// caller before whole-driver cleanup can wait on the old worker.
	t.Cleanup(func() {
		unblock()
		<-stopDone
	})
	go func() {
		defer close(stopDone)
		stopped <- driver.StopSession(1, 1)
	}()
	select {
	case <-blocked.closeStarted:
	case <-time.After(time.Second):
		t.Fatal("old owner close did not begin")
	}
	replacement, peer, _ := frameLeaseStart(t, 1)
	frameLeaseProxy(&replacement)
	driver.SessionsIn() <- replacement
	if _, opened := waitForEventOfType[EventSessionUp](t, driver.EventsOut(), time.Second); !opened {
		t.Fatal("replacement did not finish negotiation")
	}
	unblock()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("old worker did not finish")
	}
	if _, found := driver.sessionByID(1, 1); !found {
		t.Fatal("old worker removed its replacement's session key")
	}
	frameLeaseEcho(t, peer)
}

type blockingSessionLease struct {
	SessionHandle
	closeStarted chan struct{}
	releaseClose chan struct{}
	closeOnce    sync.Once
	closeErr     error
}

func (s *blockingSessionLease) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeStarted)
		<-s.releaseClose
		s.closeErr = s.SessionHandle.Close()
	})
	return s.closeErr
}

// A type can be comparable while its current interface-valued member holds a
// slice. Admission must check the actual owner value before ever comparing it.
func TestDriverRefusesRuntimeIncomparableOwner(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, owner := frameLeaseStart(t, 1)
	start.Dataplane = runtimeIncomparableOwner{SessionHandle: owner, value: []byte{1}}
	driver.SessionsIn() <- start
	if event := waitForEvent(t, driver.EventsOut(), time.Second); event == nil {
		t.Fatal("no incomparable-owner refusal")
	} else if _, rejected := event.(EventSessionRejected); !rejected {
		t.Fatalf("event %T, want incomparable-owner refusal", event)
	}
	if owner.closes.Load() != 1 {
		t.Fatalf("refused noncomparable owner released %d times", owner.closes.Load())
	}
}

type runtimeIncomparableOwner struct {
	SessionHandle
	value any
}

// A typed nil is not a releasable owner. It must be refused before a method
// call, while the test still owns and closes the unrelated borrowed stream.
func TestDriverRefusesTypedNilOwnerBeforeMethods(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, _, owner := frameLeaseStart(t, 1)
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	start.Dataplane = (*sessionLeaseCounter)(nil)
	driver.SessionsIn() <- start
	if event := waitForEvent(t, driver.EventsOut(), time.Second); event == nil {
		t.Fatal("no typed-nil owner refusal")
	} else if _, rejected := event.(EventSessionRejected); !rejected {
		t.Fatalf("event %T, want typed-nil owner refusal", event)
	}
}

// Closing one session must preserve a different session's frame owner and
// protocol progress, rather than merely leaving a stale entry in the map.
func TestDriverStoppingLeaseKeepsOtherPeerAlive(t *testing.T) {
	driver := newSessionTestDriver()
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	first, _, ended := frameLeaseStart(t, 1)
	second, peer, survivor := frameLeaseStart(t, 2)
	frameLeaseProxy(&first)
	frameLeaseProxy(&second)
	driver.SessionsIn() <- first
	drainTwoEvents(t, driver.EventsOut(), time.Second)
	driver.SessionsIn() <- second
	drainTwoEvents(t, driver.EventsOut(), time.Second)
	if err := driver.StopSession(1, 1); err != nil {
		t.Fatal(err)
	}
	if ended.closes.Load() != 1 {
		t.Fatalf("stopped lease released %d times", ended.closes.Load())
	}
	if survivor.closes.Load() != 0 {
		t.Fatal("stopping one lease closed its neighbor")
	}
	frameLeaseEcho(t, peer)
}

func frameLeaseStart(t *testing.T, sid uint16) (StartSession, net.Conn, *sessionLeaseCounter) {
	t.Helper()
	control, peer := net.Pipe()
	t.Cleanup(func() { closeConn(peer) })
	ops, _, _ := newFakeOps()
	handle := newTestSessionHandle(&fakeBackend{}, "borrowed-control-session")
	handle.ops = ops
	handle.owned = []io.Closer{control}
	owner := &sessionLeaseCounter{SessionHandle: handle}
	return StartSession{
		LCP: testLCPConfig(), TunnelID: 1, SessionID: sid,
		Frames: &testSessionFrames{control: control}, Dataplane: owner,
		LNSMode: true, DisableIPCP: true, DisableIPv6CP: true,
	}, peer, owner
}

func frameLeaseProxy(start *StartSession) {
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	start.ProxyLCPInitialRecv = stream
	start.ProxyLCPLastSent = stream
	start.ProxyLCPLastRecv = stream
}

func frameLeaseEcho(t *testing.T, peer net.Conn) {
	t.Helper()
	var frame [64]byte
	n := WriteFrame(frame[:], 0, ProtoLCP, nil)
	n += WriteLCPEcho(frame[:], n, LCPEchoRequest, 0x71, 0xCAFEBABE, nil)
	if err := peer.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(frame[:n]); err != nil {
		t.Fatal(err)
	}
	proto, payload := readPeerFrame(t, peer)
	packet, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ProtoLCP || packet.Code != LCPEchoReply || packet.Identifier != 0x71 {
		t.Fatalf("surviving peer reply protocol=%x packet=%+v", proto, packet)
	}
}
