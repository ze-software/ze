// VALIDATES: RFC 7296 Section 2.5 major-version obligations. Ze supports exactly one major
// version, 2. The set is a singleton at both inbound entry points, port 500 and port 4500.
// Every message Ze builds carries major version 2, so a request Ze accepts is answered with
// the version number it arrived under.
// PREVENTS: a dispatch change that accepts a second major version. A second version makes
// the version-interval rule and the downgrade-recovery rule reachable, and Ze implements
// neither.
package engine

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// majorVersionSweep drives one dispatch loop with every major version from 0 to 15. Each
// rejected version goes first and version 2 goes last. UDP on loopback keeps order and each
// dispatch loop is one goroutine, so a single delivery proves the other fifteen were
// dropped rather than delayed.
//
// wrap adapts an IKE message for the port under test. Port 500 sends the message as is.
// Port 4500 prepends the four-octet non-ESP marker of RFC 3948 Section 2.2.
func majorVersionSweep(
	t *testing.T,
	dispatch func(*transport.UDPTransport, *SATable, *slog.Logger),
	wrap func([]byte) []byte,
) {
	t.Helper()
	log := slogutil.DiscardLogger()

	tr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	table := NewSATable()
	sa := testSA()
	spi, err := GenerateSPI()
	if err != nil {
		t.Fatalf("GenerateSPI: %v", err)
	}
	sa.InitiatorSPI = spi
	sa.PeerName = "version-sweep"
	table.Insert(sa)

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, 32)}
	ps.ownedSA.Store(sa)
	SetActivePeersForTest(map[string]*PeerSession{sa.PeerName: ps})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	go dispatch(tr, table, log)

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("transport LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	datagram := func(major uint8) []byte {
		msg := wire.Message{Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			MajorVersion: major,
			ExchangeType: wire.ExchangeInformational,
			Flags:        wire.FlagInitiator,
			MessageID:    7,
		}}
		buf := make([]byte, 512)
		n := msg.WriteTo(buf, 0)
		return wrap(buf[:n])
	}

	rejected := 0
	for major := range uint8(16) {
		if major == 2 {
			continue
		}
		if _, err := sender.Write(datagram(major)); err != nil {
			t.Fatalf("write major version %d: %v", major, err)
		}
		rejected++
	}
	if rejected != 15 {
		t.Fatalf("the sweep offered %d rejected versions, want 15", rejected)
	}
	if _, err := sender.Write(datagram(2)); err != nil {
		t.Fatalf("write major version 2: %v", err)
	}

	select {
	case pkt := <-ps.inbound:
		if got := pkt.Data[17] >> 4; got != 2 {
			t.Errorf("the delivered datagram has major version %d, want 2", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the version-2 datagram never arrived, so the version gate cannot be judged")
	}

	// Nothing else CAN follow. All fifteen rejected datagrams were ahead of it in the queue.
	extra := 0
	for {
		select {
		case pkt := <-ps.inbound:
			extra++
			t.Errorf("major version %d was delivered, and the supported set must hold only "+
				"version 2", pkt.Data[17]>>4)
			continue
		default:
		}
		break
	}
	if extra > 0 {
		t.Errorf("%d datagrams outside major version 2 reached an SA", extra)
	}
}

// rfc-test-change-approved: 2026-08-01 owner standing approval for
// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
//
// mvAcceptedVersions is NEW and removes nothing. The RFC7296-2.5-15 echo test compared a
// literal 2 it wrote against a literal 2 the builders write. That holds for an
// implementation that accepts only version 3 and answers 2. Ze MEASURES the supported
// version from the inbound gate now.
//
// mvAcceptedVersions MEASURES which major versions ze accepts. It offers all sixteen to the
// production inbound gate and reports which ones reach an SA.
//
// A test can then state that the version ze responds with is the version ze supports, and
// state it once. Two independent constants that agree prove only that they agree. The value
// here comes from dispatchInbound, a different producer from the builders under test.
func mvAcceptedVersions(t *testing.T) []uint8 {
	t.Helper()
	log := slogutil.DiscardLogger()

	tr, err := transport.NewUDPTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	go tr.Run()

	table := NewSATable()
	sa := testSA()
	spi, err := GenerateSPI()
	if err != nil {
		t.Fatalf("GenerateSPI: %v", err)
	}
	sa.InitiatorSPI = spi
	sa.PeerName = "version-measure"
	table.Insert(sa)

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, 32)}
	ps.ownedSA.Store(sa)
	SetActivePeersForTest(map[string]*PeerSession{sa.PeerName: ps})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	go dispatchInbound(tr, table, log)

	local, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("transport LocalAddr is not *net.UDPAddr")
	}
	sender, err := net.DialUDP("udp4", nil, local)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	t.Cleanup(func() { _ = sender.Close() })

	for major := range uint8(16) {
		msg := wire.Message{Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			MajorVersion: major,
			ExchangeType: wire.ExchangeInformational,
			Flags:        wire.FlagInitiator,
			MessageID:    7,
		}}
		buf := make([]byte, 512)
		n := msg.WriteTo(buf, 0)
		if _, err := sender.Write(buf[:n]); err != nil {
			t.Fatalf("write major version %d: %v", major, err)
		}
	}

	// UDP on loopback keeps order and one dispatch loop is one goroutine, so an accepted
	// version arrives in the order it was sent. Wait for the first, then drain.
	var accepted []uint8
	select {
	case pkt := <-ps.inbound:
		accepted = append(accepted, pkt.Data[17]>>4)
	case <-time.After(5 * time.Second):
		t.Fatal("ze accepted no major version at all, so nothing it answers can echo one")
	}
	for {
		select {
		case pkt := <-ps.inbound:
			accepted = append(accepted, pkt.Data[17]>>4)
			continue
		case <-time.After(200 * time.Millisecond): // settle window; every datagram was queued ahead of the one already read
		}
		break
	}
	return accepted
}

// RFC requirement: RFC7296-2.5-14 positive -- ze's supported major-version set is the singleton
// {2}. dispatchInbound (register.go) drops every datagram whose octet-17 high nibble is not 2,
// and the sweep below proves that over all sixteen major versions. A singleton has no interior,
// so "all versions between n and m" holds with nothing left to support. This is OR-D's
// discharge by proof: the row stays gated, so a second supported version cannot pass unnoticed.
// RFC requirement: RFC7296-2.5-16 positive -- the mistaken-downgrade state this MUST governs is
// unreachable, and its remedy has no target. The sweep rejects version 0, version 1, and
// versions 3 to 15, so the set is the singleton {2}. Ze never announces a higher
// capability, because no builder sets wire.FlagVersion, which this test asserts over every
// built message. RFC 7296 defines version 2.0, and no IKEv3 exists for the reconnect the MUST
// orders. The argument depends on RFC7296-3.1-11: a V bit ze set or read would make this
// antecedent reachable. This is not RFC7296-2.5-2, which covers a HIGHER major version only.
func TestSupportedMajorVersionSetIsSingleton(t *testing.T) {
	majorVersionSweep(t, dispatchInbound, func(raw []byte) []byte { return raw })

	// Send side: every message ze builds names version 2, and none announces a higher
	// capability through the V bit.
	built := 0
	for _, m := range engineBuiltMessages(t) {
		built++
		if got := m.raw[17] >> 4; got != 2 {
			t.Errorf("%s: major version = %d, want 2", m.name, got)
		}
		if m.raw[19]&wire.FlagVersion != 0 {
			t.Errorf("%s: the V bit is set, so ze announces a higher major version", m.name)
		}
	}
	if built == 0 {
		t.Fatal("the built-message set is empty, so the send-side assertion is vacuous")
	}
}

// RFC requirement: RFC7296-2.5-14 negative -- the singleton claim is a claim about EVERY entry
// point, not about one. dispatchNATTInbound (register.go) applies the same equality test on
// port 4500, after StripNonESPMarker. TestHigherMajorVersionDropped drives port 500 only, so
// this is the only proof covering the second producer of the same rule.
// RFC requirement: RFC7296-2.5-16 negative -- one entry point accepting a second major version
// would make the downgrade antecedent reachable while ze implements no recovery. The NAT-T
// path is the entry point a port-500-only test leaves unproven.
func TestNATTDispatchAppliesTheSameVersionGate(t *testing.T) {
	majorVersionSweep(t, dispatchNATTInbound, transport.AddNonESPMarker)
}

// RFC requirement: RFC7296-2.5-15 positive -- a message arriving at a major version ze supports
// is answered with that version number. The only supported version is 2. handleSAInitRequest
// answers a version-2 request through buildSAInitResponse (responder.go), which names
// MajorVersion 2, and writeAuthHeaderWithMsgID (auth.go) names it for every later exchange. The
// test asserts the request really carried version 2 before the comparison. So the echo is a
// match rather than a comparison over an input nobody sent.
// RFC requirement: RFC7296-2.5-15 negative -- wire.Header.WriteTo packs whatever major version it
// is given into the high nibble of octet 17. An explicit 5 reaches the wire as 5. The constant
// 2 in the builders is therefore their decision, not a limit of the encoder.
func TestResponderEchoesTheSupportedMajorVersion(t *testing.T) {
	// rfc-test-change-approved: 2026-08-01 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only. Nothing is removed.
	//
	// Ze MEASURES the supported version from the production inbound gate now. It is no
	// longer a second literal 2 beside the builders' own. RFC 7296 Section 2.5 makes the
	// requirement conditional: "a message with a major version THAT IT SUPPORTS" gets
	// "that version number" back. Two literals that agree hold equally for an
	// implementation that accepts only version 3 and answers 2. A measured antecedent
	// couples the two halves that the requirement couples.
	log := slogutil.DiscardLogger()

	accepted := mvAcceptedVersions(t)
	if len(accepted) != 1 {
		t.Fatalf("the inbound gate accepts %v; this test drives the single-version case", accepted)
	}
	supported := accepted[0]

	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "version-echo-psk")

	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, testIKEGroup())

	// Anti-vacuity: the request ze answers really carries the version ze SUPPORTS. The echo
	// assertion below then stands on the antecedent the RFC states. It does not stand on a
	// number this file chose.
	reqMajor := req[17] >> 4
	if reqMajor != supported {
		t.Fatalf("the IKE_SA_INIT request carries major version %d and the inbound gate "+
			"accepts %d; ze would answer a version it never accepts", reqMajor, supported)
	}

	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("responder state = %v, want sa-init-received", resp.State)
	}
	if got := resp.LastSentMsg[17] >> 4; got != reqMajor {
		t.Errorf("the response carries major version %d, and the request carried %d",
			got, reqMajor)
	}

	// rfc-test-change-approved: 2026-08-01 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
	//
	// Every later response repeats the same version number. That number is the MEASURED
	// supported one. It is not a literal this file wrote beside the builders' own.
	responses := 0
	for _, m := range engineBuiltMessages(t) {
		if !m.isResponse {
			continue
		}
		responses++
		if got := m.raw[17] >> 4; got != supported {
			t.Errorf("%s: major version = %d, and the inbound gate accepts %d", m.name, got, supported)
		}
	}
	if responses == 0 {
		t.Fatal("the built-message set holds no response, so the echo assertion is vacuous")
	}

	// The encoder writes whatever major version it is given, so the 2 above is a decision of
	// the builders rather than a limit of the encoder.
	h := wire.Header{MajorVersion: 5, ExchangeType: wire.ExchangeIKESAInit}
	buf := make([]byte, wire.HeaderLen)
	h.WriteTo(buf, 0)
	if got := buf[17] >> 4; got != 5 {
		t.Errorf("the encoder wrote major version %d for an explicit 5, so a constant 2 "+
			"proves nothing", got)
	}
}
