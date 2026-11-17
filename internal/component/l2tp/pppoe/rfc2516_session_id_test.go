// VALIDATES: RFC 2516 SESSION_ID and error-TAG rules on the access
// concentrator side: which value each discovery packet carries in its
// SESSION_ID field, what the AC does with a packet carrying the wrong one,
// the reserved 0xffff, and the data the AC's error TAGs carry.
// PREVENTS: a builder or handler that puts a live SESSION_ID in a discovery
// packet, a table that hands out 0xffff, or an error TAG that carries
// non-UTF-8 data.

package pppoe

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// sidOf reads the SESSION_ID field straight out of a built frame, so the
// assertion does not depend on ParseDiscovery agreeing with the builder.
func sidOf(frame []byte) uint16 {
	return binary.BigEndian.Uint16(frame[EthHdrLen+2 : EthHdrLen+4])
}

// newRecordingServer builds an InterfaceServer whose frames land in the
// returned slice. Its cookie key is the zero key, read back as s.cookieKey by
// a test that mints a cookie.
func newRecordingServer(sent *[][]byte, serviceNames []string, maxPerMAC int) *InterfaceServer {
	s := &InterfaceServer{
		ifName:            "eth0",
		hwAddr:            [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		sessions:          newSessionTable("eth0", 100),
		cookieKey:         CookieKey{},
		cookieTimeout:     30 * time.Second,
		serviceNames:      serviceNames,
		maxSessionsPerMAC: maxPerMAC,
		logger:            slog.Default(),
		sendFrameFn: func(frame []byte) {
			*sent = append(*sent, frame)
		},
	}
	return s
}

// RFC requirement: RFC2516-4-1 positive — AllocSID on a fresh table returns a usable session ID, in the range 1 to 0xfffe.
func TestRFC2516AllocSIDReturnsUsableValue(t *testing.T) {
	st := newSessionTable("eth0", 0)
	sid, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	if sid == 0 || sid == reservedSID {
		t.Fatalf("AllocSID = 0x%04x, want a value in 1..0x%04x", sid, reservedSID-1)
	}
}

// RFC requirement: RFC2516-4-1 negative — the session table never hands out 0xffff: allocating every ID yields 0xfffe values none of which is 0xffff, the next call is ErrSIDExhausted rather than 0xffff, and freeing 0xffff does not reopen it.
func TestRFC2516SessionID0xffffIsNeverAllocated(t *testing.T) {
	st := newSessionTable("eth0", 0)
	for i := range usableSIDs {
		sid, err := st.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID #%d: %v", i+1, err)
		}
		if sid == reservedSID {
			t.Fatalf("AllocSID #%d = 0x%04x, the value RFC 2516 Section 4 reserves", i+1, sid)
		}
	}
	if sid, err := st.AllocSID(); !errors.Is(err, ErrSIDExhausted) {
		t.Fatalf("AllocSID after %d allocations = (0x%04x, %v), want ErrSIDExhausted", usableSIDs, sid, err)
	}
	st.freeSID(reservedSID)
	if sid, err := st.AllocSID(); !errors.Is(err, ErrSIDExhausted) {
		t.Fatalf("AllocSID after freeSID(0xffff) = (0x%04x, %v), want ErrSIDExhausted: the reserved bit must stay allocated", sid, err)
	}
}

// RFC requirement: RFC2516-5.1-4 positive — BuildPADI writes SESSION_ID 0x0000.
func TestRFC2516PADISessionIDIsZero(t *testing.T) {
	var buf [EthMaxLen]byte
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	frame := BuildPADI(buf[:], src, "svc", []byte{1, 2, 3, 4})
	if frame == nil {
		t.Fatal("BuildPADI returned nil")
	}
	if frame[EthHdrLen+1] != CodePADI {
		t.Fatalf("CODE = 0x%02x, want PADI 0x%02x", frame[EthHdrLen+1], CodePADI)
	}
	if got := sidOf(frame); got != 0 {
		t.Fatalf("PADI SESSION_ID = 0x%04x, want 0x0000", got)
	}
}

// RFC requirement: RFC2516-5.1-4 negative — handlePADI answers a PADI whose SESSION_ID is 0x0000 with a PADO and answers one carrying any other SESSION_ID with nothing.
func TestRFC2516PADIWithNonZeroSessionIDIsDropped(t *testing.T) {
	var sent [][]byte
	s := newRecordingServer(&sent, nil, 8)
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	s.handlePADI(&Packet{Code: CodePADI, SrcMAC: src, SID: 0x0001, Tags: []Tag{{Type: TagServiceName}}})
	if len(sent) != 0 {
		t.Fatalf("PADI with SESSION_ID 0x0001 drew %d frame(s), want none", len(sent))
	}

	s.handlePADI(&Packet{Code: CodePADI, SrcMAC: src, SID: 0, Tags: []Tag{{Type: TagServiceName}}})
	if len(sent) != 1 {
		t.Fatalf("PADI with SESSION_ID 0x0000 drew %d frame(s), want one PADO", len(sent))
	}
	if sent[0][EthHdrLen+1] != CodePADO {
		t.Fatalf("reply CODE = 0x%02x, want PADO 0x%02x", sent[0][EthHdrLen+1], CodePADO)
	}
}

// RFC requirement: RFC2516-5.2-6 positive — BuildPADO writes SESSION_ID 0x0000.
func TestRFC2516PADOSessionIDIsZero(t *testing.T) {
	var buf [EthMaxLen]byte
	ac := [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	padi := &Packet{Code: CodePADI, SrcMAC: [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, Tags: []Tag{{Type: TagServiceName}}}
	frame := BuildPADO(buf[:], ac, padi, "ze", nil, []byte("cookie"))
	if frame == nil {
		t.Fatal("BuildPADO returned nil")
	}
	if frame[EthHdrLen+1] != CodePADO {
		t.Fatalf("CODE = 0x%02x, want PADO 0x%02x", frame[EthHdrLen+1], CodePADO)
	}
	if got := sidOf(frame); got != 0 {
		t.Fatalf("PADO SESSION_ID = 0x%04x, want 0x0000", got)
	}
}

// RFC requirement: RFC2516-5.3-5 positive — BuildPADR writes SESSION_ID 0x0000.
func TestRFC2516PADRSessionIDIsZero(t *testing.T) {
	var buf [EthMaxLen]byte
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	pado := &Packet{Code: CodePADO, SrcMAC: [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}}
	frame := BuildPADR(buf[:], src, pado, "svc", []byte{1, 2, 3, 4})
	if frame == nil {
		t.Fatal("BuildPADR returned nil")
	}
	if frame[EthHdrLen+1] != CodePADR {
		t.Fatalf("CODE = 0x%02x, want PADR 0x%02x", frame[EthHdrLen+1], CodePADR)
	}
	if got := sidOf(frame); got != 0 {
		t.Fatalf("PADR SESSION_ID = 0x%04x, want 0x0000", got)
	}
}

// RFC requirement: RFC2516-5.3-5 negative — handlePADR drops a PADR carrying a non-zero SESSION_ID: no frame is sent and no session is allocated, where the same PADR with SESSION_ID 0x0000 is answered.
func TestRFC2516PADRWithNonZeroSessionIDIsDropped(t *testing.T) {
	var sent [][]byte
	s := newRecordingServer(&sent, nil, 8)
	key := s.cookieKey
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	cookie := GenerateCookie(key, s.hwAddr[:], src[:], nil)
	// A Service-Name the AC refuses, so the SESSION_ID 0x0000 branch below
	// is observable as a PADS without needing a kernel to admit a session.
	s.serviceNames = []string{"only-this"}
	tags := []Tag{{Type: TagACCookie, Value: cookie}, {Type: TagServiceName, Value: []byte("other")}}

	s.handlePADR(&Packet{Code: CodePADR, SrcMAC: src, SID: 0x0001, Tags: tags})
	if len(sent) != 0 {
		t.Fatalf("PADR with SESSION_ID 0x0001 drew %d frame(s), want none", len(sent))
	}
	if got := s.sessions.Count(); got != 0 {
		t.Fatalf("sessions.Count() = %d after a dropped PADR, want 0", got)
	}

	s.handlePADR(&Packet{Code: CodePADR, SrcMAC: src, SID: 0, Tags: tags})
	if len(sent) != 1 {
		t.Fatalf("PADR with SESSION_ID 0x0000 drew %d frame(s), want one PADS", len(sent))
	}
	if sent[0][EthHdrLen+1] != CodePADS {
		t.Fatalf("reply CODE = 0x%02x, want PADS 0x%02x", sent[0][EthHdrLen+1], CodePADS)
	}
}

// RFC requirement: RFC2516-5.4-3 positive — the PADS handlePADR sends for an admitted session carries that session's SESSION_ID from the table, and BuildPADSError, which confirms no session, carries 0x0000.
func TestRFC2516PADSCarriesTheSessionsID(t *testing.T) {
	var sent [][]byte
	s := newRecordingServer(&sent, nil, 8)
	key := s.cookieKey
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	cookie := GenerateCookie(key, s.hwAddr[:], src[:], nil)

	sid, err := s.sessions.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	if err := s.sessions.Add(&Session{
		SID: sid, MAC: net.HardwareAddr(src[:]), IfName: "eth0",
		State: StateSession, Cookie: append([]byte(nil), cookie...),
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	padr := &Packet{Code: CodePADR, SrcMAC: src, Tags: []Tag{{Type: TagACCookie, Value: cookie}, {Type: TagServiceName}}}
	s.handlePADR(padr)
	if len(sent) != 1 {
		t.Fatalf("handlePADR sent %d frame(s), want one PADS", len(sent))
	}
	if sent[0][EthHdrLen+1] != CodePADS {
		t.Fatalf("reply CODE = 0x%02x, want PADS 0x%02x", sent[0][EthHdrLen+1], CodePADS)
	}
	if got := sidOf(sent[0]); got != sid {
		t.Fatalf("PADS SESSION_ID = 0x%04x, want the session's 0x%04x", got, sid)
	}

	var buf [EthMaxLen]byte
	refusal := BuildPADSError(buf[:], s.hwAddr, padr, "ze", TagSvcNameError)
	if refusal == nil {
		t.Fatal("BuildPADSError returned nil")
	}
	if got := sidOf(refusal); got != 0 {
		t.Fatalf("refusing PADS SESSION_ID = 0x%04x, want 0x0000", got)
	}
}

// RFC requirement: RFC2516-5.5-3 positive -- the PADT sent after the original
// worker's actual LCP termination carries that admission's SID and unicast MAC.
// RFC requirement: RFC2516-7-3 positive -- an actual LCP Terminate-Request
// reaches the original worker's genuine Down and existing event consumer;
// its native transport closes before PADT, the worker retires and another
// admitted session remains writable and answers Echo.
func TestRFC2516SessionDownSendsPADTForThatSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sent [][]byte
		s := newRecordingServer(&sent, nil, 8)
		mac := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
		s.ifIndex = 1
		sub := newTestSubsystem(t, newRecordBus(), s.ifIndex)
		sub.servers[s.ifIndex] = s

		sid, err := s.sessions.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID: %v", err)
		}
		if err := s.sessions.Add(&Session{SID: sid, MAC: mac, IfName: "eth0", State: StateSession}); err != nil {
			t.Fatalf("Add: %v", err)
		}
		original := s.sessions.Lookup(sid)
		ended := rfc2516AdmitTerminationSession(t, sub, s, original)
		otherSID, err := s.sessions.AllocSID()
		if err != nil {
			t.Fatal(err)
		}
		otherSession := &Session{SID: otherSID, MAC: net.HardwareAddr{2, 0, 0, 0, 0, 2}, IfName: "eth0", State: StateSession}
		if err := s.sessions.Add(otherSession); err != nil {
			t.Fatal(err)
		}
		other := rfc2516AdmitTerminationSession(t, sub, s, otherSession)
		record := s.sendFrameFn
		s.sendFrameFn = func(frame []byte) {
			if _, err := ended.native.WriteString("PPP"); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("original native transport still writable at PADT: %v", err)
			}
			record(frame)
		}

		ended.protocol.send(t, ppp.ProtoLCP, ppp.LCPTerminateRequest, 0x77, nil)
		lcpDown, ok := ended.protocol.next(t).(ppp.EventLCPDown)
		if !ok || lcpDown.Control != ended.protocol.control || !lcpDown.NetworkPhase {
			t.Fatal("actual Terminate-Request did not withdraw the original network lifetime")
		}
		sub.handlePPPEvent(lcpDown)
		ack := ended.protocol.read(t, ppp.ProtoLCP, ppp.LCPTerminateAck)
		if ack.Identifier != 0x77 {
			t.Fatalf("Terminate-Ack identifier = %d, want 0x77", ack.Identifier)
		}
		down, ok := ended.protocol.next(t).(ppp.EventSessionDown)
		if !ok || down.Control != ended.protocol.control || down.Reason != "LCP terminated by peer: stopped, lower layer released" {
			t.Fatalf("terminal event = %+v, want original worker's actual LCP termination", down)
		}
		sub.handlePPPEvent(down)

		if len(sent) != 1 {
			t.Fatalf("handleSessionDown sent %d frame(s), want one PADT", len(sent))
		}
		if sent[0][EthHdrLen+1] != CodePADT {
			t.Fatalf("CODE = 0x%02x, want PADT 0x%02x", sent[0][EthHdrLen+1], CodePADT)
		}
		if got := sidOf(sent[0]); got != sid {
			t.Fatalf("PADT SESSION_ID = 0x%04x, want 0x%04x", got, sid)
		}
		if got := net.HardwareAddr(sent[0][0:EthALen]); got.String() != mac.String() {
			t.Fatalf("PADT destination = %s, want the session's MAC %s", got, mac)
		}
		if s.sessions.Lookup(sid) != nil {
			t.Fatal("session is still in the table after LCP down")
		}
		if _, err := ended.native.WriteString("PPP"); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("PPP native write after LCP termination = %v, want closed original descriptor", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		status, err := ended.protocol.control.WaitRetired(ctx)
		if err != nil || !status.Retired || !status.Released || status.Err != nil {
			t.Fatalf("original worker did not finish its own retirement: %+v, %v", status, err)
		}
		if s.sessions.Lookup(otherSID) != otherSession {
			t.Fatal("LCP termination removed another session")
		}
		rfc2516AssertSurvivingSession(t, other)
		sub.handlePPPEvent(down)
		if len(sent) != 1 {
			t.Fatal("replayed old Down emitted another PADT")
		}
		rfc2516AssertSurvivingSession(t, other)
	})
}

// RFC requirement: RFC2516-5.5-3 negative -- a correctly addressed PADT naming
// an absent SID cannot terminate an actual live admitted session.
// RFC requirement: RFC2516-7-3 negative -- an unknown-session PADT and absent
// session-down report leave the actual live worker and native transport usable.
func TestRFC2516UnknownSessionIDTerminatesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sent [][]byte
		s := newRecordingServer(&sent, nil, 8)
		mac := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
		s.ifIndex = 1
		sub := newTestSubsystem(t, newRecordBus(), s.ifIndex)
		sub.servers[s.ifIndex] = s
		sid, err := s.sessions.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID: %v", err)
		}
		if err := s.sessions.Add(&Session{SID: sid, MAC: mac, IfName: "eth0", State: StateSession}); err != nil {
			t.Fatalf("Add: %v", err)
		}
		live := rfc2516AdmitTerminationSession(t, sub, s, s.sessions.Lookup(sid))
		other := sid + 1
		var src [EthALen]byte
		copy(src[:], mac)
		s.handlePADT(&Packet{Code: CodePADT, SID: other, SrcMAC: src, DstMAC: s.hwAddr})
		if s.sessions.Lookup(sid) == nil {
			t.Fatalf("PADT naming SESSION_ID 0x%04x removed session 0x%04x", other, sid)
		}
		s.handleSessionDown(s.sessions.Lookup(other))
		if len(sent) != 0 {
			t.Fatalf("session-down for unknown SESSION_ID 0x%04x sent %d frame(s), want none", other, len(sent))
		}
		if s.sessions.Lookup(sid) == nil {
			t.Fatalf("session-down for unknown SESSION_ID 0x%04x removed session 0x%04x", other, sid)
		}
		rfc2516AssertSurvivingSession(t, live)
	})
}

// RFC requirement: RFC2516-x-12 positive — the Service-Name-Error TAG in the PADS the AC sends for a Service-Name it will not serve has TAG_LENGTH 0, so its data never starts with a nonzero octet and no non-UTF-8 text can reach the wire.
// RFC requirement: RFC2516-x-13 positive — the AC-System-Error TAG in the PADS the AC sends when it is out of resources has TAG_LENGTH 0, so its data never starts with a nonzero octet and no non-UTF-8 text can reach the wire.
func TestRFC2516ErrorTagsCarryNoData(t *testing.T) {
	src := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	errorTagOf := func(t *testing.T, frame []byte, tagType uint16) Tag {
		t.Helper()
		pkt, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatalf("ParseDiscovery: %v", err)
		}
		if pkt.Code != CodePADS {
			t.Fatalf("CODE = 0x%02x, want PADS 0x%02x", pkt.Code, CodePADS)
		}
		tag := pkt.FindTag(tagType)
		if tag == nil {
			t.Fatalf("PADS carries no tag 0x%04x", tagType)
		}
		return *tag
	}

	// Service-Name-Error: the PADR names a service the AC does not offer.
	var sent [][]byte
	s := newRecordingServer(&sent, []string{"only-this"}, 8)
	key := s.cookieKey
	cookie := GenerateCookie(key, s.hwAddr[:], src[:], nil)
	s.handlePADR(&Packet{Code: CodePADR, SrcMAC: src, Tags: []Tag{
		{Type: TagACCookie, Value: cookie}, {Type: TagServiceName, Value: []byte("other")},
	}})
	if len(sent) != 1 {
		t.Fatalf("service-name refusal sent %d frame(s), want one PADS", len(sent))
	}
	if tag := errorTagOf(t, sent[0], TagSvcNameError); len(tag.Value) != 0 {
		t.Fatalf("Service-Name-Error TAG_LENGTH = %d, want 0", len(tag.Value))
	}

	// AC-System-Error: the per-MAC cap is zero, so every PADR is a resource refusal.
	sent = nil
	s = newRecordingServer(&sent, nil, 0)
	key = s.cookieKey
	cookie = GenerateCookie(key, s.hwAddr[:], src[:], nil)
	s.handlePADR(&Packet{Code: CodePADR, SrcMAC: src, Tags: []Tag{
		{Type: TagACCookie, Value: cookie}, {Type: TagServiceName},
	}})
	if len(sent) != 1 {
		t.Fatalf("resource refusal sent %d frame(s), want one PADS", len(sent))
	}
	if tag := errorTagOf(t, sent[0], TagACSystemError); len(tag.Value) != 0 {
		t.Fatalf("AC-System-Error TAG_LENGTH = %d, want 0", len(tag.Value))
	}
}

type rfc2516TerminationSession struct {
	protocol *fixtureCurrentPPP
	native   *os.File
}

// The transport is an actual native pipe writer; its read peer remains open.
// The production OpenedSession owns its Close, separately from PPP's handle.
// The actual Driver alone emits the admission, provider request and Down.
// This boundary proof is not a claim about Linux PPP packet forwarding.
func rfc2516AdmitTerminationSession(t *testing.T, sub *Subsystem, server *InterfaceServer, session *Session) rfc2516TerminationSession {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	handle := &fixtureSessionHandle{identity: ppp.SessionIdentity{InterfaceName: "rfc2516-termination"}}
	opened, err := dataplane.NewOpenedSession(&fixtureFrames{}, handle, writer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = opened.CloseTransport() })
	session.Opened = opened
	protocol := fixtureCurrentStart(t, server, session, &fixtureCurrentStartConfig{method: ppp.AuthMethodNone, dual: false})
	server.pppDriver = protocol.driver
	protocol.authenticate(t, &fixtureCurrentAuthConfig{username: ""})
	up := protocol.up(t)
	if up.Control != protocol.control {
		t.Fatal("actual Driver changed its admitted owner")
	}
	sub.handlePPPEvent(up)
	if !up.Control.Active() {
		t.Fatal("actual admitted worker is not live")
	}
	if _, err := writer.WriteString("live"); err != nil {
		t.Fatalf("original native transport was not writable before termination: %v", err)
	}
	return rfc2516TerminationSession{protocol: protocol, native: writer}
}

func rfc2516AssertSurvivingSession(t *testing.T, owner rfc2516TerminationSession) {
	t.Helper()
	if !owner.protocol.control.Active() {
		t.Fatal("another live admission was canceled")
	}
	if _, err := owner.native.WriteString("PPP"); err != nil {
		t.Fatalf("another session's original native transport closed: %v", err)
	}
	owner.protocol.send(t, ppp.ProtoLCP, ppp.LCPEchoRequest, 0x51, []byte{0, 0, 0, 0})
	reply := owner.protocol.read(t, ppp.ProtoLCP, ppp.LCPEchoReply)
	if reply.Identifier != 0x51 {
		t.Fatalf("surviving session Echo identifier = %d, want 0x51", reply.Identifier)
	}
}
