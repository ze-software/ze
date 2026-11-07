// Design: docs/guide/l2tp.md -- PPP negotiation, the proxy-lcp policy
// Related: proxy.go -- admitProxyLCP, the gate these tests drive
// Related: session_run.go -- run, the proxy branch and the synthetic Open

package ppp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// proxyStreamCHAP is a complete proxy LCP CONFREQ stream a LAC could relay:
// MRU 1500, Auth-Protocol CHAP-MD5 and the LAC's Magic-Number 0xCAFEBABE.
func proxyStreamCHAP() []byte {
	return buildOptionStream([]LCPOption{mruOpt(1500), authProtoOpt(ProtoCHAP, 0x05), magicOpt(0xCAFEBABE)})
}

// proxyStart is a StartSession for an LNS session carrying valid proxy LCP,
// with CHAP-MD5 configured and both NCPs off so LCP and auth are all it runs.
func proxyStart(fd int, stream []byte, renegotiate bool) StartSession {
	return StartSession{
		TunnelID:            1,
		SessionID:           uint16(fd),
		ChanFD:              fd,
		UnitFD:              999,
		UnitNum:             7,
		LNSMode:             true,
		MaxMRU:              1500,
		AuthMethod:          AuthMethodCHAPMD5,
		AuthFallbackOrder:   []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP},
		AuthRequired:        true,
		DisableIPCP:         true,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
		RenegotiateProxyLCP: renegotiate,
	}
}

// readDriverFrame reads one frame the driver wrote, failing the test after 2s.
func readDriverFrame(t *testing.T, conn net.Conn) (uint16, []byte) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, MaxFrameLen)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("peer read: %v", err)
	}
	proto, payload, _, err := ParseFrame(buf[:n])
	if err != nil {
		t.Fatalf("peer parse frame: %v", err)
	}
	return proto, append([]byte(nil), payload...)
}

// writePeerLCP writes one LCP packet from the subscriber side.
func writePeerLCP(t *testing.T, conn net.Conn, code, id uint8, data []byte) {
	t.Helper()
	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoLCP, nil)
	off += WriteLCPPacket(out, off, code, id, data)
	if _, err := conn.Write(out[:off]); err != nil {
		t.Fatalf("peer write LCP: %v", err)
	}
}

// waitLCPUp returns the first EventLCPUp, failing after 2s.
func waitLCPUp(t *testing.T, ch <-chan Event) EventLCPUp {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-ch:
			if up, ok := ev.(EventLCPUp); ok {
				return up
			}
		case <-deadline:
			t.Fatal("no EventLCPUp")
		}
	}
}

// optionData returns the data of the first option of type kind, or nil.
func optionData(t *testing.T, data []byte, kind uint8) []byte {
	t.Helper()
	opts, err := ParseLCPOptions(data)
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	for _, o := range opts {
		if o.Type == kind {
			return o.Data
		}
	}
	return nil
}

// TestDriverProxyLCPRenegotiateSendsConfReq proves `proxy-lcp renegotiate`
// ignores a valid proxy: the first frame Ze sends is its own LCP
// Configure-Request, no EventLCPUp precedes the subscriber's Configure-Ack, and
// the outcome is reported as renegotiated.
//
// VALIDATES: spec-home-cpe-15 AC-2.
// PREVENTS: the flag reaching StartSession while the short-circuit still runs.
func TestDriverProxyLCPRenegotiateSendsConfReq(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1101)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	d.SessionsIn() <- proxyStart(1101, proxyStreamCHAP(), true)

	proto, payload := readDriverFrame(t, pair.peerEnd)
	if proto != ProtoLCP {
		t.Fatalf("first frame protocol = 0x%04x, want LCP", proto)
	}
	req, err := ParseLCPPacket(payload)
	if err != nil || req.Code != LCPConfigureRequest {
		t.Fatalf("first frame = code %d err %v, want Configure-Request", req.Code, err)
	}
	select {
	case ev := <-d.EventsOut():
		t.Fatalf("event %T before the subscriber acknowledged Ze's Configure-Request", ev)
	default:
	}

	writePeerLCP(t, pair.peerEnd, LCPConfigureAck, req.Identifier, req.Data)
	writePeerLCP(t, pair.peerEnd, LCPConfigureRequest, 1, buildOptionStream([]LCPOption{mruOpt(1500)}))
	proto, payload = readDriverFrame(t, pair.peerEnd)
	if ack, err := ParseLCPPacket(payload); proto != ProtoLCP || err != nil || ack.Code != LCPConfigureAck {
		t.Fatalf("Ze's answer to the subscriber's Configure-Request: proto 0x%04x code %d err %v", proto, ack.Code, err)
	}
	up := waitLCPUp(t, d.EventsOut())
	if up.ProxyLCP != ProxyLCPOutcomeRenegotiated {
		t.Fatalf("EventLCPUp.ProxyLCP = %s, want renegotiated", up.ProxyLCP)
	}
}

// TestDriverProxyLCPRenegotiateOwnMagic proves the renegotiated
// Configure-Request carries a Magic-Number Ze drew itself, not the LAC's.
//
// VALIDATES: spec-home-cpe-15 R-2 and the Security Review (an untrusted LAC
// chooses nothing).
func TestDriverProxyLCPRenegotiateOwnMagic(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1102)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	d.SessionsIn() <- proxyStart(1102, proxyStreamCHAP(), true)

	_, payload := readDriverFrame(t, pair.peerEnd)
	req, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	magic := optionData(t, req.Data, LCPOptMagic)
	if len(magic) != 4 {
		t.Fatalf("Configure-Request carries no Magic-Number: % x", req.Data)
	}
	if binary.BigEndian.Uint32(magic) == 0xCAFEBABE {
		t.Fatal("renegotiated Configure-Request reuses the LAC's proxied Magic-Number")
	}
}

// TestDriverProxyLCPRenegotiateUsesConfiguredAuth proves the renegotiated
// session authenticates with the configured method even when the LAC's proxy
// negotiated another: the Configure-Request asks for CHAP-MD5 and, once LCP is
// open, Ze sends a CHAP Challenge.
//
// VALIDATES: spec-home-cpe-15 AC-3.
func TestDriverProxyLCPRenegotiateUsesConfiguredAuth(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1103)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	papProxy := buildOptionStream([]LCPOption{mruOpt(1500), authProtoOpt(ProtoPAP), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- proxyStart(1103, papProxy, true)

	_, payload := readDriverFrame(t, pair.peerEnd)
	req, err := ParseLCPPacket(payload)
	if err != nil || req.Code != LCPConfigureRequest {
		t.Fatalf("first frame code %d err %v, want Configure-Request", req.Code, err)
	}
	auth := optionData(t, req.Data, LCPOptAuthProto)
	if len(auth) < 2 || binary.BigEndian.Uint16(auth) != ProtoCHAP {
		t.Fatalf("Auth-Protocol in Ze's Configure-Request = % x, want CHAP", auth)
	}
	writePeerLCP(t, pair.peerEnd, LCPConfigureAck, req.Identifier, req.Data)
	writePeerLCP(t, pair.peerEnd, LCPConfigureRequest, 1, buildOptionStream([]LCPOption{mruOpt(1500)}))
	_, _ = readDriverFrame(t, pair.peerEnd) // Ze's Configure-Ack
	up := waitLCPUp(t, d.EventsOut())
	if up.NegotiatedAuthMethod != AuthMethodCHAPMD5 {
		t.Fatalf("NegotiatedAuthMethod = %s, want chap-md5", up.NegotiatedAuthMethod)
	}
	proto, payload := readDriverFrame(t, pair.peerEnd)
	if proto != ProtoCHAP || len(payload) == 0 || payload[0] != CHAPCodeChallenge {
		t.Fatalf("frame after LCP Opened: proto 0x%04x % x, want a CHAP Challenge", proto, payload)
	}
}

// TestDriverProxyLCPAcceptUnchanged proves the default policy keeps today's
// short-circuit: EventLCPUp comes first with the outcome accepted, carrying
// the proxied MRU, and the first frame Ze sends is the CHAP re-challenge, not
// a Configure-Request.
//
// VALIDATES: spec-home-cpe-15 AC-1 (beside TestDriverProxyLCPSkipsNegotiation).
func TestDriverProxyLCPAcceptUnchanged(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1104)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	d.SessionsIn() <- proxyStart(1104, proxyStreamCHAP(), false)

	up := waitLCPUp(t, d.EventsOut())
	if up.ProxyLCP != ProxyLCPOutcomeAccepted || up.NegotiatedMRU != 1500 {
		t.Fatalf("EventLCPUp = %+v, want accepted with MRU 1500", up)
	}
	proto, payload := readDriverFrame(t, pair.peerEnd)
	if proto != ProtoCHAP || len(payload) == 0 || payload[0] != CHAPCodeChallenge {
		t.Fatalf("first frame: proto 0x%04x % x, want a CHAP Challenge", proto, payload)
	}
}

// TestNonLCPBeforeOpenedDiscarded proves an IPCP Configure-Request the
// subscriber sent before LCP reopened (it was Opened with the LAC) draws no
// Protocol-Reject: the next frame Ze sends after it is the LCP Configure-Ack.
//
// VALIDATES: spec-home-cpe-15 A-3.
func TestNonLCPBeforeOpenedDiscarded(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1105)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	d.SessionsIn() <- proxyStart(1105, proxyStreamCHAP(), true)

	_, payload := readDriverFrame(t, pair.peerEnd)
	req, err := ParseLCPPacket(payload)
	if err != nil || req.Code != LCPConfigureRequest {
		t.Fatalf("first frame code %d err %v, want Configure-Request", req.Code, err)
	}
	ipcp := make([]byte, MaxFrameLen)
	off := WriteFrame(ipcp, 0, ProtoIPCP, nil)
	off += WriteLCPPacket(ipcp, off, LCPConfigureRequest, 9, []byte{3, 6, 192, 0, 2, 1})
	if _, err := pair.peerEnd.Write(ipcp[:off]); err != nil {
		t.Fatalf("peer write IPCP: %v", err)
	}
	writePeerLCP(t, pair.peerEnd, LCPConfigureAck, req.Identifier, req.Data)
	writePeerLCP(t, pair.peerEnd, LCPConfigureRequest, 1, buildOptionStream([]LCPOption{mruOpt(1500)}))
	proto, payload := readDriverFrame(t, pair.peerEnd)
	ack, err := ParseLCPPacket(payload)
	if proto != ProtoLCP || err != nil || ack.Code != LCPConfigureAck {
		t.Fatalf("frame after a pre-Opened IPCP packet: proto 0x%04x code %d err %v, want LCP Configure-Ack", proto, ack.Code, err)
	}
}

// TestProxyLCPPAPWaitsForRetransmittedRequest proves the accepted proxy path
// converges for PAP: Ze sends nothing after the short-circuit and answers the
// Authenticate-Request the client keeps sending with an Authenticate-Ack.
//
// VALIDATES: spec-home-cpe-15 A-4 (today's default for proxied PAP).
func TestProxyLCPPAPWaitsForRetransmittedRequest(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 1106)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	d := makeTestDriver(&fakeBackend{}, ops)
	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer d.Stop()

	papProxy := buildOptionStream([]LCPOption{mruOpt(1500), authProtoOpt(ProtoPAP), magicOpt(0xCAFEBABE)})
	d.SessionsIn() <- proxyStart(1106, papProxy, false)

	up := waitLCPUp(t, d.EventsOut())
	if up.ProxyLCP != ProxyLCPOutcomeAccepted || up.NegotiatedAuthMethod != AuthMethodPAP {
		t.Fatalf("EventLCPUp = %+v, want accepted PAP", up)
	}
	// RFC 1334 Section 2 has the client repeat its Authenticate-Request; the
	// one sent through the bridge is the request Ze answers.
	user, pass := []byte("user@example.net"), []byte("secret")
	body := make([]byte, 0, 2+len(user)+len(pass))
	body = append(body, byte(len(user)))
	body = append(body, user...)
	body = append(body, byte(len(pass)))
	body = append(body, pass...)
	out := make([]byte, MaxFrameLen)
	off := WriteFrame(out, 0, ProtoPAP, nil)
	out[off] = PAPAuthenticateRequest
	out[off+1] = 1
	binary.BigEndian.PutUint16(out[off+2:], uint16(4+len(body)))
	off += 4 + copy(out[off+4:], body)
	if _, err := pair.peerEnd.Write(out[:off]); err != nil {
		t.Fatalf("peer write PAP: %v", err)
	}
	proto, payload := readDriverFrame(t, pair.peerEnd)
	if proto != ProtoPAP || len(payload) == 0 || payload[0] != PAPAuthenticateAck {
		t.Fatalf("answer to the Authenticate-Request: proto 0x%04x % x, want PAP Authenticate-Ack", proto, payload)
	}
}
