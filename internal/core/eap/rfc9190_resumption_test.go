// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's ticket key
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's ticket cache
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3 resumption (Sections 2.1.2, 2.1.3, 5.7)
//
// RFC 9190 Section 2.1.2 puts an unconditional obligation on the EAP-TLS server:
//
//	"To enable resumption when using EAP-TLS with TLS 1.3, the EAP-TLS server
//	MUST send one or more post-handshake NewSessionTicket messages (each
//	associated with a PSK, a PSK identity, a ticket lifetime, and other
//	parameters) in the initial authentication."
//
// Section 2.1.3 leaves USING it to each end -- "It is up to the EAP-TLS peer to
// use resumption", and "The EAP-TLS server MAY choose to require a full
// handshake instead of accepting resumption" -- which is the session-resumption
// leaf. Section 5.7 governs what a resumed session may be authorized on, and
// caps how long a peer stores what a resumption needs.
//
// VALIDATES: a ticket issued in one exchange is redeemed in the next, on both
// roles; the revocation gate of Section 5.4 still runs on a resumed session on
// both roles; a peer drops a stored ticket at the Section 5.7 ceiling; the
// session-resumption leaf off yields a full handshake with the ticket still
// issued; and a TLS 1.2 exchange is unchanged.
// PREVENTS: resumption skipping the certificate checks a full handshake runs
// (Go calls neither VerifyPeerCertificate nor its own cached-chain sweep on a
// resumed ze peer), a ticket key that only one EAP session can read, and a
// stored ticket outliving 604800 seconds.

package eap

import (
	"crypto/tls"
	"testing"
	"time"
)

// resumptionPeerConfig builds the peer half of a resuming exchange over a named
// store, so a second exchange can offer the ticket the first one was issued.
func (p *eapTLSPKI) resumptionPeerConfig(r *Resumption) *PeerTLSConfig {
	return &PeerTLSConfig{
		CertPEM:    p.clientCertPEM,
		KeyPEM:     p.clientKeyPEM,
		CACertPEM:  p.trustedCAPEM,
		CRLPEM:     p.trustedCRLPEM,
		Resumption: r,
	}
}

// driveResumingExchange runs one complete EAP-TLS 1.3 conversation over the two
// stores and returns the flight, the authenticator session and the peer, so a
// caller can ask each end whether it resumed.
//
// The peer is fresh on every call, as it is in production: a PeerSession serves
// one EAP conversation. The STORES are what the caller keeps across calls, which
// is the whole subject of these tests.
func driveResumingExchange(t *testing.T, pki *eapTLSPKI, server, peerStore *Resumption) (*eapTLSFlight, *PeerSession) {
	t.Helper()
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	fl := driveEAPTLSFlight(t, pki.serverConfigResuming(server), peer, tls.VersionTLS13, eapTLS13Rounds)
	return fl, peer
}

// requireCompleted fails the test unless both ends of one exchange authenticated
// and agreed on the MSK.
func requireCompleted(t *testing.T, fl *eapTLSFlight, what string) {
	t.Helper()
	if fl.peerErr != nil {
		t.Fatalf("%s: the peer failed an exchange both ends should accept: %v", what, fl.peerErr)
	}
	if !fl.peerDone || fl.successAt < 0 {
		t.Fatalf("%s: the exchange did not conclude: peerDone=%v successAt=%d", what, fl.peerDone, fl.successAt)
	}
	var zero [64]byte
	if fl.peerMSK == zero {
		t.Fatalf("%s: the peer MSK is all zero", what)
	}
	if fl.peerMSK != fl.serverMSK {
		t.Fatalf("%s: MSK mismatch:\n peer=  %x\n server=%x", what, fl.peerMSK, fl.serverMSK)
	}
}

// cachedTicket answers the one ticket a peer store holds, and fails the test
// when it holds any other number.
//
// The count is exactly one because ze's peer sets no ServerName and every
// session it stores therefore lands under one cache key, and because Go sends
// one ticket per connection ("ticket_nonce ... is always left at zero because we
// only ever send one ticket per connection", Conn.sendSessionTicket).
func cachedTicket(t *testing.T, r *Resumption, what string) *tls.ClientSessionState {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.cached) != 1 {
		t.Fatalf("%s: the peer cached %d tickets, want 1", what, len(r.cached))
	}
	for _, entry := range r.cached {
		return entry.state
	}
	return nil
}

// TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems drives two EAP-TLS
// conversations over one pair of resumption stores and asserts the second one
// resumed the first.
//
// RFC requirement: RFC9190-2.1.2-1 positive -- the authenticator sends a
// post-handshake NewSessionTicket in the initial authentication: the peer's TLS
// engine parses one and hands it to the cache, and the next exchange redeems it,
// which no ticket-less conversation can do.
//
// RFC requirement: RFC9190-2.1.2-2 positive -- the ticket ze issues declares a
// lifetime within the 604800 second maximum. Go's client REFUSES a longer one
// with an illegal_parameter alert (clientHandshakeStateTLS13.
// handleNewSessionTicket, crypto/tls/handshake_client_tls13.go, rejects a
// lifetime over maxSessionTicketLifetime), so a ticket this client accepted and
// stored is one whose declared lifetime is inside the bound.
//
// RFC requirement: RFC9190-2.1.2-3 positive -- the NewSessionTicket carries no
// early_data extension: the session state the client parsed out of it reports
// EarlyData false, which is the field that extension sets.
//
// RFC requirement: RFC9190-2.1.3-1 positive -- the resumption mechanism is the
// TLS 1.3 one: both ends negotiated TLS 1.3 and both report DidResume on the
// second exchange, which on TLS 1.3 is the PSK mechanism and nothing else.
//
// RFC requirement: RFC9190-2.1.3-2 positive -- psk_dh_ke is the key exchange
// mode used for resumption. Ze's peer offers that mode alone (Conn.loadSession
// sets hello.pskModes to exactly []uint8{pskModeDHE}), and ze's authenticator
// issues a ticket only to a client that offered it
// (serverHandshakeStateTLS13.shouldSendSessionTickets requires pskModeDHE), so a
// redeemed ticket proves the mode was offered and accepted.
func TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, firstPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	if firstPeer.Resumed() {
		t.Fatal("the initial authentication resumed a session that did not exist")
	}
	if first.sess.Resumed() {
		t.Fatal("the authenticator reports the initial authentication as resumed")
	}

	// RFC9190-2.1.2-1: the ticket arrived, and the peer's TLS engine parsed it.
	state := cachedTicket(t, peerStore, "the initial authentication")
	ticket, session, err := state.ResumptionState()
	if err != nil {
		t.Fatalf("read the stored resumption state: %v", err)
	}
	if len(ticket) == 0 {
		t.Fatal("the stored session carries no ticket, so no NewSessionTicket was issued")
	}
	// RFC9190-2.1.2-3: no early_data extension rode on it.
	if session.EarlyData {
		t.Fatal("the NewSessionTicket carried an early_data extension, which RFC 9190 Section 2.1.2 forbids")
	}

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() {
		t.Fatal("the peer ran a full handshake with a valid ticket in hand")
	}
	if !second.sess.Resumed() {
		t.Fatal("the authenticator ran a full handshake with a ticket it had issued itself")
	}
	if second.peerMSK == first.peerMSK {
		t.Fatal("the resumed session derived the same MSK as the initial one, so the exporter reused a secret")
	}
}

// TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain drives one exchange to
// stock a ticket, then resumes with the authenticator's certificate revoked, and
// asserts the peer refuses.
//
// It is the fail-open regression test for the peer role. Go calls neither
// VerifyPeerCertificate nor its own cached-chain sweep on a resumed ze peer
// (readServerCertificate returns early under hs.usingPSK, and Conn.loadSession
// skips anyValidVerifiedChain under InsecureSkipVerify), so
// serverChainCheck.rebuildResumedChains is the only thing that re-verifies the
// cached chain and hands it to the revocation gate.
//
// RFC requirement: RFC9190-5.7-2 positive -- a security policy for
// authorization, here the RFC 9190 Section 5.4 revocation check, is followed on
// resumption as it is on a full handshake: with the authenticator's certificate
// on the peer's revocation list, the resumed exchange is refused.
//
// RFC requirement: RFC9190-5.7-6 positive -- a decision made during the initial
// full handshake is REEVALUATED when the information behind it changed: the
// chain was unrevoked when the ticket was issued and is revoked when it is
// redeemed, and the second exchange fails where the first succeeded.
func TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	// The same store, with a revocation list that now names the authenticator.
	// A production reload would rebuild the store as well, because the peering's
	// authentication config changed (resumptionFor, internal/component/ike/
	// engine/resumption.go). Keeping it is what puts the peer in the state this
	// test is about: a ticket in hand for a certificate it must now refuse.
	peerCfg := pki.resumptionPeerConfig(peerStore)
	peerCfg.CRLPEM = pki.crlRevoking(t, eapTLSServerSerial)
	peer := NewPeerSessionTLS("eap-tls-client", peerCfg)
	second := driveEAPTLSFlight(t, pki.serverConfigResuming(server), peer, tls.VersionTLS13, eapTLS13Rounds)

	if second.peerErr == nil && second.peerDone {
		t.Fatalf("the peer accepted a resumed session whose cached authenticator certificate is revoked (resumed=%v)", peer.Resumed())
	}
}

// TestEAPTLS13ResumptionRefusesARevokedClientChain is the authenticator's half
// of the test above: the client certificate the ticket carries is revoked before
// the ticket is redeemed.
//
// The gate here is crypto/tls plus the existing Section 5.4 check rather than
// new ze code: serverHandshakeStateTLS13.checkForResumption restores the cached
// chain and readClientCertificate calls VerifyConnection over it on a resumed
// session, so newTLSMethod's revocation callback runs with the cached chain in
// ConnectionState.VerifiedChains. The test exists because that reachability is
// the thing a change to either side would silently remove.
//
// RFC requirement: RFC9190-5.7-1 positive -- authorization during resumption is
// based on cached data from the initial full handshake: the authenticator has no
// certificate on the wire to judge, and it refuses on the chain the ticket
// carried.
//
// RFC requirement: RFC9190-5.7-3 positive -- enough data was cached during the
// initial full handshake to make the authorization decision at resumption: the
// revocation gate names a certificate that this exchange never saw presented.
func TestEAPTLS13ResumptionRefusesARevokedClientChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	revoking := pki.serverConfigResuming(server)
	revoking.CRLPEM = pki.crlRevoking(t, eapTLSClientSerial)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	second := driveEAPTLSFlight(t, revoking, peer, tls.VersionTLS13, eapTLS13Rounds)

	if second.successAt >= 0 {
		t.Fatal("the authenticator sent EAP-Success for a resumed session whose cached client certificate is revoked")
	}
	if second.peerDone {
		t.Fatal("the peer completed a resumed session the authenticator should have refused")
	}
}

// TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain is the negative polarity
// of the two tests above: a gate that refused every resumed session would pass
// both of them.
//
// RFC requirement: RFC9190-5.7-2 negative -- the security policy followed on
// resumption ACCEPTS a chain nothing has revoked. Both ends resume, and both
// derive the MSK.
//
// RFC requirement: RFC9190-5.7-6 negative -- nothing behind the initial
// decision changed, so no reevaluation refuses the resumption.
func TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() || !second.sess.Resumed() {
		t.Fatalf("the exchange did not resume: peer=%v authenticator=%v", secondPeer.Resumed(), second.sess.Resumed())
	}
}

// TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling drives the cache's clock
// and asserts a ticket is gone once it has been held for 604800 seconds.
//
// RFC requirement: RFC9190-5.7-5 positive -- a stored ticket is not kept for
// longer than 604800 seconds: at exactly that age the entry is neither returned
// nor still held, and rewinding the clock does not bring it back, which is what
// separates DELETING it from declining to use it.
//
// RFC requirement: RFC9190-5.7-5 negative -- the ceiling is not a blanket
// refusal: one second under it the same ticket is still answered, so a cache
// that dropped everything would fail this half.
func TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling(t *testing.T) {
	pki := newEAPTLSPKI(t)
	stocked := NewResumption(time.Now, true)
	fl, _ := driveResumingExchange(t, pki, NewResumption(time.Now, true), stocked)
	requireCompleted(t, fl, "the initial authentication")
	state := cachedTicket(t, stocked, "the initial authentication")

	stored := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	now := stored
	cache := NewResumption(func() time.Time { return now }, true)
	cache.Put("eap", state)

	now = stored.Add(resumptionTicketMaxAge - time.Second)
	if _, ok := cache.Get("eap"); !ok {
		t.Fatalf("the peer dropped a ticket %s old, under the RFC 9190 Section 5.7 ceiling of %s",
			resumptionTicketMaxAge-time.Second, resumptionTicketMaxAge)
	}

	now = stored.Add(resumptionTicketMaxAge)
	if _, ok := cache.Get("eap"); ok {
		t.Fatalf("the peer answered a ticket it had stored for %s, which RFC 9190 Section 5.7 forbids", resumptionTicketMaxAge)
	}

	// STORED, not merely unused: the entry is gone, so rewinding the clock
	// cannot produce it again.
	now = stored
	if _, ok := cache.Get("eap"); ok {
		t.Fatal("the peer still HOLDS a ticket past the ceiling and only declined to answer it")
	}
}

// TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket asserts the
// session-resumption leaf governs accepting alone.
//
// RFC requirement: RFC9190-2.1.2-1 negative -- issuance is NOT conditional on
// the server accepting resumption. With the authenticator refusing every ticket
// it is shown, the exchange still issues a fresh one, which is what makes the
// Section 2.1.2 MUST unconditional in ze as it is in the RFC.
//
// RFC requirement: RFC9190-2.1.3-1 negative -- an authenticator that declines
// resumption runs the full handshake rather than failing the exchange: both ends
// authenticate, and neither reports DidResume.
func TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket(t *testing.T) {
	pki := newEAPTLSPKI(t)
	// The authenticator refuses resumption; the peer still offers its ticket,
	// which is what makes the refusal observable.
	server := NewResumption(time.Now, false)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	firstTicket := cachedTicket(t, peerStore, "the initial authentication")

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the second authentication")
	if secondPeer.Resumed() || second.sess.Resumed() {
		t.Fatalf("the exchange resumed with session-resumption off: peer=%v authenticator=%v",
			secondPeer.Resumed(), second.sess.Resumed())
	}

	// RFC9190-2.1.2-1: a ticket went out anyway, and it is a NEW one.
	secondTicket := cachedTicket(t, peerStore, "the second authentication")
	if secondTicket == firstTicket {
		t.Fatal("the second authentication issued no NewSessionTicket, so the stored one was never replaced")
	}
}

// TestEAPTLS13ResumedExchangeStillSendsTheSuccessIndication asserts RFC 9190
// Figure 3 as well as Figure 2: the resumed conversation ends with the same
// protected success result indication the full one does.
//
// Section 2.5 states its procedure over "an EAP-TLS server [that] has
// successfully processed the TLS client Finished and sent its last handshake
// message", with no exemption for a resumed handshake, and Figure 3 draws the
// indication in the resumption exchange.
//
// RFC requirement: RFC9190-2.5-1 positive on a resumed session -- the
// EAP-Request before EAP-Success carries application data the peer decrypts to
// the single octet 0x00, and EAP-Success is the last packet the authenticator
// sends.
func TestEAPTLS13ResumedExchangeStillSendsTheSuccessIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() || !second.sess.Resumed() {
		t.Fatalf("the exchange did not resume: peer=%v authenticator=%v", secondPeer.Resumed(), second.sess.Resumed())
	}

	if second.successAt != len(second.serverSent)-1 {
		t.Fatalf("the authenticator sent %d more packet(s) after EAP-Success on a resumed exchange",
			len(second.serverSent)-1-second.successAt)
	}
	if n := eapTLSAppDataRecords(second.serverSent[second.successAt-1]); n < 1 {
		t.Fatalf("the closing EAP-Request of a resumed exchange carries %d application_data records, "+
			"so no RFC 9190 Section 2.5 indication went out", n)
	}
	if plain := readPeerPlaintext(t, secondPeer); len(plain) != 1 || plain[0] != 0x00 {
		t.Fatalf("the resumed exchange's indication decrypts to %x, want the single octet 00", plain)
	}
}

// TestEAPTLS13ResumedSessionDerivesTheRFC9190MSK asserts the key the lower layer
// gets from a resumed session is the RFC 9190 Section 2.3 export, agreed by both
// ends.
//
// A resumed TLS 1.3 handshake assigns Conn.ekm exactly as a full one does, so
// exportEAPTLSMSK needed no change. This is what proves that: the two MSKs are
// the same 64 octets, they are not zero, and they are not the previous
// exchange's, which they would be if either end had cached the key instead of
// re-exporting it.
func TestEAPTLS13ResumedSessionDerivesTheRFC9190MSK(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() {
		t.Fatal("the exchange did not resume, so this says nothing about a resumed MSK")
	}
	if len(second.peerMSK) != 64 {
		t.Fatalf("the resumed MSK is %d octets, want the 64 RFC 9190 Section 2.3 defines", len(second.peerMSK))
	}
	if second.peerMSK != second.serverMSK {
		t.Fatalf("the resumed session derived different keys:\n peer=  %x\n server=%x", second.peerMSK, second.serverMSK)
	}
	if second.peerMSK == first.peerMSK {
		t.Fatal("the resumed session reused the initial session's MSK, so a key was cached rather than exported")
	}
}

// TestEAPTLS12ExchangeIsUnchangedByResumptionState drives a TLS 1.2 conversation
// with a resumption store on both ends and asserts it authenticates exactly as
// it did before resumption existed.
//
// RFC 9190 Section 2.1.2 opens "when using EAP-TLS with TLS 1.3", so it puts no
// obligation on this path, and RFC 5216 governs it. The test exists because the
// ticket keys and the client cache are installed on every session whatever the
// version, and a TLS 1.2 regression would be invisible in the tests above.
//
// It asserts no application_data record crosses at all, which is the Section 2.5
// scope rule (TestEAPTLS12SendsNoProtectedSuccessIndication pins the same thing
// without a store) and is also what tells a TLS 1.2 NewSessionTicket apart from
// a TLS 1.3 one: on TLS 1.2 the ticket is a plaintext handshake record, so it
// never reads as application data and cannot be confused for the indication.
func TestEAPTLS12ExchangeIsUnchangedByResumptionState(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfigResuming(NewResumption(time.Now, true)), peer, tls.VersionTLS12, eapTLS13Rounds)

	requireCompleted(t, fl, "the TLS 1.2 exchange")
	if peer.Resumed() {
		t.Fatal("the first TLS 1.2 exchange reports itself resumed")
	}
	for i, sent := range fl.serverSent {
		if n := eapTLSAppDataRecords(sent); n > 0 {
			t.Fatalf("TLS 1.2 packet %d carries %d application_data record(s): "+
				"RFC 9190 Section 2.5 only applies to TLS 1.3", i, n)
		}
	}
	if peer.indication.Load() != nil {
		t.Fatal("the TLS 1.2 peer decrypted application data, so the authenticator sent an indication")
	}
}

// TestEAPTLSResumptionRotatesItsTicketKey asserts the session-ticket key does
// not stand for the life of a peering.
//
// Setting an explicit key with Config.SetSessionTicketKeys turns Go's own
// rotation off, so this schedule is what keeps the property ze displaces rather
// than an extra mechanism: a key compromised tomorrow must not decrypt the
// tickets of every session this peering ever had. The retired key is kept for
// decryption, which is what lets a ticket minted just before a rotation still be
// redeemed for its whole life.
func TestEAPTLSResumptionRotatesItsTicketKey(t *testing.T) {
	start := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	now := start
	r := NewResumption(func() time.Time { return now }, true)

	first, err := r.TicketKeys()
	if err != nil {
		t.Fatalf("TicketKeys: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("a fresh peering holds %d ticket keys, want 1", len(first))
	}

	now = start.Add(resumptionKeyRotation - time.Second)
	steady, err := r.TicketKeys()
	if err != nil {
		t.Fatalf("TicketKeys: %v", err)
	}
	if len(steady) != 1 || steady[0] != first[0] {
		t.Fatal("the ticket key changed before its rotation was due, so a ticket issued a moment ago is already harder to redeem")
	}

	now = start.Add(resumptionKeyRotation)
	rotated, err := r.TicketKeys()
	if err != nil {
		t.Fatalf("TicketKeys: %v", err)
	}
	if len(rotated) != 2 {
		t.Fatalf("a rotated peering holds %d ticket keys, want 2: the new one and the one its live tickets were minted under", len(rotated))
	}
	if rotated[0] == first[0] {
		t.Fatal("the encryption key did not change at its rotation")
	}
	if rotated[1] != first[0] {
		t.Fatal("the retired key was dropped, so every ticket already issued became unredeemable")
	}

	now = start.Add(resumptionKeyMaxAge)
	aged, err := r.TicketKeys()
	if err != nil {
		t.Fatalf("TicketKeys: %v", err)
	}
	for _, k := range aged {
		if k == first[0] {
			t.Fatalf("a ticket key was kept for %s, past the %s at which every ticket it protects is refused anyway",
				resumptionKeyMaxAge, resumptionKeyMaxAge)
		}
	}
}

// driveResumingExchangeAt is driveResumingExchange with the AUTHENTICATORs
