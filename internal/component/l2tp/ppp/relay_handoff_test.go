package ppp

import (
	"bytes"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// relayPeerOptions is the peer's acceptable LCP Configure-Request: MRU 1500.
var relayPeerOptions = []byte{LCPOptMRU, 0x04, 0x05, 0xDC}

// relayPeerFirstOptions is the peer's first Configure-Request: MRU 1500 and
// an option no implementation knows, which the driver Configure-Rejects.
var relayPeerFirstOptions = []byte{LCPOptMRU, 0x04, 0x05, 0xDC, 0xFE, 0x02}

// relayPeerLCP plays the subscriber's side of LCP: it Acks the driver's
// request, sends a request the driver must Configure-Reject, then the
// acceptable one, and returns once the driver Acked it. It sends the
// driver's Acked request options on sent, and stops reading afterwards so
// the test owns the pipe.
func relayPeerLCP(t *testing.T, conn net.Conn, sent chan<- []byte) {
	t.Helper()
	defer close(sent)
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Errorf("peer: deadline: %v", err)
		return
	}
	defer conn.SetDeadline(time.Time{}) //nolint:errcheck // test cleanup

	driverCR := relayPeerReadLCP(t, conn, LCPConfigureRequest)
	if driverCR == nil {
		return
	}
	relayPeerWriteLCP(t, conn, LCPConfigureAck, driverCR.Identifier, driverCR.Data)
	relayPeerWriteLCP(t, conn, LCPConfigureRequest, 1, relayPeerFirstOptions)
	if relayPeerReadLCP(t, conn, LCPConfigureReject) == nil {
		return
	}
	relayPeerWriteLCP(t, conn, LCPConfigureRequest, 2, relayPeerOptions)
	if relayPeerReadLCP(t, conn, LCPConfigureAck) == nil {
		return
	}
	sent <- bytes.Clone(driverCR.Data)
}

func relayPeerReadLCP(t *testing.T, conn net.Conn, want uint8) *LCPPacket {
	t.Helper()
	buf := make([]byte, MaxFrameLen)
	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("peer: read LCP code %d: %v", want, err)
		return nil
	}
	proto, payload, _, err := ParseFrame(buf[:n])
	if err != nil || proto != ProtoLCP {
		t.Errorf("peer: want LCP, got proto=0x%04x err=%v", proto, err)
		return nil
	}
	pkt, err := ParseLCPPacket(payload)
	if err != nil || pkt.Code != want {
		t.Errorf("peer: want LCP code %d, got %d err=%v", want, pkt.Code, err)
		return nil
	}
	return &pkt
}

func relayPeerWriteLCP(t *testing.T, conn net.Conn, code, id uint8, data []byte) {
	t.Helper()
	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, code, id, data)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Errorf("peer: write LCP code %d: %v", code, err)
	}
}

// startRelaySession starts a PAP session with CaptureProxyLCP on a pipe,
// runs LCP through relayPeerLCP, sends the subscriber's Authenticate-Request
// and returns the driver, the peer end, the driver's Acked request options
// and the EventAuthRequest the driver emitted.
func startRelaySession(t *testing.T, fd int) (*Driver, net.Conn, []byte, EventAuthRequest) {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, fd)
	t.Cleanup(func() { closeConn(pair.peerEnd) })

	ops, _, _ := newFakeOps()
	d := newTestDriverNoResponder(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(d.Stop)

	sent := make(chan []byte, 1)
	go relayPeerLCP(t, pair.peerEnd, sent)
	d.SessionsIn() <- StartSession{
		TunnelID: 7, SessionID: 9, ChanFD: fd, UnitFD: 901, UnitNum: 1,
		LNSMode: true, PPPoE: true, MaxMRU: 1500,
		AuthMethod: AuthMethodPAP, AuthFallbackOrder: []AuthMethod{AuthMethodPAP},
		CaptureProxyLCP: true,
	}
	driverOptions, ok := <-sent
	if !ok {
		t.Fatal("LCP did not reach Opened")
	}
	waitForEventOfTypeOrFail[EventLCPUp](t, d.EventsOut())

	if _, err := pair.peerEnd.Write(buildPAPRequestFrame(42, "cpe@example.net", "s3cret")); err != nil {
		t.Fatalf("peer write PAP: %v", err)
	}
	select {
	case ev := <-d.AuthEventsOut():
		req, isReq := ev.(EventAuthRequest)
		if !isReq {
			t.Fatalf("auth event %T, want EventAuthRequest", ev)
		}
		return d, pair.peerEnd, driverOptions, req
	case <-time.After(2 * time.Second):
		t.Fatal("no EventAuthRequest")
	}
	return nil, nil, nil, EventAuthRequest{}
}

func waitForEventOfTypeOrFail[T Event](t *testing.T, ch <-chan Event) T {
	t.Helper()
	ev, ok := waitForEventOfType[T](t, ch, 2*time.Second)
	if !ok {
		var zero T
		t.Fatalf("no %T event", zero)
	}
	return ev
}

// TestDriverCapturesProxyLCPForRelay checks the LCP capture a realm relay
// forwards (RFC 2661 Section 4.4.5): the subscriber's first request (the
// one the driver Configure-Rejected), the driver's Acked request, and the
// subscriber's Acked request after the Reject round. Method: drive LCP with
// a scripted peer and read EventAuthRequest.ProxyLCP.
func TestDriverCapturesProxyLCPForRelay(t *testing.T) {
	_, _, driverOptions, req := startRelaySession(t, 16001)
	if req.ProxyLCP == nil {
		t.Fatal("EventAuthRequest.ProxyLCP is nil for a CaptureProxyLCP session")
	}
	if !bytes.Equal(req.ProxyLCP.InitialReceived, relayPeerFirstOptions) {
		t.Errorf("InitialReceived = %x, want %x", req.ProxyLCP.InitialReceived, relayPeerFirstOptions)
	}
	if !bytes.Equal(req.ProxyLCP.LastReceived, relayPeerOptions) {
		t.Errorf("LastReceived = %x, want %x", req.ProxyLCP.LastReceived, relayPeerOptions)
	}
	if !bytes.Equal(req.ProxyLCP.LastSent, driverOptions) {
		t.Errorf("LastSent = %x, want the driver's Acked request %x", req.ProxyLCP.LastSent, driverOptions)
	}
}

// TestDriverQuiesceStopsReaderBeforeReturning checks step 7 of the handoff:
// QuiesceSession returns only after the channel reader returned, and nothing
// reads the channel afterwards, so an Echo-Request is never answered.
// Method: on net.Pipe a write completes only when the other end reads, so a
// peer write that times out proves no reader is left.
func TestDriverQuiesceStopsReaderBeforeReturning(t *testing.T) {
	d, peer, _, _ := startRelaySession(t, 16002)
	if err := d.QuiesceSession(7, 9); err != nil {
		t.Fatalf("QuiesceSession: %v", err)
	}
	s, err := d.liveSession(7, 9)
	if err != nil {
		t.Fatalf("session gone after quiesce: %v", err)
	}
	select {
	case <-s.readerGone:
	default:
		t.Fatal("QuiesceSession returned while the channel reader still runs")
	}

	echo := make([]byte, MaxFrameLen)
	off := WriteFrame(echo, 0, ProtoLCP, nil)
	off += WriteLCPPacket(echo, off, LCPEchoRequest, 5, []byte{0, 0, 0, 0})
	if err := peer.SetWriteDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	if _, err := peer.Write(echo[:off]); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("peer Echo-Request write = %v, want a timeout: something still reads the quiesced channel", err)
	}
}

// TestDriverRelayReleaseSendsNoAuthReply checks step 9 (AC-1): after the
// bridge, ReleaseSession writes no Authenticate-Ack or -Nak, no LCP
// Terminate-Request, and reports no EventSessionDown, because the subscriber
// lives on through the bridge. Method: a peer read is pending before the
// release, so any frame the driver writes while releasing reaches it; the
// read must end with the pipe closing (ReleaseSession closes the driver's
// end), not with a frame. A net.Pipe write blocks until the peer reads, so a
// read started only after the release could never see such a frame, and
// SetReadDeadline on a pipe whose other end closed answers io.ErrClosedPipe.
func TestDriverRelayReleaseSendsNoAuthReply(t *testing.T) {
	d, peer, _, _ := startRelaySession(t, 16003)
	if err := d.QuiesceSession(7, 9); err != nil {
		t.Fatalf("QuiesceSession: %v", err)
	}
	type peerRead struct {
		frame []byte
		err   error
	}
	readCh := make(chan peerRead, 1)
	go func() {
		buf := make([]byte, MaxFrameLen)
		n, err := peer.Read(buf)
		readCh <- peerRead{frame: bytes.Clone(buf[:n]), err: err}
	}()
	if err := d.ReleaseSession(7, 9); err != nil {
		t.Fatalf("ReleaseSession: %v", err)
	}
	select {
	case got := <-readCh:
		if got.err == nil {
			t.Fatalf("driver wrote %x while releasing, want nothing", got.frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("peer read still pending after ReleaseSession, want the driver's end closed")
	}
	if ev, ok := waitForEventOfType[EventSessionDown](t, d.EventsOut(), 200*time.Millisecond); ok {
		t.Fatalf("released session reported %+v, want no EventSessionDown", ev)
	}
	if _, err := d.liveSession(7, 9); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("session still known after ReleaseSession: %v", err)
	}
}

// TestDriverRelayRefusalClosesLCP checks step 10 (AC-6, AC-16): a refusal
// after the quiesce still answers the subscriber, with an Authenticate-Nak
// carrying the message, then an LCP Terminate-Request, and the session
// ends with EventSessionDown so the transport sends PADT.
func TestDriverRelayRefusalClosesLCP(t *testing.T) {
	d, peer, _, _ := startRelaySession(t, 16004)
	if err := d.QuiesceSession(7, 9); err != nil {
		t.Fatalf("QuiesceSession: %v", err)
	}
	const message = "realm example.net: L2TP call refused"
	if err := d.RefuseAuth(7, 9, message); err != nil {
		t.Fatalf("RefuseAuth: %v", err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	buf := make([]byte, MaxFrameLen)
	n, err := peer.Read(buf)
	if err != nil {
		t.Fatalf("read Authenticate-Nak: %v", err)
	}
	proto, payload, _, err := ParseFrame(buf[:n])
	if err != nil || proto != ProtoPAP || len(payload) < 1 || payload[0] != PAPAuthenticateNak {
		t.Fatalf("first frame proto=0x%04x payload=%x err=%v, want PAP Authenticate-Nak", proto, payload, err)
	}
	if !bytes.Contains(payload, []byte(message)) {
		t.Errorf("Authenticate-Nak %x does not carry %q", payload, message)
	}
	if relayPeerReadLCP(t, peer, LCPTerminateRequest) == nil {
		t.Fatal("no LCP Terminate-Request after the refusal")
	}
	waitForEventOfTypeOrFail[EventSessionDown](t, d.EventsOut())
}
