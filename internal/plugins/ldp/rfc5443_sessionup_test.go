// RFC: rfc/short/rfc5443.md -- the LDP session an IGP waits for is the established one
// Design: docs/architecture/ldp/mpls-ldp.md -- LDP plugin, SessionUp event
// Related: establishment_rfc5036_test.go -- runSessionToOperational, expectSilence
// Related: rfc5036_test.go -- rfcTestSession, readLDPPDU, runSessionForTest, encodeInitPDU
//
// LDP-IGP synchronization (RFC 5443) holds an IGP link at maximum cost until LDP
// is operational on it, and ze's OSPF learns that from the LDP plugin's SessionUp
// event. These tests drive runSession, the function startSessionForAdj runs on the
// connection it dialed, over a pipe whose far end plays the peer, and record what
// the plugin publishes on the event bus.
//
// VALIDATES: SessionUp is published only when the session reaches OPERATIONAL,
// which RFC 5036 Section 2.5.4 places after an acceptable Initialization AND the
// peer's KeepAlive, and it names the discovering adjacency's interface and LDP
// Identifier.
// PREVENTS: an IGP that starts its LDP-sync hold-down, and then declares the link
// synchronized, for a peer that accepted TCP and never established the session.
package ldp

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/ze"
)

// sessionUpRecorder is an event bus that keeps every SessionUp the plugin emits.
// Safe for concurrent use: runSession emits from its own goroutine.
type sessionUpRecorder struct {
	mu  sync.Mutex
	ups []SessionEvent
	// arrived carries one token per recorded SessionUp, so a test waits on it
	// instead of polling.
	arrived chan struct{}
}

func (r *sessionUpRecorder) Emit(namespace, eventType string, payload any) (int, error) {
	if namespace != Namespace {
		return 0, nil
	}
	if eventType != EventSessionUp {
		return 0, nil
	}
	evt, ok := payload.(*SessionEvent)
	if !ok {
		return 0, nil
	}
	r.mu.Lock()
	r.ups = append(r.ups, *evt)
	r.mu.Unlock()
	select {
	case r.arrived <- struct{}{}:
	default:
	}
	return 0, nil
}

func (r *sessionUpRecorder) Subscribe(string, string, func(any)) func() { return func() {} }

func (r *sessionUpRecorder) recorded() []SessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]SessionEvent(nil), r.ups...)
}

var _ ze.EventBus = (*sessionUpRecorder)(nil)

// recordSessionUps installs a recorder as the plugin's event bus for the test and
// restores the previous bus when it ends.
func recordSessionUps(t *testing.T) *sessionUpRecorder {
	t.Helper()
	rec := &sessionUpRecorder{arrived: make(chan struct{}, 8)}
	prev := eventBusPtr.Load()
	setEventBus(rec)
	t.Cleanup(func() { eventBusPtr.Store(prev) })
	return rec
}

// encodeKeepAlivePDU builds a complete KeepAlive PDU from the peer 10.0.0.2:0.
func encodeKeepAlivePDU() []byte {
	var buf [ldpHeaderLen + ldpMsgHdrLen]byte
	n := encodeKeepalive(buf[ldpHeaderLen:], keepaliveMessage{MessageID: 9})
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  uint16(n + 6),
		LSRID:      [4]byte{10, 0, 0, 2},
		LabelSpace: 0,
	})
	return buf[:ldpHeaderLen+n]
}

// exchangeInitialization plays the peer's half of the Initialization exchange up
// to, and not including, the peer's KeepAlive: it reads ze's Initialization,
// answers with an acceptable one, and reads the KeepAlive ze accepts it with.
func exchangeInitialization(t *testing.T, remote net.Conn) {
	t.Helper()
	if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization (%#x)", hdr.Type, MsgTypeInitialize)
	}
	if _, err := remote.Write(encodeInitPDU(ldpVersion, 30)); err != nil {
		t.Fatalf("write peer Initialization: %v", err)
	}
	if _, hdr, _ := readLDPPDU(t, remote); hdr.Type != MsgTypeKeepAlive {
		t.Fatalf("reply to the peer Initialization = %#x, want KeepAlive (%#x)", hdr.Type, MsgTypeKeepAlive)
	}
}

// sessionPipe returns both ends of an in-memory connection, closed when the test ends.
func sessionPipe(t *testing.T) (local, remote net.Conn) {
	t.Helper()
	local, remote = net.Pipe()
	t.Cleanup(func() {
		_ = local.Close()
		_ = remote.Close()
	})
	return local, remote
}

// RFC requirement: RFC5443-2-5 negative -- no SessionUp before the session is
// established: a peer that accepted the TCP connection and the Initialization
// exchange but never sent its KeepAlive leaves the session in OPENREC, and the
// LDP plugin publishes no SessionUp, so no IGP link reads LDP as operational.
func TestRFC5443NoSessionUpBeforePeerKeepAlive(t *testing.T) {
	rec := recordSessionUps(t)
	local, remote := sessionPipe(t)

	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	exchangeInitialization(t, remote)
	expectSilence(t, remote, 300*time.Millisecond, "ze sends nothing more before the peer's KeepAlive")

	if got := rec.recorded(); len(got) != 0 {
		t.Fatalf("SessionUp published %d times before the peer's KeepAlive (%+v), want 0", len(got), got)
	}
	if sess.State() != StateOpenReceived {
		t.Errorf("state = %s, want open-received: the peer's KeepAlive has not arrived", sess.State())
	}
}

// RFC requirement: RFC5443-2-5 positive -- the peer's KeepAlive after an accepted
// Initialization establishes the session, and the LDP plugin then publishes
// exactly one SessionUp naming the discovering adjacency's interface (eth0) and
// its LDP Identifier (10.0.0.2:0), with session state operational.
func TestRFC5443SessionUpOnPeerKeepAlive(t *testing.T) {
	rec := recordSessionUps(t)
	local, remote := sessionPipe(t)

	sess := rfcTestSession(local)
	stop := runSessionForTest(t, sess)
	defer stop()

	exchangeInitialization(t, remote)
	if _, err := remote.Write(encodeKeepAlivePDU()); err != nil {
		t.Fatalf("write peer KeepAlive: %v", err)
	}
	select {
	case <-rec.arrived:
	case <-time.After(ldpReadTimeout):
		t.Fatal("no SessionUp within 2s of the peer's KeepAlive")
	}
	expectSilence(t, remote, 200*time.Millisecond, "an empty LIB advertises nothing")

	got := rec.recorded()
	if len(got) != 1 {
		t.Fatalf("SessionUp published %d times, want exactly 1: %+v", len(got), got)
	}
	want := SessionEvent{
		PeerAddress:   "10.0.0.2",
		LDPIdentifier: "10.0.0.2:0",
		SessionState:  "operational",
		Interface:     "eth0",
	}
	if got[0] != want {
		t.Errorf("SessionUp = %+v, want %+v", got[0], want)
	}
	if sess.State() != StateOperational {
		t.Errorf("state = %s, want operational", sess.State())
	}
}

// TestKeepAliveWithoutOwnInitNeverOperational guards initAccepted: a session that
// never sent its own Initialization reaches open-received on the peer's, and the
// peer's KeepAlive that follows in the same PDU leaves it there. Only an
// Initialization accepted in OPENSENT lets a KeepAlive make the session operational.
func TestKeepAliveWithoutOwnInitNeverOperational(t *testing.T) {
	local, _ := sessionPipe(t)
	sess := rfcTestSession(local) // NewSession starts in StateInitialized: no Init sent

	pdu := encodeInitPDU(ldpVersion, 30)
	if err := sess.processMessages(withPeerKeepAlive(pdu[ldpHeaderLen:]), [4]byte{10, 0, 0, 2}, 0, nil, nil, nil); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if sess.State() != StateOpenReceived {
		t.Errorf("state = %s, want open-received: ze never sent its Initialization", sess.State())
	}
}
