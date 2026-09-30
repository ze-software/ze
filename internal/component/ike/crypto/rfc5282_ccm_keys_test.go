// VALIDATES: the AES CCM half of the RFC 5282 Section 7.1 key layout: zero-length SK_ai
// and SK_ar, and SK_ei and SK_er sized and ordered as the RFC 4309 Section 7.1 KEYMAT, a
// cipher key followed by three octets of salt.
// PREVENTS: an AES CCM key derivation that reserves integrity key material it never uses,
// or that sizes SK_ei with the AES GCM four octet salt or with none.
package crypto

import (
	"bytes"
	"crypto/aes"
	"testing"

	"github.com/ze-software/ze/internal/core/ccm"
)

// ccmTransformIDs is every AES CCM Transform ID RFC 5282 Section 7.2 assigns, one per ICV
// size, with the ICV each one appends.
var ccmTransformIDs = []struct {
	id        EncryptionID
	icvOctets int
}{
	{ENCR_AES_CCM_8, 8},
	{ENCR_AES_CCM_12, 12},
	{ENCR_AES_CCM_16, 16},
}

// RFC requirement: RFC5282-7.1-1 positive -- with AES CCM, as with AES GCM, SK_ai and
// SK_ar are each zero octets, and the prf+ stream is cut as though they were: SK_ei starts
// at the octet after SK_d. Every CCM Transform ID at both AES key sizes is derived.
//
// RFC 5282 Section 7.1: "When AES GCM or AES CCM is used with the IKEv2 Encrypted Payload,
// the SK_ai and SK_ar integrity protection keys are not used; each key MUST be treated as
// having a size of zero (0) octets."
//
// The offset assertion is what makes the zero length mean something: a derivation that
// answered empty slices while still consuming integrity key material from the stream would
// hand the peer a different SK_ei.
func TestRFC5282CCMIntegrityKeysAreZeroOctets(t *testing.T) {
	for _, tc := range ccmTransformIDs {
		for _, bits := range []uint16{128, 256} {
			enc := NewEncryptionTransform(tc.id, bits)
			keys, stream := aeadSKKeys(t, enc, IntegrityTransform{ID: AUTH_NONE})

			if len(keys.SK_ai) != 0 || len(keys.SK_ar) != 0 {
				t.Fatalf("%s-%d: SK_ai is %d octets and SK_ar is %d octets, want 0 and 0",
					tc.id, bits, len(keys.SK_ai), len(keys.SK_ar))
			}
			const skDOctets = 32
			encOctets := int(bits)/8 + 3
			if !bytes.Equal(keys.SK_ei, stream[skDOctets:skDOctets+encOctets]) {
				t.Fatalf("%s-%d: SK_ei is not the prf+ octets at offset %d, so the "+
					"zero-length integrity keys did not remove their share of the stream",
					tc.id, bits, skDOctets)
			}
			if !bytes.Equal(keys.SK_er, stream[skDOctets+encOctets:skDOctets+2*encOctets]) {
				t.Fatalf("%s-%d: SK_er is not the prf+ octets at offset %d",
					tc.id, bits, skDOctets+encOctets)
			}
		}
	}
}

// RFC requirement: RFC5282-7.1-2 positive -- with AES CCM, SK_ei and SK_er each have the
// size and format of the RFC 4309 KEYMAT for the AES key size in use: the cipher key
// followed by three octets of salt, so 19 octets at 128 bits and 35 at 256. The format is
// proven by USING it: the key seals with SealIKEAEAD, and an AES CCM instance built here
// from the leading octets opens the result with the trailing three as the start of the
// eleven octet nonce. Every CCM Transform ID is sealed at both key sizes.
//
// RFC 5282 Section 7.1: "For AES CCM, each key has the size and format of the "KEYMAT
// requested" material specified in Section 7.1 of [RFC4309] for the AES key size being
// used. For example, if the AES key size is 128 bits, each encryption key is 19 octets,
// consisting of a 16-octet AES cipher key followed by 3 octets of salt."
//
// A key of the AES GCM length (four octets of salt) or with its salt in front would pass
// neither the length assertion nor the open below.
func TestRFC5282CCMEncryptionKeysCarryTheirSalt(t *testing.T) {
	for _, tc := range ccmTransformIDs {
		for _, bits := range []uint16{128, 256} {
			enc := NewEncryptionTransform(tc.id, bits)
			keys, _ := aeadSKKeys(t, enc, IntegrityTransform{ID: AUTH_NONE})
			cipherOctets := int(bits) / 8

			for name, key := range map[string][]byte{"SK_ei": keys.SK_ei, "SK_er": keys.SK_er} {
				if len(key) != cipherOctets+3 {
					t.Fatalf("%s-%d: %s is %d octets, want %d: a %d octet cipher key "+
						"followed by 3 octets of salt",
						tc.id, bits, name, len(key), cipherOctets+3, cipherOctets)
				}
				plaintext := []byte("the inner payloads")
				aad := bytes.Repeat([]byte{0x33}, 32)
				data, err := SealIKEAEAD(tc.id, key, plaintext, aad)
				if err != nil {
					t.Fatalf("%s-%d: SealIKEAEAD with %s: %v", tc.id, bits, name, err)
				}
				block, err := aes.NewCipher(key[:cipherOctets])
				if err != nil {
					t.Fatalf("%s-%d: %s does not open as a %d octet cipher key: %v",
						tc.id, bits, name, cipherOctets, err)
				}
				mode, err := ccm.New(block, tc.icvOctets, 11)
				if err != nil {
					t.Fatalf("ccm.New: %v", err)
				}
				nonce := append(append([]byte(nil), key[cipherOctets:]...), data[:8]...)
				got, err := mode.Open(nil, nonce, data[8:], aad)
				if err != nil {
					t.Fatalf("%s-%d: %s does not split as cipher key then salt: %v",
						tc.id, bits, name, err)
				}
				if !bytes.Equal(got, plaintext) {
					t.Fatalf("%s-%d: %s round trip answered %q, want %q",
						tc.id, bits, name, got, plaintext)
				}
			}
		}
	}
}
