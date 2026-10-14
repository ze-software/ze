// Design: docs/architecture/ldp/mpls-ldp.md -- KeepAlive Timer expiry Notification
// Related: rfc5036_test.go -- rfcTestSession, readLDPPDU, runSessionForTest, encodeInitPDU
// Related: rfc5036_establishment_test.go -- expectClosed, expectSilence, awaitOperational
// Related: rfc5443_sessionup_test.go -- encodeKeepAlivePDU, sessionPipe
//
// These tests prove that ze tells its peer why a session ends when the KeepAlive
// Timer expires (RFC 5036 Sections 2.5.4, 3.5.1.1 and 3.5.1.2.3): one fatal
// Notification carrying KeepAlive Timer Expired, written before the transport
// closes, bounded in time, and never followed by another PDU.
//
// VALIDATES: AC-1 to AC-5 and AC-7 of spec-ldp-keepalive-expiry-notification.
// PREVENTS: a session that closes silently on KeepAlive expiry, a PDU written
// after a fatal Notification, and an expiry stuck behind a blocked KeepAlive.
package ldp

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// expiryNotificationWarn is the Warn line ze logs when the expiry Notification
// cannot be written.
const expiryNotificationWarn = "ldp: the keepalive timer expired notification could not be sent"

// expiryHoldTime is the KeepAlive Timer the ReadLoop unit tests arm, short so the
// expiry arrives at once.
const expiryHoldTime = 100 * time.Millisecond

// expiryWriteSlack is how far past notificationWriteTimeout a bounded write may end.
const expiryWriteSlack = 2 * time.Second

// expiryNotificationPDU is the exact PDU ze writes on expiry from a session whose
// LDP Identifier is 10.0.0.1:0 and whose first message ID is 1.
var expiryNotificationPDU = []byte{
	0x00, 0x01, 0x00, 0x1c, // PDU version 1, PDU Length 28
	0x0a, 0x00, 0x00, 0x01, 0x00, 0x00, // LSR ID 10.0.0.1, label space 0
	0x00, 0x01, 0x00, 0x12, // Notification, Message Length 18
	0x00, 0x00, 0x00, 0x01, // Message ID 1
	0x03, 0x00, 0x00, 0x0a, // Status TLV, length 10
	0x80, 0x00, 0x00, 0x14, // KeepAlive Timer Expired, E set, F clear
	0x00, 0x00, 0x00, 0x00, // Message ID referred to: none
	0x00, 0x00, // Message Type referred to: none
}

// logCapture collects slog text output. Safe for concurrent use.
type logCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *logCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

// count returns how many logged lines carry msg.
func (c *logCapture) count(msg string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Count(c.buf.String(), msg)
}

func (c *logCapture) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(c, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// expirySession builds a session named 10.0.0.1:0 toward 10.0.0.2:0 that
// proposes keepalive and logs into capture.
func expirySession(conn net.Conn, keepalive time.Duration, capture *logCapture) *Session {
	return NewSession(conn, SessionConfig{
		LocalLSRID:    [4]byte{10, 0, 0, 1},
		PeerLSRID:     [4]byte{10, 0, 0, 2},
		PeerAddr:      netip.MustParseAddr("10.0.0.2"),
		KeepaliveTime: keepalive,
	}, newLIB(), capture.logger())
}

// readRawPDU reads one whole PDU from conn and returns its octets.
func readRawPDU(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(ldpReadTimeout)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var hdr [ldpHeaderLen]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		t.Fatalf("read PDU header: %v", err)
	}
	pdu, err := decodePDUHeader(hdr[:])
	if err != nil {
		t.Fatalf("decodePDUHeader: %v", err)
	}
	raw := make([]byte, ldpHeaderLen+int(pdu.PDULength)-6)
	copy(raw, hdr[:])
	if _, err := io.ReadFull(conn, raw[ldpHeaderLen:]); err != nil {
		t.Fatalf("read PDU body: %v", err)
	}
	return raw
}

// readUntilNotification reads PDUs from remote, skipping ze's KeepAlives, and
// returns the first Notification PDU. Any other message fails the test.
func readUntilNotification(t *testing.T, remote net.Conn) []byte {
	t.Helper()
	const pduMax = 64 // far more KeepAlives than any expiry in these tests lets through
	for range pduMax {
		raw := readRawPDU(t, remote)
		msgHdr, err := decodeMessageHeader(raw[ldpHeaderLen:])
		if err != nil {
			t.Fatalf("decodeMessageHeader: %v", err)
		}
		if msgHdr.Type == MsgTypeKeepAlive {
			continue
		}
		if msgHdr.Type != MsgTypeNotification {
			t.Fatalf("message type = %#x, want Notification (%#x)", msgHdr.Type, MsgTypeNotification)
		}
		return raw
	}
	t.Fatalf("no Notification within %d PDUs", pduMax)
	return nil
}

// assertExpiryNotification checks every field of an expiry Notification PDU
// except its message ID, which depends on how many PDUs the session sent first.
func assertExpiryNotification(t *testing.T, raw []byte) {
	t.Helper()
	if len(raw) != len(expiryNotificationPDU) {
		t.Fatalf("PDU = % x (%d octets), want %d octets", raw, len(raw), len(expiryNotificationPDU))
	}
	if !bytes.Equal(raw[:14], expiryNotificationPDU[:14]) {
		t.Fatalf("PDU and message header = % x, want % x", raw[:14], expiryNotificationPDU[:14])
	}
	if !bytes.Equal(raw[18:], expiryNotificationPDU[18:]) {
		t.Fatalf("Status TLV = % x, want % x", raw[18:], expiryNotificationPDU[18:])
	}
}

// wireRecorder reads everything that arrives on conn until it closes.
type wireRecorder struct {
	mu   sync.Mutex
	data []byte
}

func recordWire(t *testing.T, conn net.Conn) *wireRecorder {
	t.Helper()
	r := &wireRecorder{}
	go func() {
		var buf [512]byte
		for {
			n, err := conn.Read(buf[:])
			r.mu.Lock()
			r.data = append(r.data, buf[:n]...)
			r.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return r
}

func (r *wireRecorder) octets() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.data...)
}

// awaitOctets waits until r holds at least n octets.
func (r *wireRecorder) awaitOctets(t *testing.T, n int) []byte {
	t.Helper()
	deadline := time.Now().Add(ldpReadTimeout)
	for {
		got := r.octets()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("peer received %d octets, want at least %d", len(got), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// startRunSession runs the production runSession on sess and returns a channel
// closed when it returns. The cleanup stops the session if the test did not.
func startRunSession(t *testing.T, sess *Session) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go runSession(t.Context(), slogutil.DiscardLogger(), sess, sess.lib, "10.0.0.2:0",
		newLDPFIB(nil, slogutil.DiscardLogger()), sessionUpEvent(testAdjacency(), "10.0.0.2:0"),
		func() { close(done) })
	t.Cleanup(func() {
		sess.Stop()
		select {
		case <-done:
		case <-time.After(ldpReadTimeout):
			t.Error("runSession did not exit after Stop")
		}
	})
	return done
}

// establishOneSecond plays the peer's half of the Initialization exchange with a
// KeepAlive Time of 1 s, then sends the KeepAlive that makes the session
// operational. Nothing more is sent by the peer after it.
func establishOneSecond(t *testing.T, sess *Session, remote net.Conn) {
	t.Helper()
	if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", hdr.Type)
	}
	if _, err := remote.Write(encodeInitPDU(ldpVersion, 1)); err != nil {
		t.Fatalf("write peer Initialization: %v", err)
	}
	if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeKeepAlive {
		t.Fatalf("reply to the peer Initialization = %#x, want KeepAlive", hdr.Type)
	}
	if _, err := remote.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write peer KeepAlive: %v", err)
	}
	awaitOperational(t, sess)
}

// runReadLoop runs ReadLoop on its own goroutine and returns its result channel.
func runReadLoop(sess *Session, onOperational func()) <-chan error {
	result := make(chan error, 1)
	go func() { result <- sess.ReadLoop(nil, nil, nil, onOperational) }()
	return result
}

// awaitReadLoop waits for ReadLoop's result for up to within.
func awaitReadLoop(t *testing.T, result <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(within):
		t.Fatalf("ReadLoop did not return within %v", within)
		return nil
	}
}

// Goal: on an operational session whose peer goes silent, the next and only PDU
// ze writes is the KeepAlive Timer Expired Notification, and ReadLoop reports the
// expiry. Method: bare ReadLoop over net.Pipe with a 100 ms KeepAlive Timer; the
// peer compares every octet of the PDU it reads.
//
// RFC requirement: RFC5036-2.5.4-3 positive -- when the KeepAlive Timer of an
// OPERATIONAL session expires, the one PDU ze writes is the fatal KeepAlive Timer
// Expired Notification (the state table's Shutdown msg), octet for octet, and
// ReadLoop returns errKeepaliveExpiry.
func TestReadLoopKeepaliveExpirySendsFatalNotification(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := expirySession(local, DefaultKeepaliveTime, &logCapture{})
	sess.state = StateOperational
	sess.holdTime = expiryHoldTime

	result := runReadLoop(sess, nil)
	raw := readRawPDU(t, remote)
	if !bytes.Equal(raw, expiryNotificationPDU) {
		t.Fatalf("PDU = % x, want % x", raw, expiryNotificationPDU)
	}
	if err := awaitReadLoop(t, result, ldpReadTimeout); !errors.Is(err, errKeepaliveExpiry) {
		t.Fatalf("ReadLoop = %v, want %v", err, errKeepaliveExpiry)
	}
}

// Goal: the KeepAlive Timer times out a silent session only. A peer that sends a
// KeepAlive inside every KeepAlive Time keeps the session OPERATIONAL, and the
// timer is still armed: once the peer stops, the expiry Notification follows.
// Method: bare ReadLoop over net.Pipe with a 300 ms KeepAlive Timer; the peer
// writes a KeepAlive every 50 ms for 900 ms while a reader collects ze's PDUs.
//
// RFC requirement: RFC5036-2.5.4-3 negative -- an OPERATIONAL session whose peer
// sends a KeepAlive inside every KeepAlive Time does not take the Timeout
// transition: for three times the timer ze writes no PDU and ReadLoop keeps
// running. Once the peer goes silent, the next PDU is the KeepAlive Timer Expired
// Notification and ReadLoop returns errKeepaliveExpiry.
func TestReadLoopKeepaliveTrafficRestartsTimer(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := expirySession(local, DefaultKeepaliveTime, &logCapture{})
	sess.state = StateOperational
	const hold = 300 * time.Millisecond
	const interval = hold / 6
	sess.holdTime = hold

	written := make(chan []byte, 4)
	go func() {
		defer close(written)
		for {
			var hdr [ldpHeaderLen]byte
			if _, err := io.ReadFull(remote, hdr[:]); err != nil {
				return
			}
			pdu, err := decodePDUHeader(hdr[:])
			if err != nil {
				return
			}
			raw := make([]byte, ldpHeaderLen+int(pdu.PDULength)-6)
			copy(raw, hdr[:])
			if _, err := io.ReadFull(remote, raw[ldpHeaderLen:]); err != nil {
				return
			}
			written <- raw
		}
	}()

	result := runReadLoop(sess, nil)
	keepalive := encodeKeepAlivePDU()
	for sent := time.Duration(0); sent < 3*hold; sent += interval {
		if _, err := remote.Write(keepalive); err != nil {
			t.Fatalf("write peer KeepAlive: %v", err)
		}
		select {
		case err := <-result:
			t.Fatalf("ReadLoop returned %v while the peer sent a KeepAlive every %v", err, interval)
		case raw := <-written:
			t.Fatalf("ze wrote % x while the peer sent a KeepAlive every %v", raw, interval)
		case <-time.After(interval):
		}
	}

	select {
	case raw, ok := <-written:
		if !ok {
			t.Fatal("the connection closed before ze wrote the expiry Notification")
		}
		if !bytes.Equal(raw, expiryNotificationPDU) {
			t.Fatalf("PDU = % x, want % x", raw, expiryNotificationPDU)
		}
	case <-time.After(ldpReadTimeout):
		t.Fatalf("no PDU within %v of the peer going silent", ldpReadTimeout)
	}
	if err := awaitReadLoop(t, result, ldpReadTimeout); !errors.Is(err, errKeepaliveExpiry) {
		t.Fatalf("ReadLoop = %v, want %v", err, errKeepaliveExpiry)
	}
}

// Goal: a KeepAlive Timer that expires during initialization, in OPENSENT or in
// OPENREC, draws the same KeepAlive Timer Expired Notification (owner decision
// Q-2), and the session never goes operational. Method: bare ReadLoop per state.
func TestReadLoopKeepaliveExpiryDuringInitialization(t *testing.T) {
	for _, state := range []SessionState{StateOpenSent, StateOpenReceived} {
		t.Run(state.String(), func(t *testing.T) {
			local, remote := sessionPipe(t)
			sess := expirySession(local, DefaultKeepaliveTime, &logCapture{})
			sess.state = state
			sess.holdTime = expiryHoldTime

			var operational sync.Once
			sawOperational := false
			result := runReadLoop(sess, func() { operational.Do(func() { sawOperational = true }) })
			raw := readRawPDU(t, remote)
			if !bytes.Equal(raw, expiryNotificationPDU) {
				t.Fatalf("PDU = % x, want % x", raw, expiryNotificationPDU)
			}
			if err := awaitReadLoop(t, result, ldpReadTimeout); !errors.Is(err, errKeepaliveExpiry) {
				t.Fatalf("ReadLoop = %v, want %v", err, errKeepaliveExpiry)
			}
			if sawOperational {
				t.Fatal("the session went operational")
			}
		})
	}
}

// failingConn is a connection whose reads always time out and whose writes
// always fail: the peer's socket broke while ze still waits on its timer.
type failingConn struct {
	mu     sync.Mutex
	writes int
}

var errTestWriteFailed = errors.New("test: write failed")

func (c *failingConn) Read([]byte) (int, error) { return 0, os.ErrDeadlineExceeded }
func (c *failingConn) Write([]byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	return 0, errTestWriteFailed
}
func (c *failingConn) Close() error                     { return nil }
func (c *failingConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *failingConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c *failingConn) SetDeadline(time.Time) error      { return nil }
func (c *failingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *failingConn) SetWriteDeadline(time.Time) error { return nil }

// Goal: a Notification write that fails is logged once at Warn, and ReadLoop
// still reports the expiry, not the write error. Method: ReadLoop over a
// connection whose reads time out and whose writes fail; count the Warn line.
func TestReadLoopKeepaliveExpiryNotificationWriteFails(t *testing.T) {
	conn := &failingConn{}
	capture := &logCapture{}
	sess := expirySession(conn, DefaultKeepaliveTime, capture)
	sess.state = StateOperational
	sess.holdTime = expiryHoldTime

	err := sess.ReadLoop(nil, nil, nil, nil)
	if !errors.Is(err, errKeepaliveExpiry) {
		t.Fatalf("ReadLoop = %v, want %v", err, errKeepaliveExpiry)
	}
	if got := capture.count(expiryNotificationWarn); got != 1 {
		t.Fatalf("%q logged %d times, want once", expiryNotificationWarn, got)
	}
	if got := capture.count(errTestWriteFailed.Error()); got != 1 {
		t.Fatalf("write error logged %d times, want once in the Warn line", got)
	}
	if conn.writes != 1 {
		t.Fatalf("%d writes attempted, want one", conn.writes)
	}
}

// Goal: a peer that is connected but never reads cannot hold the session open:
// the Notification attempt ends at notificationWriteTimeout, logs once, and
// ReadLoop reports the expiry. Method: net.Pipe whose far end never reads.
func TestReadLoopKeepaliveExpiryNotificationWriteBounded(t *testing.T) {
	local, _ := sessionPipe(t)
	capture := &logCapture{}
	sess := expirySession(local, DefaultKeepaliveTime, capture)
	sess.state = StateOperational
	sess.holdTime = expiryHoldTime

	start := time.Now()
	result := runReadLoop(sess, nil)
	err := awaitReadLoop(t, result, expiryHoldTime+notificationWriteTimeout+expiryWriteSlack)
	elapsed := time.Since(start)
	if !errors.Is(err, errKeepaliveExpiry) {
		t.Fatalf("ReadLoop = %v, want %v", err, errKeepaliveExpiry)
	}
	if elapsed < notificationWriteTimeout {
		t.Fatalf("ReadLoop returned after %v, before the %v write bound: no write was attempted", elapsed, notificationWriteTimeout)
	}
	if got := capture.count(expiryNotificationWarn); got != 1 {
		t.Fatalf("%q logged %d times, want once", expiryNotificationWarn, got)
	}
}

// Goal: wiring. A peer that goes silent on an operational session dialed by ze
// receives the KeepAlive Timer Expired Notification, then end of file, and the
// session is NON EXISTENT, all through the production runSession. Method: both
// sides propose 1 s; the peer reads ze's KeepAlives until the Notification.
//
// RFC requirement: RFC5036-2.5.4-3 positive -- through runSession, an OPERATIONAL
// session whose KeepAlive Timer expires transmits the fatal KeepAlive Timer
// Expired Notification, then closes the transport connection (the peer reads end
// of file) and ends in NON EXISTENT.
func TestRunSessionKeepaliveExpiryNotifiesThenCloses(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := expirySession(local, time.Second, &logCapture{})
	done := startRunSession(t, sess)
	establishOneSecond(t, sess, remote)

	assertExpiryNotification(t, readUntilNotification(t, remote))
	expectClosed(t, remote)
	select {
	case <-done:
	case <-time.After(ldpReadTimeout):
		t.Fatal("runSession did not return after the expiry")
	}
	if got := sess.State(); got != StateNonExistent {
		t.Fatalf("state = %s, want non-existent", got)
	}
}

// Goal: D-3 lock order. When a KeepAlive is blocked mid-write on a peer that
// stopped reading, the expiry still ends within notificationWriteTimeout: the
// deadline is armed before the write lock, so it fails the blocked KeepAlive,
// the Notification attempt fails at the same deadline (one Warn line), runSession
// returns and the KeepAlive sender has exited. Method: runSession over net.Pipe,
// KeepAlive 1 s, the peer reads nothing once the session is operational.
func TestRunSessionKeepaliveExpiryBoundedBehindBlockedKeepalive(t *testing.T) {
	baseline := runtime.NumGoroutine()
	local, remote := sessionPipe(t)
	capture := &logCapture{}
	sess := expirySession(local, time.Second, capture)
	done := startRunSession(t, sess)
	establishOneSecond(t, sess, remote)
	silentSince := time.Now()
	hold := sess.currentHoldTime()

	select {
	case <-done:
	case <-time.After(hold + notificationWriteTimeout + expiryWriteSlack):
		t.Fatal("runSession did not return: the expiry waited on the write lock without bound")
	}
	elapsed := time.Since(silentSince)
	if elapsed < hold+notificationWriteTimeout-250*time.Millisecond {
		t.Fatalf("runSession returned %v after the peer went silent, before the timer %v plus the write bound %v: no Notification was attempted",
			elapsed, hold, notificationWriteTimeout)
	}
	if got := capture.count(expiryNotificationWarn); got != 1 {
		t.Fatalf("%q logged %d times, want once: the peer reads nothing, so the write must fail", expiryNotificationWarn, got)
	}
	_ = remote.Close()
	awaitGoroutines(t, baseline)
}

// awaitGoroutines waits for the goroutine count to fall back to baseline, which
// proves runSession's KeepAlive sender and the pipe readers have exited.
func awaitGoroutines(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(ldpReadTimeout)
	for runtime.NumGoroutine() > baseline {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines, want at most the %d before the session", runtime.NumGoroutine(), baseline)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Goal: after a fatal Notification nothing more reaches the wire, whichever
// sender tries, and each send says so with an error. Method: expire a bare
// ReadLoop, then call every non-Notification sender; the peer records octets.
func TestNoPDUAfterFatalNotification(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := expirySession(local, DefaultKeepaliveTime, &logCapture{})
	sess.state = StateOperational
	sess.holdTime = expiryHoldTime
	wire := recordWire(t, remote)

	if err := sess.ReadLoop(nil, nil, nil, nil); !errors.Is(err, errKeepaliveExpiry) {
		t.Fatalf("ReadLoop = %v, want %v", err, errKeepaliveExpiry)
	}
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	sends := []struct {
		name string
		send func() error
	}{
		{"KeepAlive", sess.SendKeepalive},
		{"Label Mapping", func() error { return sess.SendLabelMapping(prefix, 100) }},
		{"Label Withdraw", func() error { return sess.SendLabelWithdraw(prefix, 100) }},
		{"Initialization", sess.SendInit},
	}
	for _, s := range sends {
		if err := s.send(); !errors.Is(err, errFatalNotificationSent) {
			t.Errorf("%s after the fatal Notification = %v, want %v", s.name, err, errFatalNotificationSent)
		}
	}
	time.Sleep(200 * time.Millisecond)
	if got := wire.octets(); !bytes.Equal(got, expiryNotificationPDU) {
		t.Fatalf("peer received % x, want only the Notification % x", got, expiryNotificationPDU)
	}
}

// Goal: the write guard fails closed only for fatal Notifications. An advisory
// one (Unknown TLV, E clear) leaves the session able to send. Method: send the
// advisory, then a Label Mapping, and read both on the peer.
func TestAdvisoryNotificationDoesNotBlockWrites(t *testing.T) {
	local, remote := sessionPipe(t)
	sess := expirySession(local, DefaultKeepaliveTime, &logCapture{})
	sess.state = StateOperational
	wire := recordWire(t, remote)

	if err := sess.sendNotification(statusUnknownTLV, 5, MsgTypeLabelMapping); err != nil {
		t.Fatalf("advisory Notification: %v", err)
	}
	if err := sess.SendLabelMapping(netip.MustParsePrefix("192.0.2.0/24"), 100); err != nil {
		t.Fatalf("Label Mapping after an advisory Notification: %v", err)
	}
	got := wire.awaitOctets(t, len(expiryNotificationPDU)+ldpHeaderLen+ldpMsgHdrLen)
	second := got[len(expiryNotificationPDU):]
	msgHdr, err := decodeMessageHeader(second[ldpHeaderLen:])
	if err != nil {
		t.Fatalf("decodeMessageHeader: %v", err)
	}
	if msgHdr.Type != MsgTypeLabelMapping {
		t.Fatalf("second message = %#x, want Label Mapping (%#x)", msgHdr.Type, MsgTypeLabelMapping)
	}
}

// entryConn signals the first time Write is entered, so a test knows a writer
// holds the session's write lock before it starts a second one.
type entryConn struct {
	net.Conn
	once    sync.Once
	entered chan struct{}
}

func (c *entryConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.entered) })
	return c.Conn.Write(p)
}

// Goal: D-3 lock order for advisory Notifications. An Unknown TLV Notification
// sent while a KeepAlive is blocked mid-write arms its deadline before it takes
// the write lock, so the KeepAlive fails with a timeout and the advisory attempt
// ends within notificationWriteTimeout. Method: net.Pipe whose far end never
// reads; the advisory starts only once the KeepAlive has entered Write.
func TestAdvisoryNotificationBoundedBehindBlockedKeepalive(t *testing.T) {
	local, _ := sessionPipe(t)
	conn := &entryConn{Conn: local, entered: make(chan struct{})}
	sess := expirySession(conn, DefaultKeepaliveTime, &logCapture{})
	sess.state = StateOperational

	keepaliveResult := make(chan error, 1)
	go func() { keepaliveResult <- sess.SendKeepalive() }()
	select {
	case <-conn.entered:
	case <-time.After(ldpReadTimeout):
		t.Fatal("the KeepAlive never reached Write")
	}

	advisoryResult := make(chan error, 1)
	start := time.Now()
	go func() { advisoryResult <- sess.sendNotification(statusUnknownTLV, 5, MsgTypeLabelMapping) }()
	var advisoryErr error
	select {
	case advisoryErr = <-advisoryResult:
	case <-time.After(notificationWriteTimeout + expiryWriteSlack):
		t.Fatal("the advisory Notification waited on the write lock without bound")
	}
	if advisoryErr == nil {
		t.Fatal("advisory Notification succeeded, want a failure: the peer reads nothing")
	}
	if elapsed := time.Since(start); elapsed < notificationWriteTimeout-250*time.Millisecond {
		t.Fatalf("advisory ended after %v, before its %v deadline", elapsed, notificationWriteTimeout)
	}
	select {
	case err := <-keepaliveResult:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("blocked KeepAlive = %v, want a deadline timeout", err)
		}
	case <-time.After(ldpReadTimeout):
		t.Fatal("the blocked KeepAlive never returned")
	}
}
