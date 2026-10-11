// VALIDATES: the RFC 7296 Section 3.1 header obligations on every message the IKE engine
// emits and accepts. The list covers the SPI non-zero and zero rules (§3.1). It covers
// version numbers and the drop of a wrong version (§2.5, §3.1). It covers the X, R, V and I
// flag bits (§3.1), and SPI uniqueness (§2.6). Each test carries an `RFC requirement:` tag
// binding it to its checklist id.
// PREVENTS: a builder or dispatch change that emits a reserved flag bit, mislabels a
// request as a response, or starts accepting a version this implementation does not speak.
package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// xBitMask is the set of header flag bits RFC 7296 Section 3.1 marks 'X':
// |X|X|R|V|I|X|X|X| -- everything except R (0x20), V (0x10) and I (0x08).
const xBitMask uint8 = 0xff & ^(wire.FlagResponse | wire.FlagVersion | wire.FlagInitiator)

// engineBuiltMessages returns one wire message of every kind the engine emits, each paired
// with a label and whether it is a response. Every header obligation is asserted against
// this whole set rather than a single sample.
func engineBuiltMessages(t *testing.T) []struct {
	name       string
	raw        []byte
	isResponse bool
} {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)

	// A post-establishment INFORMATIONAL request and its response.
	probe := &wire.Message{Header: wire.Header{
		InitiatorSPI: resp.InitiatorSPI, ResponderSPI: resp.ResponderSPI,
		MajorVersion: 2, ExchangeType: wire.ExchangeInformational,
		Flags: wire.FlagInitiator, MessageID: resp.ExpectedMsgID,
	}}
	resp.lastResponse = nil
	resp.lastResponseSet = false
	ps.handleInformationalOwned(resp, probe, nil, false, nil, nil, log)
	if resp.lastResponse == nil {
		t.Fatal("no INFORMATIONAL response was built")
	}
	informationalResp := resp.lastResponse

	delReq, err := buildEncryptedMessageEx(ini,
		[]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}},
		ini.NextMsgID, wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}

	rekeyReq, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	t.Cleanup(pending.clear)

	return []struct {
		name       string
		raw        []byte
		isResponse bool
	}{
		{"IKE_SA_INIT request", ini.InitiatorSAInitMsg, false},
		{"IKE_SA_INIT response", resp.ResponderSAInitMsg, true},
		{"IKE_AUTH request", ini.LastSentMsg, false},
		{"IKE_AUTH response", resp.LastSentMsg, true},
		{"INFORMATIONAL response", informationalResp, true},
		{"INFORMATIONAL Delete request", delReq, false},
		{"CREATE_CHILD_SA rekey request", rekeyReq, false},
	}
}

// RFC requirement: RFC7296-3.1-5 positive -- every message the engine builds sets the major version
// to 2. buildSAInitRequest (initiator.go:75), buildSAInitResponse (responder.go:259) and
// writeAuthHeaderWithMsgID (auth.go:759) all set MajorVersion 2. Every encrypted exchange
// routes through writeAuthHeaderWithMsgID, and Header.WriteTo (header.go:39) packs the version
// into the high nibble.
// RFC requirement: RFC7296-3.1-6 positive -- the same builders set the minor version to 0. The low
// nibble of the version octet is therefore zero on every message ze emits.
//
// RFC requirement: RFC7296-3.1-5 negative -- version 2 is asserted on receipt too, not only on send.
// dispatchInbound drops a datagram whose major version is not 2 (register.go:645), so a
// non-version-2 message is never processed as IKEv2.
// RFC requirement: RFC7296-3.1-6 negative -- the minor nibble is written, not left to chance. An
// explicitly non-zero MinorVersion does reach the wire. The zero on every built message
// therefore comes from the builders, not from an encoder that cannot express anything else.
func TestBuiltMessagesCarryVersion2Point0(t *testing.T) {
	for _, m := range engineBuiltMessages(t) {
		if len(m.raw) < wire.HeaderLen {
			t.Fatalf("%s: %d octets, shorter than the header", m.name, len(m.raw))
		}
		if got := m.raw[17] >> 4; got != 2 {
			t.Errorf("%s: major version = %d, want 2", m.name, got)
		}
		if got := m.raw[17] & 0x0f; got != 0 {
			t.Errorf("%s: minor version = %d, want 0", m.name, got)
		}
	}

	// The encoder can express a non-zero minor version, so the zero above is a decision.
	h := wire.Header{MajorVersion: 2, MinorVersion: 3, ExchangeType: wire.ExchangeIKESAInit}
	buf := make([]byte, wire.HeaderLen)
	h.WriteTo(buf, 0)
	if buf[17]&0x0f != 3 {
		t.Errorf("encoded minor version = %d, want 3; the builders' zero must be a choice, "+
			"not an encoder limitation", buf[17]&0x0f)
	}
}

// RFC requirement: RFC7296-2.5-2 positive -- Ze drops a message whose major version is higher than 2.
// dispatchInbound tests the high nibble of octet 17 (register.go:645-647). It then continues,
// and it delivers the packet to no SA.
// RFC requirement: RFC7296-2.5-2 negative -- the drop is version-specific. An otherwise identical
// datagram whose major version IS 2 is delivered to the owning session. dispatchInbound does
// not discard everything.
func TestHigherMajorVersionDropped(t *testing.T) {
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
	sa.PeerName = "version-drop"
	table.Insert(sa)

	ps := &PeerSession{peerName: sa.PeerName, inbound: make(chan transport.Packet, 4)}
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
		return buf[:n]
	}

	// Send the version-3 datagram FIRST, then the version-2 one. UDP on loopback keeps order,
	// and dispatchInbound is a single goroutine. The version-2 packet on ps.inbound therefore
	// proves the version-3 packet was dropped rather than merely delayed.
	if _, err := sender.Write(datagram(3)); err != nil {
		t.Fatalf("Write version 3: %v", err)
	}
	if _, err := sender.Write(datagram(2)); err != nil {
		t.Fatalf("Write version 2: %v", err)
	}

	select {
	case pkt := <-ps.inbound:
		if got := pkt.Data[17] >> 4; got != 2 {
			t.Errorf("the delivered datagram has major version %d, want 2; a higher major "+
				"version must be dropped", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the version-2 datagram was never delivered, so the version check cannot be judged")
	}

	// Nothing else CAN follow. The version-3 datagram was ahead of it in the queue.
	select {
	case pkt := <-ps.inbound:
		t.Errorf("a second datagram was delivered (major version %d); only the version-2 one "+
			"may pass", pkt.Data[17]>>4)
	default:
	}
}

// RFC requirement: RFC7296-3.1-3 positive -- the initiator SPI is never zero. GenerateSPI redraws
// until the eight octets are not all zero (sa.go:179-189). newInitiatorSA and newResponderSA
// both take their SPI from it.
// RFC requirement: RFC7296-3.1-4 positive -- the responder SPI is zero in the first message of the
// initial exchange. newInitiatorSA leaves SA.ResponderSPI at its zero value, and
// buildSAInitRequest copies only the initiator SPI into the header (initiator.go:70-81).
// Octets 8..15 of the IKE_SA_INIT request are therefore zero.
//
// RFC requirement: RFC7296-3.1-4 negative -- the zero is confined to that first message. The
// IKE_SA_INIT RESPONSE and every later message carry a non-zero responder SPI, so a peer can
// name the SA.
func TestSPIZeroRules(t *testing.T) {
	// GenerateSPI never returns the zero SPI.
	for range 64 {
		spi, err := GenerateSPI()
		if err != nil {
			t.Fatalf("GenerateSPI: %v", err)
		}
		if spi == ([8]byte{}) {
			t.Fatal("GenerateSPI returned an all-zero SPI; the initiator SPI must not be zero")
		}
	}

	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "spi-psk")
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	if ini.InitiatorSPI == ([8]byte{}) {
		t.Error("a new initiator SA has an all-zero initiator SPI")
	}
	if ini.ResponderSPI != ([8]byte{}) {
		t.Error("a new initiator SA already carries a responder SPI; it is unknown until the " +
			"IKE_SA_INIT response arrives")
	}

	req := buildSAInitRequest(ini, testIKEGroup())
	for i := 8; i < 16; i++ {
		if req[i] != 0 {
			t.Errorf("IKE_SA_INIT request responder SPI octet %d = %02x, want 00", i-8, req[i])
			break
		}
	}
	nonZero := false
	for i := range 8 {
		if req[i] != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("IKE_SA_INIT request initiator SPI is all zeroes")
	}

	// Negative for 3.1-4: the response and later messages carry a responder SPI.
	log := slogutil.DiscardLogger()
	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	if resp.ResponderSPI == ([8]byte{}) {
		t.Fatal("a new responder SA has an all-zero responder SPI")
	}
	handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("responder state = %v, want sa-init-received", resp.State)
	}
	respRaw := resp.LastSentMsg
	allZero := true
	for i := 8; i < 16; i++ {
		if respRaw[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("the IKE_SA_INIT response carries an all-zero responder SPI; only the first " +
			"message of the exchange may")
	}
}

// RFC requirement: RFC7296-2.6-2 positive -- each endpoint's SPI is a unique identifier of an IKE SA.
// GenerateSPI draws eight random octets, so two SAs created back to back hold different SPIs.
// SATable keys on the PAIR (SPIPairKey, sa.go:202), so exactly one key addresses one SA.
// RFC requirement: RFC7296-2.6-2 negative -- uniqueness is enforced, not assumed. SATable.Insert
// refuses a second SA whose SPI pair is already present (table.go:19-28). It rejects the
// collision, and it does not overwrite the first SA.
func TestSPIsAreUniqueIdentifiers(t *testing.T) {
	seen := make(map[[8]byte]bool, 128)
	for range 128 {
		spi, err := GenerateSPI()
		if err != nil {
			t.Fatalf("GenerateSPI: %v", err)
		}
		if seen[spi] {
			t.Fatalf("GenerateSPI repeated %x; an SPI must uniquely identify an IKE SA", spi)
		}
		seen[spi] = true
	}

	table := NewSATable()
	a := testSA()
	a.InitiatorSPI = [8]byte{1, 1, 1, 1, 1, 1, 1, 1}
	a.ResponderSPI = [8]byte{2, 2, 2, 2, 2, 2, 2, 2}
	if !table.Insert(a) {
		t.Fatal("the first Insert failed")
	}
	if got := table.Lookup(a.InitiatorSPI, a.ResponderSPI); got != a {
		t.Error("the SA is not addressable by its own SPI pair")
	}

	// Negative: a duplicate pair is refused, and the original stays.
	b := testSA()
	b.InitiatorSPI = a.InitiatorSPI
	b.ResponderSPI = a.ResponderSPI
	if table.Insert(b) {
		t.Error("Insert accepted a second SA with the same SPI pair; the pair must identify " +
			"one SA")
	}
	if got := table.Lookup(a.InitiatorSPI, a.ResponderSPI); got != a {
		t.Error("the duplicate Insert replaced the original SA")
	}
}

// RFC requirement: RFC7296-3.1-7 positive -- the engine clears the X bits in every message it sends.
// Every message the engine builds carries flags drawn only from FlagInitiator and FlagResponse
// (initiator.go:78, responder.go:262, initiatorFlag rekey.go:45-50). A mask of the flags octet
// with the X-bit set therefore yields zero.
// RFC requirement: RFC7296-3.1-11 positive -- the engine clears the V bit in every message it sends.
// No builder sets wire.FlagVersion, so bit 4 is zero on every message ze emits.
//
// RFC requirement: RFC7296-3.1-7 negative -- "cleared" is scoped to the X bits. The defined R and I
// bits ARE set where the RFC requires them, so the flags octet is not always zero.
// RFC requirement: RFC7296-3.1-11 negative -- the V bit is expressible. wire.FlagVersion is a defined
// constant the encoder writes when asked. Its absence on every built message is therefore a
// decision, not an encoder that cannot set it.
func TestBuiltMessagesClearXAndVBits(t *testing.T) {
	sawInitiator := false
	sawResponse := false
	for _, m := range engineBuiltMessages(t) {
		flags := m.raw[19]
		if flags&xBitMask != 0 {
			t.Errorf("%s: flags = %08b, X bits (mask %08b) must be cleared when sending",
				m.name, flags, xBitMask)
		}
		if flags&wire.FlagVersion != 0 {
			t.Errorf("%s: flags = %08b, the V bit must be cleared when sending", m.name, flags)
		}
		if flags&wire.FlagInitiator != 0 {
			sawInitiator = true
		}
		if flags&wire.FlagResponse != 0 {
			sawResponse = true
		}
	}
	if !sawInitiator || !sawResponse {
		t.Errorf("the built-message set exercised I=%v R=%v; both defined bits must appear or "+
			"the X-bit assertion is vacuous", sawInitiator, sawResponse)
	}

	// The V bit is expressible, so its absence above is a choice.
	h := wire.Header{MajorVersion: 2, Flags: wire.FlagVersion}
	buf := make([]byte, wire.HeaderLen)
	h.WriteTo(buf, 0)
	if buf[19]&wire.FlagVersion == 0 {
		t.Error("the encoder cannot set the V bit, so clearing it proves nothing")
	}
}

// RFC requirement: RFC7296-3.1-9 positive -- the R bit is clear in every request and set in every
// response. Each request the engine builds passes flags without wire.FlagResponse. Each
// response ORs it in (handleInformationalOwned inbound.go:339, buildSAInitResponse
// responder.go:262).
// RFC requirement: RFC7296-3.1-12 negative -- handleResponderInbound does not process, and so
// does not answer, an IKE_SA_INIT request whose R bit is set: the responder stays idle. The
// same request with the R bit clear IS processed (sa-init-received), so the refusal is the
// R bit's doing.
func TestResponseBitMatchesDirection(t *testing.T) {
	for _, m := range engineBuiltMessages(t) {
		set := m.raw[19]&wire.FlagResponse != 0
		if set != m.isResponse {
			t.Errorf("%s: R bit set = %v, want %v", m.name, set, m.isResponse)
		}
	}

	// Negative: the receiver acts on the bit.
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rbit-psk")
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, testIKEGroup())

	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}

	// The same IKE_SA_INIT with the R bit forced on is not processed as a request.
	asResponse := append([]byte(nil), req...)
	asResponse[19] |= wire.FlagResponse
	ps.handleResponderInbound(resp, parseMsg(t, asResponse), transport.Packet{Data: asResponse}, nil, log)
	if resp.State != StateIdle {
		t.Errorf("responder state = %v after a message marked as a response, want idle; the R "+
			"bit must decide direction", resp.State)
	}

	// With the bit clear the same message IS processed.
	ps.handleResponderInbound(resp, parseMsg(t, req), transport.Packet{Data: req}, nil, log)
	if resp.State != StateSAInitReceived {
		t.Errorf("responder state = %v after a request, want sa-init-received", resp.State)
	}
}

// VALIDATES: a set X or V bit changes no receive decision, on each of the three sites that
// read the flags octet. The fresh IKE_SA_INIT request enters at tryResponderSAInit, the
// function dispatchInbound calls for a datagram with no SA, and the first site that reads
// pkt.Data[19]. The protected INFORMATIONAL request enters at handleOwnedInbound, which
// derives isResponse from the flags of an established SA's traffic. The IKE_SA_INIT
// response enters at handleInbound, the initiator's state machine, which tests the R bit
// before it hands the response on.
// RFC requirement: RFC7296-3.1-8 positive -- the receive path ignores the X bits. An
// IKE_SA_INIT request with all five X bits set creates one responder SA in
// sa-init-received and draws a response, as the clean request does. A protected
// INFORMATIONAL request with them set is answered and advances the expected Message ID.
// An IKE_SA_INIT response with them set moves the initiator to auth-sent.
// RFC requirement: RFC7296-3.1-11 negative -- the receive path ignores the V bit as well.
// The same three messages with the V bit set reach the same three outcomes, and the
// responder's answer to the fresh request still carries major version 2.
//
// RFC requirement: RFC7296-3.1-8 negative -- the tolerance is scoped to the X bits. The
// same IKE_SA_INIT request with the R bit set, which is not an X bit, is refused by
// tryResponderSAInit and creates no SA.
func TestXBitsIgnoredOnReceipt(t *testing.T) {
	log := slogutil.DiscardLogger()
	flagBits := []struct {
		name string
		mask uint8
	}{{"every X bit", xBitMask}, {"the V bit", wire.FlagVersion}}

	// The fresh request, through the dispatch entry for a datagram that names no SA.
	fresh := func(mutate func([]byte)) (bool, *SATable, *SA) {
		ps, table := sesRespondPeer(t)
		iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
		ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
		if err != nil {
			t.Fatalf("newInitiatorSA: %v", err)
		}
		req := buildSAInitRequest(ini, testIKEGroup())
		mutate(req)
		pkt := transport.Packet{
			Data:       req,
			RemoteAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 500},
		}
		var iSPI, rSPI [8]byte
		copy(iSPI[:], ini.InitiatorSPI[:])
		consumed := tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log)
		return consumed, table, ps.getSA()
	}

	consumed, table, clean := fresh(func([]byte) {})
	if !consumed || table.Len() != 1 || clean == nil || clean.State != StateSAInitReceived {
		t.Fatalf("baseline: consumed=%v SAs=%d; the clean IKE_SA_INIT did not reach sa-init-received",
			consumed, table.Len())
	}

	for _, bit := range flagBits {
		consumed, table, resp := fresh(func(raw []byte) { raw[19] |= bit.mask })
		if !consumed || table.Len() != 1 {
			t.Errorf("an IKE_SA_INIT with %s set: consumed=%v SAs=%d, want true and 1; it "+
				"must be accepted as the clean request is", bit.name, consumed, table.Len())
			continue
		}
		if resp == nil || resp.State != StateSAInitReceived {
			t.Errorf("an IKE_SA_INIT with %s set did not reach sa-init-received", bit.name)
			continue
		}
		if len(resp.LastSentMsg) <= wire.HeaderLen {
			t.Errorf("no response was built for an IKE_SA_INIT with %s set", bit.name)
			continue
		}
		if resp.LastSentMsg[17]>>4 != 2 {
			t.Errorf("an IKE_SA_INIT with %s set drew a response of major version %d, want 2",
				bit.name, resp.LastSentMsg[17]>>4)
		}
	}

	// An established SA. The flags travel inside the integrity check, so they are set
	// before the ICV is computed, as a peer that sets them would.
	link := errLink(t)
	for _, bit := range flagBits {
		msgID := link.resp.ExpectedMsgID
		req, err := buildEncryptedMessageEx(link.ini, nil, msgID, wire.ExchangeInformational,
			initiatorFlag(link.ini)|bit.mask)
		if err != nil {
			t.Fatalf("build INFORMATIONAL with %s: %v", bit.name, err)
		}
		out := link.ps.handleOwnedInbound(link.resp, transport.Packet{Data: req}, link.myTr, nil, log)
		if !out.peerAlive {
			t.Errorf("a protected INFORMATIONAL request with %s set was not processed", bit.name)
		}
		if link.resp.ExpectedMsgID == msgID {
			t.Errorf("a protected INFORMATIONAL request with %s set left ExpectedMsgID at %d",
				bit.name, msgID)
		}
		if rtxRecv(t, link.peerTr) == nil {
			t.Errorf("a protected INFORMATIONAL request with %s set drew no response", bit.name)
		}
	}

	// The initiator's side. The responder's IKE_SA_INIT response carries the bits.
	for _, bit := range flagBits {
		iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "xbit-psk")
		ikeTable := NewSATable()
		ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
		if err != nil {
			t.Fatalf("newInitiatorSA: %v", err)
		}
		ikeTable.Insert(ini)
		req := buildSAInitRequest(ini, testIKEGroup())
		ini.InitiatorSAInitMsg = req
		ini.State = StateSAInitSent
		resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
		if err != nil {
			t.Fatalf("newResponderSA: %v", err)
		}
		handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
		answer := append([]byte(nil), resp.LastSentMsg...)
		if len(answer) <= wire.HeaderLen {
			t.Fatal("the responder built no IKE_SA_INIT response")
		}
		answer[19] |= bit.mask
		handleInbound(ini, transport.Packet{Data: answer}, ikeTable, nil, log)
		if ini.State != StateAuthSent {
			t.Errorf("initiator state = %v after an IKE_SA_INIT response with %s set, want "+
				"auth-sent", ini.State, bit.name)
		}
	}

	// Negative: the R bit is not an X bit and is not ignored.
	consumed, table, _ = fresh(func(raw []byte) { raw[19] |= wire.FlagResponse })
	if consumed || table.Len() != 0 {
		t.Errorf("an IKE_SA_INIT with the R bit set: consumed=%v SAs=%d, want false and 0; a "+
			"bit that is not an X bit must not be ignored", consumed, table.Len())
	}
}
