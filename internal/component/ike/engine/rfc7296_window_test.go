// RFC 7296 Section 2.3 request-window obligations. These tests cover two of them.
//
// Section 2.3 allows one outstanding self-initiated request per IKE SA, because Ze
// never sends a SET_WINDOW_SIZE notify and never reads one. Four paths raise such a
// request on an established SA. They are the DPD probe, the Child SA rekey, the IKE
// SA rekey, and the two Delete senders. One shared window couples them.
//
// Each test carries an `RFC requirement:` tag for the row it proves. Helpers here start with
// `win`, so they cannot collide with the sibling RFC files in this package. This file
// reuses the `rtx` loopback helpers and the `rky` dataplane recorder.

package engine

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// winESPSPI is the ESP SPI the tests hand to a Delete sender. Its value carries no
// meaning beyond a recognizable non-zero.
const winESPSPI uint32 = 0x0A0B0C0D

// winDueDPD returns DPD state that is due to send a probe and cannot time out during
// a test. The long timeout keeps the owner loop away from its dead-peer branch.
func winDueDPD() *dpdState {
	return &dpdState{
		interval: time.Millisecond,
		timeout:  time.Hour,
		lastSent: time.Now().Add(-time.Hour),
	}
}

// winSoftExpired returns lifetime state whose soft time has passed and whose hard
// time is far away. A tick therefore raises a rekey and never a hard expiry.
func winSoftExpired() *lifetimeState {
	return &lifetimeState{
		softTime: time.Now().Add(-time.Hour),
		hardTime: time.Now().Add(time.Hour),
	}
}

// winChildRekeyResponse builds the payload chain of a peer CREATE_CHILD_SA response
// to a Child SA rekey we initiated. RFC 7296 Section 1.3.2: SA and Nr.
func winChildRekeyResponse(peerESPSPI uint32, nr []byte) []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: espSAPayload(peerESPSPI)},
		{Payload: &wire.PayloadNonce{NonceData: nr}},
	}
}

// winForge returns a copy of raw whose last byte is flipped. The header and every
// payload length are untouched, so the message still parses and still names the same
// SPIs and message id. The integrity check over the encrypted payload fails, which is
// what an off-path forgery looks like to the engine.
func winForge(raw []byte) []byte {
	bad := bytes.Clone(raw)
	bad[len(bad)-1] ^= 0xFF
	return bad
}

// winInformationalAnswer builds the peer's INFORMATIONAL response at msgID. It is the
// answer a conforming peer returns for a DPD probe or a Delete.
func winInformationalAnswer(t *testing.T, peer *SA, msgID uint32) []byte {
	t.Helper()
	raw, err := buildEncryptedMessageEx(peer, nil, msgID,
		wire.ExchangeInformational, initiatorFlag(peer)|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the peer answer at id %d: %v", msgID, err)
	}
	return raw
}

// VALIDATES: one maintainSA tick that raises a DPD probe and a Child SA rekey writes
// one request. The second request waits for the answer to the first.
// PREVENTS: two uncoupled guards each taking a message id in the same tick, which
// puts two unanswered requests on the wire.
//
// RFC requirement: RFC7296-2.3-2 positive -- one tick runs the DPD branch
// (established.go:189-191) and then the Child SA rekey branch (established.go:197-208).
// Both reserve the one window on the SA (sa.go, msgid.go), so the peer reads one
// request and the rekey is never recorded as outstanding.
//
// RFC requirement: RFC7296-2.3-2 negative -- the deferred rekey is not a dead producer. Once
// the peer answers the probe, the same branch sends its request and the peer reads
// it. The silence above therefore comes from the wait and not from a broken path.
func TestWinOneRequestPerTick(t *testing.T) {
	log := slogutil.DiscardLogger()
	peer, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := sa.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA has no resolvable peer address")
	}

	dp := &rkyDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)

	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(sa, winDueDPD(), winSoftExpired(), nil,
			testIKEGroup(), NewSATable(), dp, myTr, nil, log)
		close(done)
	}()

	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the tick wrote no request at all")
	}

	// Stop the loop before the silence check. The tick that wrote the first datagram
	// ran to its end before the loop read stopCh again. A second write of that tick
	// is therefore already queued. After the loop returns, no later write is
	// possible, and the sentinel needs no timing assumption.
	close(ps.stopCh)
	<-done

	hdr := parseMsg(t, first).Header
	if hdr.ExchangeType != wire.ExchangeInformational {
		t.Errorf("the first request exchange = %d, want INFORMATIONAL (the DPD probe)", hdr.ExchangeType)
	}
	if hdr.Flags&wire.FlagResponse != 0 {
		t.Error("the first datagram carries the Response flag, so it is not a request")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "one tick that raised a probe and a rekey")
	if ps.pendingRekey != nil {
		t.Error("the rekey exchange started while the probe was unanswered")
	}

	// Negative. The peer answers the probe, which frees the window. The rekey branch
	// the tick refused now sends its request, and the peer reads it. The stop path
	// cleared the Child SA, so hand it back before the branch runs.
	answer := winInformationalAnswer(t, peer, hdr.MessageID)
	if out := ps.handleOwnedInbound(sa, transport.Packet{Data: answer}, myTr, nil, log); !out.dpdResp {
		t.Fatalf("the peer answer was not read as a response to the probe: %+v", out)
	}
	ps.setChildSA(old)
	ps.startChildRekey(sa, myTr, log)
	if ps.pendingRekey == nil {
		t.Fatal("the rekey branch refused again after the window freed")
	}
	defer ps.pendingRekey.clear()
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the deferred rekey wrote no datagram after the window freed")
	}
}

// VALIDATES: Ze holds one request at a time, and the peer's answer frees the window
// for the next one. The release is keyed to the message id of that request.
// PREVENTS: a Delete slipping out beside an unanswered probe, which is the path that
// carried no guard at all.
//
// RFC requirement: RFC7296-2.3-4 positive -- the window is one request wide, which is the size
// Ze offers and the size it assumes of a peer. sendDPD (dpd.go) reserves it, and
// sendDeleteESP (inbound.go) finds it held, so the count in flight never passes one.
// An answer for another message id leaves it held (msgid.go), so a stale or replayed
// answer cannot widen the window.
//
// RFC requirement: RFC7296-2.3-4 negative -- the refusal is a window and not a mute sender.
// The same Delete goes out once the peer answers the probe. The assertion above is
// therefore not the trivial observation that Ze writes nothing.
//
// RFC requirement: RFC7296-2.3-4 negative -- a datagram that fails its integrity check does
// not free the window. It names the right SPIs and the exact outstanding message id.
// The window answers to a real reply, and not to anything that resembles one, so an
// off-path forgery cannot widen it. strongSwan orders it the same way:
// process_response clears the slot after parse_body verifies the message.
func TestWinResponseReleasesSlot(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	sendDPD(ini, myTr, winDueDPD(), log)
	probe := rtxRecv(t, peerTr)
	if probe == nil {
		t.Fatal("the DPD probe never reached the peer")
	}
	probeID := parseMsg(t, probe).Header.MessageID

	// Positive. A second self-initiated request finds the window held.
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an ESP Delete while the probe is unanswered")

	// An answer for another message id is not the answer to our request.
	wrong := winInformationalAnswer(t, resp, probeID+9)
	ps.handleOwnedInbound(ini, transport.Packet{Data: wrong}, myTr, nil, log)
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an ESP Delete after an answer for another id")

	// A datagram that fails its integrity check is not an answer at all. This one
	// names the right SPIs and the exact outstanding message id.
	right := winInformationalAnswer(t, resp, probeID)
	if forged := ps.handleOwnedInbound(ini, transport.Packet{Data: winForge(right)}, myTr, nil, log); forged.dpdResp {
		t.Fatal("a datagram that failed its integrity check was read as an answer")
	}
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an ESP Delete after a forged answer")

	// AC-4. The real answer travels the real inbound path, and the window frees only
	// after the engine authenticates it.
	out := ps.handleOwnedInbound(ini, transport.Packet{Data: right}, myTr, nil, log)
	if !out.dpdResp || out.dpdRespMsgID != probeID {
		t.Fatalf("the matching answer was read as %+v, want a response at id %d", out, probeID)
	}

	// Negative. The Delete the engine refused twice now goes out.
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	del := rtxRecv(t, peerTr)
	if del == nil {
		t.Fatal("the deferred Delete never went out after the window freed")
	}
	sent := parseMsg(t, del)
	if sent.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("the Delete exchange = %d, want INFORMATIONAL", sent.Header.ExchangeType)
	}
	if sent.Header.Flags&wire.FlagResponse != 0 {
		t.Error("the Delete carries the Response flag, so it is not a request")
	}
	if sent.Header.MessageID == probeID {
		t.Errorf("the Delete reused the probe message id %d", probeID)
	}
	inner, err := decryptAndParse(resp, sent, del)
	if err != nil {
		t.Fatalf("the peer could not decrypt the Delete: %v", err)
	}
	sawESPDelete := false
	for i := range inner {
		if d, ok := inner[i].Payload.(*wire.PayloadDelete); ok && d.ProtocolID == wire.ProtocolESP {
			sawESPDelete = true
		}
	}
	if !sawESPDelete {
		t.Error("the datagram that followed the answer is not an ESP Delete")
	}
}

// VALIDATES: the Delete senders respect the window. They are the path the Task table
// records as guarded by nothing beyond a present transport.
// PREVENTS: a make-before-break Delete raised on the inbound path from riding out
// beside an unanswered probe.
func TestWinDeleteDefersWhileProbeOutstanding(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	dp := &rkyDP{}
	old, err := createFirstChildSA(ini, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	ps.setChildSA(old)

	sendDPD(ini, myTr, winDueDPD(), log)
	probe := rtxRecv(t, peerTr)
	if probe == nil {
		t.Fatal("the DPD probe never reached the peer")
	}
	probeID := parseMsg(t, probe).Header.MessageID

	// The IKE Delete of a teardown finds the window held.
	ps.sendDeleteIKE(ini, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an IKE Delete while the probe is unanswered")

	// The make-before-break Delete at inbound.go:131 finds it held as well. The rekey
	// itself still completes, so the window costs the exchange nothing.
	newSPI, err := generateESPSPI()
	if err != nil {
		t.Fatalf("GenerateESPSPI: %v", err)
	}
	ps.pendingRekey = &pendingRekey{
		kind:          rekeyChild,
		messageID:     probeID + 1,
		sentAt:        time.Now(),
		localNonce:    testNonce(3),
		newInboundSPI: newSPI,
		oldChild:      old,
	}
	respMsg := &wire.Message{Header: wire.Header{MessageID: ps.pendingRekey.messageID}}
	out := ps.handleCreateChildSAOwned(ini, respMsg, winChildRekeyResponse(winESPSPI, testNonce(5)), true, myTr, dp, log)
	if out.newChild == nil {
		t.Fatal("the rekey response installed no replacement Child SA")
	}
	if dp.installedSA(newSPI) == nil {
		t.Fatal("the replacement Child SA is not installed, so the rekey did not complete")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "the make-before-break Delete of a completed rekey")

	// Negative. Once the peer answers the probe, the same sender writes its Delete.
	answer := winInformationalAnswer(t, resp, probeID)
	if out := ps.handleOwnedInbound(ini, transport.Packet{Data: answer}, myTr, nil, log); !out.dpdResp {
		t.Fatalf("the peer answer was not read as a response to the probe: %+v", out)
	}
	ps.sendDeleteIKE(ini, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the IKE Delete never went out after the window freed")
	}
}

// VALIDATES: a request the peer never answers ends the SA after the bound, and only
// when no other timer covers its holder.
// PREVENTS: the third exit RFC 7296 Section 2.1 does not offer, where the request is
// forgotten and the SA it belonged to keeps running.
//
// RFC requirement: RFC7296-2.1-7 positive -- serviceRequestWindow (established.go) takes the
// section's second exit once every repeat has gone out and requestWindowTimeout has
// passed. It marks the SA dead, and maintainSA's StateDead branch discards the Child SAs
// with it.
//
// RFC requirement: RFC7296-2.1-7 negative -- a hold still inside the bound leaves the SA alive
// and its window held, so the failure above belongs to the bound and not to any
// unanswered request at all.
func TestWinUnansweredRequestFailsTheSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	// A best-effort Delete takes the window, and the peer never answers it.
	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the ESP Delete never reached the peer")
	}
	now := time.Now()

	// Negative. A fresh hold neither frees the window nor fails the SA.
	ps.serviceRequestWindow(ini, nil, now, log)
	ps.sendDeleteIKE(ini, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "a hold that is still inside the bound")
	if ini.State == StateDead {
		t.Fatal("a hold inside the bound already failed the SA")
	}

	ini.requestSentAt = now.Add(-requestWindowTimeout - time.Second)

	// A rekey ends the session on its own retransmissions, so the bound leaves it be.
	ps.pendingRekey = &pendingRekey{kind: rekeyChild, messageID: ini.requestMsgID}
	ps.serviceRequestWindow(ini, nil, now, log)
	ps.sendDeleteIKE(ini, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "a stale hold while a rekey is outstanding")
	ps.pendingRekey = nil

	// A DPD probe ends the session on its own timeout, so the bound leaves it be too.
	ps.serviceRequestWindow(ini, &dpdState{awaitReply: true}, now, log)
	ps.sendDeleteIKE(ini, myTr, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "a stale hold while a probe is outstanding")
	if ini.State == StateDead {
		t.Fatal("a hold another timer covers failed the SA anyway")
	}

	// Positive. Nothing else covers this Delete, so Section 2.1's second exit is
	// taken and the SA is deemed failed. The window frees with it, so the SA that
	// outlives this call in a broken build still sends again.
	ps.serviceRequestWindow(ini, nil, now, log)
	if ini.State != StateDead {
		t.Fatalf("the unanswered request left the SA in state %v, want StateDead", ini.State)
	}
	if ini.requestOutstanding {
		t.Error("the failed SA still holds its request window")
	}
	if len(ini.requestMsg) != 0 {
		t.Error("the failed SA still holds the datagram of the request it gave up on")
	}
	ps.sendDeleteIKE(ini, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the window stayed held past the bound, so the SA raises no later request")
	}
}

// winDeleteSPIs returns the ESP SPIs the Delete payload of raw names, read back
// through the PEER's own decrypt. The peer is the authority on what it received.
func winDeleteSPIs(t *testing.T, peer *SA, raw []byte) []uint32 {
	t.Helper()
	inner, err := decryptAndParse(peer, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the peer could not decrypt the Delete: %v", err)
	}
	for i := range inner {
		if d, ok := inner[i].Payload.(*wire.PayloadDelete); ok && d.ProtocolID == wire.ProtocolESP {
			return deleteSPIs(d)
		}
	}
	t.Fatal("the datagram carries no ESP Delete payload")
	return nil
}

// TestWinDeferredDeleteWaitsForTheWindow was REMOVED here, together with the
// deferral queue it exercised. It is a removed feature, not a relaxed assertion.

// VALIDATES: a Delete that went out is kept and repeated until the peer answers it.
// PREVENTS: a lost Delete that nothing ever repeats, which leaves the peer holding an
// SA Ze destroyed and leaves NextMsgID one past the id the peer still expects.
//
// RFC requirement: RFC7296-2.1-5 positive -- Section 2.1 MUST: "The initiator MUST remember
// each request until it receives the corresponding response." writeDelete (delete.go)
// arms the window's slot with the datagram. serviceRequestRetransmit (established.go)
// repeats it under the SAME Message ID, which the same section requires.
//
// RFC requirement: RFC7296-2.1-5 negative -- the memory ends where the section ends it. An SA
// that sent no Delete holds no datagram. The answer to the Delete drops the one it
// held, so no later tick repeats a request the peer has already answered.
func TestWinDeleteIsRememberedUntilAnswered(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	// Negative. An SA that sent no request remembers no datagram.
	if len(ini.requestMsg) != 0 {
		t.Fatalf("a fresh SA already holds %d remembered bytes", len(ini.requestMsg))
	}

	ps.sendDeleteESP(ini, myTr, winESPSPI, log)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the ESP Delete never reached the peer")
	}
	firstID := parseMsg(t, first).Header.MessageID
	if !bytes.Equal(ini.requestMsg, first) {
		t.Fatalf("the SA remembered %d bytes, want the %d it sent", len(ini.requestMsg), len(first))
	}

	// The answer has not arrived, so the owner loop repeats the request once its
	// backoff passes.
	due := ini.requestLastSent.Add(retransmitBackoff(1))
	ps.serviceRequestRetransmit(ini, nil, myTr, due, log)
	again := rtxRecv(t, peerTr)
	if again == nil {
		t.Fatal("the unanswered Delete was never repeated")
	}
	if got := parseMsg(t, again).Header.MessageID; got != firstID {
		t.Errorf("the repeat carries Message ID %d, want %d, the id of the request it repeats", got, firstID)
	}
	if got := winDeleteSPIs(t, resp, again); len(got) != 1 || got[0] != winESPSPI {
		t.Errorf("the repeat names SPIs %v, want [%#x]", got, winESPSPI)
	}

	// Negative. The peer answers, and the memory ends with the exchange.
	answer := winInformationalAnswer(t, resp, firstID)
	if out := ps.handleOwnedInbound(ini, transport.Packet{Data: answer}, myTr, nil, log); !out.dpdResp {
		t.Fatalf("the peer answer was not read as a response to the Delete: %+v", out)
	}
	if len(ini.requestMsg) != 0 {
		t.Errorf("the SA still holds %d bytes of an answered request", len(ini.requestMsg))
	}
	ps.serviceRequestRetransmit(ini, nil, myTr, due.Add(time.Minute), log)
	rtxExpectSilence(t, peerTr, myTr, remote, "a Delete the peer has already answered")
}

// VALIDATES: a teardown notify that the request window refuses spends no Message ID.
// PREVENTS: a spent-but-unused id. RFC 7296 Section 2.2 makes the peer expect the next
// request at the id no message carried, and with a window of one every later request
// on the SA then falls outside it.
func TestWinRefusedTeardownSpendsNoMessageID(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	sendDPD(ini, myTr, winDueDPD(), log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the DPD probe that holds the window never reached the peer")
	}
	held := ini.NextMsgID

	sendIKESATeardown(ini, myTr, wire.NotifyTSUnacceptable, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "a teardown notify while a request is outstanding")
	if ini.NextMsgID != held {
		t.Errorf("the refused teardown moved NextMsgID from %d to %d, so it spent an id on a message it never sent",
			held, ini.NextMsgID)
	}

	// Negative. With a free window the teardown goes out, and only then does it spend
	// the id. The refusal above is therefore about the window and not a mute sender.
	ini.releaseRequestWindow()
	sendIKESATeardown(ini, myTr, wire.NotifyTSUnacceptable, log)
	sent := rtxRecv(t, peerTr)
	if sent == nil {
		t.Fatal("a teardown notify with a free window wrote nothing")
	}
	if ini.NextMsgID != held+1 {
		t.Errorf("NextMsgID = %d after the teardown went out, want %d", ini.NextMsgID, held+1)
	}
	if got := parseMsg(t, sent).Header.MessageID; got != held+1 {
		t.Errorf("the teardown carries Message ID %d, want %d", got, held+1)
	}
	if _, err := decryptAndParse(resp, parseMsg(t, sent), sent); err != nil {
		t.Fatalf("the peer could not decrypt the teardown: %v", err)
	}
}

// VALIDATES: an operator clear returns promptly, and the goodbye Delete obeys the one
// request window. It goes out when no request is outstanding. It WAITS when one is, and
// sends as soon as the peer answers. It is not sent at all when that answer never comes.
// PREVENTS: two of Ze's own requests unanswered at the peer at once, and a teardown
// that blocks the operator's command behind a dead peer.
//
// RFC requirement: RFC7296-2.3-2 positive -- Section 2.3 MUST: "An IKE endpoint MUST wait for a
// response to each of its messages before sending a subsequent message unless it has
// received a SET_WINDOW_SIZE Notify message from its peer". Ze sends no SET_WINDOW_SIZE,
// so its window is one. sayGoodbye (established.go) waits rather than sends, and it
// sends the moment waitForRequestWindow sees an authenticated answer.
//
// RFC requirement: RFC7296-2.3-2 negative -- the wait is not a mute sender. The free-window case
// writes the same Delete at once. The silence of the unanswered case therefore belongs
// to the window rule, and not to a Delete path that never works.
func TestWinTeardownDoesNotHang(t *testing.T) {
	cases := []struct {
		name        string
		holdWindow  bool
		answerProbe bool
		wantGoodbye bool
	}{
		{name: "window free", wantGoodbye: true},
		{name: "window held and the peer answers", holdWindow: true, answerProbe: true, wantGoodbye: true},
		{name: "window held and the peer stays silent", holdWindow: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			peer, sa, ps := establishPSK(t)
			peerTr, myTr := rtxPeerLink(t)
			sa.PeerCfg.RemoteAddress = "127.0.0.1"
			remote := sa.remoteUDPAddr()
			if remote == nil {
				t.Fatal("the SA has no resolvable peer address")
			}
			ps.stopCh = make(chan struct{})
			ps.supersede = make(chan struct{}, 1)
			ps.inbound = make(chan transport.Packet, 1)
			ps.graceful.Store(true)

			var probeID uint32
			if tc.holdWindow {
				sendDPD(sa, myTr, winDueDPD(), log)
				probe := rtxRecv(t, peerTr)
				if probe == nil {
					t.Fatal("the probe that holds the window never reached the peer")
				}
				probeID = parseMsg(t, probe).Header.MessageID
			}

			close(ps.stopCh)
			done := make(chan struct{})
			go func() {
				_ = ps.maintainSA(sa, nil, nil, nil,
					testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
				close(done)
			}()
			if tc.answerProbe {
				// The answer travels the real inbound path, so only an
				// authenticated one can end the wait.
				ps.inbound <- transport.Packet{Data: winInformationalAnswer(t, peer, probeID)}
			}

			// goodbyeWindowWait bounds the silent case, so the loop returns inside
			// it plus slack rather than waiting on the peer.
			select {
			case <-done:
			case <-time.After(goodbyeWindowWait + rtxArrive):
				t.Fatal("the owner loop did not return on a graceful stop")
			}

			if !tc.wantGoodbye {
				rtxExpectSilence(t, peerTr, myTr, remote,
					"a graceful stop whose outstanding request was never answered")
				return
			}
			got := rtxRecv(t, peerTr)
			if got == nil {
				t.Fatal("a graceful stop wrote no goodbye Delete")
			}
			sent := parseMsg(t, got)
			if sent.Header.ExchangeType != wire.ExchangeInformational {
				t.Error("the goodbye datagram is not an INFORMATIONAL message")
			}
			if sent.Header.Flags&wire.FlagResponse != 0 {
				t.Error("the goodbye carries the Response flag, so it is not a request")
			}
			if tc.holdWindow && sent.Header.MessageID == probeID {
				t.Errorf("the goodbye reused the probe's Message ID %d", probeID)
			}
			// The peer is the authority on what it received, so the payload is read
			// back through its own decrypt rather than from our own build.
			inner, err := decryptAndParse(peer, sent, got)
			if err != nil {
				t.Fatalf("the peer could not decrypt the goodbye: %v", err)
			}
			sawIKEDelete := false
			for i := range inner {
				if d, ok := inner[i].Payload.(*wire.PayloadDelete); ok && d.ProtocolID == wire.ProtocolIKE {
					sawIKEDelete = true
				}
			}
			if !sawIKEDelete {
				t.Error("the goodbye datagram carries no IKE Delete payload")
			}
		})
	}
}
