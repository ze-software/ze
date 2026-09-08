// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's ticket key
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's ticket cache
// Related: rfc9190_resumption_test.go -- the acceptance half of these pairs
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3 resumption (Sections 2.1.2, 2.1.3, 5.7)
//
// Every case here is a REFUSAL: a ticket ze must not redeem, or a check a
// resumed session must not skip. A gate that refused everything would pass all
// of them, so each one names the acceptance test in
// rfc9190_resumption_test.go that is its other polarity.
//
// VALIDATES: a ticket is redeemable only under the key of the peering that
// issued it, only inside the RFC 9190 Section 5.7 lifetime, only while the
// certificate it carries still verifies against the current trust anchors, and
// only when the Section 5.4 revocation check can still be answered.
// PREVENTS: the fail-open shape resumption invites, where an exchange that
// presents no certificate is authorized on a decision nobody re-made.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"
	"time"
)

// resumptionPKIValidity outlives every clock these tests drive. The Section 5.7
// ceiling is seven days, so a certificate valid for an hour would expire under
// the moved clock and the test would be measuring expiry instead of the ticket.
const resumptionPKIValidity = 30 * 24 * time.Hour

// driveResumingExchangeAt is driveResumingExchange with the AUTHENTICATOR's
// clock moved.
//
// tls.Config.Time is what crypto/tls judges a ticket's age and the expiry of the
// certificate inside it against (serverHandshakeStateTLS13.checkForResumption,
// crypto/tls/handshake_server_tls13.go), and nothing in ze reaches it. The hook
// touches this exchange's own config copy alone.
func driveResumingExchangeAt(t *testing.T, pki *eapTLSPKI, server, peerStore *Resumption, at time.Time) (*eapTLSFlight, *PeerSession) {
	t.Helper()
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	fl := driveTunedEAPTLSFlight(t, pki.serverConfigResuming(server), peer, tls.VersionTLS13, eapTLS13Rounds,
		func(cfg *tls.Config) { cfg.Time = func() time.Time { return at } })
	return fl, peer
}

// TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode asserts the change is
// invisible to a peer that does not resume.
//
// It is the strongSwan shape in unit form. charon's TLS client sends no
// psk_key_exchange_modes extension at all (src/libtls/tls_peer.c
// send_client_hello writes server_name, supported_groups, ec_point_formats,
// supported_versions, cookie, signature_algorithms, signature_algorithms_cert
// and key_share, and nothing else), and Go's server issues a ticket only to a
// client that offered psk_dhe_ke (shouldSendSessionTickets,
// crypto/tls/handshake_server_tls13.go). A ze peer with no resumption store
// offers no mode either, so it stands for that client here.
//
// RFC requirement: RFC9190-2.1.2-1 negative -- the obligation is to enable
// resumption for a client that can use it. A client that offers no PSK mode gets
// exactly the conversation it got before tickets existed: ONE application_data
// record in the closing EAP-Request, the RFC 9190 Section 2.5 indication, and
// nothing beside it.
func TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	fl := driveEAPTLSFlight(t, pki.serverConfigResuming(NewResumption(time.Now, true)), peer, tls.VersionTLS13, eapTLS13Rounds)

	requireCompleted(t, fl, "an exchange with a non-resuming peer")
	if n := eapTLSAppDataRecords(fl.serverSent[fl.successAt-1]); n != 1 {
		t.Fatalf("the closing EAP-Request carries %d application_data records, want 1: "+
			"a peer that offered no psk_dhe_ke was sent a NewSessionTicket it can never redeem", n)
	}
}

// TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey asserts the ticket
// key belongs to ONE peering.
//
// It is the test behind the design's security argument. Every ze EAP-TLS
// connection collapses onto one client-cache key, so the isolation between
// peerings has to come from the stores themselves, and a process-wide ticket key
// would let a ticket minted for one peer be redeemed on another.
//
// RFC requirement: RFC9190-5.7-1 negative -- authorization during resumption is
// based on cached data from the initial full handshake, and a ticket the
// authenticator cannot decrypt carries no such data. It runs the full handshake
// rather than authorizing on nothing.
func TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, NewResumption(time.Now, true), peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	// A different peering: its own store, and therefore its own ticket key.
	second, secondPeer := driveResumingExchange(t, pki, NewResumption(time.Now, true), peerStore)
	requireCompleted(t, second, "the authentication against another peering")
	if secondPeer.Resumed() || second.sess.Resumed() {
		t.Fatal("a ticket minted under one peering's key was redeemed under another's, " +
			"so the ticket key is shared and one peer's cached certificate chain can authorize another")
	}
}

// TestEAPTLS13RefusesATicketPastTheSection57Lifetime asserts the authenticator
// stops honoring a ticket at the ceiling.
//
// The peer's own cache enforces the same bound on STORING
// (TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling); this is the
// authenticator refusing to redeem one, which is what protects ze from a peer
// that keeps a ticket longer than it should.
//
// RFC requirement: RFC9190-5.7-5 negative -- the ceiling is enforced on the
// redeeming side too: at 604800 seconds and one second the ticket is refused,
// and the exchange falls back to a full handshake instead of failing.
func TestEAPTLS13RefusesATicketPastTheSection57Lifetime(t *testing.T) {
	pki := newEAPTLSPKIValidFor(t, resumptionPKIValidity)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	stale, stalePeer := driveResumingExchangeAt(t, pki, server, peerStore,
		time.Now().Add(resumptionTicketMaxAge+time.Second))
	requireCompleted(t, stale, "the authentication with a stale ticket")
	if stalePeer.Resumed() || stale.sess.Resumed() {
		t.Fatalf("the authenticator redeemed a ticket %s old, past the RFC 9190 Section 5.7 ceiling of %s",
			resumptionTicketMaxAge+time.Second, resumptionTicketMaxAge)
	}
}

// TestEAPTLS13RefusesAResumptionWhoseCachedCertificateExpired asserts the
// certificate inside a ticket is judged against the CURRENT time.
//
// RFC requirement: RFC9190-5.7-6 positive -- the information behind the initial
// authorization changed, because the client certificate expired, so the decision
// is reevaluated: the exchange runs a full handshake, where the peer presents a
// renewed certificate and authenticates on that instead.
func TestEAPTLS13RefusesAResumptionWhoseCachedCertificateExpired(t *testing.T) {
	pki := newEAPTLSPKIValidFor(t, resumptionPKIValidity)
	// A short-lived client certificate, so the clock can pass its expiry while
	// its issuing CA is still valid.
	shortLived, shortLivedKey := newLeafValidFor(t, pki.trustedCA, pki.trustedCAKey,
		"eap-tls-client", eapTLSClientSerial, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, 10*time.Minute)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	expiring := pki.resumptionPeerConfig(peerStore)
	expiring.CertPEM, expiring.KeyPEM = shortLived, shortLivedKey
	first := driveEAPTLSFlight(t, pki.serverConfigResuming(server),
		NewPeerSessionTLS("eap-tls-client", expiring), tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	// The clock passes the cached certificate's expiry. The peer now holds the
	// renewed certificate newEAPTLSPKIValidFor issued, which is what makes the
	// full handshake succeed while the resumption is refused.
	renewed, renewedPeer := driveResumingExchangeAt(t, pki, server, peerStore, time.Now().Add(20*time.Minute))
	requireCompleted(t, renewed, "the authentication after the cached certificate expired")
	if renewedPeer.Resumed() || renewed.sess.Resumed() {
		t.Fatal("the authenticator resumed a session whose cached client certificate had expired")
	}
}

// TestEAPTLS13RefusesAResumptionAgainstAReplacedTrustAnchor asserts the cached
// chain is re-verified against the trust anchor the authenticator holds NOW.
//
// RFC requirement: RFC9190-5.7-2 negative for the trust-anchor policy -- the
// security policy in force at resumption is the current one. With the CA
// replaced, the cached chain no longer verifies, so the ticket is refused and no
// exchange completes: the peer's certificate does not chain to the new anchor
// either.
func TestEAPTLS13RefusesAResumptionAgainstAReplacedTrustAnchor(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	replacedCA, replacedKey, replacedPEM := newCA(t, "eap-tls-replacement-ca", 200)
	replaced := pki.serverConfigResuming(server)
	replaced.CACertPEM = replacedPEM
	replaced.CRLPEM = newCRL(t, replacedCA, replacedKey)

	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	second := driveEAPTLSFlight(t, replaced, peer, tls.VersionTLS13, eapTLS13Rounds)
	if second.successAt >= 0 {
		t.Fatal("the authenticator sent EAP-Success after its trust anchor was replaced, " +
			"so the cached chain was authorized against a CA that no longer configures it")
	}
	if second.sess.Resumed() {
		t.Fatal("the authenticator resumed a session whose cached chain does not verify against the current ClientCAs")
	}
}

// TestEAPTLS13ResumptionStillNeedsARevocationSource asserts the Section 5.4
// refusal survives resumption on the authenticator.
//
// RFC requirement: RFC9190-5.4-1 negative for the resumed path -- Section 5.4's
// "the revocation status of all the certificates in the certificate chains MUST
// be checked" has no resumption exemption, so a TLS 1.3 authenticator with no
// list configured refuses a resumed session exactly as it refuses a full one
// (errNoRevocationSource, revocation.go).
func TestEAPTLS13ResumptionStillNeedsARevocationSource(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	sourceless := pki.serverConfigResuming(server)
	sourceless.CRLPEM = nil
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	second := driveEAPTLSFlight(t, sourceless, peer, tls.VersionTLS13, eapTLS13Rounds)

	if second.successAt >= 0 {
		t.Fatal("the authenticator completed a resumed TLS 1.3 session with no revocation source configured")
	}
	if err := second.sess.Err(); !errors.Is(err, errNoRevocationSource) {
		t.Fatalf("the refusal is %v, want %v", err, errNoRevocationSource)
	}
}

// TestEAPTLS13PeerRefusesAResumptionItCannotRebuildAChainFor asserts the peer's
// rebuild is a real check.
//
// Go hands a resumed ze peer no server Certificate message and restores an EMPTY
// verifiedChains, because this peer sets InsecureSkipVerify
// (Conn.verifyServerCertificate assigns c.verifiedChains only in the branch that
// flag skips). serverChainCheck.rebuildResumedChains is therefore the ONLY thing
// that re-verifies the cached chain on this role, and a peer whose trust anchor
// no longer covers that chain must refuse.
//
// RFC requirement: RFC9190-5.7-2 positive for the peer role -- the peer's
// certificate-path policy is applied on resumption as it is on a full handshake.
func TestEAPTLS13PeerRefusesAResumptionItCannotRebuildAChainFor(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	cachedTicket(t, peerStore, "the initial authentication")

	replacedCA, replacedKey, replacedPEM := newCA(t, "eap-tls-peer-replacement-ca", 201)
	replacedPeer := pki.resumptionPeerConfig(peerStore)
	replacedPeer.CACertPEM = replacedPEM
	replacedPeer.CRLPEM = newCRL(t, replacedCA, replacedKey)

	peer := NewPeerSessionTLS("eap-tls-client", replacedPeer)
	second := driveEAPTLSFlight(t, pki.serverConfigResuming(server), peer, tls.VersionTLS13, eapTLS13Rounds)
	if second.peerDone {
		t.Fatal("the peer completed a resumed session whose cached authenticator chain does not " +
			"verify against its current trust anchor, so the rebuild is not checking anything")
	}
}

// TestVerifyConnectionRefusesAnEmptyChainSetOnAFullHandshake keeps the rebuild
// from loosening the gate it was added to.
//
// The rebuild is reached only on a resumed session. A NON-resumed handshake that
// somehow reaches verifyConnection with no chain built is a failure to check,
// and it must error exactly as it did before resumption existed
// (ai/rules/principles.md).
//
// It drives serverChainCheck directly, because crypto/tls aborts a full
// handshake at verifyPeerCertificate and never calls verifyConnection with an
// empty chain set, so no exchange can reach this state from the wire.
func TestVerifyConnectionRefusesAnEmptyChainSetOnAFullHandshake(t *testing.T) {
	pki := newEAPTLSPKI(t)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pki.trustedCAPEM) {
		t.Fatal("parse the trusted CA")
	}
	crls, err := parseCRLs(pki.trustedCRLPEM)
	if err != nil {
		t.Fatalf("parseCRLs: %v", err)
	}
	check := &serverChainCheck{roots: roots, crls: crls}

	if err := check.verifyConnection(tls.ConnectionState{Version: tls.VersionTLS13}); err == nil {
		t.Fatal("a full handshake with no verified chain was accepted, so the resumption rebuild " +
			"widened the gate rather than filling the one state that needs it")
	}

	// And a RESUMED session that carries no certificate to rebuild from is a
	// failure to check too. An empty result is never a clean answer.
	if err := check.verifyConnection(tls.ConnectionState{Version: tls.VersionTLS13, DidResume: true}); err == nil {
		t.Fatal("a resumed session carrying no peer certificate was accepted")
	}
}

// TestResumptionCacheIsPerPeering asserts two stores share nothing.
//
// Conn.clientSessionCacheKey (crypto/tls/handshake_client.go) falls back to the
// transport's remote address when there is no ServerName, and
// eapTLSTransport.RemoteAddr answers eapAddr{}, whose String is the constant
// "eap". Every EAP-TLS connection in the process therefore files its ticket
// under that one key, so ONE shared cache would offer peer A's ticket, with peer
// A's cached certificate chain, on peer B's session.
func TestResumptionCacheIsPerPeering(t *testing.T) {
	pki := newEAPTLSPKI(t)
	stocked := NewResumption(time.Now, true)
	fl, _ := driveResumingExchange(t, pki, NewResumption(time.Now, true), stocked)
	requireCompleted(t, fl, "the initial authentication")
	state := cachedTicket(t, stocked, "the initial authentication")

	const key = "eap"
	a := NewResumption(time.Now, true)
	b := NewResumption(time.Now, true)
	a.Put(key, state)

	if _, ok := a.Get(key); !ok {
		t.Fatal("the store that was given the ticket does not answer it")
	}
	if _, ok := b.Get(key); ok {
		t.Fatalf("a second peering answered a ticket stored on another under the shared cache key %q", key)
	}
}

// TestResumptionCacheDropsAnExpiredEntryOnPut asserts the RFC 9190 Section 5.7
// ceiling is applied on the STORE path as well as the read path.
//
// Get alone would leave an expired ticket sitting in memory for as long as
// nothing asked for it, and the sentence is about storing.
func TestResumptionCacheDropsAnExpiredEntryOnPut(t *testing.T) {
	pki := newEAPTLSPKI(t)
	stocked := NewResumption(time.Now, true)
	fl, _ := driveResumingExchange(t, pki, NewResumption(time.Now, true), stocked)
	requireCompleted(t, fl, "the initial authentication")
	state := cachedTicket(t, stocked, "the initial authentication")

	stored := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	now := stored
	cache := NewResumption(func() time.Time { return now }, true)
	cache.Put("first", state)

	now = stored.Add(resumptionTicketMaxAge)
	cache.Put("second", state)

	cache.mu.Lock()
	_, keptFirst := cache.cached["first"]
	_, keptSecond := cache.cached["second"]
	cache.mu.Unlock()

	if keptFirst {
		t.Fatalf("a ticket stored %s ago survived a later Put, which RFC 9190 Section 5.7 forbids", resumptionTicketMaxAge)
	}
	if !keptSecond {
		t.Fatal("the Put that swept the expired entry dropped the entry it was storing")
	}
}

// TestResumptionCacheIsBounded keeps one peering's cache from growing without a
// limit.
//
// A ze peer files every session under one key, so the live count is one. The cap
// exists for the case where that key ever varies, and an unbounded map fed by
// the far end is the shape ai/rules/evidence.md refuses.
func TestResumptionCacheIsBounded(t *testing.T) {
	pki := newEAPTLSPKI(t)
	stocked := NewResumption(time.Now, true)
	fl, _ := driveResumingExchange(t, pki, NewResumption(time.Now, true), stocked)
	requireCompleted(t, fl, "the initial authentication")
	state := cachedTicket(t, stocked, "the initial authentication")

	now := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	cache := NewResumption(func() time.Time { return now }, true)
	for i := range resumptionMaxCachedSessions * 3 {
		now = now.Add(time.Second)
		cache.Put(string(rune('a'+i)), state)
	}

	cache.mu.Lock()
	held := len(cache.cached)
	cache.mu.Unlock()
	if held > resumptionMaxCachedSessions {
		t.Fatalf("the cache holds %d sessions, over its %d ceiling", held, resumptionMaxCachedSessions)
	}
}

// TestResumptionOffOffersNoTicketOnThePeer asserts the session-resumption leaf
// stops the peer offering, not only the authenticator accepting.
//
// RFC 9190 Section 2.1.3: "It is up to the EAP-TLS peer to use resumption." A
// peer with the leaf off must send no PSK identity, and Conn.loadSession returns
// before it writes psk_key_exchange_modes when there is no cache, so the
// authenticator issues nothing either.
func TestResumptionOffOffersNoTicketOnThePeer(t *testing.T) {
	pki := newEAPTLSPKI(t)
	off := NewResumption(time.Now, false)
	if off.ClientCache() != nil {
		t.Fatal("a peering with session-resumption off still offers a client session cache")
	}

	server := NewResumption(time.Now, true)
	first, _ := driveResumingExchange(t, pki, server, off)
	requireCompleted(t, first, "the initial authentication")

	off.mu.Lock()
	cached := len(off.cached)
	off.mu.Unlock()
	if cached != 0 {
		t.Fatalf("a peer with session-resumption off stored %d tickets", cached)
	}
	if n := eapTLSAppDataRecords(first.serverSent[first.successAt-1]); n != 1 {
		t.Fatalf("the closing EAP-Request carries %d application_data records, want 1: "+
			"a peer offering no psk_dhe_ke was still sent a NewSessionTicket", n)
	}

	second, secondPeer := driveResumingExchange(t, pki, server, off)
	requireCompleted(t, second, "the second authentication")
	if secondPeer.Resumed() || second.sess.Resumed() {
		t.Fatal("an exchange resumed although the peer stores no ticket")
	}
}
