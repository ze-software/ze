// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's EAP-TLS method
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's Identity Response
// RFC: rfc/short/rfc9190.md -- EAP-TLS authorization and privacy (Sections 5.6, 5.8)
//
// VALIDATES: ze's authenticator grants nothing to a peer it did not authenticate
// with a certificate, and a resumed session carries exactly the client identity
// the original one authenticated (Section 5.6); ze's peer never builds a
// privacy-friendly username at all, because every Identity Response it sends is
// an anonymous NAI that carries no part of the user (Section 5.8).
// PREVENTS: an authenticator that lets a certificate-less peer through, a
// resumption that authenticates as someone the original session did not, and a
// peer identity from which a user can be told apart inside one realm.

package eap

import (
	"strings"
	"testing"
	"time"
)

// serverPeerLeafName answers the common name of the client certificate the
// authenticator's TLS engine holds for a completed exchange.
func serverPeerLeafName(t *testing.T, fl *eapTLSFlight) string {
	t.Helper()
	method, ok := fl.sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", fl.sess.method)
	}
	certs := method.conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		t.Fatal("the authenticator completed holding no client certificate")
	}
	return certs[0].Subject.CommonName
}

// TestEAPTLSAuthenticatorGrantsNoAccessToAPeerWithoutACertificate runs the
// authenticator's own TLS configuration against a client that sends an empty
// certificate_list.
//
// RFC requirement: RFC9190-5.6-1 positive -- "When peer authentication is not
// used, EAP-TLS server implementations MUST take care to limit network access
// appropriately for unauthenticated peers". Ze limits it to nothing: the
// authenticator refuses the handshake, so no key material exists from which the
// IKE AUTH payload, and with it any Child SA, could be built.
func TestEAPTLSAuthenticatorGrantsNoAccessToAPeerWithoutACertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	method, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	t.Cleanup(method.Close)

	err = runNAIHandshake(t, method.tlsConfig, naiClientConfig(t, pki, nil, nil))
	if err == nil {
		t.Fatal("the authenticator completed a handshake with a peer that presented no certificate")
	}
	if !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("the authenticator refused with %v, which names no certificate", err)
	}
}

// TestEAPTLS13AuthenticatorGrantsAccessToTheCertificateItVerified drives a full
// and a resumed exchange through the authenticator Session and reads which
// client the authenticator holds for each.
//
// RFC requirement: RFC9190-5.6-1 negative -- the limit is on UNAUTHENTICATED
// peers: a peer that authenticates with a certificate the authenticator verified
// concludes with EAP-Success and an MSK, so the refusal above is not a refusal
// of every peer.
//
// RFC requirement: RFC9190-5.6-2 positive -- "implementations MUST use
// resumption with caution to ensure that a resumed session is not granted more
// privilege than was intended for the original session". The resumed session
// authenticates as the same client certificate the original one verified, and
// as nothing else.
func TestEAPTLS13AuthenticatorGrantsAccessToTheCertificateItVerified(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	if !first.sess.Succeeded() {
		t.Fatal("the authenticator did not conclude with success for a verified certificate")
	}
	if got := serverPeerLeafName(t, first); got != "eap-tls-client" {
		t.Fatalf("the authenticator authenticated %q, want the peer's certificate eap-tls-client", got)
	}

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() || !second.sess.Resumed() {
		t.Fatal("the second exchange did not resume, so it cannot show what a resumed session is granted")
	}
	if got := serverPeerLeafName(t, second); got != "eap-tls-client" {
		t.Fatalf("the resumed session authenticated %q, not the client the original session verified", got)
	}
}

// TestEAPTLS13PeerIdentityCarriesNoPartOfTheUser drives the peer's Identity
// Response for two users of one realm.
//
// Section 5.8's three obligations are conditional: "If anonymous NAIs are not
// used, the privacy-friendly identifiers need to be generated with care." Ze's
// peer always uses one (anonymousNAI, nai.go, is the one writer of the identity
// it sends), so the test asserts that the condition never arises, and that what
// is sent has every property the three sentences ask of the alternative.
//
// RFC requirement: RFC9190-5.8-1 positive -- "it is computationally infeasible
// for an attacker to differentiate two identities belonging to the same user
// from two identities belonging to different users in the same realm": two users
// of example.com send the same octets.
//
// RFC requirement: RFC9190-5.8-2 positive -- the identity includes no substring
// of either username.
//
// RFC requirement: RFC9190-5.8-3 positive -- no per-user mapping exists, fixed
// or otherwise: the identity is the anonymous NAI "@realm", which depends on the
// realm alone.
func TestEAPTLS13PeerIdentityCarriesNoPartOfTheUser(t *testing.T) {
	_, alice := driveEAPTLS13Identity(t, "alice@example.com")
	_, bob := driveEAPTLS13Identity(t, "bob@example.com")

	if alice != "@example.com" {
		t.Fatalf("alice's Identity Response is %q, want the anonymous NAI @example.com", alice)
	}
	if alice != bob {
		t.Fatalf("two users of one realm sent %q and %q, so an observer can tell them apart", alice, bob)
	}
	for _, user := range []string{"alice", "bob"} {
		if strings.Contains(alice, user) {
			t.Fatalf("the Identity Response %q carries the username %q", alice, user)
		}
	}
}

// TestEAPTLS13PeerIdentityKeepsTheRealmApart is the negative of the test above.
//
// RFC requirement: RFC9190-5.8-1 negative -- the property is scoped to "users in
// the same realm": users of two realms send two identities, so the equality
// above comes from dropping the username and not from a peer that sends one
// constant whatever it is given.
//
// RFC requirement: RFC9190-5.8-2 negative -- the realm is not a substring that
// relates the identity to a user, and it is kept: the identity is exactly
// "@" and the realm, nothing removed and nothing added.
//
// RFC requirement: RFC9190-5.8-3 negative -- what the identity does depend on is
// the realm, so a change of realm changes it on the next authentication.
func TestEAPTLS13PeerIdentityKeepsTheRealmApart(t *testing.T) {
	_, first := driveEAPTLS13Identity(t, "alice@example.com")
	_, second := driveEAPTLS13Identity(t, "alice@example.net")

	if first != "@example.com" || second != "@example.net" {
		t.Fatalf("the Identity Responses are %q and %q, want @example.com and @example.net", first, second)
	}
}
