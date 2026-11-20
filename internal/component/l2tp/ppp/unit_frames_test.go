// Design: internal/component/l2tp/ppp/session_run.go -- readUnitFrames, the unit descriptor reader
// Related: ncp_helpers_test.go -- newNCPTestDriverUnit and completeIPCP
//
// Goal: prove that an NCP control frame the kernel delivers on the PPP unit
// descriptor after PPPIOCCONNECT reaches the session's NCP automaton.
//
// Method: a net.Pipe stands in for the unit descriptor. The test opens LCP by
// proxy and IPCP over the channel pipe, waits for EventSessionUp (which the
// session sends only after its connect call), then writes IPCP packets into
// the unit pipe, never the channel, the way Linux ppp_input routes a frame
// whose protocol is below 0xc000 once the channel is attached to a unit
// (drivers/net/ppp/ppp_generic.c, v6.12 lines 2318 and 2472). Ze answers on
// the channel. With the unit unread the write into the pipe never completes
// and the test fails on its deadline.

package ppp

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// writeUnitNCPPacket writes one control packet for proto into the unit pipe,
// within a deadline, so an unread unit fails the test rather than hanging it.
func writeUnitNCPPacket(t *testing.T, unit net.Conn, proto uint16, code, id uint8, data []byte) {
	t.Helper()
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, proto, nil)
	off += WriteLCPPacket(buf, off, code, id, data)
	if err := unit.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("unit SetWriteDeadline: %v", err)
	}
	if _, err := unit.Write(buf[:off]); err != nil {
		t.Fatalf("ze never read the IPCP frame the kernel queued on the unit descriptor: %v", err)
	}
}

// TestUnitDescriptorNCPFramesAreHandled drives IPCP to Opened, then delivers
// an IPCP packet with the unused Code 12 and an IPCP Terminate-Request on the
// unit descriptor, and requires the Code-Reject and the Terminate-Ack that
// handleIPCPPacket writes for them.
func TestUnitDescriptorNCPFramesAreHandled(t *testing.T) {
	driverUnit, peerUnit := net.Pipe()
	t.Cleanup(func() { _ = peerUnit.Close() }) //nolint:errcheck // test cleanup

	td := newNCPTestDriverUnit(t, &StartSession{DisableIPv6CP: true}, autoAcceptIP, discardLogger(), driverUnit)
	td.completeIPCP(t)
	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp, so the session never connected the channel to the unit")
	}

	// RFC 1332 Section 2: "Other Codes should be treated as unrecognized and
	// should result in Code-Rejects."
	writeUnitNCPPacket(t, peerUnit, ProtoIPCP, unusedNCPCode, 0x42, []byte{1, 2, 3, 4})
	reject := td.readPeerNCPPacket(t, ProtoIPCP)
	if reject.Code != LCPCodeReject {
		t.Fatalf("answer to IPCP Code 12 on the unit has Code %d, want Code-Reject", reject.Code)
	}
	if len(reject.Data) < 2 {
		t.Fatalf("Code-Reject carries %d octets, no rejected packet", len(reject.Data))
	}
	if reject.Data[0] != unusedNCPCode {
		t.Fatalf("Code-Reject rejects Code %d, want %d", reject.Data[0], unusedNCPCode)
	}

	// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
	// Terminate-Ack MUST be transmitted."
	writeUnitNCPPacket(t, peerUnit, ProtoIPCP, LCPTerminateRequest, 0x77, nil)
	ack := td.readPeerNCPPacket(t, ProtoIPCP)
	if ack.Code != LCPTerminateAck {
		t.Fatalf("answer to IPCP Terminate-Request on the unit has Code %d, want Terminate-Ack", ack.Code)
	}
	if ack.Identifier != 0x77 {
		t.Fatalf("Terminate-Ack Identifier = 0x%02x, want the request's 0x77", ack.Identifier)
	}
}

// scriptedUnit is a unit descriptor whose every Read reports its call on
// entered, then returns the next result the test sends on results.
type scriptedUnit struct {
	entered chan struct{}
	results chan unitRead
}

type unitRead struct {
	frame []byte
	err   error
}

func newScriptedUnit() *scriptedUnit {
	return &scriptedUnit{entered: make(chan struct{}, 1), results: make(chan unitRead)}
}

func (u *scriptedUnit) Read(buf []byte) (int, error) {
	u.entered <- struct{}{}
	r := <-u.results
	return copy(buf, r.frame), r.err
}

func (u *scriptedUnit) Close() error { return nil }

// readEntered reports whether the reader is inside a Read, once every
// goroutine in the bubble has blocked.
func (u *scriptedUnit) readEntered() bool {
	synctest.Wait()
	select {
	case <-u.entered:
		return true
	default:
		return false
	}
}

// TestReadUnitFramesSleepsUntilEachConnect drives readUnitFrames over a
// scripted unit: it reads nothing before the first wake, forwards a frame,
// signals unitIdle and sleeps on EOF (the disconnect resetNCP makes) without
// reading again, reads again after the next wake, and ends on a read error by
// sending it on done and closing unitGone.
func TestReadUnitFramesSleepsUntilEachConnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		unit := newScriptedUnit()
		s := &pppSession{
			unitFile: unit, unitConnected: make(chan struct{}, 1),
			unitIdle: make(chan struct{}, 1), unitGone: make(chan struct{}), logger: discardLogger(),
		}
		out := make(chan []byte, 4)
		done := make(chan error, 1)
		stop := make(chan struct{})
		defer close(stop)
		go s.readUnitFrames(out, done, stop)

		if unit.readEntered() {
			t.Fatal("the unit was read before any connect")
		}
		for connect := byte(1); connect <= 2; connect++ {
			s.wakeUnitReader()
			if !unit.readEntered() {
				t.Fatalf("connect %d: the unit was not read", connect)
			}
			unit.results <- unitRead{frame: []byte{0x80, 0x21, 5, connect, 0, 4}}
			if frame := <-out; frame[3] != connect {
				t.Fatalf("connect %d: forwarded % x", connect, frame)
			}
			if !unit.readEntered() {
				t.Fatalf("connect %d: the reader stopped after one frame", connect)
			}
			unit.results <- unitRead{err: io.EOF}
			if unit.readEntered() {
				t.Fatalf("connect %d: the reader read again after EOF, before the next connect", connect)
			}
			select {
			case <-s.unitIdle:
			default:
				t.Fatalf("connect %d: EOF did not signal unitIdle", connect)
			}
		}

		s.wakeUnitReader()
		if !unit.readEntered() {
			t.Fatal("the unit was not read after the third wake")
		}
		unit.results <- unitRead{err: errors.New("unit gone")}
		synctest.Wait()
		select {
		case err := <-done:
			if err == nil || err.Error() != "unit gone" {
				t.Fatalf("done carried %v, want the read error", err)
			}
		default:
			t.Fatal("a unit read error was not reported on done")
		}
		select {
		case <-s.unitGone:
		default:
			t.Fatal("unitGone stayed open after the reader returned")
		}
	})
}

// kernelUnit behaves as a Linux PPP unit's read queue does: Read returns a
// queued frame whether or not a channel is connected, blocks while the queue
// is empty and a channel is connected, and answers EOF once the queue is empty
// with no channel (ppp_read, drivers/net/ppp/ppp_generic.c).
type kernelUnit struct {
	mu        sync.Mutex
	cond      *sync.Cond
	queue     [][]byte
	connected bool
	closed    bool
}

func newKernelUnit() *kernelUnit {
	u := &kernelUnit{}
	u.cond = sync.NewCond(&u.mu)
	return u
}

func (u *kernelUnit) Read(buf []byte) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for len(u.queue) == 0 && u.connected && !u.closed {
		u.cond.Wait()
	}
	if u.closed {
		return 0, net.ErrClosed
	}
	if len(u.queue) == 0 {
		return 0, io.EOF
	}
	frame := u.queue[0]
	u.queue = u.queue[1:]
	return copy(buf, frame), nil
}

func (u *kernelUnit) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.closed = true
	u.cond.Broadcast()
	return nil
}

// setConnected moves the unit's channel, queueing first what the peer sent
// just before the change.
func (u *kernelUnit) setConnected(connected bool, queued ...[]byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.queue = append(u.queue, queued...)
	u.connected = connected
	u.cond.Broadcast()
}

func ipcpTerminateRequest(id uint8) []byte {
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, ProtoIPCP, nil)
	off += WriteLCPPacket(buf, off, LCPTerminateRequest, id, nil)
	return buf[:off]
}

// TestReconnectDiscardsFramesOfTheEndedLifetime brings a session up through
// afterLCPOpen, makes resetNCP disconnect it while six IPCP Terminate-Requests
// the peer sent just before sit in the unit's queue (more than unitFramesIn
// holds, so some are still in the kernel when the next connect comes), runs
// afterLCPOpen again, and requires the first frame the reconnected unit
// delivers to be the one the peer sent after it, never one of the six.
func TestReconnectDiscardsFramesOfTheEndedLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, _ := newRFC1661Session(LCPStateOpened)
		unit := newKernelUnit()
		s.unitFile = unit
		testHandleFor(s).ops.connect = func(int, int) error { unit.setConnected(true); return nil }
		stale := make([][]byte, 6)
		for i := range stale {
			stale[i] = ipcpTerminateRequest(0x60 + uint8(i))
		}
		testHandleFor(s).ops.disconnect = func(int) error { unit.setConnected(false, stale...); return nil }

		frames := make(chan []byte, 4)
		done := make(chan error, 1)
		stop := make(chan struct{})
		s.unitFramesIn = frames
		s.unitConnected = make(chan struct{}, 1)
		s.unitIdle = make(chan struct{}, 1)
		s.unitGone = make(chan struct{})
		var reader sync.WaitGroup
		reader.Go(func() { s.readUnitFrames(frames, done, stop) })
		t.Cleanup(func() {
			close(stop)
			_ = unit.Close() //nolint:errcheck // test cleanup
			reader.Wait()
		})

		stopFirstReply := replyNextAuthRequest(t, s, authResponseMsg{accept: true})
		if !s.afterLCPOpen() {
			t.Fatal("the first afterLCPOpen failed")
		}
		stopFirstReply()
		if !s.resetNCP() {
			t.Fatal("resetNCP failed")
		}
		// Authentication must not consume the old unit backlog before the
		// reconnect drain sees it. The actual auth-success event marks that wait's
		// end; runNCPPhase must acquire mu before it can reach awaitUnitDrained.
		s.unitFramesIn = nil
		authEvents := make(chan AuthEvent, 1)
		s.authEventsOut = authEvents
		secondResult, secondDone := make(chan bool, 1), make(chan struct{})
		go func() { defer close(secondDone); secondResult <- s.afterLCPOpen() }()
		t.Cleanup(func() {
			s.sessStopOnce.Do(func() { close(s.sessStop) })
			<-secondDone
		})
		var auth EventAuthRequest
		select {
		case raw := <-authEvents:
			var ok bool
			auth, ok = raw.(EventAuthRequest)
			if !ok {
				t.Fatalf("second authentication event %T, want actual request", raw)
			}
		case <-secondDone:
			t.Fatal("second Opened phase ended before its authentication request")
		}
		// All goroutines are now blocked: awaitAuthDecision has captured its nil
		// unit input and is waiting for this exact answer, not reading stale frames.
		synctest.Wait()
		s.mu.Lock()
		if err := auth.answer(authResponseMsg{accept: true}); err != nil {
			s.mu.Unlock()
			t.Fatalf("answer second authentication request: %v", err)
		}
		select {
		case raw := <-authEvents:
			if _, ok := raw.(eventAuthSuccess); !ok {
				s.mu.Unlock()
				t.Fatalf("second authentication result %T, want success", raw)
			}
		case <-secondDone:
			s.mu.Unlock()
			t.Fatal("second Opened phase ended before authentication completed")
		}
		s.unitFramesIn = frames
		s.mu.Unlock()
		if !<-secondResult {
			t.Fatal("the second afterLCPOpen failed")
		}

		fresh := ipcpTerminateRequest(0x77)
		unit.setConnected(true, fresh)
		select {
		case frame := <-frames:
			if frame[3] != 0x77 {
				t.Fatalf("the reconnected unit delivered Terminate-Request 0x%02x from the ended NCP lifetime", frame[3])
			}
			putFrameBuf(frame)
		case <-time.After(2 * time.Second):
			t.Fatal("the reconnected unit delivered nothing")
		}
	})
}

// failingUnit blocks every Read until fail closes or the unit closes, then
// answers an error.
type failingUnit struct {
	fail chan struct{}
	once sync.Once
}

func (u *failingUnit) Read([]byte) (int, error) {
	<-u.fail
	return 0, errors.New("unit read failed")
}

func (u *failingUnit) Close() error {
	u.once.Do(func() { close(u.fail) })
	return nil
}

// TestUnitReadErrorEndsTheSession opens IPCP, fails the unit's read, and
// requires EventSessionDown naming the unit read error: without the unit, the
// session could no longer answer IPCP.
func TestUnitReadErrorEndsTheSession(t *testing.T) {
	unit := &failingUnit{fail: make(chan struct{})}
	td := newNCPTestDriverUnit(t, &StartSession{DisableIPv6CP: true}, autoAcceptIP, discardLogger(), unit)
	td.completeIPCP(t)
	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
	}
	_ = unit.Close() //nolint:errcheck // fails the read; the session's own Close is then a no-op
	down, ok := waitForEventOfType[EventSessionDown](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("a unit read error did not end the session")
	}
	if !strings.HasPrefix(down.Reason, "unit fd read error: ") {
		t.Fatalf("EventSessionDown reason %q does not name the unit read error", down.Reason)
	}
}

// TestUnknownProtocolOnUnitFDProtocolRejected proves AC-9: once the session
// is up, a CCP Configure-Request (Protocol 0x80fd, which Ze does not
// implement) that the kernel queues on the unit descriptor is read from the
// unit and answered on the channel with an LCP Protocol-Reject whose
// Rejected-Protocol is 0x80fd and whose Rejected-Information is the CCP
// packet. Linux ppp_receive_nonmp_frame queues a protocol with no
// network-protocol index on the unit ("control or unknown frame - pass it to
// pppd"), and RFC 1661 Section 3.6 asks the answer: "While LCP is in the
// Opened state, any protocol packet which is unsupported by the
// implementation MUST be returned in a Protocol-Reject (described later)."
// The channel-side answer is rfc1661_protocol_reject_test.go's; this test
// proves the unit-side route into it, a proof of landed behavior.
//
// MUTATION: make the unitFramesIn arm of run (session_run.go) return the
// frame to the pool without calling handleFrame: the unit is still read, so
// the write succeeds, but no Protocol-Reject reaches the channel and the read
// fails on its deadline.
func TestUnknownProtocolOnUnitFDProtocolRejected(t *testing.T) {
	driverUnit, peerUnit := net.Pipe()
	t.Cleanup(func() { _ = peerUnit.Close() }) //nolint:errcheck // test cleanup

	td := newNCPTestDriverUnit(t, &StartSession{DisableIPv6CP: true}, autoAcceptIP, discardLogger(), driverUnit)
	td.completeIPCP(t)
	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp, so the session never connected the channel to the unit")
	}

	options := []byte{0x11, 0x04, 0x00, 0x01}
	writeUnitNCPPacket(t, peerUnit, ProtoCCP, LCPConfigureRequest, 0x33, options)
	reject := td.readPeerNCPPacket(t, ProtoLCP)
	if reject.Code != LCPProtocolReject {
		t.Fatalf("answer to CCP on the unit has LCP Code %d, want Protocol-Reject", reject.Code)
	}
	// Rejected-Protocol, then the CCP packet: Code, Identifier, Length, options.
	want := append([]byte{0x80, 0xfd, LCPConfigureRequest, 0x33, 0x00, byte(4 + len(options))}, options...)
	if !bytes.Equal(reject.Data, want) {
		t.Fatalf("Protocol-Reject data = % x, want % x", reject.Data, want)
	}
}

// countedUnit is a kernelUnit that counts the reads in progress, so a test
// sees the unit reader inside a read and sees it leave.
type countedUnit struct {
	*kernelUnit
	reading atomic.Int32
}

func (u *countedUnit) Read(buf []byte) (int, error) {
	u.reading.Add(1)
	defer u.reading.Add(-1)
	return u.kernelUnit.Read(buf)
}

func (u *countedUnit) isClosed() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.closed
}

// runUnitReaderSession runs session through run with unit as its unit
// descriptor. The fake connect and disconnect move the unit's channel as
// PPPIOCCONNECT and PPPIOCDISCONN do, and the session owner closes the unit
// at teardown as the kernel handle does. stop ends the session as Driver.Stop
// does; done closes once run has returned.
func runUnitReaderSession(t *testing.T, session *pppSession, unit *countedUnit) (net.Conn, func(), <-chan struct{}) {
	t.Helper()
	server, peer := net.Pipe()
	session.chanFile = server
	session.unitFile = unit
	session.frames = &testSessionFrames{control: server, opened: unit}
	owner := testHandleFor(session)
	owner.owned = append(owner.owned, server, unit)
	owner.ops.connect = func(int, int) error { unit.setConnected(true); return nil }
	owner.ops.disconnect = func(int) error { unit.setConnected(false); return nil }
	stopCh := make(chan struct{})
	session.stopCh = stopCh
	var once sync.Once
	stop := func() { once.Do(func() { close(stopCh) }) }
	stopRelay := relayLifecycleEvents(session)
	done := make(chan struct{})
	go func() {
		defer close(done)
		session.run(&StartSession{})
	}()
	t.Cleanup(func() {
		stop()
		_ = peer.Close() //nolint:errcheck // test cleanup
		<-done
		stopRelay()
	})
	return peer, stop, done
}

// awaitUnitReaderExit requires run to return after a stop, with the unit
// reader goroutine already gone, no unit read in progress, and the unit
// closed. run joins both readers before it returns, so the reader's own exit
// signal, unitGone, is checked with no wait: a reader run did not join can
// still be running here.
func awaitUnitReaderExit(t *testing.T, session *pppSession, unit *countedUnit, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after the stop: it is still joining a reader")
	}
	select {
	case <-session.unitGone:
	default:
		t.Fatal("run returned while the unit reader goroutine was still running")
	}
	if n := unit.reading.Load(); n != 0 {
		t.Fatalf("%d unit reads still in progress after the session ended", n)
	}
	if !unit.isClosed() {
		t.Fatal("the session ended without closing its unit descriptor")
	}
}

// TestUnitReaderExitsOnClose proves AC-11 and A-2 through run: the session's
// one unit reader ends with the session, whether the session ends before the
// unit was ever connected, while the reader is blocked in a unit read, or
// after LCP went down and the disconnect put the reader back to sleep. LCP
// going down alone does not end the reader: the peer's Terminate-Request
// leaves the session waiting in Stopped for a new Configure-Request (RFC 1661
// Section 4.3, RTR), so the reader leaves its read on the disconnect's EOF
// and sleeps until the next connect, and it exits when that session ends.
// synctest.Test fails a bubble whose goroutines stay blocked after the test
// returns, so no goroutine of the session outlives it.
//
// MUTATION: remove the stop arm from the connect wait in readUnitFrames
// (session_run.go): the reader never leaves that wait, run blocks joining it,
// and the first and third subtests fail on their deadline.
// MUTATION: remove the closeFiles call from run's teardown (session_run.go):
// the unit stays open, the reader stays in its read, and the second subtest
// fails on its deadline.
func TestUnitReaderExitsOnClose(t *testing.T) {
	t.Run("session ends before the unit connects", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			session, _, _ := newRFC1661Session(LCPStateInitial)
			authEvents, _ := prepareIPCPLifecycle(session)
			unit := &countedUnit{kernelUnit: newKernelUnit()}
			peer, stop, done := runUnitReaderSession(t, session, unit)
			openLifecycleLCP(t, peer)
			// Authentication is never answered, so the channel is never
			// connected and the reader waits for its first wake.
			awaitAuthRequest(t, authEvents)
			synctest.Wait()
			if n := unit.reading.Load(); n != 0 {
				t.Fatalf("%d unit reads before any connect", n)
			}
			stop()
			awaitUnitReaderExit(t, session, unit, done)
		})
	})
	t.Run("session ends during a unit read", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			session, _, events := newRFC1661Session(LCPStateInitial)
			authEvents, ips := prepareIPCPLifecycle(session)
			unit := &countedUnit{kernelUnit: newKernelUnit()}
			peer, stop, done := runUnitReaderSession(t, session, unit)
			openLifecycleIPCP(t, peer, negotiateIPCPLifecycle(t, peer, authEvents, ips), events)
			synctest.Wait()
			if n := unit.reading.Load(); n != 1 {
				t.Fatalf("%d unit reads in progress after session-up, want the reader blocked in one", n)
			}
			stop()
			awaitUnitReaderExit(t, session, unit, done)
		})
	})
	t.Run("LCP goes down, then the session ends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			session, _, events := newRFC1661Session(LCPStateInitial)
			authEvents, ips := prepareIPCPLifecycle(session)
			unit := &countedUnit{kernelUnit: newKernelUnit()}
			peer, stop, done := runUnitReaderSession(t, session, unit)
			openLifecycleIPCP(t, peer, negotiateIPCPLifecycle(t, peer, authEvents, ips), events)
			synctest.Wait()
			if n := unit.reading.Load(); n != 1 {
				t.Fatalf("%d unit reads in progress after session-up, want the reader blocked in one", n)
			}
			writeLifecyclePacket(t, peer, ProtoLCP, LCPTerminateRequest, 0x61, nil)
			if ack := readLifecyclePacket(t, peer, ProtoLCP, LCPTerminateAck); ack.Identifier != 0x61 {
				t.Fatalf("LCP Terminate-Ack Identifier = %#02x, want the request's 0x61", ack.Identifier)
			}
			synctest.Wait()
			if n := unit.reading.Load(); n != 0 {
				t.Fatalf("%d unit reads in progress after LCP down disconnected the unit, want none", n)
			}
			select {
			case <-done:
				t.Fatal("the session ended on the peer's LCP Terminate-Request, want it waiting for a new Configure-Request")
			default:
			}
			stop()
			awaitUnitReaderExit(t, session, unit, done)
		})
	})
}
