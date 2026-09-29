// VALIDATES: RFC 2516 access concentrator admission, driven through
// handlePADI and handlePADR: what the PADO and PADS carry, which SESSION_ID
// and peer MAC the session stage is bound to, and what a Service-Name the AC
// does not serve draws.
// PREVENTS: a PADS naming a SESSION_ID other than the admitted session's, a
// session socket bound to another SID or another peer, a PADO for a service
// the AC does not offer, and a refusal PADS that drops the Relay-Session-Id.

package pppoe

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// admissionBackend satisfies ppp.IfaceBackend for an unstarted driver: the
// tests only enqueue StartSession, so no backend method is ever called.
type admissionBackend struct{ ppp.IfaceBackend }

// boundTransport is one session-stage socket handlePADR asked the kernel for.
type boundTransport struct {
	sid       uint16
	remoteMAC [EthALen]byte
}

// newAdmittingServer builds a recording server that admits sessions: the
// kernel setup is replaced by recorders, and the PPP driver is constructed but
// never started, so StartSession lands in its buffered queue.
func newAdmittingServer(t *testing.T, sent *[][]byte, bound *[]boundTransport, serviceNames []string) *InterfaceServer {
	t.Helper()
	s := newRecordingServer(sent, serviceNames, 8)
	s.acName = "ze-ac"
	s.pppDriver = ppp.NewProductionDriver(slog.Default(), admissionBackend{})
	s.pppoeCreateFn = func(_ string, sid uint16, remoteMAC [EthALen]byte) (int, error) {
		*bound = append(*bound, boundTransport{sid: sid, remoteMAC: remoteMAC})
		return -1, nil
	}
	s.devPPPSetupFn = func(int) (int, int, int, error) { return -1, -1, 7, nil }
	return s
}

// padrFrom builds the PADR a host at src sends after a PADO from s, carrying
// a valid AC-Cookie and the given Service-Name, Host-Uniq and Relay-Session-Id
// (nil omits the tag).
func padrFrom(s *InterfaceServer, src [EthALen]byte, serviceName string, hostUniq, relay []byte) *Packet {
	cookie := GenerateCookie(s.cookieKey, s.hwAddr[:], src[:], relay)
	pkt := &Packet{Code: CodePADR, DstMAC: s.hwAddr, SrcMAC: src, Tags: []Tag{
		{Type: TagServiceName, Value: []byte(serviceName)},
		{Type: TagACCookie, Value: cookie},
	}}
	if hostUniq != nil {
		pkt.Tags = append(pkt.Tags, Tag{Type: TagHostUniq, Value: hostUniq})
	}
	if relay != nil {
		pkt.Tags = append(pkt.Tags, Tag{Type: TagRelaySessionID, Value: relay})
	}
	return pkt
}

// parseSent parses one frame the server sent, failing the test on error.
func parseSent(t *testing.T, frame []byte) *Packet {
	t.Helper()
	pkt, err := ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery of a sent frame: %v", err)
	}
	return &pkt
}

// RFC requirement: RFC2516-5.4-3 positive — handlePADR admits two hosts; each gets one PADS with CODE 0x65 whose SESSION_ID is non-zero, equals the SID the table holds for that host, and differs from the other host's.
// RFC requirement: RFC2516-7-1 positive — for each admitted session the session-stage socket is bound to the same SESSION_ID the PADS assigned, once, and the table keeps that SID for the session.
// RFC requirement: RFC2516-x-6 positive — for each admitted session the session-stage socket is bound to the PADR's unicast source MAC, the peer address determined in Discovery.
// RFC requirement: RFC2516-5.4-2 negative — a PADR naming a Service-Name the AC serves draws a PADS with a non-zero SESSION_ID and no Service-Name-Error TAG.
// RFC requirement: RFC2516-5.2-1 positive — the PADS handlePADR sends for each admitted host carries that host's PADR Host-Uniq TAG with its value unmodified.
func TestRFC2516AdmittedPADSCarriesTheNewSessionsID(t *testing.T) {
	var sent [][]byte
	var bound []boundTransport
	s := newAdmittingServer(t, &sent, &bound, []string{"internet"})
	hosts := [][EthALen]byte{
		{0x02, 0x00, 0x00, 0x00, 0x00, 0x01},
		{0x02, 0x00, 0x00, 0x00, 0x00, 0x02},
	}

	sids := make([]uint16, 0, len(hosts))
	for i, host := range hosts {
		hostUniq := []byte{0x48, 0x55, byte(i), 0xff}
		s.handlePADR(padrFrom(s, host, "internet", hostUniq, nil))
		if len(sent) != i+1 {
			t.Fatalf("host %d: handlePADR sent %d frame(s) in all, want %d", i, len(sent), i+1)
		}
		if code := sent[i][EthHdrLen+1]; code != CodePADS {
			t.Fatalf("host %d: CODE = 0x%02x, want PADS 0x%02x", i, code, CodePADS)
		}
		pads := parseSent(t, sent[i])
		if pads.FindTag(TagSvcNameError) != nil {
			t.Fatalf("host %d: PADS for a served Service-Name carries Service-Name-Error", i)
		}
		if echoed := pads.FindTag(TagHostUniq); echoed == nil || !bytes.Equal(echoed.Value, hostUniq) {
			t.Fatalf("host %d: PADS does not carry the PADR's Host-Uniq %x unmodified", i, hostUniq)
		}
		held := s.sessions.sessionsByMAC(net.HardwareAddr(host[:]))
		if len(held) != 1 {
			t.Fatalf("host %d: table holds %d session(s) for the host, want 1", i, len(held))
		}
		sid := sidOf(sent[i])
		if sid == 0 {
			t.Fatalf("host %d: PADS SESSION_ID = 0x0000 for an admitted session", i)
		}
		if sid != held[0].SID {
			t.Fatalf("host %d: PADS SESSION_ID = 0x%04x, table holds 0x%04x", i, sid, held[0].SID)
		}
		if len(bound) != i+1 {
			t.Fatalf("host %d: %d session socket(s) bound in all, want %d", i, len(bound), i+1)
		}
		if bound[i].sid != sid {
			t.Fatalf("host %d: session socket bound to SESSION_ID 0x%04x, PADS assigned 0x%04x", i, bound[i].sid, sid)
		}
		if bound[i].remoteMAC != host {
			t.Fatalf("host %d: session socket bound to peer %s, want the PADR source %s",
				i, net.HardwareAddr(bound[i].remoteMAC[:]), net.HardwareAddr(host[:]))
		}
		sids = append(sids, sid)
	}
	if sids[0] == sids[1] {
		t.Fatalf("two sessions share SESSION_ID 0x%04x", sids[0])
	}
}

// RFC requirement: RFC2516-7-1 negative — a retransmitted PADR for an admitted session draws a PADS with the same SESSION_ID, binds no second socket and adds no session, so the session's SESSION_ID never changes.
func TestRFC2516PADRReplayKeepsTheSessionID(t *testing.T) {
	var sent [][]byte
	var bound []boundTransport
	s := newAdmittingServer(t, &sent, &bound, nil)
	host := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	padr := padrFrom(s, host, "", nil, nil)

	s.handlePADR(padr)
	s.handlePADR(padr)

	if len(sent) != 2 {
		t.Fatalf("two PADRs drew %d frame(s), want two PADS", len(sent))
	}
	first, second := sidOf(sent[0]), sidOf(sent[1])
	if first == 0 {
		t.Fatal("first PADS SESSION_ID = 0x0000")
	}
	if second != first {
		t.Fatalf("replayed PADR drew SESSION_ID 0x%04x, the session holds 0x%04x", second, first)
	}
	if len(bound) != 1 {
		t.Fatalf("%d session socket(s) bound, want the one bound at admission", len(bound))
	}
	if n := s.sessions.Count(); n != 1 {
		t.Fatalf("table holds %d session(s), want 1", n)
	}
}

// RFC requirement: RFC2516-x-6 negative — a PADR whose source MAC is broadcast or multicast is refused by ParseDiscovery, the gate the discovery reader applies before HandleDiscovery, so no session socket is bound to a non-unicast peer; the same PADR from a unicast source binds one to that source.
func TestRFC2516SessionNeverBoundToANonUnicastPeer(t *testing.T) {
	var sent [][]byte
	var bound []boundTransport
	s := newAdmittingServer(t, &sent, &bound, nil)

	// dispatch mirrors Subsystem.discoveryReader: parse, then hand over.
	dispatch := func(src [EthALen]byte) {
		pkt := padrFrom(s, src, "", nil, nil)
		var buf [EthMaxLen]byte
		b := NewBuilder(buf[:], src, s.hwAddr, CodePADR, 0)
		for i := range pkt.Tags {
			b.AddTagCopy(&pkt.Tags[i])
		}
		frame := b.Finish()
		parsed, err := ParseDiscovery(frame)
		if err != nil {
			return
		}
		s.HandleDiscovery(&parsed)
	}

	dispatch([EthALen]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	dispatch([EthALen]byte{0x01, 0x00, 0x5e, 0x00, 0x00, 0x01})
	if len(bound) != 0 {
		t.Fatalf("a non-unicast source bound %d session socket(s) to %s", len(bound), net.HardwareAddr(bound[0].remoteMAC[:]))
	}

	unicast := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x09}
	dispatch(unicast)
	if len(bound) != 1 {
		t.Fatalf("a unicast source bound %d session socket(s), want 1", len(bound))
	}
	if bound[0].remoteMAC != unicast {
		t.Fatalf("session socket bound to %s, want %s", net.HardwareAddr(bound[0].remoteMAC[:]), net.HardwareAddr(unicast[:]))
	}
}

// RFC requirement: RFC2516-5.4-2 positive — handlePADR answers a PADR carrying a Service-Name the AC does not serve with one PADS carrying a Service-Name-Error TAG and SESSION_ID 0x0000, and admits no session.
// RFC requirement: RFC2516-5.2-4 negative — handlePADI answers a PADI naming a served Service-Name with one PADO, so the refusal is specific to a service the AC cannot serve.
// RFC requirement: RFC2516-5.2-4 positive — handlePADI sends nothing for a PADI naming a Service-Name the AC does not serve.
func TestRFC2516UnservedServiceNameIsRefused(t *testing.T) {
	var sent [][]byte
	var bound []boundTransport
	s := newAdmittingServer(t, &sent, &bound, []string{"internet"})
	host := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}

	s.handlePADI(&Packet{Code: CodePADI, DstMAC: [EthALen]byte(BroadcastMAC), SrcMAC: host, Tags: []Tag{{Type: TagServiceName, Value: []byte("gaming")}}})
	if len(sent) != 0 {
		t.Fatalf("PADI for an unserved Service-Name drew %d frame(s), want none", len(sent))
	}
	s.handlePADI(&Packet{Code: CodePADI, DstMAC: [EthALen]byte(BroadcastMAC), SrcMAC: host, Tags: []Tag{{Type: TagServiceName, Value: []byte("internet")}}})
	if len(sent) != 1 {
		t.Fatalf("PADI for a served Service-Name drew %d frame(s), want one PADO", len(sent))
	}
	if code := sent[0][EthHdrLen+1]; code != CodePADO {
		t.Fatalf("CODE = 0x%02x, want PADO 0x%02x", code, CodePADO)
	}

	sent = nil
	s.handlePADR(padrFrom(s, host, "gaming", nil, nil))
	if len(sent) != 1 {
		t.Fatalf("PADR for an unserved Service-Name drew %d frame(s), want one PADS", len(sent))
	}
	pads := parseSent(t, sent[0])
	if pads.Code != CodePADS {
		t.Fatalf("CODE = 0x%02x, want PADS 0x%02x", pads.Code, CodePADS)
	}
	if pads.FindTag(TagSvcNameError) == nil {
		t.Fatal("refusing PADS carries no Service-Name-Error TAG")
	}
	if got := sidOf(sent[0]); got != 0 {
		t.Fatalf("refusing PADS SESSION_ID = 0x%04x, want 0x0000", got)
	}
	if len(bound) != 0 || s.sessions.Count() != 0 {
		t.Fatalf("refused PADR bound %d socket(s) and left %d session(s)", len(bound), s.sessions.Count())
	}
}

// RFC requirement: RFC2516-5.2-2 positive — the PADO handlePADI sends carries exactly one AC-Name TAG holding the configured name, a first Service-Name TAG identical to the PADI's, and one Service-Name TAG for each other configured service.
func TestRFC2516PADOCarriesACNameAndServiceNames(t *testing.T) {
	var sent [][]byte
	var bound []boundTransport
	s := newAdmittingServer(t, &sent, &bound, []string{"internet", "voip"})
	host := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}

	s.handlePADI(&Packet{Code: CodePADI, DstMAC: [EthALen]byte(BroadcastMAC), SrcMAC: host, Tags: []Tag{{Type: TagServiceName, Value: []byte("voip")}}})
	if len(sent) != 1 {
		t.Fatalf("PADI drew %d frame(s), want one PADO", len(sent))
	}
	pado := parseSent(t, sent[0])
	names := pado.FindAllTags(TagACName)
	if len(names) != 1 {
		t.Fatalf("PADO carries %d AC-Name TAG(s), want exactly 1", len(names))
	}
	if got := string(names[0].Value); got != "ze-ac" {
		t.Fatalf("AC-Name = %q, want %q", got, "ze-ac")
	}
	services := pado.FindAllTags(TagServiceName)
	want := []string{"voip", "internet"}
	if len(services) != len(want) {
		t.Fatalf("PADO carries %d Service-Name TAG(s), want %d", len(services), len(want))
	}
	for i := range want {
		if got := string(services[i].Value); got != want[i] {
			t.Fatalf("Service-Name[%d] = %q, want %q", i, got, want[i])
		}
	}
}

// RFC requirement: RFC2516-x-5 positive — the refusing PADS handlePADR sends for an unserved Service-Name, and the admitting PADS, each carry the PADR's Relay-Session-Id TAG with its value unmodified.
// RFC requirement: RFC2516-x-5 negative — the refusing PADS for a PADR that carried no Relay-Session-Id carries none.
func TestRFC2516RefusingPADSEchoesRelaySessionID(t *testing.T) {
	var sent [][]byte
	var bound []boundTransport
	s := newAdmittingServer(t, &sent, &bound, []string{"internet"})
	host := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	relay := []byte{0x52, 0x45, 0x4c, 0x41, 0x59, 0x2d, 0x31, 0x32}

	s.handlePADR(padrFrom(s, host, "gaming", nil, relay))
	s.handlePADR(padrFrom(s, host, "internet", nil, relay))
	s.handlePADR(padrFrom(s, host, "gaming", nil, nil))
	if len(sent) != 3 {
		t.Fatalf("three PADRs drew %d frame(s), want three PADS", len(sent))
	}
	for i, frame := range sent[:2] {
		tag := parseSent(t, frame).FindTag(TagRelaySessionID)
		if tag == nil {
			t.Fatalf("PADS %d carries no Relay-Session-Id", i)
		}
		if !bytes.Equal(tag.Value, relay) {
			t.Fatalf("PADS %d Relay-Session-Id = %x, want %x", i, tag.Value, relay)
		}
	}
	if parseSent(t, sent[0]).FindTag(TagSvcNameError) == nil {
		t.Fatal("first PADS is not the Service-Name-Error refusal")
	}
	if tag := parseSent(t, sent[2]).FindTag(TagRelaySessionID); tag != nil {
		t.Fatalf("refusing PADS invented a Relay-Session-Id %x", tag.Value)
	}
}

// RFC requirement: RFC2516-x-7 positive — every discovery builder writes the MAC address it is given for the sending device into SOURCE_ADDR: the host's for PADI and PADR, the AC's for PADO, PADS, the refusing PADS and PADT.
func TestRFC2516BuildersWriteTheSendersMAC(t *testing.T) {
	hostMAC := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	acMAC := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0xac}
	fromHost := &Packet{SrcMAC: hostMAC, Tags: []Tag{{Type: TagServiceName}}}
	fromAC := &Packet{SrcMAC: acMAC, Tags: []Tag{{Type: TagACCookie, Value: []byte("c")}}}

	var buf [EthMaxLen]byte
	frames := []struct {
		name   string
		frame  func() []byte
		source [EthALen]byte
	}{
		{"PADI", func() []byte { return BuildPADI(buf[:], hostMAC, "", nil) }, hostMAC},
		{"PADR", func() []byte { return BuildPADR(buf[:], hostMAC, fromAC, "", nil) }, hostMAC},
		{"PADO", func() []byte { return BuildPADO(buf[:], acMAC, fromHost, "ze-ac", nil, []byte("c")) }, acMAC},
		{"PADS", func() []byte { return BuildPADS(buf[:], acMAC, fromHost, "ze-ac", 5) }, acMAC},
		{"refusing PADS", func() []byte { return BuildPADSError(buf[:], acMAC, fromHost, "ze-ac", TagSvcNameError) }, acMAC},
		{"PADT", func() []byte { return BuildPADT(buf[:], acMAC, hostMAC, 5, "ze-ac") }, acMAC},
	}
	for _, tc := range frames {
		frame := tc.frame()
		if frame == nil {
			t.Fatalf("%s: builder returned nil", tc.name)
		}
		if got := [EthALen]byte(frame[EthALen : 2*EthALen]); got != tc.source {
			t.Fatalf("%s: SOURCE_ADDR = %s, want %s", tc.name, net.HardwareAddr(got[:]), net.HardwareAddr(tc.source[:]))
		}
	}
}

// endOfListTags counts the End-Of-List TAGs in a built frame's tag list,
// walking the raw TLVs so the parser's own End-Of-List handling cannot hide one.
func endOfListTags(frame []byte) int {
	payload := frame[MinDiscFrame:]
	count := 0
	for off := 0; off+TagHdrLen <= len(payload); {
		if binary.BigEndian.Uint16(payload[off:off+2]) == TagEndOfList {
			count++
		}
		off += TagHdrLen + int(binary.BigEndian.Uint16(payload[off+2:off+4]))
	}
	return count
}

// RFC requirement: RFC2516-x-4 positive — the PADO and PADS Ze sends carry no End-Of-List TAG, so no End-Of-List with a non-zero TAG_LENGTH is ever sent.
// RFC requirement: RFC2516-x-4 negative — a PADI and a PADR whose tag list ends with an End-Of-List TAG of TAG_LENGTH 4 are answered with a PADO and a PADS that carry no End-Of-List TAG: the received one is never copied out.
func TestRFC2516NoSentFrameCarriesEndOfList(t *testing.T) {
	acMAC := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0xac}
	host := [EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	withEOL := func(code byte) []byte {
		var buf [EthMaxLen]byte
		b := NewBuilder(buf[:], host, acMAC, code, 0)
		b.AddTagString(TagServiceName, "")
		b.addTag(TagHostUniq, []byte{1, 2})
		b.addTag(TagEndOfList, []byte{9, 9, 9, 9})
		return append([]byte(nil), b.Finish()...)
	}

	var out [EthMaxLen]byte
	for _, code := range []byte{CodePADI, CodePADR} {
		plain, err := ParseDiscovery(withEOL(code))
		if err != nil {
			t.Fatalf("ParseDiscovery: %v", err)
		}
		var reply []byte
		if code == CodePADI {
			reply = BuildPADO(out[:], acMAC, &plain, "ze-ac", nil, []byte("c"))
		} else {
			reply = BuildPADS(out[:], acMAC, &plain, "ze-ac", 5)
		}
		if reply == nil {
			t.Fatalf("code 0x%02x: builder returned nil", code)
		}
		if n := endOfListTags(reply); n != 0 {
			t.Fatalf("reply to code 0x%02x carries %d End-Of-List TAG(s), want 0", code, n)
		}
	}
}
