// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- nonce size against the negotiated PRF
// Related: rfc7296_rekey_payloads_test.go -- pfsRekeyRequest, the captured Child SA rekey request

package engine

import (
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
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

// nszNonceOf returns the Nonce payload of a decrypted message, failing the test when the
// message carries none.
func nszNonceOf(t *testing.T, where string, inner []wire.PayloadEntry) []byte {
	t.Helper()
	for _, pe := range inner {
		if n, ok := pe.Payload.(*wire.PayloadNonce); ok {
			return n.NonceData
		}
	}
	t.Fatalf("the %s message carries no Nonce payload", where)
	return nil
}

// nszIKERekey establishes a session, rekeys its IKE SA with group as both the offer and
// the responder's policy, and returns the Ni and Nr each side decrypts off the wire, with
// the key length in octets of the PRF the replacement SA negotiated.
func nszIKERekey(t *testing.T, group ipsec.IKEGroup) (ni, nr []byte, prfKeyOctets int) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	req, pending, err := initiateIKERekey(ini, group)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	t.Cleanup(pending.clear)
	reqInner := lcyDecrypt(t, resp, req)
	resp.IKEGroup = group
	answer, replacement, err := respondIKERekey(resp, reqInner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	ni = nszNonceOf(t, "IKE SA rekey request", reqInner)
	nr = nszNonceOf(t, "IKE SA rekey response", lcyDecrypt(t, ini, answer))
	return ni, nr, int(replacement.Proposal.PRF.KeyLength)
}

// nszChildRekeyNr answers a real Child SA rekey request from the responder side and
// returns the Nr the initiator decrypts off the wire.
func nszChildRekeyNr(t *testing.T) []byte {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, peer, _, pending, raw := pfsRekeyRequest(t, ipsec.PFSEnable)
	t.Cleanup(pending.clear)
	// The responder answers from the same PFS esp-group the initiator offered.
	peer.ESPGroup = ini.ESPGroup
	peerChild, err := createFirstChildSA(peer, peer.ESPGroup, "10.0.0.2", "10.0.0.1", 1, &mockDP{}, log)
	if err != nil {
		t.Fatalf("createFirstChildSA on the responder: %v", err)
	}
	answer, _, err := respondChildRekey(peer, lcyDecrypt(t, peer, raw), peerChild,
		parseMsg(t, raw).Header.MessageID, &mockDP{}, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	return nszNonceOf(t, "CREATE_CHILD_SA rekey response", lcyDecrypt(t, ini, answer))
}

// nszEmittedNonces returns the nonce octets ze emits in every place this file reads: the
// initiator's and the responder's IKE_SA_INIT nonce, the Ni and the Nr of a Child SA rekey,
// and the Ni and the Nr of an IKE SA rekey, each rekey nonce as the other side decrypts it
// off the wire.
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

	_, peer, _, pending, raw := pfsRekeyRequest(t, ipsec.PFSEnable)
	t.Cleanup(pending.clear)
	out["CREATE_CHILD_SA rekey Ni"] = nszNonceOf(t, "CREATE_CHILD_SA rekey request", lcyDecrypt(t, peer, raw))
	out["CREATE_CHILD_SA rekey Nr"] = nszChildRekeyNr(t)
	out["IKE SA rekey Ni"], out["IKE SA rekey Nr"], _ = nszIKERekey(t, testIKEGroup())
	return out
}

// VALIDATES: every nonce ze emits is at least half the key size of each PRF it can negotiate
// and at least 128 bits, so no negotiated PRF can meet a nonce below its bound.
// PREVENTS: a nonce sized against the 16-octet wire minimum, which falls short of half the
// key of a PRF with a key longer than 32 octets.
//
// RFC requirement: RFC7296-2.10-3 positive -- RFC 7296 Section 2.10: nonces "MUST be at least
// half the key size of the negotiated pseudorandom function (PRF)". For every PRF the crypto
// registry lists (SupportedPRFNames), the initiator's and responder's IKE_SA_INIT nonces, the
// Ni and the Nr of a real Child SA rekey and the Ni and the Nr of a real IKE SA rekey, the
// rekey nonces read off the wire, are each at least half that PRF's key length.
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
// side: an IKE SA rekey that NEGOTIATES the PRF with the largest key this build offers
// (prf-hmac-sha2-512, a 64-octet key), the PRF a nonce at the 16-octet wire minimum violates.
// The replacement SA is checked to carry that PRF and a half above 16 octets, and the Ni and
// Nr of that exchange, decrypted off the wire, each meet half of the negotiated PRF's key.
func TestRFC7296EmittedNoncesMeetTheLargestPRFBound(t *testing.T) {
	name, half := nszLargestPRFHalf(t)
	if half <= 16 {
		t.Fatalf("the largest PRF %s needs only %d octets, so a 16-octet nonce would pass and this "+
			"case discriminates nothing", name, half)
	}
	group := testIKEGroup()
	group.Proposals[0].Hash = ipsec.HashSHA512
	ni, nr, keyOctets := nszIKERekey(t, group)
	if negotiated := (keyOctets + 1) / 2; negotiated != half {
		t.Fatalf("the rekey negotiated a PRF with a %d-octet key, not the largest %s, so this case "+
			"does not push the input to the violating side", keyOctets, name)
	}
	for where, nonce := range map[string][]byte{"IKE SA rekey Ni": ni, "IKE SA rekey Nr": nr} {
		if len(nonce) < half {
			t.Errorf("%s is %d octets, below half the %d-octet key of the negotiated PRF %s",
				where, len(nonce), keyOctets, name)
		}
	}
}
