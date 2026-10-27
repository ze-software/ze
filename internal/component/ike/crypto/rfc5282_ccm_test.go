// VALIDATES: the ICV lengths an AES CCM implementation may support (RFC 5282 §3.2), over
// the table that turns a wire Transform ID into an ICV length and over the seal and open
// pair the engine drives.
// PREVENTS: a forbidden AES CCM ICV length reaching an IKE Encrypted payload.
package crypto

import (
	"bytes"
	"crypto/aes"
	"errors"
	"testing"
)

// ccmKeyWithSalt is one AES CCM 256 key material block: a 32 octet cipher key followed by
// the three octet salt of RFC 4309 Section 7.1, which RFC 5282 Section 7.1 carries into
// the IKE SA.
func ccmKeyWithSalt() []byte {
	material := make([]byte, 35)
	for i := range material {
		material[i] = byte(i + 1)
	}
	return material
}

// RFC requirement: RFC5282-3.2-4 positive -- each AES CCM Transform ID returns and
// emits its assigned 8, 12 or 16 octet ICV, and the IKE opener refuses valid CCM
// ciphertexts carrying the primitive's other ICV sizes (4, 6, 10 and 14).
// This constrains AES CCM, not the population of unrelated AEAD transforms.
//
// RFC 5282 Section 3.2: "AES CCM provides an encrypted ICV. Implementations MUST support
// ICV sizes of 8 octets and 16 octets. Implementations MAY also support 12 octet ICVs and
// MUST NOT support other ICV lengths."
//
// RFC 5282 Section 7.2 assigns one identifier per AES CCM ICV size. The earlier
// test pinned the entire AEAD population to four entries; that rejected ESP
// ChaCha20-Poly1305 even though Section 3.2 imposes no constraint on that cipher.
func TestRFC5282CCMRefusesEveryOtherICVLength(t *testing.T) {
	key := ccmKeyWithSalt()
	plaintext := []byte("inner payloads and the pad length")
	aad := bytes.Repeat([]byte{0x11}, 32)
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[EncryptionID]int{
		ENCR_AES_CCM_8:  8,
		ENCR_AES_CCM_12: 12,
		ENCR_AES_CCM_16: 16,
	} {
		got, err := AEADICVOctets(id)
		if err != nil || got != want {
			t.Errorf("AEADICVOctets(%s) = %d, %v; want %d", id, got, err, want)
		}
		sealed, err := SealIKEAEAD(id, key, plaintext, aad)
		if err != nil {
			t.Fatalf("SealIKEAEAD(%s): %v", id, err)
		}
		if got := len(sealed) - ikeIVOctets - len(plaintext); got != want {
			t.Errorf("%s emitted a %d octet ICV, want %d", id, got, want)
		}
		for _, forbidden := range []int{4, 6, 10, 14} {
			// The CCM primitive permits these sizes (RFC 3610 Section 2), but
			// IKE's AES CCM profile does not. Construct authentic ciphertexts,
			// not merely corrupt or truncated tags.
			mode, err := newAESCCM(block, forbidden, 11)
			if err != nil {
				t.Fatal(err)
			}
			iv := make([]byte, ikeIVOctets)
			iv[0], iv[1] = byte(id), byte(forbidden)
			ciphertext := mode.Seal(iv, ikeAEADNonce(key[32:], iv), plaintext, aad)
			opened, err := OpenIKEAEAD(id, key, ciphertext, aad)
			if !errors.Is(err, ErrDecryptionFailed) || len(opened) != 0 {
				t.Errorf("%s opened a forbidden %d octet CCM ICV: plaintext=%x err=%v",
					id, forbidden, opened, err)
			}
		}
	}
}

// RFC requirement: RFC5282-3.2-4 negative -- the refusal above is specific to the lengths
// RFC 5282 Section 3.2 forbids rather than a blanket one. Each of the three AES CCM
// identifiers seals a message whose ciphertext is longer than its plaintext by exactly the
// ICV that identifier names, and opens it back to the same plaintext.
//
// Successful opening is the control for the forbidden-tag refusals: an opener
// that rejects every ciphertext must not satisfy the pair.
func TestRFC5282CCMSupportsTheThreePermittedICVLengths(t *testing.T) {
	key := ccmKeyWithSalt()
	plaintext := []byte("inner payloads and the pad length")
	aad := bytes.Repeat([]byte{0x11}, 32)

	for id, icvOctets := range map[EncryptionID]int{
		ENCR_AES_CCM_8:  8,
		ENCR_AES_CCM_12: 12,
		ENCR_AES_CCM_16: 16,
	} {
		data, err := SealIKEAEAD(id, key, plaintext, aad)
		if err != nil {
			t.Fatalf("SealIKEAEAD(%s): %v", id, err)
		}
		// The payload data is the eight octet IV of Section 3.1, then the ciphertext
		// with its ICV, so the expansion beyond the plaintext is IV plus ICV.
		if got := len(data) - ikeIVOctets - len(plaintext); got != icvOctets {
			t.Errorf("%s sealed a %d octet ICV, want %d", id, got, icvOctets)
		}
		opened, err := OpenIKEAEAD(id, key, data, aad)
		if err != nil {
			t.Fatalf("OpenIKEAEAD(%s): %v", id, err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Errorf("%s opened %q, want %q", id, opened, plaintext)
		}
	}
}
