// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- the anonymous NAI the EAP-TLS peer sends
// RFC: rfc/short/rfc9190.md -- Section 2.1.8, Privacy
// Detail: rfc/full/rfc7542.txt -- Section 2.2, the NAI grammar
//
// RFC 9190 Section 2.1.8 puts three obligations on the identity an EAP-TLS 1.3
// exchange carries:
//
//	"EAP-TLS peer and server implementations supporting TLS 1.3 MUST support
//	anonymous Network Access Identifiers (NAIs) (Section 2.4 of [RFC7542]).  A
//	client supporting TLS 1.3 MUST NOT send its username (or any other permanent
//	identifiers) in cleartext in the Identity Response (or any message used
//	instead of the Identity Response).  Following [RFC7542], it is RECOMMENDED
//	to omit the username (i.e., the NAI is @realm) [...] Note that the NAI MUST
//	be a UTF-8 string as defined by the grammar in Section 2.2 of [RFC7542]."
//
// VALIDATES: ze's EAP-TLS peer puts an anonymous NAI in the Identity Response
// whatever the operator configured, the realm survives and the username does
// not, every NAI ze emits matches the RFC 7542 Section 2.2 grammar, and ze's
// authenticator completes an exchange whose peer identified itself with either
// anonymous form.
// PREVENTS: the configured local-id reaching the wire in cleartext, a derived
// NAI no grammar accepts, and an authenticator that needs a username to
// authenticate.

package eap

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"strings"
	"testing"
	"time"
)

// naiExchangeRounds bounds a conversation this file drives to completion. A
// successful EAP-TLS 1.3 exchange takes 7 authenticator packets
// (eapTLS13Rounds, rfc9190_test.go), and the slack absorbs a fragment without
// absorbing an extra round.
const naiExchangeRounds = 12

// driveEAPTLS13Identity drives one complete EAP-TLS 1.3 exchange from the
// operator's configured identity and answers what the peer put in the Identity
// Response.
//
// It reads peerSent[0] because the authenticator opens every exchange with the
// EAP-Request/Identity that Session.Begin builds (eap.go), so the peer's first
// packet is its Identity Response and its TypeData is the NAI itself.
func driveEAPTLS13Identity(t *testing.T, identity string) (*eapTLSFlight, string) {
	t.Helper()

	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS(identity, pki.peerConfigWithCRL(pki.trustedCRLPEM))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, naiExchangeRounds)

	if len(fl.peerSent) == 0 {
		t.Fatal("the peer answered nothing, so the exchange carried no Identity Response")
	}
	first := fl.peerSent[0]
	if first.Type != TypeIdentity {
		t.Fatalf("the peer's first packet is Type %d, want the Identity Response (Type %d)", first.Type, TypeIdentity)
	}
	return fl, string(first.TypeData)
}

// TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm drives a complete EAP-TLS
// 1.3 exchange for an operator who configured a full NAI, and reads the Identity
// Response off the wire.
//
// RFC requirement: RFC9190-2.1.8-2 positive -- the configured identity is
// "alice@example.com" and the Identity Response carries "@example.com", so the
// username RFC 9190 Section 2.1.8 forbids in cleartext is absent while the realm
// that routes the exchange survives. The assertion covers every packet the peer
// sent and not the Identity Response alone, because the MUST NOT also names "any
// message used instead of the Identity Response".
func TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm(t *testing.T) {
	fl, nai := driveEAPTLS13Identity(t, "alice@example.com")

	if nai != "@example.com" {
		t.Fatalf("Identity Response carries %q, want %q: RFC 9190 Section 2.1.8 omits the username", nai, "@example.com")
	}
	for i, p := range fl.peerSent {
		if strings.Contains(string(p.TypeData), "alice") {
			t.Fatalf("peer packet %d carries the configured username in cleartext: RFC 9190 Section 2.1.8 forbids it", i)
		}
	}

	if !fl.peerDone {
		t.Fatal("the exchange did not complete, so the anonymous NAI cost the peer its authentication")
	}
	if fl.peerMSK != fl.serverMSK {
		t.Fatal("the two ends derived different MSKs from an exchange the anonymous NAI opened")
	}
}

// TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm drives the same
// exchange for an operator whose local-id carries no realm, which is the shape
// every EAP-TLS scenario in this repository configures.
//
// RFC requirement: RFC9190-2.1.8-5 positive -- "it is RECOMMENDED to omit the
// username (i.e., the NAI is @realm), but other constructions such as a fixed
// username (e.g., anonymous@realm) [...] are allowed". "ze-test-client" gives no
// realm to omit a username from, and the RFC 7542 Section 2.2 grammar has no NAI
// that is neither a username nor a realm, so the peer takes the fixed username
// that sentence allows and the configured identity reaches no packet.
func TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm(t *testing.T) {
	fl, nai := driveEAPTLS13Identity(t, "ze-test-client")

	if nai != anonymousUser {
		t.Fatalf("Identity Response carries %q, want %q", nai, anonymousUser)
	}
	for i, p := range fl.peerSent {
		if strings.Contains(string(p.TypeData), "ze-test-client") {
			t.Fatalf("peer packet %d carries the configured identity in cleartext: RFC 9190 Section 2.1.8 forbids it", i)
		}
	}

	if !fl.peerDone {
		t.Fatal("the exchange did not complete, so the anonymous NAI cost the peer its authentication")
	}
}

// TestEAPTLSPeerAnonymizesEveryConfiguredIdentity reads the derivation over the
// identity shapes an operator can write, including the ones no grammar accepts.
//
// It is also what keeps RFC 9190 Section 5.8's three MUSTs out of reach. Those
// open "If anonymous NAIs are not used", and no row here produces a username, so
// ze composes no privacy-friendly username that could carry a substring of the
// user's identity or a fixed mapping across authentications.
//
// RFC requirement: RFC9190-2.1.8-3 positive -- "the NAI MUST be a UTF-8 string
// as defined by the grammar in Section 2.2 of [RFC7542]". Every NAI the peer can
// emit is checked against that grammar here, including the ones derived from an
// identity the grammar itself refuses.
func TestEAPTLSPeerAnonymizesEveryConfiguredIdentity(t *testing.T) {
	cases := []struct {
		configured string
		nai        string
		why        string
	}{
		{"alice@example.com", "@example.com", "the username is omitted and the realm is kept"},
		{"@example.com", "@example.com", "an identity that is already anonymous is unchanged"},
		{"alice@sub.example.com", "@sub.example.com", "a subdomain realm routes, so it survives whole"},
		{"ze-test-client", anonymousUser, "no realm to keep, so the fixed username"},
		{"192.0.2.1", anonymousUser, "an address is a permanent identifier, and it carries no realm"},
		{"DOMAIN\\alice", anonymousUser, "a backslash is no utf8-atext, and there is no realm"},
		{"alice@@example.com", anonymousUser, "the realm would be @example.com, which utf8-rtext refuses"},
		{"alice@localhost", anonymousUser, "utf8-realm needs a dot, so localhost is no realm"},
		{"alice@-example.com", anonymousUser, "a label cannot open with a hyphen"},
		{"alice@example.com.", anonymousUser, "a trailing dot leaves an empty label"},
		{"", anonymousUser, "an unset identity still owes a valid NAI"},
		{"alice@ex\xffample.com", anonymousUser, "a realm that is not UTF-8 cannot be a UTF-8 NAI"},
	}

	for _, tc := range cases {
		got := anonymousNAI(tc.configured)
		if got != tc.nai {
			t.Errorf("anonymousNAI(%q) = %q, want %q: %s", tc.configured, got, tc.nai, tc.why)
			continue
		}
		if !validNAI(got) {
			t.Errorf("anonymousNAI(%q) = %q, which the RFC 7542 Section 2.2 grammar refuses", tc.configured, got)
		}

		// The username half of the configured identity is what the MUST NOT
		// forbids on the wire. An identity that carries none leaks nothing to
		// look for, so the search runs only where there is something to find.
		user, _, _ := strings.Cut(tc.configured, "@")
		if user == "" {
			continue
		}
		if strings.Contains(got, user) {
			t.Errorf("anonymousNAI(%q) = %q, which still carries the configured username", tc.configured, got)
		}
	}
}

// TestNAIGrammarMatchesRFC7542Section22 reads the grammar checker over strings
// it must refuse, because a checker that accepted everything would pass every
// assertion the positive tests make.
//
// RFC requirement: RFC9190-2.1.8-3 negative -- the grammar of RFC 7542 Section
// 2.2 is a constraint and not a formality, so each invalid row breaks exactly
// one of its rules: an empty string between dots, a hyphen at a label edge, a
// realm of one label, a character outside utf8-atext, and a byte sequence that
// is not UTF-8. The valid rows are the three productions of "nai" itself.
func TestNAIGrammarMatchesRFC7542Section22(t *testing.T) {
	valid := []string{
		"alice",
		"alice.smith",
		"a!#$%&'*+-/=?^_`{|}~",
		"@example.com",
		"@ex-ample.com",
		"@a.b.c.example.com",
		"alice@example.com",
		"anonymous",
		"anonymous@example.com",
		"ålice@exämple.com",
	}
	for _, nai := range valid {
		if !validNAI(nai) {
			t.Errorf("validNAI(%q) = false, want true", nai)
		}
	}

	invalid := []string{
		"",
		"@",
		".alice",
		"alice.",
		"ali..ce",
		"alice@",
		"alice@localhost",
		"@localhost",
		"@example..com",
		"@-example.com",
		"@example-.com",
		"@example.com-",
		"@exam ple.com",
		"@example.com@extra.com",
		"al ice@example.com",
		"al\\ice@example.com",
		"alice@exam_ple.com",
		"\xff@example.com",
		"@ex\xffample.com",
	}
	for _, nai := range invalid {
		if validNAI(nai) {
			t.Errorf("validNAI(%q) = true, want false", nai)
		}
	}
}

// TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity pins the boundary of the
// anonymization, from the peer's real Identity Response.
//
// RFC requirement: RFC9190-2.1.8-2 negative -- RFC 9190 governs EAP-TLS, and
// Section 2.1.8's MUST NOT is addressed to "a client supporting TLS 1.3". A
// password method carries its own username inside the method exchange (RFC 2759
// Section 4), so its Identity Response is unchanged. A peer that anonymized
// every method would pass the positive tests above and fail this one.
func TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity(t *testing.T) {
	peer := NewPeerSession(TypeMSCHAPv2, "alice@example.com", "secret")
	t.Cleanup(peer.Close)

	res := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	if res.Response == nil {
		t.Fatal("the peer answered no Identity Response")
	}
	if got := string(res.Response.TypeData); got != "alice@example.com" {
		t.Fatalf("Identity Response carries %q, want the configured identity %q", got, "alice@example.com")
	}
}

// TestEAPTLS13AuthenticatorAcceptsAnAnonymousNAI drives complete exchanges whose
// peer identified itself with each anonymous form RFC 9190 Section 2.1.8 names,
// and reads the identity the authenticator recorded.
//
// RFC requirement: RFC9190-2.1.8-1 positive -- "EAP-TLS peer and server
// implementations supporting TLS 1.3 MUST support anonymous Network Access
// Identifiers (NAIs)". Support on the server role is the exchange completing
// with a shared MSK: the authenticator asks the NAI for nothing, so a peer that
// reveals no username still authenticates. The fixed-username row is written
// onto the peer directly, because ze's own peer never composes that form; it is
// what another implementation puts on the wire.
func TestEAPTLS13AuthenticatorAcceptsAnAnonymousNAI(t *testing.T) {
	for _, nai := range []string{"@example.com", "anonymous@example.com"} {
		pki := newEAPTLSPKI(t)
		peer := NewPeerSessionTLS("unused", pki.peerConfigWithCRL(pki.trustedCRLPEM))
		peer.identity = nai

		fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, naiExchangeRounds)

		if !fl.peerDone {
			t.Errorf("the exchange for NAI %q did not complete: %v", nai, fl.peerErr)
			continue
		}
		if fl.peerMSK != fl.serverMSK {
			t.Errorf("the two ends derived different MSKs for NAI %q", nai)
		}
		if got := fl.sess.Identity(); got != nai {
			t.Errorf("the authenticator recorded identity %q, want %q", got, nai)
		}
	}
}

// naiClientConfig builds a TLS client for the authenticator's own tls.Config,
// with the client certificate named or with none at all.
//
// The client trusts the harness CA, because a client that refused the server
// would fail before it ever sent its own certificate_list and the test would
// read the wrong refusal.
func naiClientConfig(t *testing.T, pki *eapTLSPKI, certPEM, keyPEM []byte) *tls.Config {
	t.Helper()

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pki.trustedCAPEM) {
		t.Fatal("the harness CA did not parse")
	}
	cfg := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // EAP-TLS carries no server hostname, so the harness client verifies nothing else either
		MaxVersion:         tls.VersionTLS13,
		MinVersion:         tls.VersionTLS13,
		RootCAs:            roots,
	}
	if len(certPEM) > 0 {
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("load client cert: %v", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg
}

// runNAIHandshake runs one TLS 1.3 handshake between the authenticator's own
// tls.Config and the client config given, over a pipe, and answers the error the
// SERVER saw.
//
// The pair speaks TLS directly rather than through the EAP fragmenter, because
// certificate_list processing is TLS's and the fragmenter carries the same bytes
// either way. The server side is the production configuration newTLSMethod
// built, so the policy under test is the one an EAP-TLS session runs.
//
// The server's pipe end is closed as soon as its handshake returns. A refused
// client is parked reading a reply that will never come, because net.Pipe is
// unbuffered and nothing else will read from it, so the close is what ends the
// client rather than the deadline. The deadline stays as a backstop.
func runNAIHandshake(t *testing.T, serverCfg, clientCfg *tls.Config) error {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	deadline := time.Now().Add(10 * time.Second)
	if err := serverConn.SetDeadline(deadline); err != nil {
		t.Fatalf("server deadline: %v", err)
	}
	if err := clientConn.SetDeadline(deadline); err != nil {
		t.Fatalf("client deadline: %v", err)
	}

	ctx := t.Context()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := tls.Client(clientConn, clientCfg)
		client.HandshakeContext(ctx) //nolint:errcheck // the server's verdict is what this test reads
		clientConn.Close()           //nolint:errcheck // closing releases a server parked on the pipe
	}()

	server := tls.Server(serverConn, serverCfg)
	err := server.HandshakeContext(ctx)
	serverConn.Close() //nolint:errcheck // closing releases the client goroutine

	<-clientDone
	return err
}

// TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal drives the
// authenticator's own TLS configuration against a client that sends no
// certificate, and against one that sends the certificate it holds.
//
// RFC requirement: RFC9190-2.1.8-4 positive -- "When EAP-TLS is used with TLS
// version 1.3, the EAP-TLS peer and EAP-TLS server SHALL follow the processing
// specified by version 1.3 of TLS.  This means that the EAP-TLS peer only sends
// an empty certificate_list if it does not have an appropriate certificate to
// send, and the EAP-TLS server MAY treat an empty certificate_list as a terminal
// condition." Ze's authenticator takes that MAY: ClientAuth is
// RequireAndVerifyClientCert (newTLSMethod, eap_tls.go), so the handshake ends
// on the empty list and completes on the certificate. The second polarity is
// what makes the first mean anything, because a server that refused every client
// would pass the empty-list assertion on its own.
func TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal(t *testing.T) {
	pki := newEAPTLSPKI(t)

	empty, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	t.Cleanup(empty.Close)

	err = runNAIHandshake(t, empty.tlsConfig, naiClientConfig(t, pki, nil, nil))
	if err == nil {
		t.Fatal("the authenticator accepted an empty certificate_list, so no EAP-TLS peer is authenticated")
	}
	if !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("the authenticator refused with %v, which names no certificate: the refusal is some other failure", err)
	}

	held, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	t.Cleanup(held.Close)

	if err := runNAIHandshake(t, held.tlsConfig, naiClientConfig(t, pki, pki.clientCertPEM, pki.clientKeyPEM)); err != nil {
		t.Fatalf("the authenticator refused a client that sent its certificate: %v", err)
	}
}
