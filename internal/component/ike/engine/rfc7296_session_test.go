package engine

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// sesSamples is the number of nonces the randomness check draws. Across 256 draws
// over 256 byte values, one byte position holds about 162 distinct values.
const sesSamples = 256

// sesVarietyFloor is the least number of distinct values one byte position must
// hold across sesSamples draws. The floor sits far below the expected 162, so a
// real random source clears it every time. A counter scores 1.
const sesVarietyFloor = 32

// sesMinPositionVariety returns the smallest number of distinct byte values that
// any single byte position holds across the samples. This metric separates random
// bytes from a counter. A counter varies one byte and holds the rest fixed, so it
// scores 1 whatever its length and whatever its distinctness.
func sesMinPositionVariety(t *testing.T, samples [][]byte) int {
	t.Helper()
	if len(samples) == 0 || len(samples[0]) == 0 {
		t.Fatal("the variety metric needs at least one sample of non-zero width")
	}
	width := len(samples[0])
	worst := -1
	for pos := range width {
		var seen [256]bool
		count := 0
		for _, s := range samples {
			if len(s) != width {
				t.Fatalf("sample width = %d, want %d", len(s), width)
			}
			if !seen[s[pos]] {
				seen[s[pos]] = true
				count++
			}
		}
		if worst < 0 || count < worst {
			worst = count
		}
	}
	return worst
}

// sesDistinctCount returns how many of the samples are unique as byte strings.
func sesDistinctCount(samples [][]byte) int {
	seen := make(map[string]bool, len(samples))
	for _, s := range samples {
		seen[string(s)] = true
	}
	return len(seen)
}

// sesCounterNonces returns a sequence shaped like the output of a counter. Every
// value is unique and every value has the full width. Only the last two bytes
// change, which is what the variety metric detects.
func sesCounterNonces(n, width int) [][]byte {
	out := make([][]byte, 0, n)
	for i := range n {
		b := make([]byte, width)
		b[width-1] = byte(i)
		b[width-2] = byte(i >> 8)
		out = append(out, b)
	}
	return out
}

// sesConstantNonces returns n copies of one value, the shape of a producer that
// reuses a nonce.
func sesConstantNonces(n, width int) [][]byte {
	out := make([][]byte, 0, n)
	for range n {
		b := make([]byte, width)
		b[0] = 0xA5
		out = append(out, b)
	}
	return out
}

// VALIDATES: IKE nonces come from a random source and are never handed out twice.
// RFC requirement: RFC7296-2.10-4 positive -- GenerateNonce (sa.go:158) fills the whole
// buffer from crypto/rand, so every byte position holds many distinct values.
// RFC requirement: RFC7296-2.10-4 negative -- a counter of the same width and the same
// distinctness scores 1 on that metric, so unique output alone proves nothing.
// RFC requirement: RFC7296-3.9-2 positive -- 256 draws from GenerateNonce yield 256
// distinct values, so no nonce value returns a second time.
// RFC requirement: RFC7296-3.9-2 negative -- a producer that repeats one value scores 1 on
// the same comparison, so the count reads the draw and not the sample shape.
func TestSesNoncesAreRandomlyChosenAndNeverReused(t *testing.T) {
	nonces := make([][]byte, 0, sesSamples)
	for range sesSamples {
		n, err := GenerateNonce(nonceLen)
		if err != nil {
			t.Fatalf("GenerateNonce: %v", err)
		}
		if len(n) != nonceLen {
			t.Fatalf("nonce width = %d, want %d", len(n), nonceLen)
		}
		nonces = append(nonces, n)
	}

	// RFC7296-2.10-4. Every byte position varies, which no counter achieves.
	variety := sesMinPositionVariety(t, nonces)
	if variety < sesVarietyFloor {
		t.Errorf("weakest nonce byte position held %d distinct values, want at least %d",
			variety, sesVarietyFloor)
	}

	// RFC7296-3.9-2. No value repeats across the draws.
	if got := sesDistinctCount(nonces); got != sesSamples {
		t.Errorf("distinct nonces = %d, want %d", got, sesSamples)
	}

	// Negative for RFC7296-2.10-4. A counter passes both the width check and the
	// distinctness check, so the variety metric is what carries the randomness claim.
	counter := sesCounterNonces(sesSamples, nonceLen)
	if got := sesDistinctCount(counter); got != sesSamples {
		t.Fatalf("the counter reference is not fully distinct: %d values", got)
	}
	if got := sesMinPositionVariety(t, counter); got >= sesVarietyFloor {
		t.Fatalf("the variety metric scored a counter at %d, so it does not detect one", got)
	}

	// Negative for RFC7296-3.9-2. A producer that repeats one value fails the
	// distinctness comparison, so a pass belongs to the draw.
	repeated := sesConstantNonces(sesSamples, nonceLen)
	if got := sesDistinctCount(repeated); got != 1 {
		t.Fatalf("the repeated reference scored %d distinct values, want 1", got)
	}
}

// VALIDATES: an IKE SA rekey draws a new nonce on each side instead of a reuse.
// RFC requirement: RFC7296-3.9-2 positive -- initiateIKERekey draws Ni at rekey.go:303 and
// respondIKERekey draws Nr at rekey.go:519, so neither side resends an old nonce.
// RFC requirement: RFC7296-3.9-2 negative -- both old SAs already hold a non-empty nonce,
// so a reuse would be visible. All four nonces differ from each other.
func TestSesRekeyDrawsFreshNoncesOnBothSides(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)

	// Negative. The old nonces exist, so "different" is a real change.
	if len(ini.LocalNonce) != nonceLen || len(resp.LocalNonce) != nonceLen {
		t.Fatalf("old nonce widths = ini:%d resp:%d, want %d each",
			len(ini.LocalNonce), len(resp.LocalNonce), nonceLen)
	}
	if bytes.Equal(ini.LocalNonce, resp.LocalNonce) {
		t.Fatal("the two IKE_SA_INIT nonces already match, so the rekey check proves nothing")
	}

	req, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()

	reqInner, err := decryptAndParse(resp, parseMsg(t, req), req)
	if err != nil {
		t.Fatalf("responder decrypt of the rekey request: %v", err)
	}
	respBytes, newResp, err := respondIKERekey(resp, reqInner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	respInner, err := decryptAndParse(ini, parseMsg(t, respBytes), respBytes)
	if err != nil {
		t.Fatalf("initiator decrypt of the rekey response: %v", err)
	}
	newIni, err := applyIKERekeyResponse(ini, pending, respInner, log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}

	// Positive. Each side put a new nonce on the wire for the new SA.
	all := [][]byte{ini.LocalNonce, resp.LocalNonce, newIni.LocalNonce, newResp.LocalNonce}
	for _, n := range all {
		if len(n) != nonceLen {
			t.Fatalf("nonce width = %d, want %d", len(n), nonceLen)
		}
	}
	if got := sesDistinctCount(all); got != len(all) {
		t.Errorf("distinct nonces across the rekey = %d, want %d", got, len(all))
	}
	// The responder's new nonce also reached the initiator, so the reuse check
	// covers the wire value and not a private field alone.
	if !bytes.Equal(newIni.RemoteNonce, newResp.LocalNonce) {
		t.Error("the initiator did not receive the responder's new nonce")
	}
}

// sesSilentHandshake runs runInitiator against a peer that never answers, with the
// wait replaced by an instant channel. It returns the waits the loop asked for, the
// SA, and the error the session ended with. Each stub call rewinds the deadline, so
// the next pass through the loop retransmits without real elapsed time.
func sesSilentHandshake(t *testing.T) (waits []time.Duration, sa *SA, err error) {
	t.Helper()
	log := slogutil.DiscardLogger()
	_, myTr := rtxPeerLink(t)

	peer := testPeer()
	peer.LocalAddress = "127.0.0.1"
	peer.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{
		peerName: "ze",
		peerCfg:  peer,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
		stopCh:   make(chan struct{}),
	}

	var mu sync.Mutex
	recorded := make([]time.Duration, 0, 16)
	old := afterFunc
	// The stub runs on the session goroutine, so the deadline rewind below writes
	// the SA field from its owning goroutine.
	afterFunc = func(d time.Duration) <-chan time.Time {
		mu.Lock()
		recorded = append(recorded, d)
		mu.Unlock()
		if cur := ps.getSA(); cur != nil {
			cur.RetransmitTime = time.Now().Add(-time.Millisecond)
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	done := make(chan struct{})
	go func() {
		err = ps.runInitiator(peer, testIKEGroup(), NewSATable(), myTr, nil, log)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(rtxArrive):
		close(ps.stopCh)
		<-done
		t.Fatal("the silent handshake never reached its verdict")
	}
	afterFunc = old

	mu.Lock()
	waits = append([]time.Duration(nil), recorded...)
	mu.Unlock()
	return waits, ps.getSA(), err
}

// VALIDATES: the wait between handshake retransmissions doubles each attempt.
// RFC requirement: RFC7296-2.4-13 positive -- retransmitBackoff (fsm.go:768) doubles the
// base delay per attempt, and runInitiator applies it at fsm.go:136.
// RFC requirement: RFC7296-2.4-13 negative -- a constant timer repeats one wait, and every
// recorded wait sits above three quarters of the exact backoff for its attempt.
func TestSesRetransmitWaitIncreasesExponentially(t *testing.T) {
	// The pure function doubles from the base and stops at the cap.
	if got := retransmitBackoff(1); got != retransmitBase {
		t.Errorf("retransmitBackoff(1) = %v, want %v", got, retransmitBase)
	}
	for attempt := 2; attempt <= 7; attempt++ {
		want := retransmitBackoff(attempt-1) * 2
		if got := retransmitBackoff(attempt); got != want {
			t.Errorf("retransmitBackoff(%d) = %v, want %v", attempt, got, want)
		}
	}
	if got := retransmitBackoff(20); got != retransmitMax {
		t.Errorf("retransmitBackoff(20) = %v, want the cap %v", got, retransmitMax)
	}

	waits, _, _ := sesSilentHandshake(t)
	if len(waits) < maxRetransmissions+1 {
		t.Fatalf("the loop asked for %d waits, want at least %d", len(waits), maxRetransmissions+1)
	}

	// waits[0] is the first wait after the original request. waits[n] is the wait
	// the loop applied after retransmission n. The expected value is computed here
	// rather than read from retransmitBackoff, so a change to that function cannot
	// move the target it is measured against.
	if waits[0] > retransmitBase {
		t.Errorf("the first wait was %v, want at most the base %v", waits[0], retransmitBase)
	}
	for attempt := 1; attempt <= maxRetransmissions; attempt++ {
		exact := min(retransmitBase*time.Duration(1<<uint(attempt-1)), retransmitMax)
		got := waits[attempt]
		if got > exact {
			t.Errorf("wait after retransmission %d = %v, want at most %v", attempt, got, exact)
		}
		// A constant 500 ms timer fails this bound from the third attempt onward.
		if got*4 <= exact*3 {
			t.Errorf("wait after retransmission %d = %v, want more than %v",
				attempt, got, exact*3/4)
		}
	}
}

// VALIDATES: an endpoint is declared failed only after repeated silence.
// RFC requirement: RFC7296-2.4-11 positive -- runInitiator returns errTimeout only once
// RetransmitCount reaches maxRetransmissions (fsm.go), after 7 resends.
// The second producer is Dead Peer Detection. dpdState.timedOut (dpd.go) reports
// failure only past the timeout, and maintainSA (established.go) reads it for the
// verdict.
// RFC requirement: RFC7296-2.4-11 negative -- one unanswered attempt does not reach the
// verdict, and a probe that draws an answer never reaches the timeout at all.
func TestSesPeerFailedOnlyAfterRepeatedSilence(t *testing.T) {
	log := slogutil.DiscardLogger()

	// Producer one. The handshake tries repeatedly before it declares failure.
	_, sa, err := sesSilentHandshake(t)
	if !errors.Is(err, errTimeout) {
		t.Fatalf("the silent handshake ended with %v, want errTimeout", err)
	}
	if sa == nil {
		t.Fatal("the silent handshake left no SA to inspect")
	}
	if sa.RetransmitCount != maxRetransmissions {
		t.Errorf("retransmissions before the verdict = %d, want %d",
			sa.RetransmitCount, maxRetransmissions)
	}
	// Negative. One attempt is not enough, because the cap is above one.
	if maxRetransmissions < 2 {
		t.Fatal("the retransmission cap is 1, so the verdict follows a single attempt")
	}

	// Producer two. A probe under Dead Peer Detection.
	dpd := newDPDState(ipsec.DPDConfig{Interval: 30, Timeout: 90, Action: ipsec.DPDActionRestart})
	if dpd == nil {
		t.Fatal("newDPDState returned nil")
	}
	probeSA, _, _, _, probeTr := dpdProbeLink(t)
	probeSA.NextMsgID = 1
	sendDPD(probeSA, probeTr, dpd, log)
	if !dpd.awaitingReply() {
		t.Fatal("the probe left no outstanding wait")
	}

	// Negative. The peer is not failed while the timeout runs.
	if dpd.timedOut(dpd.sentAt) {
		t.Error("the peer was declared failed at the moment of the probe")
	}
	if dpd.timedOut(dpd.sentAt.Add(dpd.timeout - time.Millisecond)) {
		t.Error("the peer was declared failed before the timeout elapsed")
	}
	// Past the timeout with nothing repeated the verdict does NOT stand: Section
	// 2.4 asks for repeated attempts, and the elapsed budget is only half of that.
	// This assertion read the budget alone before 2026-08-18, which is the shape
	// the requirement refuses.
	past := dpd.sentAt.Add(dpd.timeout)
	if dpd.timedOut(past) {
		t.Error("the peer was declared failed on one unanswered attempt")
	}
	// Positive. Past the timeout, with a repeat that also went unanswered, it does.
	dpd.noteRetransmit(past)
	if !dpd.timedOut(past) {
		t.Error("the peer was not declared failed after a repeat also went unanswered")
	}

	// Negative. An answered probe clears the wait, so no timeout is ever reached.
	handleDPDResponse(dpd, log, "ze")
	if dpd.timedOut(dpd.sentAt.Add(10 * dpd.timeout)) {
		t.Error("an answered probe still declared the peer failed")
	}
}

// sesFirstSAPayload returns the SA payload among the entries, or fails.
func sesFirstSAPayload(t *testing.T, entries []wire.PayloadEntry, what string) *wire.PayloadSA {
	t.Helper()
	for i := range entries {
		if p, ok := entries[i].Payload.(*wire.PayloadSA); ok {
			return p
		}
	}
	t.Fatalf("%s carried no SA payload", what)
	return nil
}

// sesSoleProtocol returns the protocol id of a response that carries exactly one
// proposal, and fails when the count differs.
func sesSoleProtocol(t *testing.T, p *wire.PayloadSA, what string) uint8 {
	t.Helper()
	if len(p.Proposals) != 1 {
		t.Fatalf("%s carried %d proposals, want exactly 1", what, len(p.Proposals))
	}
	return p.Proposals[0].ProtocolID
}

// VALIDATES: an accepted proposal comes back under the protocol it was offered on.
// RFC requirement: RFC7296-2.7-2 positive -- two producers echo the offered protocol. SAr1
// carries IKE through chosenIKEProposalToWire (responder.go:273) and SAr2 carries
// ESP through buildWireESPProposals (initiator.go:414).
// RFC requirement: RFC7296-2.7-2 negative -- the two responses carry different protocol
// ids, so neither is a fixed constant. An ESP offer sent under the IKE protocol id
// is refused by selectResponderESP (responder.go:431) and is never echoed.
func TestSesAcceptedProposalKeepsItsProtocol(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	// IKE_SA_INIT. The offer is IKE and the single chosen proposal is IKE.
	offer1 := sesFirstSAPayload(t, parseMsg(t, ini.InitiatorSAInitMsg).Payloads, "SAi1")
	if len(offer1.Proposals) == 0 {
		t.Fatal("SAi1 offered no proposals")
	}
	for _, p := range offer1.Proposals {
		if p.ProtocolID != wire.ProtocolIKE {
			t.Fatalf("SAi1 offered protocol %d, want IKE", p.ProtocolID)
		}
	}
	answer1 := sesFirstSAPayload(t, parseMsg(t, resp.ResponderSAInitMsg).Payloads, "SAr1")
	gotIKE := sesSoleProtocol(t, answer1, "SAr1")
	if gotIKE != wire.ProtocolIKE {
		t.Errorf("SAr1 answered with protocol %d, want IKE (%d)", gotIKE, wire.ProtocolIKE)
	}

	// IKE_AUTH. The offer is ESP and the single chosen proposal is ESP.
	authReq, err := decryptAndParse(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg)
	if err != nil {
		t.Fatalf("responder decrypt of the IKE_AUTH request: %v", err)
	}
	offer2 := sesFirstSAPayload(t, authReq, "SAi2")
	for _, p := range offer2.Proposals {
		if p.ProtocolID != wire.ProtocolESP {
			t.Fatalf("SAi2 offered protocol %d, want ESP", p.ProtocolID)
		}
	}
	authResp, err := decryptAndParse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg)
	if err != nil {
		t.Fatalf("initiator decrypt of the IKE_AUTH response: %v", err)
	}
	answer2 := sesFirstSAPayload(t, authResp, "SAr2")
	gotESP := sesSoleProtocol(t, answer2, "SAr2")
	if gotESP != wire.ProtocolESP {
		t.Errorf("SAr2 answered with protocol %d, want ESP (%d)", gotESP, wire.ProtocolESP)
	}

	// Negative. The two answers differ, so the echo is not one hardcoded value.
	if gotIKE == gotESP {
		t.Fatal("both responses named the same protocol, so the echo proves nothing")
	}

	// Negative. A proposal offered under the wrong protocol id is not accepted, so
	// the responder can never echo a protocol it did not agree to.
	wrong := &wire.PayloadSA{Proposals: buildWireESPProposals(testESPGroup(), 0x11223344)}
	probe := &SA{ESPGroup: testESPGroup()}
	if err := selectResponderESP(probe, wrong); err != nil {
		t.Fatalf("the reference ESP offer was refused before its protocol id changed: %v", err)
	}
	for i := range wrong.Proposals {
		wrong.Proposals[i].ProtocolID = wire.ProtocolIKE
	}
	probe = &SA{ESPGroup: testESPGroup()}
	if err := selectResponderESP(probe, wrong); !errors.Is(err, crypto.ErrNoProposalChosen) {
		t.Errorf("an ESP offer under the IKE protocol id returned %v, want ErrNoProposalChosen", err)
	}
}

// sesRespondPeer registers one `respond` peer whose configured remote address is
// 10.0.0.1 and returns its session and an empty SA table.
func sesRespondPeer(t *testing.T) (*PeerSession, *SATable) {
	t.Helper()
	// The COOKIE challenge is an admission gate these tests must pass to reach their
	// own subject; rfc7296_cookie_test.go proves the gate itself.
	admitWithoutCookieChallenge(t)
	_, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	ps := &PeerSession{
		peerName: "ze",
		peerCfg:  respPeer,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
	}
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })
	return ps, NewSATable()
}

// VALIDATES: an inbound request is accepted whatever source port it arrives on.
// RFC requirement: RFC7296-2.11-1 positive -- matchResponderPeer (register.go:540) compares
// the source address alone, so ports outside 500 and 4500 are accepted.
// tryResponderSAInit then builds the responder SA from such a datagram.
// RFC requirement: RFC7296-2.11-1 negative -- a datagram from another address on port 500
// is refused, so the source is genuinely examined and acceptance is not blanket.
func TestSesAcceptsRequestFromAnySourcePort(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps, table := sesRespondPeer(t)

	ports := []int{500, 4500, 1024, 33333, 65535}
	for _, port := range ports {
		addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: port}
		if got := matchResponderPeer(addr); got != ps {
			t.Errorf("matchResponderPeer on source port %d returned %v, want the session", port, got)
		}
	}

	// Negative. Another address is refused on the well-known port.
	stranger := &net.UDPAddr{IP: net.ParseIP("203.0.113.9"), Port: 500}
	if got := matchResponderPeer(stranger); got != nil {
		t.Error("an unconfigured source address matched a responder peer")
	}

	// The whole acceptance path runs on a datagram from an ephemeral source port.
	ini, err := newInitiatorSA("ze", func() ipsec.SiteToSitePeer {
		p, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
		return p
	}(), testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, testIKEGroup())
	pkt := transport.Packet{
		Data:       req,
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 33333},
	}
	var iSPI, rSPI [8]byte
	copy(iSPI[:], ini.InitiatorSPI[:])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("an IKE_SA_INIT from source port 33333 was not accepted")
	}
	if table.Len() != 1 {
		t.Fatalf("the accepted request created %d SAs, want 1", table.Len())
	}
	if sa := ps.getSA(); sa == nil || sa.State != StateSAInitReceived {
		t.Fatal("the request from source port 33333 did not advance the responder SA")
	}

	// Negative. The same request from an unconfigured address creates nothing.
	ps.responderBusy.Store(false)
	strangerPkt := transport.Packet{Data: req, RemoteAddr: stranger}
	if tryResponderSAInit(strangerPkt, iSPI, rSPI, table, nil, log) {
		t.Error("an IKE_SA_INIT from an unconfigured address was consumed")
	}
	if table.Len() != 1 {
		t.Errorf("the refused request left %d SAs, want 1", table.Len())
	}
}

// VALIDATES: the rate of work done for unprotected messages is bounded.
// RFC requirement: RFC7296-2.4-12 positive -- inboundRateLimiter.allow (register.go:414)
// denies once the burst is spent. Both read loops call it at register.go:458 and
// register.go:628 before any other work. A second producer at register.go:586
// bounds the responder to one half-open handshake per peer.
// RFC requirement: RFC7296-2.4-12 negative -- the first request is served. The bucket also
// refills with time, so the limiter bounds a rate and refuses nothing outright.
func TestSesLimitsWorkOnUnprotectedMessages(t *testing.T) {
	log := slogutil.DiscardLogger()

	// The production settings, as both read loops construct them.
	const perSecond, burst = 100, 200
	limiter := newInboundRateLimiter(perSecond, burst)

	// Negative. The first request is served, so the limit is not a blanket refusal.
	if !limiter.allow() {
		t.Fatal("the first unprotected message was refused")
	}
	for i := 2; i <= burst; i++ {
		if !limiter.allow() {
			t.Fatalf("the burst ended at message %d, want %d", i, burst)
		}
	}

	// Positive. Past the burst the limiter denies. A refill needs 10 ms per token,
	// so a tight loop of 200 more calls cannot earn its way through.
	denied := false
	for range burst {
		if !limiter.allow() {
			denied = true
			break
		}
	}
	if !denied {
		t.Errorf("%d messages beyond the burst of %d were all served", burst, burst)
	}

	// Negative. Time refills the bucket, so the limiter bounds a rate.
	limiter.lastFill = time.Now().Add(-2 * time.Second)
	if !limiter.allow() {
		t.Error("the limiter stayed shut after a full refill interval")
	}

	// Second producer. A flood of unsolicited IKE_SA_INIT builds one half-open SA.
	ps, table := sesRespondPeer(t)
	for i := range 50 {
		ini, err := newInitiatorSA("ze", func() ipsec.SiteToSitePeer {
			p, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
			return p
		}(), testIKEGroup(), testESPGroup())
		if err != nil {
			t.Fatalf("newInitiatorSA %d: %v", i, err)
		}
		req := buildSAInitRequest(ini, testIKEGroup())
		pkt := transport.Packet{
			Data:       req,
			RemoteAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 500},
		}
		var iSPI, rSPI [8]byte
		copy(iSPI[:], ini.InitiatorSPI[:])
		tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log)
	}
	if table.Len() != 1 {
		t.Errorf("a flood of 50 unprotected requests built %d SAs, want 1", table.Len())
	}
	if !ps.responderBusy.Load() {
		t.Error("the responder did not hold its half-open gate after the flood")
	}
}

// sesNATHashes returns the NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP
// notification data found among the payloads. A missing payload returns nil.
func sesNATHashes(entries []wire.PayloadEntry) (src, dst []byte) {
	for i := range entries {
		p, ok := entries[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		switch p.NotifyMsgType {
		case wire.NotifyNATDetectionSourceIP:
			src = p.NotificationData
		case wire.NotifyNATDetectionDestIP:
			dst = p.NotificationData
		}
	}
	return src, dst
}

// VALIDATES: both ends put both NAT detection notifies in IKE_SA_INIT.
// RFC requirement: RFC7296-2.23-7 positive -- the initiator adds both at initiator.go:69
// and the responder adds both at responder.go:235. Both call buildNATDetectionPayloads.
// RFC requirement: RFC7296-2.23-7 negative -- each end computes its own pair of hashes from
// its own addresses and SPIs. The responder pair is therefore neither one payload
// sent twice nor an echo of the initiator pair.
func TestSesBothEndsSendNATDetectionNotifies(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	iniSrc, iniDst := sesNATHashes(parseMsg(t, ini.InitiatorSAInitMsg).Payloads)
	if len(iniSrc) == 0 {
		t.Error("the initiator IKE_SA_INIT carried no NAT_DETECTION_SOURCE_IP notify")
	}
	if len(iniDst) == 0 {
		t.Error("the initiator IKE_SA_INIT carried no NAT_DETECTION_DESTINATION_IP notify")
	}

	respSrc, respDst := sesNATHashes(parseMsg(t, resp.ResponderSAInitMsg).Payloads)
	if len(respSrc) == 0 {
		t.Error("the responder IKE_SA_INIT carried no NAT_DETECTION_SOURCE_IP notify")
	}
	if len(respDst) == 0 {
		t.Error("the responder IKE_SA_INIT carried no NAT_DETECTION_DESTINATION_IP notify")
	}
	if t.Failed() {
		t.Fatal("a NAT detection notify is missing, so the hash comparisons cannot run")
	}

	// Negative. Within one message the two hashes differ, so neither end sends one
	// payload twice under two notify types.
	if bytes.Equal(iniSrc, iniDst) {
		t.Error("the initiator sent the same hash as source and as destination")
	}
	if bytes.Equal(respSrc, respDst) {
		t.Error("the responder sent the same hash as source and as destination")
	}

	// Negative. The responder pair is computed locally, so it matches neither of
	// the initiator hashes.
	if bytes.Equal(respSrc, iniSrc) || bytes.Equal(respSrc, iniDst) {
		t.Error("the responder source hash repeats an initiator hash, so it is an echo")
	}
	if bytes.Equal(respDst, iniSrc) || bytes.Equal(respDst, iniDst) {
		t.Error("the responder destination hash repeats an initiator hash, so it is an echo")
	}
}
