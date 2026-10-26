// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- Child SA key derivation
// Related: keys.go -- DeriveChildSAKeys and encKeyMaterialLen
// Related: ../dataplane/rfc4106_gcm_install_linux_test.go -- the state the key is installed in
//
// VALIDATES: the KEYMAT an AES-GCM-ESP Child SA takes from prf+ (RFC 7296 Section 2.17):
// each direction's key carries the AES key followed by a four-octet salt, cut in order
// from the prf+ stream, so the two directions of one Child SA get different salts.
// PREVENTS: an AES-GCM key installed without its salt, and two SAs sharing a salt.
package crypto

import (
	"bytes"
	"testing"
)

// rfc4106ChildKeys derives one Child SA's keys for enc with integrity NONE, and answers the
// prf+ stream they were cut from.
func rfc4106ChildKeys(t *testing.T, enc EncryptionTransform, integ IntegrityTransform) (*ChildSAKeys, []byte) {
	t.Helper()
	skD := bytes.Repeat([]byte{0x3c}, 32)
	ni := []byte("rfc4106-nonce-initiator")
	nr := []byte("rfc4106-nonce-responder")
	keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys(%+v): %v", enc, err)
	}
	total := 2*len(keys.EncryptKeyI) + 2*int(integ.KeyLength)
	stream, err := PRFPlus(PRF_HMAC_SHA2_256, skD, append(append([]byte(nil), ni...), nr...), total)
	if err != nil {
		t.Fatalf("PRFPlus: %v", err)
	}
	return keys, stream
}

// TestRFC4106KeymatCarriesTheSalt checks the KEYMAT size of an AES-GCM-ESP key.
//
// RFC 4106 Section 8.1: "The size of the KEYMAT for the AES-GCM-ESP MUST be four octets
// longer than is needed for the associated AES key."
func TestRFC4106KeymatCarriesTheSalt(t *testing.T) {
	none := IntegrityTransform{ID: AUTH_NONE}
	// RFC requirement: RFC4106-8.1-1 positive -- an ENCR_AES_GCM_16 Child SA with a 128-bit
	// and a 256-bit key takes 20 and 36 octets of KEYMAT per direction, and each key is
	// the next run of the prf+ stream: the initiator's first, the responder's after it.
	for bits, octets := range map[uint16]int{128: 20, 256: 36} {
		keys, stream := rfc4106ChildKeys(t, NewEncryptionTransform(ENCR_AES_GCM_16, bits), none)
		if len(keys.EncryptKeyI) != octets || len(keys.EncryptKeyR) != octets {
			t.Fatalf("AES-GCM %d: keys are %d and %d octets, want %d (the AES key and four octets)",
				bits, len(keys.EncryptKeyI), len(keys.EncryptKeyR), octets)
		}
		if !bytes.Equal(keys.EncryptKeyI, stream[:octets]) || !bytes.Equal(keys.EncryptKeyR, stream[octets:2*octets]) {
			t.Errorf("AES-GCM %d: the keys are not consecutive runs of the prf+ stream", bits)
		}
	}
	// RFC requirement: RFC4106-8.1-1 negative -- an ENCR_AES_CBC Child SA with the same key
	// sizes takes 16 and 32 octets per direction, so the four extra octets belong to
	// AES-GCM.
	integ, err := LookupESPIntegrity("sha256")
	if err != nil {
		t.Fatalf("LookupESPIntegrity(sha256): %v", err)
	}
	for bits, octets := range map[uint16]int{128: 16, 256: 32} {
		keys, _ := rfc4106ChildKeys(t, NewEncryptionTransform(ENCR_AES_CBC, bits), integ)
		if len(keys.EncryptKeyI) != octets || len(keys.EncryptKeyR) != octets {
			t.Errorf("AES-CBC %d: keys are %d and %d octets, want %d",
				bits, len(keys.EncryptKeyI), len(keys.EncryptKeyR), octets)
		}
	}
}

// TestRFC4106DirectionsTakeDifferentSalts checks that the two SAs of one AES-GCM Child SA
// carry different salts.
//
// RFC 4106 Section 10: "Thus, to avoid counter block collisions, ESP implementations that
// permit use of the same key for encrypting and decrypting packets with the same peer MUST
// ensure that the two peers assign different salt values to the security association
// (SA)."
func TestRFC4106DirectionsTakeDifferentSalts(t *testing.T) {
	// RFC requirement: RFC4106-10-1 positive -- for a 128-bit and a 256-bit ENCR_AES_GCM_16
	// Child SA, the salt (the last four octets) of the initiator's key is the run of the
	// prf+ stream after its AES key, the responder's salt is the run after the responder's
	// AES key, and the two salts differ, as do the two AES keys.
	for bits, keyOctets := range map[uint16]int{128: 16, 256: 32} {
		keys, stream := rfc4106ChildKeys(t, NewEncryptionTransform(ENCR_AES_GCM_16, bits), IntegrityTransform{ID: AUTH_NONE})
		whole := keyOctets + 4
		saltI := keys.EncryptKeyI[keyOctets:]
		saltR := keys.EncryptKeyR[keyOctets:]
		if !bytes.Equal(saltI, stream[keyOctets:whole]) || !bytes.Equal(saltR, stream[whole+keyOctets:2*whole]) {
			t.Fatalf("AES-GCM %d: the salts are not the prf+ runs after each AES key", bits)
		}
		if bytes.Equal(saltI, saltR) {
			t.Errorf("AES-GCM %d: both directions carry the salt %x", bits, saltI)
		}
		if bytes.Equal(keys.EncryptKeyI[:keyOctets], keys.EncryptKeyR[:keyOctets]) {
			t.Errorf("AES-GCM %d: both directions carry the same AES key", bits)
		}
	}
}
