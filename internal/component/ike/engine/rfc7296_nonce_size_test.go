// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- nonce size against the negotiated PRF
// Related: rfc7296_rekey_payloads_test.go -- pfsRekeyRequest, the captured Child SA rekey request

package engine

import (
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// nszLargestPRFHalf returns the largest half key size, in octets rounded up, over every PRF
// this build can negotiate, read from the crypto registry so a PRF added later is covered
// without editing this test.
func nszLargestPRFHalf(t *testing.T) (string, int) {
	t.Helper()
	names := ikecrypto.SupportedPRFNames()
	if len(names) == 0 {
		t.Fatal("the crypto registry lists no PRF, so no bound can be checked")
	}
	var largestName string
	largestHalf := 0
	for _, name := range names {
		prf, err := ikecrypto.LookupPRF(name)
		if err != nil {
			t.Fatalf("LookupPRF(%q): %v", name, err)
		}
		half := (int(prf.KeyLength) + 1) / 2
		if half > largestHalf {
			largestName, largestHalf = name, half
		}
	}
	return largestName, largestHalf
}

// nszEmittedNonces returns the nonce octets ze emits in the three places this file reads:
// the initiator's and the responder's IKE_SA_INIT nonce, and the Ni of a Child SA rekey
// request as the peer decrypts it off the wire.
func nszEmittedNonces(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "nonce-size-psk")
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	out["IKE_SA_INIT initiator Ni"] = ini.LocalNonce
	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	out["IKE_SA_INIT responder Nr"] = resp.LocalNonce

	_, peer, _, _, raw := pfsRekeyRequest(t, ipsec.PFSEnable)
	for _, pe := range lcyDecrypt(t, peer, raw) {
		if n, ok := pe.Payload.(*wire.PayloadNonce); ok {
			out["CREATE_CHILD_SA rekey Ni"] = n.NonceData
		}
	}
	if _, ok := out["CREATE_CHILD_SA rekey Ni"]; !ok {
		t.Fatal("the captured rekey request carries no Ni payload")
	}
	return out
}

// VALIDATES: every nonce ze emits is at least half the key size of each PRF it can negotiate
// and at least 128 bits, so no negotiated PRF can meet a nonce below its bound.
// PREVENTS: a nonce sized against the 16-octet wire minimum, which falls short of half the
// key of a PRF with a key longer than 32 octets.
//
// RFC requirement: RFC7296-2.10-3 positive -- RFC 7296 Section 2.10: nonces "MUST be at least
// half the key size of the negotiated pseudorandom function (PRF)". For every PRF the crypto
// registry lists (SupportedPRFNames), the initiator's and responder's IKE_SA_INIT nonces and
// the Ni of a real Child SA rekey request are each at least half that PRF's key length.
func TestRFC7296EmittedNoncesMeetHalfOfEveryPRFKey(t *testing.T) {
	nonces := nszEmittedNonces(t)
	for _, name := range ikecrypto.SupportedPRFNames() {
		prf, err := ikecrypto.LookupPRF(name)
		if err != nil {
			t.Fatalf("LookupPRF(%q): %v", name, err)
		}
		half := (int(prf.KeyLength) + 1) / 2
		for where, nonce := range nonces {
			if len(nonce) < half {
				t.Errorf("%s is %d octets, below half the %d-octet key of PRF %s",
					where, len(nonce), prf.KeyLength, name)
			}
		}
	}
}

// RFC requirement: RFC7296-2.10-3 negative -- the producer's input pushed to the violating
// side: the PRF with the LARGEST key in the registry is the one a short nonce would violate
// first (a nonce at the 16-octet wire minimum is below half of it whenever that key exceeds
// 32 octets). Every nonce ze emits still meets half of that largest key, and the largest half
// is above 16 octets, so this case separates a nonce sized to the PRF from one sized to the
// wire minimum.
func TestRFC7296EmittedNoncesMeetTheLargestPRFBound(t *testing.T) {
	name, half := nszLargestPRFHalf(t)
	if half <= 16 {
		t.Fatalf("the largest PRF %s needs only %d octets, so a 16-octet nonce would pass and this "+
			"case discriminates nothing", name, half)
	}
	for where, nonce := range nszEmittedNonces(t) {
		if len(nonce) < half {
			t.Errorf("%s is %d octets, below half the key of the largest PRF %s (%d octets)",
				where, len(nonce), name, half)
		}
	}
}
