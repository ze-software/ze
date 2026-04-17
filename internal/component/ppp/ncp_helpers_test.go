package ppp

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"
)

// Standard addresses used by NCP-phase tests. Kept package-local so
// every test fixture sees the same "assigned" values.
var (
	ipcpTestLocal = netip.MustParseAddr("10.0.0.1")
	ipcpTestPeer  = netip.MustParseAddr("10.0.0.2")
	ipcpTestDNS1  = netip.MustParseAddr("1.1.1.1")
	ipcpTestDNS2  = netip.MustParseAddr("8.8.8.8")
)

// Peer interface-identifier used by IPv6CP tests. Non-zero,
// non-all-ones, distinct from anything crypto/rand is likely to draw.
var ipv6cpTestPeerID = [8]byte{0x02, 0x00, 0x5E, 0xFF, 0xFE, 0x00, 0x12, 0x34}

// ncpTestDriver bundles the driver, fake backend, pipe pair, and
// auto-responder state used by every NCP-phase test. Constructed via
// newNCPTestDriver; torn down via cleanup().
type ncpTestDriver struct {
	t       *testing.T
	driver  *Driver
	backend *fakeBackend
	peer    net.Conn
	peerFD  int
}

// newNCPTestDriver starts a Driver with auto-accept auth + auto-accept
// IP responders and runs an LCP-only scripted peer through to Opened.
// On return:
//   - LCP is Opened
//   - auth has been accepted (AuthMethodNone)
//   - the NCP phase has begun and the first IPCP CONFREQ has been sent
//     (read it with readPeerNCPPacket)
//
// By default both NCPs are enabled so callers can test each in isolation.
func newNCPTestDriver(t *testing.T) *ncpTestDriver {
	t.Helper()
	return newNCPTestDriverCfg(t, StartSession{})
}

// newNCPTestDriverCfg is the explicit variant; fields on overrides are
// merged into the StartSession.
func newNCPTestDriverCfg(t *testing.T, overrides StartSession) *ncpTestDriver {
	t.Helper()
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	const fd = 5001
	pair := newPipePair(reg, fd)
	// peer stays live through cleanup.
	t.Cleanup(func() { _ = pair.peerEnd.Close() }) //nolint:errcheck // exit cleanup

	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	go autoAcceptIP(d)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { d.Stop() })

	// Short-circuit LCP via proxy AVPs so the test does not have to
	// play the full LCP exchange as well; spec 6c's focus is NCP.
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	start := StartSession{
		TunnelID:            1,
		SessionID:           1,
		ChanFD:              fd,
		UnitFD:              9001,
		UnitNum:             42,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthTimeout:         2 * time.Second,
		IPTimeout:           2 * time.Second,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}
	// Apply caller overrides.
	if overrides.TunnelID != 0 {
		start.TunnelID = overrides.TunnelID
	}
	if overrides.SessionID != 0 {
		start.SessionID = overrides.SessionID
	}
	start.DisableIPCP = overrides.DisableIPCP
	start.DisableIPv6CP = overrides.DisableIPv6CP
	if overrides.IPTimeout != 0 {
		start.IPTimeout = overrides.IPTimeout
	}
	d.SessionsIn() <- start
	// Drain EventLCPUp; the session is now in auth phase. autoAcceptAuth
	// immediately responds, the NCP phase begins.
	if ev := waitForEvent(t, d.EventsOut(), 2*time.Second); ev == nil {
		t.Fatal("no LCPUp event")
	}
	return &ncpTestDriver{
		t:       t,
		driver:  d,
		backend: backend,
		peer:    pair.peerEnd,
		peerFD:  fd,
	}
}

// cleanup is a no-op now that the test-scoped Cleanups handle
// teardown; kept as the API the tests use, for symmetry.
func (td *ncpTestDriver) cleanup() {}

// readPeerNCPPacket waits for one frame from the driver, asserts the
// protocol matches wantProto, parses and returns the LCP-shaped packet.
func (td *ncpTestDriver) readPeerNCPPacket(t *testing.T, wantProto uint16) LCPPacket {
	t.Helper()
	_ = td.peer.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // test helper
	buf := make([]byte, MaxFrameLen)
	n, err := td.peer.Read(buf)
	if err != nil {
		t.Fatalf("peer read: %v", err)
	}
	proto, payload, _, err := ParseFrame(buf[:n])
	if err != nil {
		t.Fatalf("peer ParseFrame: %v", err)
	}
	if proto != wantProto {
		t.Fatalf("peer proto = 0x%04x, want 0x%04x", proto, wantProto)
	}
	pkt, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("peer ParseLCPPacket: %v", err)
	}
	return pkt
}

// writePeerNCPPacket sends one LCP-shaped packet for the given protocol.
func (td *ncpTestDriver) writePeerNCPPacket(t *testing.T, proto uint16, code, id uint8, data []byte) {
	t.Helper()
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, proto, nil)
	off += WriteLCPPacket(buf, off, code, id, data)
	_ = td.peer.SetWriteDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // test helper
	if _, err := td.peer.Write(buf[:off]); err != nil {
		t.Fatalf("peer write: %v", err)
	}
}

// waitForEvent reads one event from the driver with a timeout.
func (td *ncpTestDriver) waitForEvent(t *testing.T, timeout time.Duration) Event {
	t.Helper()
	select {
	case ev := <-td.driver.EventsOut():
		return ev
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event after %s", timeout)
	}
	return nil
}

// autoAcceptIP answers every EventIPRequest with the stock assignment
// defined above. Runs until IPEventsOut is closed (Driver.Stop).
func autoAcceptIP(d *Driver) {
	for ev := range d.IPEventsOut() {
		req, ok := ev.(EventIPRequest)
		if !ok {
			continue
		}
		args := IPResponseArgs{Accept: true, Family: req.Family}
		switch req.Family {
		case AddressFamilyIPv4:
			args.Local = ipcpTestLocal
			args.Peer = ipcpTestPeer
			args.DNSPrimary = ipcpTestDNS1
			args.DNSSecondary = ipcpTestDNS2
		case AddressFamilyIPv6:
			// Accept peer's chosen identifier.
		}
		_ = d.IPResponse(req.TunnelID, req.SessionID, args) //nolint:errcheck // ignore teardown race
	}
}

// completeIPCP plays the peer side of a full IPCP happy-path exchange
// against the driver:
//  1. Read driver's initial CR (IP-Address=local + DNS hints) and Ack.
//  2. Send the peer's CR with IP-Address=0.0.0.0 (request).
//  3. Read driver's Nak carrying the peer's assigned address.
//  4. Send the peer's CR with IP-Address=ipcpTestPeer.
//  5. Read driver's Ack.
//
// On return, IPCP is Opened on ze's side.
func (td *ncpTestDriver) completeIPCP(t *testing.T) {
	t.Helper()
	// Step 1: driver's CR -> Ack.
	cr := td.readPeerNCPPacket(t, ProtoIPCP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("step 1: got code %d, want CR", cr.Code)
	}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)

	// Step 2: peer CR with IP-Address=0.0.0.0.
	zeroAddr := []byte{3, 6, 0, 0, 0, 0}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x10, zeroAddr)

	// Step 3: driver's Nak with peer's assigned address.
	nak := td.readPeerNCPPacket(t, ProtoIPCP)
	if nak.Code != LCPConfigureNak {
		t.Fatalf("step 3: got code %d, want Nak", nak.Code)
	}

	// Step 4: peer CR with proper address.
	peerCR := []byte{3, 6}
	a4 := ipcpTestPeer.As4()
	peerCR = append(peerCR, a4[:]...)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x11, peerCR)

	// Step 5: driver Ack.
	ack := td.readPeerNCPPacket(t, ProtoIPCP)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("step 5: got code %d, want Ack", ack.Code)
	}
}

// completeIPv6CP plays the peer side of a full IPv6CP happy-path
// exchange. Uses ipv6cpTestPeerID as the peer's proposed identifier.
func (td *ncpTestDriver) completeIPv6CP(t *testing.T) {
	t.Helper()
	// Step 1: driver's CR -> Ack.
	cr := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("IPv6CP step 1: got code %d, want CR", cr.Code)
	}
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureAck, cr.Identifier, cr.Data)

	// Step 2: peer CR with Interface-Identifier option.
	peerCR := []byte{1, 10}
	peerCR = append(peerCR, ipv6cpTestPeerID[:]...)
	td.writePeerNCPPacket(t, ProtoIPv6CP, LCPConfigureRequest, 0x20, peerCR)

	// Step 3: driver Ack.
	ack := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("IPv6CP step 3: got code %d, want Ack", ack.Code)
	}
}

// runParallelNCPPeer plays the peer side of an IPCP + IPv6CP handshake
// against the driver, dispatching each frame to its family's handler in
// one loop. Used by tests that enable both NCPs and cannot rely on the
// sequential completeIPCP / completeIPv6CP helpers (the driver sends
// both initial CRs back-to-back from the same goroutine, so a
// sequential peer deadlocks on the second CR).
//
// Exits when both families observe a Configure-Ack from the driver or
// the 3-second deadline fires. Signals completion by closing done.
func runParallelNCPPeer(t *testing.T, conn net.Conn, expectIPCP, expectIPv6CP bool, done chan<- struct{}) {
	t.Helper()
	defer close(done)
	ipcpDone := !expectIPCP
	ipv6cpDone := !expectIPv6CP
	ipcpSentCR := false
	ipv6cpSentCR := false
	buf := make([]byte, MaxFrameLen)
	deadline := time.Now().Add(3 * time.Second)
	for !ipcpDone || !ipv6cpDone {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Errorf("peer SetReadDeadline: %v", err)
			return
		}
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("peer read: %v", err)
			return
		}
		proto, payload, _, perr := ParseFrame(buf[:n])
		if perr != nil {
			t.Errorf("peer ParseFrame: %v", perr)
			return
		}
		pkt, perr := ParseLCPPacket(payload)
		if perr != nil {
			t.Errorf("peer ParseLCPPacket: %v", perr)
			return
		}
		switch proto {
		case ProtoIPCP:
			switch pkt.Code {
			case LCPConfigureRequest:
				writePeerNCPFrame(t, conn, ProtoIPCP, LCPConfigureAck, pkt.Identifier, pkt.Data)
				if !ipcpSentCR {
					peerCR := []byte{3, 6}
					a4 := ipcpTestPeer.As4()
					peerCR = append(peerCR, a4[:]...)
					writePeerNCPFrame(t, conn, ProtoIPCP, LCPConfigureRequest, 0x40, peerCR)
					ipcpSentCR = true
				}
			case LCPConfigureAck:
				ipcpDone = true
			}
		case ProtoIPv6CP:
			switch pkt.Code {
			case LCPConfigureRequest:
				writePeerNCPFrame(t, conn, ProtoIPv6CP, LCPConfigureAck, pkt.Identifier, pkt.Data)
				if !ipv6cpSentCR {
					peerCR := []byte{1, 10}
					peerCR = append(peerCR, ipv6cpTestPeerID[:]...)
					writePeerNCPFrame(t, conn, ProtoIPv6CP, LCPConfigureRequest, 0x50, peerCR)
					ipv6cpSentCR = true
				}
			case LCPConfigureAck:
				ipv6cpDone = true
			}
		}
	}
}

// writePeerNCPFrame is the standalone equivalent of
// ncpTestDriver.writePeerNCPPacket, callable from the goroutine peer
// loop where the ncpTestDriver is not accessible.
func writePeerNCPFrame(t *testing.T, conn net.Conn, proto uint16, code, id uint8, data []byte) {
	t.Helper()
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, proto, nil)
	off += WriteLCPPacket(buf, off, code, id, data)
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("peer SetWriteDeadline: %v", err)
		return
	}
	if _, err := conn.Write(buf[:off]); err != nil {
		t.Errorf("peer write: %v", err)
	}
}

// waitForEventOfType drains events until one of type T arrives or the
// deadline fires. Returns the zero value + false on timeout.
func waitForEventOfType[T Event](t *testing.T, ch <-chan Event, timeout time.Duration) (T, bool) {
	t.Helper()
	var zero T
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if got, ok := ev.(T); ok {
				return got, true
			}
		case <-deadline:
			return zero, false
		}
	}
}

// iface addrCall / routeCall getters are provided in helpers_test.go;
// this file intentionally adds only NCP-specific fixtures.
var _ = binary.BigEndian
