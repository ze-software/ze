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
	"errors"
	"io"
	"net"
	"strings"
	"sync"
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
	s, _, _ := newRFC1661Session(LCPStateOpened)
	unit := newKernelUnit()
	s.unitFile = unit
	s.ops.connect = func(int, int) error { unit.setConnected(true); return nil }
	stale := make([][]byte, 6)
	for i := range stale {
		stale[i] = ipcpTerminateRequest(0x60 + uint8(i))
	}
	s.ops.disconnect = func(int) error { unit.setConnected(false, stale...); return nil }

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

	s.authRespCh <- authResponseMsg{accept: true}
	if !s.afterLCPOpen() {
		t.Fatal("the first afterLCPOpen failed")
	}
	if !s.resetNCP() {
		t.Fatal("resetNCP failed")
	}
	s.authRespCh <- authResponseMsg{accept: true}
	if !s.afterLCPOpen() {
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
