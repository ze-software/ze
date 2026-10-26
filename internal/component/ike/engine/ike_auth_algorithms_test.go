// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKE_AUTH authentication methods
// Related: auth.go -- computePSKAuth, signDigest, verifySignature, hashFromAlgID
// Related: initiator.go -- espProposalWire, buildSignatureHashAlgosNotify
// Related: rfc4555_mobike_offer_test.go -- rfc4555OfferSA and rfc4555Inner
//
// VALIDATES: the authentication and proposal obligations RFC 8247, RFC 7634 and RFC 5998
// place on Ze's own IKE_AUTH and Child SA proposal code: the shared-key AUTH value, RSA
// signatures at 1024 and 2048 bits, the SHA-2 hash list and the SHA-1 signature
// identifiers Ze refuses, the ChaCha20-Poly1305 proposal shape, and the absence of the
// EAP-only notification from the IKE_AUTH request.
// PREVENTS: a shared-key AUTH computed with another pad or PRF, a SHA-1 signature
// accepted, an INTEG transform offered beside an AEAD cipher, and EAP_ONLY_AUTHENTICATION
// sent by an implementation that does not implement RFC 5998.
package engine

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/binary"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestRFC8247SharedKeyAuth checks the Shared Key Message Integrity Code method.
//
// RFC 8247 Section 3.1: "Shared Key Message Integrity Code is widely deployed and
// mandatory to implement in the IKEv2 in RFC 7296. The status remains MUST."
func TestRFC8247SharedKeyAuth(t *testing.T) {
	psk := []byte("a shared secret")
	signed := []byte("the octets RFC 7296 Section 2.15 signs")
	// RFC requirement: RFC8247-3.1-2 positive -- the AUTH value Ze computes for a shared key
	// is prf(prf(Shared Secret, "Key Pad for IKEv2"), signed octets) with PRF_HMAC_SHA2_256,
	// and Ze's verifier accepts it.
	got, err := computeAuthFromSharedSecret(ikecrypto.PRF_HMAC_SHA2_256, psk, signed)
	if err != nil {
		t.Fatalf("computeAuthFromSharedSecret: %v", err)
	}
	inner := hmac.New(sha256.New, psk)
	inner.Write([]byte("Key Pad for IKEv2"))
	outer := hmac.New(sha256.New, inner.Sum(nil))
	outer.Write(signed)
	if want := outer.Sum(nil); !bytes.Equal(got, want) {
		t.Fatalf("shared-key AUTH = %x, want %x", got, want)
	}
	if err := verifyAuthFromSharedSecret(ikecrypto.PRF_HMAC_SHA2_256, psk, signed, got); err != nil {
		t.Errorf("verifyAuthFromSharedSecret refused the AUTH it computed: %v", err)
	}
	// RFC requirement: RFC8247-3.1-2 negative -- the verifier refuses the AUTH computed
	// under another shared secret.
	other, err := computeAuthFromSharedSecret(ikecrypto.PRF_HMAC_SHA2_256, []byte("another secret"), signed)
	if err != nil {
		t.Fatalf("computeAuthFromSharedSecret: %v", err)
	}
	if err := verifyAuthFromSharedSecret(ikecrypto.PRF_HMAC_SHA2_256, psk, signed, other); err == nil {
		t.Error("verifyAuthFromSharedSecret accepted an AUTH computed under another shared secret")
	}
}

// TestRFC8247RSAKeySizes checks RSA signatures with the two key sizes RFC 7296 mandates.
//
// RFC 8247 Section 3.1.1: "IKEv2 [RFC7296] mandates support for the RSA keys of the bit
// size 1024 or 2048"
func TestRFC8247RSAKeySizes(t *testing.T) {
	digest := sha256.Sum256([]byte("the octets the AUTH payload signs"))
	for _, bits := range []int{1024, 2048} {
		key, err := rsa.GenerateKey(crand.Reader, bits)
		if err != nil {
			t.Fatalf("rsa.GenerateKey(%d): %v", bits, err)
		}
		// RFC requirement: RFC8247-3.1.1-1 positive -- signDigest signs a SHA2-256 digest with
		// a 1024-bit and a 2048-bit RSA key, and verifySignature accepts each signature under
		// the matching public key.
		sig, err := signDigest(key, digest[:], crypto.SHA256)
		if err != nil {
			t.Fatalf("signDigest with a %d-bit key: %v", bits, err)
		}
		if err := verifySignature(&key.PublicKey, digest[:], sig, sha256.New); err != nil {
			t.Errorf("verifySignature refused a %d-bit RSA signature: %v", bits, err)
		}
		// RFC requirement: RFC8247-3.1.1-1 negative -- verifySignature refuses the same
		// signature under another public key of the same size.
		other, err := rsa.GenerateKey(crand.Reader, bits)
		if err != nil {
			t.Fatalf("rsa.GenerateKey(%d): %v", bits, err)
		}
		if err := verifySignature(&other.PublicKey, digest[:], sig, sha256.New); err == nil {
			t.Errorf("verifySignature accepted a %d-bit signature under another key", bits)
		}
	}
}

// rfc8247AlgID answers the DER AlgorithmIdentifier SEQUENCE { OID } for oid.
func rfc8247AlgID(t *testing.T, oid asn1.ObjectIdentifier) []byte {
	t.Helper()
	der, err := asn1.Marshal(struct{ Algorithm asn1.ObjectIdentifier }{oid})
	if err != nil {
		t.Fatalf("asn1.Marshal(%v): %v", oid, err)
	}
	return der
}

// TestRFC8247SignatureHashes checks the hash functions and the signature algorithm
// identifiers RFC 8247 Section 3.2 grades for the Digital Signature method.
//
// RFC 8247 Section 3.2 grades the hash functions (SHA1 MUST NOT, SHA2-256 MUST) under
// "When a Digital Signature authentication method is implemented, the following
// recommendations are applied for hash functions:" and the signature algorithms
// (sha1WithRSAEncryption, dsa-with-sha1, ecdsa-with-sha1 and RSASSA-PSS with default
// parameters MUST NOT) under "These recommendations are applied only if the Digital
// Signature authentication method is implemented."
func TestRFC8247SignatureHashes(t *testing.T) {
	notify := buildSignatureHashAlgosNotify()
	var announced []uint16
	for i := 0; i+1 < len(notify.NotificationData); i += 2 {
		announced = append(announced, binary.BigEndian.Uint16(notify.NotificationData[i:]))
	}
	// RFC requirement: RFC8247-3.2-1 positive -- the SIGNATURE_HASH_ALGORITHMS notify Ze
	// sends lists SHA2-256 (2), and hashFromAlgID answers SHA-256 for sha256WithRSAEncryption.
	if !bytes.Contains(notify.NotificationData, []byte{0, 2}) {
		t.Errorf("the SIGNATURE_HASH_ALGORITHMS notify lists %v, want SHA2-256 (2) among them", announced)
	}
	if h := hashFromAlgID(algIDSHA256WithRSA); h == nil || h().Size() != sha256.Size {
		t.Error("hashFromAlgID does not answer SHA-256 for sha256WithRSAEncryption")
	}
	// RFC requirement: RFC8247-3.2-1 negative -- the same notify does not list SHA1 (1).
	for _, id := range announced {
		if id == 1 {
			t.Errorf("the SIGNATURE_HASH_ALGORITHMS notify lists SHA1: %v", announced)
		}
	}

	// RFC requirement: RFC8247-3.2-3 negative -- hashFromAlgID answers no hash, so
	// verifyX509Auth refuses the AUTH payload, for sha1WithRSAEncryption, dsa-with-sha1,
	// ecdsa-with-sha1 and id-RSASSA-PSS, each as a bare OID and as an AlgorithmIdentifier;
	// and selectSignatureAlgorithm refuses to sign with an RSA key for a peer that offers
	// SHA1 alone.
	for name, oid := range map[string]asn1.ObjectIdentifier{
		"sha1WithRSAEncryption": {1, 2, 840, 113549, 1, 1, 5},
		"dsa-with-sha1":         {1, 2, 840, 10040, 4, 3},
		"ecdsa-with-sha1":       {1, 2, 840, 10045, 4, 1},
		"id-RSASSA-PSS":         {1, 2, 840, 113549, 1, 1, 10},
	} {
		bare, err := asn1.Marshal(oid)
		if err != nil {
			t.Fatalf("asn1.Marshal(%v): %v", oid, err)
		}
		if hashFromAlgID(bare) != nil {
			t.Errorf("hashFromAlgID answered a hash for the bare %s OID", name)
		}
		if hashFromAlgID(rfc8247AlgID(t, oid)) != nil {
			t.Errorf("hashFromAlgID answered a hash for the %s AlgorithmIdentifier", name)
		}
	}
	key, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	if algID, _, err := selectSignatureAlgorithm(key, []uint16{1}); err == nil {
		t.Errorf("selectSignatureAlgorithm signed for a SHA1-only peer with %x", algID)
	}
	// RFC requirement: RFC8247-3.2-3 positive -- the same two functions accept
	// sha256WithRSAEncryption as an AlgorithmIdentifier, and an RSA key signs with it for a
	// peer that offers SHA2-256.
	if hashFromAlgID(rfc8247AlgID(t, asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})) == nil {
		t.Error("hashFromAlgID answered no hash for sha256WithRSAEncryption")
	}
	if _, h, err := selectSignatureAlgorithm(key, []uint16{hashAlgoSHA2256}); err != nil || h != crypto.SHA256 {
		t.Errorf("selectSignatureAlgorithm for a SHA2-256 peer = %v, %v; want SHA-256", h, err)
	}
}

// TestRFC7634ChaChaProposalCarriesNoInteg checks the shape of a ChaCha20-Poly1305 ESP
// proposal.
//
// RFC 7634 Section 4: "As with other AEAD algorithms, INTEG (type 3) transform
// substructures MUST NOT be specified, or just one INTEG transform MAY be included with
// value NONE (0)."
func TestRFC7634ChaChaProposalCarriesNoInteg(t *testing.T) {
	// RFC requirement: RFC7634-4-1 positive -- the ESP proposal Ze sends for
	// chacha20poly1305 carries ENCR_CHACHA20_POLY1305 (28) and no INTEG transform, while
	// the aes128 proposal with sha256 carries one.
	chacha, err := espProposalWire(ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionChaCha20Poly}, 0x1234, 1, dhGroupNone)
	if err != nil {
		t.Fatalf("espProposalWire(chacha20poly1305): %v", err)
	}
	cbc, err := espProposalWire(ipsec.ESPProposal{Number: 1, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA256}, 0x1234, 1, dhGroupNone)
	if err != nil {
		t.Fatalf("espProposalWire(aes128): %v", err)
	}
	count := func(p wire.Proposal, kind uint8) (n int, ids []uint16) {
		for _, tr := range p.Transforms {
			if tr.Type == kind {
				n++
				ids = append(ids, tr.ID)
			}
		}
		return n, ids
	}
	if n, ids := count(chacha, wire.TransformTypeENCR); n != 1 || ids[0] != uint16(ikecrypto.ENCR_CHACHA20_POLY1305) {
		t.Errorf("the chacha20poly1305 proposal carries ENCR %v, want [28]", ids)
	}
	if n, ids := count(chacha, wire.TransformTypeINTG); n != 0 {
		t.Errorf("the chacha20poly1305 proposal carries INTEG %v, want none", ids)
	}
	if n, _ := count(cbc, wire.TransformTypeINTG); n != 1 {
		t.Errorf("the aes128 proposal carries %d INTEG transforms, want 1", n)
	}
	// RFC requirement: RFC7634-4-1 negative -- an accepted answer selecting
	// ENCR_CHACHA20_POLY1305 with INTEG AUTH_HMAC_SHA2_256_128 is refused by
	// crypto.NegotiateESP, while the same answer with INTEG NONE is accepted.
	sent := ikecrypto.ESPProposal{
		Number:     1,
		Encryption: ikecrypto.NewEncryptionTransform(ikecrypto.ENCR_CHACHA20_POLY1305, 0),
		Integrity:  ikecrypto.IntegrityTransform{ID: ikecrypto.AUTH_NONE},
	}
	withInteg := sent
	withInteg.Integrity = ikecrypto.IntegrityTransform{ID: ikecrypto.AUTH_HMAC_SHA2_256_128}
	if got, err := ikecrypto.NegotiateESP([]ikecrypto.ESPProposal{withInteg}, []ikecrypto.ESPProposal{sent}); err == nil {
		t.Errorf("NegotiateESP accepted ChaCha20-Poly1305 with an INTEG transform: %+v", got)
	}
	if _, err := ikecrypto.NegotiateESP([]ikecrypto.ESPProposal{sent}, []ikecrypto.ESPProposal{sent}); err != nil {
		t.Errorf("NegotiateESP refused ChaCha20-Poly1305 with INTEG NONE: %v", err)
	}
}

// rfc5998NotifyEAPOnlyAuthentication is the EAP_ONLY_AUTHENTICATION status type RFC 5998
// Section 6 registers. Ze implements no part of RFC 5998 and names no constant for it.
const rfc5998NotifyEAPOnlyAuthentication uint16 = 16417

// TestRFC5998IKEAuthCarriesNoEAPOnlyNotify checks that Ze's IKE_AUTH request never carries
// the EAP-only extension.
//
// RFC 5998 Section 3: "Therefore, during the IKE_AUTH exchange of the resumed session, this
// extension MUST NOT be sent by the initiator."
func TestRFC5998IKEAuthCarriesNoEAPOnlyNotify(t *testing.T) {
	sa := rfc4555OfferSA(t)
	request, err := buildAuthRequest(sa)
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	inner := rfc4555Inner(t, sa, request)
	if notifyOf(inner, wire.NotifyMobikeSupported) == nil {
		t.Fatal("control: the request carries no MOBIKE_SUPPORTED, so the notify scan reads nothing")
	}
	// RFC requirement: RFC5998-3-5 positive -- the first IKE_AUTH request of an EAP-mode
	// initiator, decrypted as its peer reads it, carries no EAP_ONLY_AUTHENTICATION (16417)
	// notify, while the same scan finds its MOBIKE_SUPPORTED notify.
	if n := notifyOf(inner, rfc5998NotifyEAPOnlyAuthentication); n != nil {
		t.Errorf("the IKE_AUTH request carries EAP_ONLY_AUTHENTICATION: %+v", n)
	}
}
