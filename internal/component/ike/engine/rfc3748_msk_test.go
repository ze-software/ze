// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP inside IKE_AUTH
// RFC: rfc/short/rfc3748.md -- the Section 7.10 keying rows
//
// RFC 3748 Section 7.10 bounds what an EAP method's exported key may be used
// for. Ze exports the MSK and hands it to the IKEv2 AUTH construction (RFC 7296
// Section 2.16), and this file proves that the keys which protect user data do
// not follow it: the ESP keys ze installs into the dataplane come from SK_d
// through prf+, and two exchanges differing only in their MSK install the same
// ones.
//
// VALIDATES: two EAP-MSCHAPv2 exchanges that export different MSKs install
// byte-identical ESP encryption and integrity keys, and no installed key is a
// window of either MSK, while the AUTH secret of each SA is that SA's own MSK.
// PREVENTS: an MSK, or a slice of one, reaching a kernel SA as the key that
// encrypts or authenticates user data.

package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// mskExchangeRounds bounds the EAP-MSCHAPv2 conversation below. Identity,
// Challenge, MS-CHAPv2 Success and EAP-Success are four rounds; the bound is
// there so a method that never concludes ends the test rather than the machine.
const mskExchangeRounds = 8

// succeededMSCHAPv2 runs one complete EAP-MSCHAPv2 conversation between ze's
// authenticator and ze's peer, and returns the authenticator, which holds the
// MSK that conversation exported.
func succeededMSCHAPv2(t *testing.T, password string) *eap.Session {
	t.Helper()

	auth, err := eap.NewSession(eap.TypeMSCHAPv2, eap.MethodConfig{Password: password})
	if err != nil {
		t.Fatalf("stand up the EAP-MSCHAPv2 authenticator: %v", err)
	}
	t.Cleanup(auth.Close)

	peer := eap.NewPeerSession(eap.TypeMSCHAPv2, "user", password)
	t.Cleanup(peer.Close)

	packet := auth.Begin()
	for range mskExchangeRounds {
		res := peer.Process(packet)
		if res.Err != nil {
			t.Fatalf("the peer refused the exchange: %v", res.Err)
		}
		if res.Done {
			break
		}
		if res.Response == nil {
			t.Fatalf("the peer answered %+v with no Response", packet)
		}
		if packet = auth.Process(res.Response); packet == nil {
			t.Fatal("the authenticator answered the peer with no packet at all")
		}
	}
	if !auth.Succeeded() {
		t.Fatalf("the EAP-MSCHAPv2 exchange did not succeed: %v", auth.Err())
	}
	return auth
}

// mskKeyedSA parks one succeeded EAP exchange on an IKE SA whose every other key
// input is fixed, so two calls differ in the MSK and in nothing else.
func mskKeyedSA(sess *eap.Session) *SA {
	sa := testSA()
	sa.LocalNonce = bytes.Repeat([]byte{0x11}, 32)
	sa.RemoteNonce = bytes.Repeat([]byte{0x22}, 32)
	sa.SKKeys = &crypto.SKKeys{SK_d: bytes.Repeat([]byte{0x33}, 32)}
	sa.EAPSession = sess
	sa.EAPMSK = sess.MSK()
	return sa
}

// mskInstalledSAs drives the production Child SA installer and returns every
// SAParams that reached a dataplane backend, which carry the keys that protect
// user data.
func mskInstalledSAs(t *testing.T, sa *SA) []dataplane.SAParams {
	t.Helper()

	dp := &mockDP{}
	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.sas) == 0 {
		t.Fatal("the Child SA installer programmed no SA, so no key that protects data was measured")
	}
	return dp.sas
}

// TestRFC3748TheEAPMSKNeverKeysTheDataItProtects installs the Child SA of two
// EAP exchanges whose MSKs differ and compares the keys that reach the
// dataplane, then asks each SA what its AUTH payload is keyed by.
func TestRFC3748TheEAPMSKNeverKeysTheDataItProtects(t *testing.T) {
	first := mskKeyedSA(succeededMSCHAPv2(t, "first-secret"))
	second := mskKeyedSA(succeededMSCHAPv2(t, "second-secret"))

	var zero [64]byte
	if first.EAPMSK == zero || second.EAPMSK == zero {
		t.Fatal("an exchange exported an all-zero MSK, so there is no key here to trace")
	}
	if first.EAPMSK == second.EAPMSK {
		t.Fatal("the two exchanges exported the same MSK, so nothing below could tell the two apart")
	}

	// RFC requirement: RFC3748-7.10-5 positive -- RFC 3748 Section 7.10: "The MSK
	// and EMSK MUST NOT be used directly to protect data; however, they are of
	// sufficient size to enable derivation of a AAA-Key subsequently used to derive
	// Transient Session Keys (TSKs) for use with the selected ciphersuite." The
	// keys that protect data on ze are the ESP keys it installs into the
	// dataplane. The two SAs below carry different MSKs and the same SK_d and
	// nonces, and every key installed is byte for byte the same across them, so
	// none of them follows the MSK. No installed key is a window of either MSK
	// either, so no part of one is being installed verbatim.
	firstSAs := mskInstalledSAs(t, first)
	secondSAs := mskInstalledSAs(t, second)
	if len(firstSAs) != len(secondSAs) {
		t.Fatalf("the two exchanges installed %d and %d SAs", len(firstSAs), len(secondSAs))
	}
	for i := range firstSAs {
		if !bytes.Equal(firstSAs[i].EncKey, secondSAs[i].EncKey) {
			t.Fatalf("SA %d: the installed encryption key follows the MSK", i)
		}
		if !bytes.Equal(firstSAs[i].AuthKey, secondSAs[i].AuthKey) {
			t.Fatalf("SA %d: the installed integrity key follows the MSK", i)
		}
		if len(firstSAs[i].EncKey) == 0 || len(firstSAs[i].AuthKey) == 0 {
			t.Fatalf("SA %d: an installed key is empty, so the comparison above compared nothing", i)
		}
		for _, msk := range [][64]byte{first.EAPMSK, second.EAPMSK} {
			if bytes.Contains(msk[:], firstSAs[i].EncKey) {
				t.Fatalf("SA %d: the installed encryption key is a window of an EAP MSK", i)
			}
			if bytes.Contains(msk[:], firstSAs[i].AuthKey) {
				t.Fatalf("SA %d: the installed integrity key is a window of an EAP MSK", i)
			}
		}
	}

	// RFC requirement: RFC3748-7.10-5 negative -- the MSK is carried by these SAs
	// and read where RFC 7296 Section 2.16 keys the AUTH payload with it: "For EAP
	// methods that create a shared key as a side effect of authentication, that
	// shared key MUST be used by both the initiator and responder to generate AUTH
	// payloads in messages 7 and 8." Each SA's AUTH secret is its own MSK, and the
	// two differ, so the installed keys above being equal is the derivation not
	// reading the MSK rather than the MSK being absent or all zero.
	firstSecret, err := eapAuthSecret(first, true)
	if err != nil {
		t.Fatalf("the first SA has no AUTH secret: %v", err)
	}
	secondSecret, err := eapAuthSecret(second, true)
	if err != nil {
		t.Fatalf("the second SA has no AUTH secret: %v", err)
	}
	if !bytes.Equal(firstSecret, first.EAPMSK[:]) {
		t.Fatal("the first SA keys its AUTH payload with something other than its EAP MSK")
	}
	if !bytes.Equal(secondSecret, second.EAPMSK[:]) {
		t.Fatal("the second SA keys its AUTH payload with something other than its EAP MSK")
	}
	if bytes.Equal(firstSecret, secondSecret) {
		t.Fatal("the two SAs key their AUTH payloads with the same octets, so the AUTH secret does not follow the MSK")
	}
}
