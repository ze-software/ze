// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- Child SA key derivation
// Related: keys.go -- DeriveChildSAKeys, the producer
// RFC: rfc/short/rfc4301.md -- key splitting rule (Section 4.5.2)
package crypto

import (
	"bytes"
	"testing"
)

// rfc4301Keymat derives one Child SA key set and the raw KEYMAT string it was cut
// from. The encryption key is 16 octets and the integrity key 32, so the two slices
// of one direction can never be confused by length alone.
func rfc4301Keymat(t *testing.T) (*ChildSAKeys, []byte, int, int) {
	t.Helper()
	skD := make([]byte, 32)
	for i := range skD {
		skD[i] = byte(0xA0 + i)
	}
	ni := []byte("rfc4301-nonce-i!")
	nr := []byte("rfc4301-nonce-r!")
	enc := EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128}
	integ := IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16}
	encLen, integLen := 16, 32

	keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys: %v", err)
	}
	t.Cleanup(keys.Clear)

	seed := append(append([]byte(nil), ni...), nr...)
	keymat, err := PRFPlus(PRF_HMAC_SHA2_256, skD, seed, 2*(encLen+integLen))
	if err != nil {
		t.Fatalf("PRFPlus: %v", err)
	}
	return keys, keymat, encLen, integLen
}

// VALIDATES: RFC4301-4.5.2-1. Within the keying material of each SA, the encryption
// key is the leading (left-most, high-order) slice and the integrity key is the slice
// that follows it, for the initiator direction and then for the responder direction.
// PREVENTS: a split that reads the integrity key first, which a peer that follows the
// rule would decrypt with the wrong key and detect only as an ICV failure.
// RFC requirement: RFC4301-4.5.2-1 positive -- each SA's encryption key is the leading bits of its keying material and its integrity key the remaining bits.
func TestRFC4301KeymatEncryptionKeysLeadEachSA(t *testing.T) {
	keys, keymat, encLen, integLen := rfc4301Keymat(t)

	off := 0
	if !bytes.Equal(keys.EncryptKeyI, keymat[off:off+encLen]) {
		t.Fatalf("EncryptKeyI is not the leading %d octets of the initiator SA material", encLen)
	}
	off += encLen
	if !bytes.Equal(keys.IntegKeyI, keymat[off:off+integLen]) {
		t.Fatalf("IntegKeyI is not the %d octets that follow the initiator encryption key", integLen)
	}
	off += integLen
	if !bytes.Equal(keys.EncryptKeyR, keymat[off:off+encLen]) {
		t.Fatalf("EncryptKeyR is not the leading %d octets of the responder SA material", encLen)
	}
	off += encLen
	if !bytes.Equal(keys.IntegKeyR, keymat[off:off+integLen]) {
		t.Fatalf("IntegKeyR is not the %d octets that follow the responder encryption key", integLen)
	}
}

// VALIDATES: RFC4301-4.5.2-1. The reversed split never appears: no integrity key is
// cut from the leading bits of its SA's material, and no encryption key is cut from
// the bits that follow an integrity key.
// PREVENTS: a swap of the two keys passing unnoticed because both slices are the
// right length and both come from the same PRF+ output.
// RFC requirement: RFC4301-4.5.2-1 negative -- an integrity key is never taken from the leading bits of its SA's keying material.
func TestRFC4301KeymatIntegrityKeysNeverLead(t *testing.T) {
	keys, keymat, encLen, integLen := rfc4301Keymat(t)

	perSA := encLen + integLen
	if bytes.Equal(keys.IntegKeyI, keymat[0:integLen]) {
		t.Fatal("IntegKeyI was cut from the leading bits of the initiator SA material")
	}
	if bytes.Equal(keys.EncryptKeyI, keymat[integLen:integLen+encLen]) {
		t.Fatal("EncryptKeyI was cut from the bits after the integrity key")
	}
	if bytes.Equal(keys.IntegKeyR, keymat[perSA:perSA+integLen]) {
		t.Fatal("IntegKeyR was cut from the leading bits of the responder SA material")
	}
	if bytes.Equal(keys.EncryptKeyR, keymat[perSA+integLen:perSA+integLen+encLen]) {
		t.Fatal("EncryptKeyR was cut from the bits after the integrity key")
	}
}
