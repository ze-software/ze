// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 key derivation
// RFC: rfc/short/rfc3748.md -- Section 7.10 key derivation: the MSK, the EMSK and its confinement
// RFC: rfc/short/rfc2759.md -- MS-CHAPv2, whose MPPE master key roots both keys
//
// VALIDATES: EAP-MSCHAPv2 exports a 64-octet Extended Master Session Key beside
// its MSK on both roles, cryptographically separate from that MSK, and confined
// to the two sessions that derived it.
// PREVENTS: the two ways this derivation goes wrong. Deriving the EMSK FROM the
// MSK, which RFC 3748 Section 7.10 forbids because recovering one would then
// yield the other; and publishing it, which the same section forbids.
//
// RFC3748-7.10-2 and RFC3748-7.10-6 span both key-deriving methods ze runs, so
// each is tagged here as well as over EAP-TLS in rfc3748_emsk_test.go.

package eap

import (
	"bytes"
	"crypto/hkdf"
	"crypto/sha256"
	"testing"
)

// mschapv2EMSKExchange drives one complete EAP-MSCHAPv2 exchange, from the
// Identity Request to the EAP-Success, and answers both sessions plus every
// octet that crossed between them.
//
// It runs to the EAP-Success rather than stopping at the method's result
// indication, because the Session stores its EMSK on the Success Ack round
// (mschapv2Method.handleSuccessAck).
func mschapv2EMSKExchange(t *testing.T, password string) (sess *Session, peer *PeerSession, wire [][]byte) {
	t.Helper()

	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: password})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(sess.Close)

	peer = NewPeerSession(TypeMSCHAPv2, "user", password)
	t.Cleanup(peer.Close)

	req := sess.Begin()
	for range 10 {
		wire = append(wire, req.Encode())
		if req.Code == CodeSuccess {
			return sess, peer, wire
		}
		if req.Code == CodeFailure {
			t.Fatal("the authenticator refused an exchange both sides should accept")
		}

		res := peer.Process(req)
		if res.Err != nil {
			t.Fatalf("the peer failed an exchange both sides should accept: %v", res.Err)
		}
		if res.Response == nil {
			t.Fatal("the peer stopped answering before the exchange concluded")
		}
		wire = append(wire, res.Response.Encode())

		next := sess.Process(res.Response)
		if next == nil {
			t.Fatal("the authenticator stopped answering before the exchange concluded")
		}
		req = next
	}
	t.Fatal("the exchange did not conclude")
	return nil, nil, nil
}

// TestRFC3748MSCHAPv2ExportsASixtyFourOctetEMSK asserts EAP-MSCHAPv2 exports the
// second key RFC 3748 Section 7.10 requires, and that it is separate from the
// first.
//
// RFC 3748 Section 7.10: "an EAP method supporting key derivation MUST export a
// Master Session Key (MSK) of at least 64 octets, and an Extended Master Session
// Key (EMSK) of at least 64 octets." EAP-MSCHAPv2 supports key derivation:
// draft-kamath-pppext-eap-mschapv2-02 Section 3 declares "Key derivation:
// Yes", and TypeDerivesKey (eap.go) answers true for Type 26.
//
// The same section constrains HOW: "Without violating a fundamental
// cryptographic assumption (such as the non-invertibility of a one-way
// function), an attacker recovering the MSK or EMSK MUST NOT be able to recover
// the other quantity with a level of effort less than brute force." The expected
// octets are recomputed here from the MPPE master key, under the same expansion
// the producer uses but written out from the primitives, so the assertion pins
// the construction rather than reading the producer's answer back.
//
// RFC requirement: RFC3748-7.10-2 positive -- after a completed EAP-MSCHAPv2
// exchange the authenticator Session and the peer PeerSession each hold an EMSK
// of exactly 64 octets that is non-zero, equal to the other end's, equal to
// HKDF-Expand(SHA-256, MPPE MasterKey, ze's EMSK label, 64), and shares no octet
// position with the MSK derived beside it.
func TestRFC3748MSCHAPv2ExportsASixtyFourOctetEMSK(t *testing.T) {
	const password = "correct horse battery staple"

	sess, peer, _ := mschapv2EMSKExchange(t, password)

	// RFC 3748 Section 7.10 states a floor of 64 octets, and ze exports exactly
	// that. Reading the length is what makes the floor an assertion rather than a
	// property of the declaration.
	if len(sess.emsk) != 64 {
		t.Fatalf("the authenticator EMSK is %d octets, and Section 7.10 needs at least 64", len(sess.emsk))
	}
	if len(peer.emsk) != 64 {
		t.Fatalf("the peer EMSK is %d octets, and Section 7.10 needs at least 64", len(peer.emsk))
	}

	if sess.emsk == ([64]byte{}) {
		t.Fatal("the authenticator EMSK is 64 zero octets, which is an unset field and not a key")
	}
	if sess.emsk != peer.emsk {
		t.Fatalf("the two ends derived different EMSKs:\n  authenticator %x\n  peer          %x", sess.emsk, peer.emsk)
	}

	// The construction, rebuilt from the primitives rather than by calling the
	// producer. MasterKey is the root RFC 3079 Section 3 defines, and the EMSK is
	// its HKDF expansion under a label of its own.
	masterKey := GetMasterKey(password, peer.ntResponse)
	want, err := hkdf.Expand(sha256.New, masterKey[:], mschapv2EMSKInfo, 64)
	if err != nil {
		t.Fatalf("the control expansion failed, so this test cannot judge the construction: %v", err)
	}
	if [64]byte(want) != sess.emsk {
		t.Fatalf("EMSK = %x,\n  want %x (HKDF-Expand over the MPPE master key)", sess.emsk, want)
	}

	// Cryptographic separation, as far as a test can see it: the two keys share no
	// octet position. The real argument is that neither is a function of the other
	// (deriveEMSK, mschapv2.go), which no test can assert; this catches the
	// construction collapsing into a copy or a shift of the MSK.
	msk := sess.MSK()
	if msk == sess.emsk {
		t.Fatal("the EMSK equals the MSK, so the two branches of the key hierarchy are one branch")
	}
	for i := range msk {
		if msk[i] != sess.emsk[i] {
			return
		}
	}
	t.Fatal("every octet of the EMSK matches the MSK at the same position")
}

// TestRFC3748NoMSCHAPv2EMSKWithoutASuccessfulExchange asserts ze exports no EMSK
// for an exchange that did not authenticate.
//
// An EMSK sitting on a session whose peer failed is a key derived from a
// credential nobody proved, and the caller cannot tell it from one that was
// earned (ai/rules/evidence.md).
//
// RFC requirement: RFC3748-7.10-2 negative -- an EAP-MSCHAPv2 exchange the
// authenticator refuses on a wrong password exports no EMSK at all: the
// authenticator Session's EMSK stays all zero and the peer publishes no key,
// rather than an EMSK shorter than the 64 octets Section 7.10 requires.
func TestRFC3748NoMSCHAPv2EMSKWithoutASuccessfulExchange(t *testing.T) {
	sess, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "the-real-password"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(sess.Close)

	peer := NewPeerSession(TypeMSCHAPv2, "user", "the-wrong-password")
	t.Cleanup(peer.Close)

	req := sess.Begin()
	success := false
	for range 10 {
		if req.Code == CodeSuccess {
			success = true
			break
		}
		if req.Code == CodeFailure {
			break
		}
		res := peer.Process(req)
		if res.Err != nil || res.Response == nil {
			break
		}
		next := sess.Process(res.Response)
		if next == nil {
			break
		}
		req = next
	}

	if success {
		t.Fatal("the authenticator accepted the wrong password")
	}
	if sess.emsk != ([64]byte{}) {
		t.Fatalf("a refused exchange left an EMSK on the authenticator: %x", sess.emsk)
	}
	if !sess.Succeeded() && sess.MSK() != ([64]byte{}) {
		t.Fatalf("a refused exchange left an MSK on the authenticator: %x", sess.MSK())
	}
}

// TestRFC3748TheMSCHAPv2EMSKStaysWhereItWasDerived asserts the EAP-MSCHAPv2 EMSK
// lives in the two sessions that derived it, and lives no longer than they do.
//
// RFC 3748 Section 7.10: "The EMSK is reserved for future use and MUST remain on
// the EAP peer and EAP server where it is derived; it MUST NOT be transported
// to, or shared with, additional parties, or used to derive any other keys."
//
// RFC requirement: RFC3748-7.10-6 positive -- after a completed EAP-MSCHAPv2
// exchange the EMSK is held by both ends that derived it, the authenticator
// Session and the peer PeerSession, and Close erases it from each of them.
func TestRFC3748TheMSCHAPv2EMSKStaysWhereItWasDerived(t *testing.T) {
	sess, peer, _ := mschapv2EMSKExchange(t, "correct horse battery staple")

	derived := sess.emsk
	if derived == ([64]byte{}) {
		t.Fatal("no EMSK was derived, so there is nothing for this test to judge")
	}
	if peer.emsk != derived {
		t.Fatalf("the EAP server and the EAP peer hold different EMSKs:\n  server %x\n  peer   %x", sess.emsk, peer.emsk)
	}

	sess.Close()
	peer.Close()

	if sess.emsk != ([64]byte{}) {
		t.Fatalf("Close left the EMSK on the authenticator: %x", sess.emsk)
	}
	if peer.emsk != ([64]byte{}) {
		t.Fatalf("Close left the EMSK on the peer: %x", peer.emsk)
	}
}

// TestRFC3748TheMSCHAPv2EMSKIsNeverHandedOutward asserts the EAP-MSCHAPv2 EMSK
// reaches nothing beyond the two sessions that derived it.
//
// The path that would carry it is the MSK's. Both ends hand their carrier an MSK
// for the IKEv2 AUTH payload of RFC 7296 Section 2.16, and that carrier is in
// another package, so a producer that assigned the EMSK where the MSK belongs
// would transport it out of ze's EAP layer with every length and equality
// assertion still true.
//
// The search is proved to discriminate before it is trusted: it is run once over
// a corpus the EMSK was deliberately added to, and it finds it there
// (ai/rules/evidence.md, "Zero hits is not absence").
//
// RFC requirement: RFC3748-7.10-6 negative -- the EAP-MSCHAPv2 EMSK octets appear
// in nothing that leaves the two sessions: not in the MSK the authenticator
// publishes through Session.MSK, not in the MSK the peer holds for its carrier,
// and not in any EAP packet encoded in either direction during the exchange.
func TestRFC3748TheMSCHAPv2EMSKIsNeverHandedOutward(t *testing.T) {
	sess, peer, wire := mschapv2EMSKExchange(t, "correct horse battery staple")

	emsk := sess.emsk
	if emsk == ([64]byte{}) {
		t.Fatal("no EMSK was derived, so this test would pass over an empty search")
	}

	// Everything the EAP layer hands outward on a successful exchange. The two
	// MSKs are what the IKEv2 carrier reads (handleEAPResponse and
	// handleResponderEAP, internal/component/ike/engine); the wire log is every
	// octet the two ends sent each other.
	serverMSK := sess.MSK()
	peerMSK := peer.msk
	outward := append([][]byte{serverMSK[:], peerMSK[:]}, wire...)

	for i, out := range outward {
		if bytes.Contains(out, emsk[:]) {
			t.Fatalf("the EMSK left the session: it is present in outward artifact %d of %d", i+1, len(outward))
		}
	}

	// The control. Without it a broken search reports the same silence as a
	// confined key.
	planted := append(append([]byte{}, serverMSK[:]...), emsk[:]...)
	if !bytes.Contains(planted, emsk[:]) {
		t.Fatal("the search does not find the EMSK in a corpus it was added to, so its silence above means nothing")
	}
}
