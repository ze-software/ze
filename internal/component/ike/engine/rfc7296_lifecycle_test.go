package engine

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// lcyLoopback returns an established IKE SA pair wired to a loopback stand-in for
// the far end. local is the SA under test and ps is the session that owns it. peer
// is the far-end SA, and it decrypts what ze writes. peerTr reads every datagram ze
// sends, and myTr is the socket ze sends from.
func lcyLoopback(t *testing.T) (local, peer *SA, ps *PeerSession, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	peer, local, ps = establishPSK(t)
	peerTr, myTr = rtxPeerLink(t)
	local.PeerCfg.RemoteAddress = "127.0.0.1"
	peer.PeerCfg.RemoteAddress = "127.0.0.1"
	return local, peer, ps, peerTr, myTr
}

// lcyDecrypt returns the inner payload chain of raw as sa reads it. It fails the
// test when the message does not authenticate under sa.
func lcyDecrypt(t *testing.T, sa *SA, raw []byte) []wire.PayloadEntry {
	t.Helper()
	inner, err := decryptAndParse(sa, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the message did not authenticate under the expected IKE SA: %v", err)
	}
	return inner
}

// lcyDeletes returns every Delete payload in a decrypted payload chain.
func lcyDeletes(inner []wire.PayloadEntry) []*wire.PayloadDelete {
	var out []*wire.PayloadDelete
	for i := range inner {
		if d, ok := inner[i].Payload.(*wire.PayloadDelete); ok {
			out = append(out, d)
		}
	}
	return out
}

// lcyOneESPDelete returns the single ESP SPI a Child SA Delete names. It fails the
// test when the chain does not hold exactly one well-formed ESP Delete.
func lcyOneESPDelete(t *testing.T, inner []wire.PayloadEntry) uint32 {
	t.Helper()
	dels := lcyDeletes(inner)
	if len(dels) != 1 {
		t.Fatalf("the message carries %d Delete payloads, want 1", len(dels))
	}
	d := dels[0]
	if d.ProtocolID != wire.ProtocolESP {
		t.Fatalf("Delete protocol = %d, want ESP", d.ProtocolID)
	}
	if d.SPISize != 4 || d.NumSPIs != 1 || len(d.SPIs) != 4 {
		t.Fatalf("Delete holds size:%d count:%d bytes:%d, want one 4-byte SPI",
			d.SPISize, d.NumSPIs, len(d.SPIs))
	}
	return binary.BigEndian.Uint32(d.SPIs)
}

// lcyRequest builds an INFORMATIONAL request from peer at the given Message ID,
// carrying the payloads the caller supplies.
func lcyRequest(t *testing.T, peer *SA, msgID uint32, inner []wire.PayloadEntry) []byte {
	t.Helper()
	raw, err := buildEncryptedMessageEx(peer, inner, msgID, wire.ExchangeInformational, initiatorFlag(peer))
	if err != nil {
		t.Fatalf("build INFORMATIONAL request at id %d: %v", msgID, err)
	}
	return raw
}

// lcyESPDeleteChain is the payload chain of an INFORMATIONAL request that deletes
// one ESP SPI.
func lcyESPDeleteChain(spi uint32) []wire.PayloadEntry {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, spi)
	return []wire.PayloadEntry{{Payload: &wire.PayloadDelete{
		ProtocolID: wire.ProtocolESP, SPISize: 4, NumSPIs: 1, SPIs: b,
	}}}
}

// lcyRetireChildSA runs one Child SA rekey to completion on the owner loop. That
// exchange is the path where ze decides to delete a Child SA of its own accord. It
// returns the retired Child SA and the replacement the exchange installed. The
// rekey request never reaches the wire here, so the only datagram the caller reads
// afterwards is the one the retirement produced.
func lcyRetireChildSA(t *testing.T, local, peer *SA, ps *PeerSession, myTr *transport.UDPTransport) (retired, replacement *ChildSA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	retired = ps.getChildSA()
	if retired == nil {
		t.Fatal("the session holds no Child SA to rekey")
	}
	req, pending, err := initiateChildRekey(local, retired)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	respBytes, _, err := respondChildRekey(peer, lcyDecrypt(t, peer, req), retired, pending.messageID, nil, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	ps.pendingRekey = pending
	out := ps.handleOwnedInbound(local, transport.Packet{Data: respBytes}, myTr, nil, log)
	if out.newChild == nil {
		t.Fatal("the rekey response installed no replacement Child SA")
	}
	return retired, out.newChild
}

// VALIDATES: every control message rides the IKE SA it belongs to.
// RFC requirement: RFC7296-1.4-3 positive -- sendDeleteESP builds the Child SA control
// message under the IKE SA it was called on (inbound.go:335). sendDeleteIKE does the
// same for the IKE SA's own control message (inbound.go:259). Each datagram carries
// that SA's header SPIs and authenticates under that SA.
// RFC requirement: RFC7296-1.4-3 negative -- a second, unrelated IKE SA is established in the
// same test. Its keys reject the same bytes and its SPIs differ, so "sent under some
// IKE SA" is not a free pass.
func TestLcyControlMessagesRideTheirOwnIKESA(t *testing.T) {
	log := slogutil.DiscardLogger()
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	otherPeer, otherLocal, _ := establishPSK(t)

	// Negative. The two IKE SAs are distinguishable, so a header SPI match is a real
	// claim about which SA carried the message.
	if otherLocal.InitiatorSPI == local.InitiatorSPI || otherLocal.ResponderSPI == local.ResponderSPI {
		t.Fatal("the two IKE SAs share an SPI, so the header check proves nothing")
	}

	// A Child SA control message. It must ride the IKE SA that generated the Child SA.
	const retiredSPI uint32 = 0x0BADF00D
	ps.sendDeleteESP(local, myTr, retiredSPI, log)
	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the Child SA Delete never reached the peer")
	}
	hdr := parseMsg(t, got).Header
	if hdr.InitiatorSPI != local.InitiatorSPI || hdr.ResponderSPI != local.ResponderSPI {
		t.Error("the Child SA Delete carries the SPIs of another IKE SA")
	}
	if spi := lcyOneESPDelete(t, lcyDecrypt(t, peer, got)); spi != retiredSPI {
		t.Errorf("Delete names SPI %#x, want %#x", spi, retiredSPI)
	}
	if _, err := decryptAndParse(otherPeer, parseMsg(t, got), got); err == nil {
		t.Error("the Child SA Delete authenticated under an unrelated IKE SA")
	}

	// RFC 7296 Section 2.3 keeps one self-initiated request in flight, and this test
	// sends a second one. The window is freed by hand because no answer arrives here.
	local.releaseRequestWindow()

	// An IKE SA control message. It must ride that same IKE SA.
	ps.sendDeleteIKE(local, myTr, log)
	got = rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the IKE SA Delete never reached the peer")
	}
	hdr = parseMsg(t, got).Header
	if hdr.InitiatorSPI != local.InitiatorSPI || hdr.ResponderSPI != local.ResponderSPI {
		t.Error("the IKE SA Delete carries the SPIs of another IKE SA")
	}
	dels := lcyDeletes(lcyDecrypt(t, peer, got))
	if len(dels) != 1 || dels[0].ProtocolID != wire.ProtocolIKE {
		t.Fatalf("the IKE SA Delete holds %d payloads, want one IKE Delete", len(dels))
	}
	if _, err := decryptAndParse(otherPeer, parseMsg(t, got), got); err == nil {
		t.Error("the IKE SA Delete authenticated under an unrelated IKE SA")
	}
}

// VALIDATES: the recipient answers every INFORMATIONAL request it accepts.
// RFC requirement: RFC7296-1.4-4 positive -- handleInformationalOwned builds an answer for
// any accepted request and writes it (inbound.go:293-299). An empty probe and a
// Delete-bearing request each draw one, and each answer echoes the request id.
// RFC requirement: RFC7296-1.4-4 negative -- an INFORMATIONAL response draws nothing back.
// The answer is therefore a reply to a request and not an unconditional write.
func TestLcyEveryInformationalRequestDrawsAResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	remote := local.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA under test has no resolvable peer address")
	}

	// Message ID 2 is the next request this side expects after IKE_AUTH.
	for _, tc := range []struct {
		name  string
		msgID uint32
		inner []wire.PayloadEntry
	}{
		{"empty liveness probe", 2, nil},
		{"Delete request", 3, lcyESPDeleteChain(0x11223344)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := lcyRequest(t, peer, tc.msgID, tc.inner)
			if out := ps.handleOwnedInbound(local, transport.Packet{Data: req}, myTr, nil, log); !out.peerAlive {
				t.Fatal("the request never reached the INFORMATIONAL handler")
			}
			answer := rtxRecv(t, peerTr)
			if answer == nil {
				t.Fatal("the request drew no answer")
			}
			ans := parseMsg(t, answer).Header
			if ans.ExchangeType != wire.ExchangeInformational {
				t.Errorf("answer exchange = %d, want INFORMATIONAL", ans.ExchangeType)
			}
			if ans.Flags&wire.FlagResponse == 0 {
				t.Error("the answer is missing the Response flag")
			}
			if ans.MessageID != tc.msgID {
				t.Errorf("answer Message ID = %d, want %d", ans.MessageID, tc.msgID)
			}
		})
	}

	// Negative. A response is not a request, so it draws nothing and cannot loop.
	resp, err := buildEncryptedMessageEx(peer, nil, 7, wire.ExchangeInformational,
		initiatorFlag(peer)|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build INFORMATIONAL response: %v", err)
	}
	ps.handleOwnedInbound(local, transport.Packet{Data: resp}, myTr, nil, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an INFORMATIONAL response")
}

// VALIDATES: a closed Child SA leaves neither half behind, and its Delete names the
// incoming half this end owns.
// RFC requirement: RFC7296-1.4.1-1 positive -- the retirement Delete carries the retired
// InboundSPI (inbound.go:142), which is the incoming SA of this end. The close path
// runs cleanupChild (established.go:389) into removeChildSA, and that drops the
// inbound state and the outbound state together (child.go:339-343).
// RFC requirement: RFC7296-1.4.1-1 negative -- the two SPIs of a pair differ in this fixture,
// so naming one of them is a choice. The dataplane held no removal before the close,
// so two removals afterwards are an effect of the close.
func TestLcyClosingAChildSAClosesBothHalvesAndDeletesOurInbound(t *testing.T) {
	log := slogutil.DiscardLogger()
	local, peer, ps, peerTr, myTr := lcyLoopback(t)

	before := ps.getChildSA()
	if before == nil {
		t.Fatal("the session holds no Child SA")
	}
	// Negative. A pair with two distinct, non-zero SPIs makes the next check real.
	if before.InboundSPI == 0 || before.OutboundSPI == 0 || before.InboundSPI == before.OutboundSPI {
		t.Fatalf("Child SA SPIs = in:%#x out:%#x, want two distinct non-zero values",
			before.InboundSPI, before.OutboundSPI)
	}

	retired, replacement := lcyRetireChildSA(t, local, peer, ps, myTr)
	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the retirement produced no Delete")
	}
	spi := lcyOneESPDelete(t, lcyDecrypt(t, peer, got))
	if spi != retired.InboundSPI {
		t.Errorf("Delete names SPI %#x, want the retired inbound SPI %#x", spi, retired.InboundSPI)
	}
	if spi == retired.OutboundSPI {
		t.Error("the Delete names the outbound half, which the far end owns")
	}

	// Both members of the pair leave the dataplane when the Child SA closes.
	dp := &mockDP{}
	if len(dp.removed) != 0 {
		t.Fatal("the fake dataplane starts with a removal already recorded")
	}
	if replacement.InboundSPI == replacement.OutboundSPI {
		t.Fatal("the replacement pair shares one SPI, so counting two removals proves nothing")
	}
	ps.cleanupChild(dp, nil, log)
	if len(dp.removed) != 2 {
		t.Fatalf("the close removed %d dataplane SAs, want 2", len(dp.removed))
	}
	seen := map[uint32]bool{dp.removed[0]: true, dp.removed[1]: true}
	if !seen[replacement.InboundSPI] || !seen[replacement.OutboundSPI] {
		t.Errorf("the close removed %v, want the pair in:%#x out:%#x",
			dp.removed, replacement.InboundSPI, replacement.OutboundSPI)
	}
	if ps.getChildSA() != nil {
		t.Error("the session still holds the Child SA it closed")
	}
}

// VALIDATES: the answer to a Delete request carries no Delete payload of its own.
// RFC requirement: RFC7296-1.4.1-4 positive -- handleInformationalOwned passes a nil payload
// chain to the response builder (inbound.go:293), and it does so unconditionally. Ze
// therefore attaches a Delete to NO informational response, so it attaches none in the
// crossed-close case this row governs. The only two PayloadDelete constructions in the
// file build REQUESTS (inbound.go:258 sendDeleteIKE, inbound.go:334 sendDeleteESP).
// RFC requirement: RFC7296-1.4.1-4 negative -- the same decode helper finds a Delete inside the
// encrypted request. A count of zero in the answer is therefore a real reading of a real
// answer. That answer also parses as a response at the request id.
//
// Read the scope of this row before you change the test. RFC 7296 section 1.4.1 gives the
// ordinary case first, where the response usually carries Delete payloads for the paired
// SAs. It then names ONE exception. That exception is a delete request for SAs we already
// sent a delete request for. The MUST NOT belongs to that exception alone.
//
// This test does not set that case up, and it does not need to. A proof that Ze never
// attaches a Delete to any response is strictly stronger.
//
// The same behavior is a DEFECT against the ordinary case. It is already recorded as the
// {gap} annotation on RFC7296-1.4-1 in rfc/short/rfc7296.md. This green test does not say
// that Ze answers a Delete correctly. It says only that Ze never commits the duplicate
// deletion the crossed case forbids.
func TestLcyInformationalResponseCarriesNoDeletePayload(t *testing.T) {
	log := slogutil.DiscardLogger()
	local, peer, ps, peerTr, myTr := lcyLoopback(t)

	const doomed uint32 = 0xA5A5A5A5
	req := lcyRequest(t, peer, 2, lcyESPDeleteChain(doomed))

	// Negative. The decoder does see a Delete through the encrypted payload.
	if spi := lcyOneESPDelete(t, lcyDecrypt(t, local, req)); spi != doomed {
		t.Fatalf("the request names SPI %#x, want %#x", spi, doomed)
	}

	if out := ps.handleOwnedInbound(local, transport.Packet{Data: req}, myTr, nil, log); !out.peerAlive {
		t.Fatal("the Delete request never reached the INFORMATIONAL handler")
	}
	answer := rtxRecv(t, peerTr)
	if answer == nil {
		t.Fatal("the Delete request drew no answer")
	}
	hdr := parseMsg(t, answer).Header
	if hdr.Flags&wire.FlagResponse == 0 || hdr.MessageID != 2 {
		t.Fatalf("answer flags=%#x id=%d, want the Response flag at id 2", hdr.Flags, hdr.MessageID)
	}

	inner := lcyDecrypt(t, peer, answer)
	if dels := lcyDeletes(inner); len(dels) != 0 {
		t.Errorf("the answer carries %d Delete payloads, want none", len(dels))
	}
	if len(inner) != 0 {
		t.Errorf("the answer carries %d payloads, want an empty chain", len(inner))
	}
}

// VALIDATES: a retired SPI is never handed to a replacement SA.
// RFC requirement: RFC7296-1.4.1-5 positive -- a Child SA draws its inbound SPI from
// GenerateESPSPI (initiator.go:255 and rekey.go:162). An IKE SA draws its own from
// GenerateSPI (rekey.go:305 and rekey.go:474). A rekey therefore replaces every SPI
// of the pair and reuses none. Both generators also refuse the reserved zero.
// RFC requirement: RFC7296-1.4.1-5 negative -- the retired SPIs are non-zero and differ from
// each other. A fresh value is therefore an observed change, not a test against zero.
func TestLcyRetiredSPIsAreNeverReused(t *testing.T) {
	log := slogutil.DiscardLogger()
	local, peer, ps, _, myTr := lcyLoopback(t)

	retired, replacement := lcyRetireChildSA(t, local, peer, ps, myTr)

	// Negative. The retired pair holds two distinct non-zero SPIs.
	if retired.InboundSPI == 0 || retired.OutboundSPI == 0 || retired.InboundSPI == retired.OutboundSPI {
		t.Fatalf("retired SPIs = in:%#x out:%#x, want two distinct non-zero values",
			retired.InboundSPI, retired.OutboundSPI)
	}
	for _, c := range []struct {
		name string
		spi  uint32
	}{
		{"replacement inbound", replacement.InboundSPI},
		{"replacement outbound", replacement.OutboundSPI},
	} {
		if c.spi == 0 {
			t.Errorf("%s SPI is the reserved zero value", c.name)
		}
		if c.spi == retired.InboundSPI || c.spi == retired.OutboundSPI {
			t.Errorf("%s SPI %#x repeats a retired SPI", c.name, c.spi)
		}
	}

	// The IKE SA layer is the second producer of this rule.
	oldIni, oldResp, _ := establishPSK(t)
	req, pending, err := initiateIKERekey(oldIni, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	respBytes, newResp, err := respondIKERekey(oldResp, lcyDecrypt(t, oldResp, req), pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	newIni, err := applyIKERekeyResponse(oldIni, pending, lcyDecrypt(t, oldIni, respBytes), log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}
	var zero [8]byte
	for _, c := range []struct {
		name string
		sa   *SA
	}{{"rekey initiator", newIni}, {"rekey responder", newResp}} {
		if c.sa.InitiatorSPI == zero || c.sa.ResponderSPI == zero {
			t.Errorf("%s new IKE SA holds a zero SPI", c.name)
		}
		if c.sa.InitiatorSPI == oldIni.InitiatorSPI {
			t.Errorf("%s reused the retired initiator SPI", c.name)
		}
	}

	// Every draw of a fresh Child SA SPI avoids the reserved zero.
	for range 256 {
		spi, err := GenerateESPSPI()
		if err != nil {
			t.Fatalf("GenerateESPSPI: %v", err)
		}
		if spi == 0 {
			t.Fatal("GenerateESPSPI returned the reserved zero value")
		}
	}
}

// VALIDATES: one Child SA lives under one IKE SA, so no two of them can fail apart.
// RFC requirement: RFC7296-2.4-9 positive -- PeerSession holds a single childSA slot
// (reconcile.go:54) and setChildSA overwrites it (reconcile.go:172-176). A rekey
// moves that slot to the replacement (inbound.go:140) instead of a second entry, so
// two Child SAs never share one IKE SA. The rule demands separate IKE SAs only for
// Child SAs that can fail apart, and this shape admits none.
// RFC requirement: RFC7296-2.4-9 negative -- the retired and the replacement Child SA carry
// different SPIs, so "the slot holds the replacement" is an observed change.
func TestLcyOneChildSALivesUnderOneIKESA(t *testing.T) {
	local, peer, ps, _, myTr := lcyLoopback(t)

	retired, replacement := lcyRetireChildSA(t, local, peer, ps, myTr)
	// Negative. The two Child SAs are distinguishable.
	if retired.InboundSPI == replacement.InboundSPI {
		t.Fatal("the retired and the replacement Child SA share an SPI")
	}
	if held := ps.getChildSA(); held != replacement {
		t.Fatalf("the session holds Child SA %p after the rekey, want the replacement %p",
			held, replacement)
	}

	// The slot overwrites. It never grows a second entry.
	ps.setChildSA(retired)
	if ps.getChildSA() != retired {
		t.Fatal("the Child SA slot did not take the value it was given")
	}
	ps.setChildSA(replacement)
	if ps.getChildSA() != replacement {
		t.Fatal("the Child SA slot kept an earlier value, so it holds more than one")
	}
	ps.setChildSA(nil)
	if ps.getChildSA() != nil {
		t.Fatal("the Child SA slot survived a clear, so it holds more than one")
	}
}

// VALIDATES: ze tells the peer whenever it deletes a Child SA of its own accord.
// RFC requirement: RFC7296-2.4-10 positive -- the owner loop retires the old Child SA after a
// rekey response and sends an INFORMATIONAL Delete for it first (inbound.go:142). The
// datagram is a request, it authenticates on the peer, and it names the retired SPI.
// RFC requirement: RFC7296-2.4-10 negative -- the rekey request that precedes it is a
// CREATE_CHILD_SA and carries no Delete, and no further datagram follows the Delete.
// The Delete therefore belongs to the retirement and is not a blanket write.
func TestLcyRetiringAChildSASendsADeletePayload(t *testing.T) {
	log := slogutil.DiscardLogger()
	local, peer, ps, peerTr, myTr := lcyLoopback(t)
	remote := local.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA under test has no resolvable peer address")
	}
	retired := ps.getChildSA()
	if retired == nil {
		t.Fatal("the session holds no Child SA to rekey")
	}

	req, pending, err := initiateChildRekey(local, retired)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	ps.pendingRekey = pending

	// Negative. The rekey request reaches the peer first and holds no Delete.
	sendRaw(local, myTr, req, log)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the rekey request never reached the peer")
	}
	if got := parseMsg(t, first).Header.ExchangeType; got != wire.ExchangeCreateChildSA {
		t.Fatalf("first datagram exchange = %d, want CREATE_CHILD_SA", got)
	}
	if dels := lcyDeletes(lcyDecrypt(t, peer, first)); len(dels) != 0 {
		t.Fatalf("the rekey request already carries %d Delete payloads", len(dels))
	}

	respBytes, _, err := respondChildRekey(peer, lcyDecrypt(t, peer, req), retired, pending.messageID, nil, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	if out := ps.handleOwnedInbound(local, transport.Packet{Data: respBytes}, myTr, nil, log); out.newChild == nil {
		t.Fatal("the rekey response installed no replacement Child SA")
	}

	// Positive. The retirement announced itself to the peer.
	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the retired Child SA drew no Delete")
	}
	hdr := parseMsg(t, got).Header
	if hdr.ExchangeType != wire.ExchangeInformational {
		t.Errorf("Delete exchange = %d, want INFORMATIONAL", hdr.ExchangeType)
	}
	if hdr.Flags&wire.FlagResponse != 0 {
		t.Error("the Delete carries the Response flag, so it awaits no acknowledgement")
	}
	if spi := lcyOneESPDelete(t, lcyDecrypt(t, peer, got)); spi != retired.InboundSPI {
		t.Errorf("Delete names SPI %#x, want the retired SPI %#x", spi, retired.InboundSPI)
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "the Child SA retirement")
}
