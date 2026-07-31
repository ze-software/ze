package engine

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ckeRespondPeer registers a `respond` peer whose configured remote is 10.0.0.1 and
// returns it with an empty SA table. The cookie challenge is left ON: these tests are
// about the challenge itself.
func ckeRespondPeer(t *testing.T) (*PeerSession, *SATable) {
	t.Helper()
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

// ckeInboundInit builds an IKE_SA_INIT a configured peer would send, optionally
// carrying a cookie, and returns the initiator SA beside the packet.
func ckeInboundInit(t *testing.T, cookie []byte) (*SA, transport.Packet) {
	t.Helper()
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	ini.Cookie = cookie
	req := buildSAInitRequest(ini, testIKEGroup())
	return ini, transport.Packet{
		Data:       req,
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 500},
	}
}

// ckeCookieResponse builds the IKE_SA_INIT response a pressured responder sends.
func ckeCookieResponse(spiI, spiR [8]byte, cookie []byte) []byte {
	return kegNotifyResponse(spiI, spiR, wire.NotifyCookie, cookie)
}

// VALIDATES: every cookie this node mints is 1..64 octets, and sendCookieChallenge
// refuses to put one of any other length on the wire.
// PREVENTS: a cookie outside the bound reaching a peer, which RFC 7296 Section 2.6
// forbids outright.
// RFC requirement: RFC7296-2.6-3 positive -- mintCookie (cookie.go) returns
// cookieDataLen octets and sendCookieChallenge (responder.go) re-checks the 1..64 bound
// before it encodes, so the constant alone is never the guarantee.
func TestCkeMintedCookieIsWithinTheLengthBound(t *testing.T) {
	resetCookieSecret(t)
	log := slogutil.DiscardLogger()
	now := time.Now()

	for i := range 256 {
		spiI := [8]byte{byte(i), 2, 3, 4, 5, 6, 7, byte(i >> 4)}
		ni := bytes.Repeat([]byte{byte(i)}, 16+i%17)
		ip := net.IPv4(198, 51, 100, byte(i))
		cookie := mintCookie(spiI, ni, ip, now)
		if len(cookie) < minCookieLen || len(cookie) > maxCookieLen {
			t.Fatalf("mint %d produced a %d-octet cookie, outside the 1..64 bound", i, len(cookie))
		}
	}

	// The sender refuses a cookie outside the bound. The counter rises only when a
	// challenge is actually put on the wire, so it reports the refusal.
	spiI := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	before := cookieChallengeCount("ze")
	sendCookieChallenge(nil, &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}, spiI, nil, "ze", log)
	sendCookieChallenge(nil, &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}, spiI, bytes.Repeat([]byte{1}, maxCookieLen+1), "ze", log)
	if got := cookieChallengeCount("ze"); got != before {
		t.Errorf("a challenge was sent for an out-of-bound cookie: count went %d -> %d", before, got)
	}
	otherBefore := cookieChallengeCount("other-peer")
	sendCookieChallenge(nil, &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}, spiI, bytes.Repeat([]byte{1}, maxCookieLen), "ze", log)
	if got := cookieChallengeCount("ze"); got != before+1 {
		t.Errorf("a 64-octet cookie was refused: count went %d -> %d", before, got)
	}
	// The counter is labeled by peer, so an operator can tell which peer is being
	// probed. A single shared total would read the same on this assertion.
	if got := cookieChallengeCount("other-peer"); got != otherBefore {
		t.Errorf("a challenge to one peer moved another peer's counter: %d -> %d", otherBefore, got)
	}
}

// VALIDATES: the bound holds on the ECHO path too. A 65-octet cookie from a peer is
// refused, a 64-octet one is echoed.
// PREVENTS: reflecting a peer-chosen blob of any size back at it. The bound governs two
// paths, and a mint-side-only check leaves the attacker-controlled one open.
// RFC requirement: RFC7296-2.6-3 negative -- retrySAInit (sa_init_retry.go) checks the
// received length before it stores sa.Cookie. The 64/65 pair proves the bound is
// inclusive, not exclusive.
func TestCkeEchoedCookieIsBoundedToo(t *testing.T) {
	log := slogutil.DiscardLogger()
	for _, tc := range []struct {
		name      string
		length    int
		wantRetry bool
	}{
		{"64 octets, the largest allowed", maxCookieLen, true},
		{"65 octets, one past the bound", maxCookieLen + 1, false},
		{"1 octet, the smallest allowed", minCookieLen, true},
		{"an empty body", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa, table := kegInitiator(t, kegIKEGroup())
			first := append([]byte(nil), sa.LastSentMsg...)
			resp := ckeCookieResponse(sa.InitiatorSPI, [8]byte{}, bytes.Repeat([]byte{0x5a}, tc.length))
			handleSAInitResponse(sa, parseMsg(t, resp), resp, table, nil, nil, log)

			retried := !bytes.Equal(sa.LastSentMsg, first)
			if retried != tc.wantRetry {
				t.Errorf("retry sent = %v, want %v for %s", retried, tc.wantRetry, tc.name)
			}
			if !tc.wantRetry && sa.State != StateDead {
				t.Errorf("state = %v, want dead after refusing %s", sa.State, tc.name)
			}
		})
	}
}

// VALIDATES: the retry carries the received cookie as its FIRST payload and changes
// nothing else -- same nonce bytes, same key exchange bytes, same initiator SPI, message
// ID still zero, and a responder SPI back to zero.
// PREVENTS: the plausible wrong implementation, which calls newInitiatorSA again and
// mints a fresh nonce and a fresh key. The responder computed its cookie over the nonce
// it already saw, so a fresh one never verifies and two conforming implementations loop
// until the retry budget is spent.
// RFC requirement: RFC7296-2.6-4 positive -- retrySAInit (sa_init_retry.go) touches only
// sa.Cookie on this path, and buildSAInitRequest (initiator.go) prepends the notify at
// slice position zero, which Message.WriteTo emits first.
func TestCkeRetryCarriesCookieFirstAndNothingElseChanged(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegIKEGroup())
	first := append([]byte(nil), sa.LastSentMsg...)
	firstNonce := kegRawPayload(t, first, wire.PayloadTypeNonce)
	firstKE := kegRawPayload(t, first, wire.PayloadTypeKE)
	spiBefore := sa.InitiatorSPI
	dhBefore := append([]byte(nil), sa.LocalDH.PublicKey...)
	nonceBefore := append([]byte(nil), sa.LocalNonce...)

	// A responder that sets a NON-zero responder SPI on its challenge. RFC 7296
	// Section 2.6 says it will be zero, and a peer that does otherwise must not make Ze
	// claim an IKE SA that does not exist.
	want := []byte{0xde, 0xad, 0xbe, 0xef}
	resp := ckeCookieResponse(sa.InitiatorSPI, [8]byte{9, 9, 9, 9, 9, 9, 9, 9}, want)
	handleSAInitResponse(sa, parseMsg(t, resp), resp, table, nil, nil, log)

	if sa.State != StateSAInitSent {
		t.Fatalf("state = %v, want sa-init-sent; the challenge was not answered", sa.State)
	}
	retry := parseMsg(t, sa.LastSentMsg)
	if len(retry.Payloads) == 0 {
		t.Fatal("the retry carries no payloads")
	}
	notify, ok := retry.Payloads[0].Payload.(*wire.PayloadNotify)
	if !ok || notify.NotifyMsgType != wire.NotifyCookie {
		t.Fatalf("payload 0 is %T, want a COOKIE notify", retry.Payloads[0].Payload)
	}
	if !bytes.Equal(notify.NotificationData, want) {
		t.Errorf("the echoed cookie is %x, want the received %x", notify.NotificationData, want)
	}
	if got := kegRawPayload(t, sa.LastSentMsg, wire.PayloadTypeNonce); !bytes.Equal(got, firstNonce) {
		t.Error("the retry carries a different nonce; the responder's cookie was minted over the first one")
	}
	if got := kegRawPayload(t, sa.LastSentMsg, wire.PayloadTypeKE); !bytes.Equal(got, firstKE) {
		t.Error("the retry carries a different key exchange payload; RFC 7296 Section 2.6 requires all other payloads unchanged")
	}
	if !bytes.Equal(sa.LocalNonce, nonceBefore) {
		t.Error("sa.LocalNonce was regenerated on a cookie retry")
	}
	if !bytes.Equal(sa.LocalDH.PublicKey, dhBefore) {
		t.Error("sa.LocalDH was rebuilt on a cookie retry")
	}
	if sa.InitiatorSPI != spiBefore {
		t.Error("the initiator SPI changed, so the retry is a different initiation")
	}
	if retry.Header.MessageID != 0 {
		t.Errorf("the retry's message ID is %d, want 0", retry.Header.MessageID)
	}
	if retry.Header.ResponderSPI != ([8]byte{}) {
		t.Errorf("the retry claims responder SPI %x; a cookie exchange creates no IKE SA, so it must be zero",
			retry.Header.ResponderSPI)
	}
	if sa.ResponderSPI != ([8]byte{}) {
		t.Error("sa.ResponderSPI kept the value the challenge carried")
	}
}

// VALIDATES: a first IKE_SA_INIT, sent before any challenge, carries no COOKIE payload.
// PREVENTS: a builder that always emits one. Without this, "payload 0 is a cookie" would
// pass over such a builder, and every peer would see a cookie it never issued.
// RFC requirement: RFC7296-2.6-4 negative -- the cookie is present only in ANSWER to a
// challenge. buildSAInitRequest emits it only when sa.Cookie holds octets.
func TestCkeCookieIsAbsentWithoutAChallenge(t *testing.T) {
	sa, _ := kegInitiator(t, kegIKEGroup())
	for _, pe := range parseMsg(t, sa.LastSentMsg).Payloads {
		if n, ok := pe.Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == wire.NotifyCookie {
			t.Fatal("the first IKE_SA_INIT carries a COOKIE the peer never issued")
		}
	}
	if sa.Cookie != nil {
		t.Error("a fresh initiator SA already holds a cookie")
	}
}

// VALIDATES: a cookie that does not match is IGNORED -- the responder answers with a new
// cookie, takes no half-open slot, creates no SA, and consumes the packet.
// PREVENTS: the "hardening" this row exists to forbid. Rejecting the message (a
// NO_PROPOSAL_CHOSEN, a silent drop, or killing the SA) is non-conformant, and it is the
// change a future reader is most likely to make.
// RFC requirement: RFC7296-2.6-5 positive -- tryResponderSAInit (register.go) treats a
// failed verifyCookie as a cookie-less initiation and challenges it afresh, BEFORE the
// compare-and-swap that would commit the peer's only half-open slot.
func TestCkeMismatchedCookieIsIgnoredNotRejected(t *testing.T) {
	resetCookieSecret(t)
	withCookieThreshold(t, 0)
	log := slogutil.DiscardLogger()
	ps, table := ckeRespondPeer(t)

	_, pkt := ckeInboundInit(t, bytes.Repeat([]byte{0x77}, cookieDataLen))
	var iSPI, rSPI [8]byte
	copy(iSPI[:], pkt.Data[0:8])

	before := cookieChallengeCount("ze")
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("the datagram was not consumed; a mismatched cookie must be processed, not passed on")
	}
	if got := cookieChallengeCount("ze"); got != before+1 {
		t.Errorf("no new cookie was issued: count went %d -> %d", before, got)
	}
	if ps.responderBusy.Load() {
		t.Error("the half-open slot was taken by an initiation that never proved its address")
	}
	if table.Len() != 0 {
		t.Errorf("the SA table holds %d entries, want 0; state was committed before the address was proven", table.Len())
	}
	if ps.getSA() != nil {
		t.Error("a responder SA was published for an unverified initiation")
	}
}

// VALIDATES: an initiation carrying a cookie this node really minted reaches the
// handshake -- it takes the half-open slot and creates the SA.
// PREVENTS: the positive above being vacuous. A tryResponderSAInit that refused
// EVERYTHING would satisfy it; only this test says the gate can be passed.
// RFC requirement: RFC7296-2.6-5 negative -- verifyCookie (cookie.go) accepts the value
// this node minted for this initiator, nonce and address, so the challenge is a check
// rather than a blanket refusal.
func TestCkeValidCookieReachesTheHandshake(t *testing.T) {
	resetCookieSecret(t)
	withCookieThreshold(t, 0)
	log := slogutil.DiscardLogger()
	ps, table := ckeRespondPeer(t)

	// First attempt: no cookie, so it is challenged.
	ini, pkt := ckeInboundInit(t, nil)
	var iSPI, rSPI [8]byte
	copy(iSPI[:], pkt.Data[0:8])
	if !tryResponderSAInit(pkt, iSPI, rSPI, table, nil, log) {
		t.Fatal("the first initiation was not consumed")
	}
	if table.Len() != 0 {
		t.Fatal("the unchallenged first attempt created an SA")
	}

	// Second attempt: the same initiation, now carrying a cookie this node minted for
	// it. Everything else is unchanged, which is what RFC 7296 Section 2.6 requires.
	cookie := mintCookie(ini.InitiatorSPI, ini.LocalNonce, net.ParseIP("10.0.0.1"), time.Now())
	if len(cookie) == 0 {
		t.Fatal("mintCookie produced nothing")
	}
	ini.Cookie = cookie
	req := buildSAInitRequest(ini, testIKEGroup())
	answered := transport.Packet{Data: req, RemoteAddr: pkt.RemoteAddr}

	if !tryResponderSAInit(answered, iSPI, rSPI, table, nil, log) {
		t.Fatal("an initiation carrying a valid cookie was not accepted")
	}
	if !ps.responderBusy.Load() {
		t.Error("the half-open slot was not taken by a verified initiation")
	}
	if table.Len() != 1 {
		t.Errorf("the SA table holds %d entries, want 1", table.Len())
	}
}

// VALIDATES: a responder that answers a cookie-carrying retry with a NEW cookie does not
// make this node fail. The new cookie replaces the old one and the exchange continues.
// PREVENTS: the failure RFC 7296 Section 2.6.1's MUST NOT names. A responder that folds
// KEi into its cookie calculation does exactly this, and an implementation that treated
// the second challenge as an error would never reach such a peer.
// RFC requirement: RFC7296-2.6.1-1 positive -- retrySAInit replaces sa.Cookie and leaves
// the SA in sa-init-sent, so a second, third or further challenge is absorbed rather than
// treated as a failure.
func TestCkeSecondCookieReplacesTheFirstWithoutFailing(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegIKEGroup())
	nonceBefore := append([]byte(nil), sa.LocalNonce...)
	dhBefore := append([]byte(nil), sa.LocalDH.PublicKey...)

	x := []byte{0x01, 0x02, 0x03}
	respX := ckeCookieResponse(sa.InitiatorSPI, [8]byte{}, x)
	handleSAInitResponse(sa, parseMsg(t, respX), respX, table, nil, nil, log)
	if sa.State != StateSAInitSent {
		t.Fatalf("state = %v after the first cookie, want sa-init-sent", sa.State)
	}
	if !bytes.Equal(sa.Cookie, x) {
		t.Fatalf("cookie = %x after the first challenge, want %x", sa.Cookie, x)
	}

	y := []byte{0x0a, 0x0b, 0x0c, 0x0d}
	respY := ckeCookieResponse(sa.InitiatorSPI, [8]byte{}, y)
	handleSAInitResponse(sa, parseMsg(t, respY), respY, table, nil, nil, log)

	if sa.State != StateSAInitSent {
		t.Fatalf("state = %v after the second cookie, want sa-init-sent; the SA failed on a second challenge", sa.State)
	}
	if !bytes.Equal(sa.Cookie, y) {
		t.Errorf("cookie = %x after the second challenge, want %x; the new cookie did not replace the old", sa.Cookie, y)
	}
	notify, ok := parseMsg(t, sa.LastSentMsg).Payloads[0].Payload.(*wire.PayloadNotify)
	if !ok || !bytes.Equal(notify.NotificationData, y) {
		t.Error("the second retry does not carry the second cookie as its first payload")
	}
	if !bytes.Equal(sa.LocalNonce, nonceBefore) || !bytes.Equal(sa.LocalDH.PublicKey, dhBefore) {
		t.Error("the nonce or the key changed across the two challenges")
	}
}

// VALIDATES: a COOKIE followed by an INVALID_KE_PAYLOAD produces a request carrying BOTH
// the retained cookie AND the corrected group, and the tolerance is bounded rather than
// unbounded.
// PREVENTS: reading the two MUSTs as contradictory. RFC 7296 Section 2.6 says "all other
// payloads unchanged" and Section 1.2 says the group must change; Section 2.6.1's shorter
// exchange settles it by showing a changed KEi' beside a retained N(COOKIE) and an
// unchanged Ni. It also prevents an unbounded COOKIE / INVALID_KE oscillation between two
// implementations.
// RFC requirement: RFC7296-2.6.1-1 negative -- the tolerance ends. maxSAInitRetries is one
// counter shared by both causes, so alternating them cannot pump it.
func TestCkeCookieAndInvalidKECombineWithoutFailing(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa, table := kegInitiator(t, kegIKEGroup())
	nonceBefore := append([]byte(nil), sa.LocalNonce...)

	cookie := []byte{0xc0, 0x0c}
	respCookie := ckeCookieResponse(sa.InitiatorSPI, [8]byte{}, cookie)
	handleSAInitResponse(sa, parseMsg(t, respCookie), respCookie, table, nil, nil, log)

	respKE := kegNotifyResponse(sa.InitiatorSPI, [8]byte{}, wire.NotifyInvalidKEPayload, []byte{0x00, 19})
	handleSAInitResponse(sa, parseMsg(t, respKE), respKE, table, nil, nil, log)

	if sa.State != StateSAInitSent {
		t.Fatalf("state = %v, want sa-init-sent; the combined exchange failed", sa.State)
	}
	third := parseMsg(t, sa.LastSentMsg)
	notify, ok := third.Payloads[0].Payload.(*wire.PayloadNotify)
	if !ok || notify.NotifyMsgType != wire.NotifyCookie || !bytes.Equal(notify.NotificationData, cookie) {
		t.Error("the third request dropped the retained cookie when the group was corrected")
	}
	ke := kegPayload[*wire.PayloadKE](t, sa.LastSentMsg)
	if ke == nil || ke.DHGroup != 19 {
		t.Error("the third request did not carry the corrected group")
	}
	if !bytes.Equal(sa.LocalNonce, nonceBefore) {
		t.Error("the nonce changed across the combined exchange")
	}

	// The tolerance is bounded. Two retries are spent; maxSAInitRetries allows one more,
	// and the one after that must end the exchange.
	respThird := ckeCookieResponse(sa.InitiatorSPI, [8]byte{}, cookie)
	handleSAInitResponse(sa, parseMsg(t, respThird), respThird, table, nil, nil, log)
	if sa.State != StateSAInitSent {
		t.Fatalf("retry %d of %d was refused early", sa.SAInitRetries, maxSAInitRetries)
	}
	respFourth := ckeCookieResponse(sa.InitiatorSPI, [8]byte{}, cookie)
	handleSAInitResponse(sa, parseMsg(t, respFourth), respFourth, table, nil, nil, log)
	if sa.State != StateDead {
		t.Errorf("state = %v after %d retries, want dead; the retry budget is unbounded",
			sa.State, sa.SAInitRetries)
	}
	if got := saInitRetryCount("ze", retryCookie); got == 0 {
		t.Error("no cookie retry was counted, so an operator cannot see a forged-notify flood")
	}
}
