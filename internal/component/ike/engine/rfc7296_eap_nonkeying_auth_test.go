// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- AUTH of an EAP method that derives no key

package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/core/eap"
)

// nonKeyingEAPSecret is the shared secret both halves of the MD5-Challenge fixture
// below are configured with.
const nonKeyingEAPSecret = "md5-challenge-secret"

// mustSharedSecretAuth returns the AUTH value RFC 7296 Section 2.15's construction
// gives for one secret over one set of signed octets.
func mustSharedSecretAuth(t *testing.T, prfID crypto.PRFID, secret, signedOctets []byte) []byte {
	t.Helper()
	auth, err := computeAuthFromSharedSecret(prfID, secret, signedOctets)
	if err != nil {
		t.Fatalf("computeAuthFromSharedSecret: %v", err)
	}
	return auth
}

// runNonKeyingEAP drives a real EAP-MD5-Challenge exchange to success and returns the
// authenticator session and the peer session that ran it.
//
// MD5-Challenge is the method RFC 7296 Section 2.16's SK_pi/SK_pr sentence exists for.
// RFC 3748 Section 5.4 records "Key derivation:            No" in its security claims,
// so eap.Session.DerivesKey and eap.PeerSession.DerivesKey both answer false for it and
// a successful exchange leaves no MSK behind. Both halves are the real package types
// driven by real packets; nothing here stands in for a method.
func runNonKeyingEAP(t *testing.T) (*eap.Session, *eap.PeerSession) {
	t.Helper()

	sess, err := eap.NewSession(eap.TypeMD5Challenge, eap.MethodConfig{Password: nonKeyingEAPSecret})
	if err != nil {
		t.Fatalf("eap.NewSession(MD5-Challenge): %v", err)
	}
	t.Cleanup(sess.Close)
	peer := eap.NewPeerSession(eap.TypeMD5Challenge, "md5-user", nonKeyingEAPSecret)
	t.Cleanup(peer.Close)

	// The conversation is Identity, Challenge, Success: three deliveries to the peer.
	// The bound is generous and is here so a method that never concludes fails this
	// test rather than hanging it.
	pkt := sess.Begin()
	for range 8 {
		if pkt == nil {
			t.Fatal("the authenticator stopped answering before the exchange concluded")
		}
		result := peer.Process(pkt)
		if result.Err != nil {
			t.Fatalf("the EAP peer refused the exchange: %v", result.Err)
		}
		if result.Done {
			break
		}
		if result.Response == nil {
			t.Fatalf("the EAP peer answered nothing to a Code %d Type %d packet", pkt.Code, pkt.Type)
		}
		pkt = sess.Process(result.Response)
	}

	if !sess.Succeeded() || !peer.Succeeded() {
		t.Fatalf("the MD5-Challenge exchange did not succeed (authenticator=%v peer=%v)",
			sess.Succeeded(), peer.Succeeded())
	}
	if sess.DerivesKey() || peer.DerivesKey() {
		t.Fatal("MD5-Challenge reported that it derives a key, so this fixture exercises the wrong branch")
	}
	if sess.MSK() != ([64]byte{}) {
		t.Fatal("a method that derives no key left an MSK behind")
	}
	return sess, peer
}

// nonKeyingEAPSAs returns an initiator SA and a responder SA whose key material and
// signed octets come from a real IKE_SA_INIT and IKE_AUTH, carrying a real EAP exchange
// whose method derives no key.
//
// Only the EAP exchange is swapped. ipsec.AuthEAPMD5 selects a non-key-deriving method,
// and eapMethodType (eap_auth.go) maps it to MD5-Challenge, so this is a state an
// operator's configuration reaches rather than one built only for the test. The SAs come
// from autEAPHandshake, whose MS-CHAPv2 exchange left the non-zero MSK the assertions
// below use as their discriminator, so the session is replaced and the handshake is not.
func nonKeyingEAPSAs(t *testing.T) (ini, resp *SA) {
	t.Helper()
	ini, resp, _ = autEAPHandshake(t)

	sess, peer := runNonKeyingEAP(t)
	ini.EAPSession = peer
	resp.EAPSession = sess

	// The MSK that the MS-CHAPv2 handshake left on both SAs stays where it is, and it
	// is the discriminator: a producer that still keyed from sa.EAPMSK would build a
	// different AUTH from the ones asserted below, and would build it in silence.
	if ini.EAPMSK == ([64]byte{}) || ini.EAPMSK != resp.EAPMSK {
		t.Fatal("the fixture holds no shared non-zero MSK, so no assertion below can tell SK_pi from it")
	}
	if bytes.Equal(ini.SKKeys.SK_pi, ini.SKKeys.SK_pr) {
		t.Fatal("SK_pi equals SK_pr in this fixture, so the two directions cannot be told apart")
	}
	if !bytes.Equal(ini.SKKeys.SK_pi, resp.SKKeys.SK_pi) || !bytes.Equal(ini.SKKeys.SK_pr, resp.SKKeys.SK_pr) {
		t.Fatal("the two SAs derived different SK_p keys, so neither side can verify the other")
	}
	return ini, resp
}

// VALIDATES: an EAP method that derives no key produces the AUTH payloads of messages 7
// and 8 from SK_pi and SK_pr, and each side verifies the other's.
//
// PREVENTS: the state before this change. computeEAPAuth keyed every EAP AUTH from
// sa.EAPMSK, and verifyRemoteAuth accepted one only while sa.EAPMSK was non-zero. A
// method that derives no key leaves that array zero, so message 7 went out signed with
// 64 zero octets and message 8 was refused outright. The path RFC 7296 Section 2.16
// specifies could not complete at all.
//
// RFC 7296 Section 2.16: "If EAP methods that do not generate a shared key are used, the
// AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr,
// respectively."
//
// RFC requirement: RFC7296-2.16-5 positive -- an EAP exchange whose method derives no
// key, a real MD5-Challenge conversation run to success on both roles, keys message 7
// with SK_pi and message 8 with SK_pr. The body asserts each payload equals the Section
// 2.15 shared-secret construction over that key, that verifyRemoteAuth on the opposite
// side accepts it, that neither equals the same construction over sa.EAPMSK, which both
// SAs hold non-zero throughout, and that the two payloads differ from each other.
func TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr(t *testing.T) {
	ini, resp := nonKeyingEAPSAs(t)
	prfID := ini.Proposal.PRF.ID

	// Message 7, which the initiator sends. Section 2.16 names SK_pi for it.
	auth7 := eapProducerAUTH(t, ini, "initiator, method that derives no key")
	iniOctets := mustSignedOctets(t, ini)
	if !bytes.Equal(auth7.AuthData, mustSharedSecretAuth(t, prfID, ini.SKKeys.SK_pi, iniOctets)) {
		t.Error("message 7 was not keyed by SK_pi")
	}
	if bytes.Equal(auth7.AuthData, mustSharedSecretAuth(t, prfID, ini.EAPMSK[:], iniOctets)) {
		t.Error("message 7 was keyed by the MSK, which the method never derived")
	}
	if err := verifyRemoteAuth(resp, auth7); err != nil {
		t.Errorf("the responder refused the initiator's SK_pi AUTH: %v", err)
	}

	// Message 8, which the responder sends. Section 2.16 names SK_pr for it.
	auth8 := eapProducerAUTH(t, resp, "responder, method that derives no key")
	respOctets := mustSignedOctets(t, resp)
	if !bytes.Equal(auth8.AuthData, mustSharedSecretAuth(t, prfID, resp.SKKeys.SK_pr, respOctets)) {
		t.Error("message 8 was not keyed by SK_pr")
	}
	if bytes.Equal(auth8.AuthData, mustSharedSecretAuth(t, prfID, resp.EAPMSK[:], respOctets)) {
		t.Error("message 8 was keyed by the MSK, which the method never derived")
	}
	if err := verifyRemoteAuth(ini, auth8); err != nil {
		t.Errorf("the initiator refused the responder's SK_pr AUTH: %v", err)
	}

	if bytes.Equal(auth7.AuthData, auth8.AuthData) {
		t.Error("messages 7 and 8 carry one value, so one key produced both")
	}
}

// VALIDATES: a method that DOES derive a key is untouched by the change above. Both
// AUTH payloads stay keyed by the MSK, and neither is keyed by SK_pi or SK_pr.
//
// PREVENTS: a fix for the non-key-deriving case that routes every EAP method to
// SK_pi/SK_pr. RFC 7296 Section 2.16's first sentence would then be violated for
// MS-CHAPv2 and EAP-TLS, and the test above would still be green.
//
// RFC requirement: RFC7296-2.16-5 negative -- the SK_pi and SK_pr construction is
// conditional on the method deriving no key, and is not applied to every EAP exchange.
// The body runs a real MS-CHAPv2 handshake, whose method does derive a key, and asserts
// for the initiator and for the responder that the AUTH payload equals the Section 2.15
// shared-secret construction over sa.EAPMSK and does not equal it over SK_pi or SK_pr.
//
// It carries no claim on RFC7296-2.16-12, the row for the first sentence of Section
// 2.16. That row is proven by TestEapAuthProducerIsKeyedByTheNegotiatedMSK
// (rfc7296_eap_auth_producer_test.go), and a second claim would say the same sentence
// twice.
func TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK(t *testing.T) {
	ini, resp, _ := autEAPHandshake(t)
	prfID := ini.Proposal.PRF.ID

	if ini.EAPMSK == ([64]byte{}) {
		t.Fatal("the MS-CHAPv2 handshake left no MSK, so nothing below is keyed by one")
	}

	for _, tc := range []struct {
		sa   *SA
		what string
		skP  []byte
	}{
		{ini, "initiator, MS-CHAPv2", ini.SKKeys.SK_pi},
		{resp, "responder, MS-CHAPv2", resp.SKKeys.SK_pr},
	} {
		auth := eapProducerAUTH(t, tc.sa, tc.what)
		octets := mustSignedOctets(t, tc.sa)
		if !bytes.Equal(auth.AuthData, mustSharedSecretAuth(t, prfID, tc.sa.EAPMSK[:], octets)) {
			t.Errorf("%s: the AUTH payload was not keyed by the MSK", tc.what)
		}
		if bytes.Equal(auth.AuthData, mustSharedSecretAuth(t, prfID, tc.skP, octets)) {
			t.Errorf("%s: the AUTH payload was keyed by SK_pi/SK_pr, which Section 2.16 reserves "+
				"for a method that derives no key", tc.what)
		}
	}
}

// VALIDATES: an EAP AUTH payload is refused, and none is built, until the EAP exchange
// it belongs to has succeeded.
//
// PREVENTS: the authentication bypass that arrives with SK_pi and SK_pr if nothing
// checks the order. Those two keys descend from SKEYSEED, which anybody who completed
// IKE_SA_INIT holds, so an AUTH keyed by SK_pi proves only that its sender did the
// unauthenticated half of the exchange. A responder that verified one mid-EAP would
// establish the SA with the method never having authenticated anybody. The
// sa.EAPMSK != [64]byte{} test this replaced refused such an AUTH by accident, because
// a key-deriving method leaves the MSK zero until it succeeds; a method that derives no
// key has no such accident to rely on.
//
// Untagged: RFC 7296 Section 2.16 places the AUTH payloads after the EAP Success
// message (RFC7296-2.16-13), and this body asserts the receive-side refusal of one that
// arrives before it rather than the placement of one that is sent.
func TestEAPAuthIsRefusedBeforeTheEAPExchangeSucceeds(t *testing.T) {
	ini, resp := nonKeyingEAPSAs(t)

	// The AUTH the peer would legitimately send once the exchange concluded.
	auth7 := eapProducerAUTH(t, ini, "initiator, method that derives no key")

	// Now put the responder back where it sits mid-exchange: a fresh session that has
	// answered nothing. Every other input, the keys and the signed octets included, is
	// the one that produced the AUTH above.
	fresh, err := eap.NewSession(eap.TypeMD5Challenge, eap.MethodConfig{Password: nonKeyingEAPSecret})
	if err != nil {
		t.Fatalf("eap.NewSession(MD5-Challenge): %v", err)
	}
	t.Cleanup(fresh.Close)
	if fresh.Succeeded() {
		t.Fatal("a session that has answered nothing reports success")
	}
	resp.EAPSession = fresh

	if err := verifyRemoteAuth(resp, auth7); err == nil {
		t.Error("the responder accepted an EAP AUTH before the EAP exchange succeeded")
	}

	// The send side refuses too, so no such payload is built either.
	ini.EAPSession = eap.NewPeerSession(eap.TypeMD5Challenge, "md5-user", nonKeyingEAPSecret)
	if _, err := computeEAPAuth(ini); err == nil {
		t.Error("computeEAPAuth built an AUTH payload for an EAP exchange that had not succeeded")
	}

	// An SA that ran no EAP exchange at all answers with an error rather than with a
	// secret. sa.EAPSession is typed any, so an absent session is a nil interface.
	resp.EAPSession = nil
	if err := verifyRemoteAuth(resp, auth7); err == nil {
		t.Error("the responder accepted an EAP AUTH on an SA that ran no EAP exchange")
	}
}
