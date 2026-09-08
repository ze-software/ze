// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS peer
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3, protected success result indication (Section 2.5)
//
// The PEER half of Section 2.5. rfc9190_test.go pins the AUTHENTICATOR half,
// which SENDS the indication; this file pins the peer that REQUIRES it.
//
// NO TEST HERE CARRIES AN `RFC requirement:` TAG, and that is deliberate. RFC
// 9190 Section 2.5 addresses its procedure to the EAP-TLS server, and
// rfc/short/rfc9190.md declares three requirement ids for the section:
// RFC9190-2.5-1 and RFC9190-2.5-2, both on what the server sends, and
// RFC9190-2.5-3, a SHOULD about TLS error alerts. None of the three binds a
// peer. Errata 7577 proposes a peer-side requirement and is in state Reported
// rather than Verified, so it is a proposal and not an approved correction.
// Tagging this behavior with any published id would put a claim on the public
// conformance ledger that the RFC does not make (ai/rules/rfc-compliance.md:
// "The claim MUST state what the test body checks, and MUST NOT state more").
//
// VALIDATES: on TLS 1.3 the peer accepts the EAP-Success only after it has
// decrypted exactly one octet of application data equal to 0x00; on TLS 1.2 it
// requires nothing and derives the RFC 5216 MSK unchanged.
// PREVENTS: a ze peer completing an EAP-TLS 1.3 exchange, and handing the MSK
// to the IKEv2 AUTH computation, on the word of an unprotected EAP-Success
// alone.

package eap

import (
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"
)

// driveEAPTLSPeerWithoutIndication drives the real authenticator against the
// real peer over TLS 1.3, and DROPS the authenticator's closing EAP-Request,
// the one carrying the Section 2.5 indication. It hands the peer a bare
// EAP-Success in its place and returns what the peer answered.
//
// That substitution IS the non-conformant authenticator this file exists to
// refuse, modeled at the EAP layer: RFC 9190 Section 2.5 step 3 lets a server
// send EAP-Success after the indication, and a server that skipped the
// procedure sends the same EAP-Success with nothing before it. Nothing inside
// ze's own authenticator is reached into, so the test says what a ze PEER does
// about a far end it cannot change rather than what ze does to itself.
//
// The peer already holds its MSK when the substitute arrives: the round before
// this one completed the handshake and derived it (handleTLSRequest, peer.go),
// which is exactly why an unrequired indication left the exchange concluding.
func driveEAPTLSPeerWithoutIndication(t *testing.T, cfg MethodConfig, peer *PeerSession) PeerResult {
	t.Helper()

	sess, err := NewSession(TypeTLS, cfg)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}
	method.tlsConfig.MinVersion = tls.VersionTLS13
	method.tlsConfig.MaxVersion = tls.VersionTLS13

	req := sess.Begin()
	for range eapTLS13Rounds {
		if req.Code == CodeSuccess {
			t.Fatal("the authenticator concluded before it sent the indication, so this test never substituted anything")
		}
		// The closing EAP-Request is the first one carrying application data:
		// RFC9190-2.5-2, proven by TestEAPTLS13SendsProtectedSuccessIndication,
		// is what makes no earlier packet carry one.
		if eapTLSAppDataRecords(req) > 0 {
			return peer.Process(&Packet{Code: CodeSuccess, Identifier: req.Identifier})
		}

		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("the peer failed before the authenticator reached the indication: %v", pres.Err)
		}
		if pres.Response == nil {
			t.Fatal("the peer stopped answering before the authenticator reached the indication")
		}
		next := sess.Process(pres.Response)
		if next == nil {
			t.Fatal("the authenticator stopped answering before it reached the indication")
		}
		req = next
	}
	t.Fatalf("the authenticator never reached the indication within %d rounds", eapTLS13Rounds)
	return PeerResult{}
}

// TestEAPTLS13RequiresProtectedSuccessIndication is AC-2 of
// plan/spec-ipsec-rfc9190.md: a ze peer refuses an EAP-TLS 1.3 exchange whose
// authenticator sent no protected success result indication.
//
// The authenticator here is conformant everywhere else: the handshake
// completes, both chains verify, and the peer has already derived its MSK when
// the EAP-Success arrives. The ONLY thing missing is the Section 2.5 record. So
// the refusal is attributable to that record and to nothing else, and before
// this change the same exchange concluded with Done and a real MSK.
//
// It is stricter than the published RFC. See peer_indication.go for why ze
// takes that position, and the file comment above for why nothing here is
// tagged with a published requirement id.
func TestEAPTLS13RequiresProtectedSuccessIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	res := driveEAPTLSPeerWithoutIndication(t, pki.serverConfig(), peer)

	if res.Done {
		t.Fatal("the peer concluded an EAP-TLS 1.3 exchange that carried no RFC 9190 Section 2.5 protected success result indication")
	}
	if res.Err == nil {
		t.Fatalf("the peer neither concluded nor failed: %+v", res)
	}
	// The operator gets a cause they can act on, not a bare refusal.
	for _, want := range []string{"protected success result indication", "TLS 1.3"} {
		if !strings.Contains(res.Err.Error(), want) {
			t.Fatalf("the refusal does not name %q: %v", want, res.Err)
		}
	}
	// The MSK is DENIED, which is the point: the caller turns a Done result into
	// the IKEv2 AUTH payload, so a refusal that still handed one out would
	// change nothing.
	var zero [64]byte
	if res.MSK != zero {
		t.Fatal("the peer handed out an MSK on an exchange it refused")
	}
	if peer.Succeeded() {
		t.Fatal("the peer session reports success after refusing the exchange")
	}
}

// TestEAPTLS13PeerCompletesWithTheIndication is the negative polarity of the
// test above: a peer that refused every exchange would pass it, and this is
// what says the refusal is about the missing record.
//
// It drives the SAME PKI and the same authenticator config, changing one thing:
// the closing EAP-Request is delivered instead of dropped.
func TestEAPTLS13PeerCompletesWithTheIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "an EAP-TLS 1.3 exchange carrying the indication")

	if got := readPeerPlaintext(t, peer); len(got) != 1 || got[0] != 0x00 {
		t.Fatalf("the peer accepted application data % x, want the single octet 00", got)
	}
}

// TestEAPTLS12PeerCompletesWithNoIndication pins the version gate: RFC 9190
// Section 2.5 "only applies to TLS 1.3", so a TLS 1.2 exchange is governed by
// RFC 5216, which defines no indication at all.
//
// A requirement gated on tlsCfg or on MinVersion rather than on the NEGOTIATED
// version would refuse this exchange, because MinVersion is TLS 1.2 on every ze
// EAP-TLS session including the TLS 1.3 ones. The MSK assertion is what says
// the RFC 5216 derivation is untouched.
func TestEAPTLS12PeerCompletesWithNoIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, eapTLS13Rounds)
	requireCompleted(t, fl, "a TLS 1.2 EAP-TLS exchange")

	if peer.indication.Load() != nil {
		t.Fatalf("the TLS 1.2 authenticator sent application data: % x", *peer.indication.Load())
	}
}

// TestEAPTLS13PeerRefusesTheWrongIndicationPayload asserts the peer checks the
// VALUE of the application data and not merely that some arrived.
//
// It runs a complete, conformant exchange first, then re-judges the same
// session against each payload a broken or hostile authenticator could have
// sent. Driving each payload down a real TLS record would need an authenticator
// ze does not have -- tlsMethod writes eapTLSSuccessIndication and nothing else
// -- and the value check is the whole of what changes between these cases.
// TestEAPTLS13RequiresProtectedSuccessIndication is what proves the judge is
// wired into the EAP-Success round at all.
//
// strongSwan's client_process requires exactly one octet equal to 0, so the
// two-octet case is a payload a real EAP-TLS peer also refuses.
func TestEAPTLS13PeerRefusesTheWrongIndicationPayload(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "the conformant exchange this test then re-judges")

	// The session accepted the real indication, so any refusal below is the
	// payload's doing.
	if err := peer.requireSuccessIndication(); err != nil {
		t.Fatalf("the conformant exchange was refused: %v", err)
	}

	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"a non-zero octet", []byte{0x01}},
		{"the indication twice", []byte{0x00, 0x00}},
		{"the indication with a trailer", []byte{0x00, 0x01}},
		{"no octets at all", []byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.payload
			peer.indication.Store(&payload)
			err := peer.requireSuccessIndication()
			if err == nil {
				t.Fatalf("the peer accepted application data % x as the protected success result indication", payload)
			}
			if !strings.Contains(err.Error(), "want the single octet 00") {
				t.Fatalf("the refusal does not say what was wanted: %v", err)
			}
		})
	}
}

// TestEAPTLSPeerAccumulatesTheApplicationDataItReads asserts the peer judges the
// WHOLE of the application data an authenticator sent, across reads.
//
// A field holding only the most recent read would report an authenticator that
// sent 0x00 twice, or 0x01 then 0x00, as conformant: crypto/tls hands back
// whatever fits the buffer on each Read, so the split between reads is chosen
// by the record layer and not by the sender.
//
// eapTLSIndicationKept is the bound, and it is asserted rather than assumed: an
// authenticator choosing how much this session accumulates is a memory growth
// path it controls.
func TestEAPTLSPeerAccumulatesTheApplicationDataItReads(t *testing.T) {
	ps := &PeerSession{}
	ps.recordIndication([]byte{0x00})
	ps.recordIndication([]byte{0x00})

	seen := ps.indication.Load()
	if seen == nil {
		t.Fatal("two reads of one octet produced no application data at all")
	}
	if len(*seen) != 2 {
		t.Fatalf("two reads of one octet accumulated % x, want two octets", *seen)
	}

	for range eapTLSIndicationKept {
		ps.recordIndication([]byte{0xff, 0xff, 0xff, 0xff})
	}
	if got := len(*ps.indication.Load()); got != eapTLSIndicationKept {
		t.Fatalf("the peer kept %d octets, want the eapTLSIndicationKept bound of %d", got, eapTLSIndicationKept)
	}
}

// TestEAPTLS13PeerTellsAnUnreadableIndicationFromAnAbsentOne asserts the two
// outcomes are different answers with different messages.
//
// A read that FAILED and an authenticator that sent NOTHING both leave the peer
// with no indication, and collapsing them would name the wrong repair: the
// first is a record this peer could not decrypt, the second is a far end that
// never followed Section 2.5 (ai/rules/principles.md, a zero that reads as a
// valid answer).
func TestEAPTLS13PeerTellsAnUnreadableIndicationFromAnAbsentOne(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "the conformant exchange this test then re-judges")

	peer.indication.Store(nil)
	absent := peer.requireSuccessIndication()
	if absent == nil {
		t.Fatal("a session with no application data at all was accepted")
	}

	readFailed := errors.New("tls: bad record MAC")
	peer.indicationErr.Store(&readFailed)
	unreadable := peer.requireSuccessIndication()
	if unreadable == nil {
		t.Fatal("a session whose post-handshake read failed was accepted")
	}
	if !errors.Is(unreadable, readFailed) {
		t.Fatalf("the refusal drops the read failure it wrapped: %v", unreadable)
	}
	if unreadable.Error() == absent.Error() {
		t.Fatalf("an unreadable indication and an absent one report the same thing: %v", absent)
	}
}

// TestEAPTLS13ResumedExchangeStillRequiresTheIndication carries the requirement
// onto a resumed exchange.
//
// RFC 9190 Figure 3 draws the 0x00 indication on the resumption flow exactly as
// Figure 1 draws it on the full handshake, so a peer that required it only
// after a full handshake would let an authenticator skip it by offering a
// ticket. The resumption path is a separate code path on this role -- Go hands a
// resumed client no Certificate message and skips VerifyPeerCertificate
// entirely (rebuildResumedChains, peer_chain.go) -- so it is asserted rather
// than assumed.
//
// TestEAPTLS13ResumedExchangeStillSendsTheSuccessIndication is the
// authenticator half of the same pair, and
// TestEAPTLS13ResumedSessionDerivesTheRFC9190MSK is the positive polarity: a
// resumed exchange that CARRIES the indication still concludes.
func TestEAPTLS13ResumedExchangeStillRequiresTheIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server, peerStore := NewResumption(time.Now, true), NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the full handshake that issues the ticket")

	resuming := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	res := driveEAPTLSPeerWithoutIndication(t, pki.serverConfigResuming(server), resuming)

	if !resuming.Resumed() {
		t.Fatal("the second exchange ran a full handshake, so this test never reached the resumption path")
	}
	if res.Done {
		t.Fatal("the peer concluded a RESUMED exchange that carried no protected success result indication")
	}
	if res.Err == nil {
		t.Fatalf("the resumed exchange neither concluded nor failed: %+v", res)
	}
}
