// VALIDATES: the RFC 5282 obligations the IKE crypto package meets for an authenticated
// encryption algorithm. The list covers the ICV lengths an AES GCM implementation may
// support (§3.2), the zero-length SK_ai and SK_ar and the KEYMAT layout of SK_ei and SK_er
// (§7.1), and the Key Length attribute an AES GCM transform must carry and the three values
// it may take (§7.3).
// PREVENTS: an AEAD key derivation that reserves integrity key material it never uses, one
// that sizes SK_ei without its salt, an AES-GCM transform accepted with no Key Length
// attribute, and an ICV length RFC 5282 forbids being opened.
package crypto

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	"errors"
	"testing"
)

// aeadKeyWithSalt is one AES-GCM-256 key material block: a 32 octet cipher key followed by
// the four octet salt (RFC 5282 Section 7.1).
func aeadKeyWithSalt() []byte {
	material := make([]byte, 36)
	for i := range material {
		material[i] = byte(i + 1)
	}
	return material
}

// sealSKData answers the SK payload data a peer sends: the eight octet IV followed by the
// AES-GCM output at the given ICV length. The nonce is the salt and the IV, in that order.
func sealSKData(t *testing.T, keyWithSalt, plaintext, aad []byte, icvOctets int) []byte {
	t.Helper()
	iv := bytes.Repeat([]byte{0x42}, 8)
	block, err := aes.NewCipher(keyWithSalt[:len(keyWithSalt)-4])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	var gcm gocipher.AEAD
	if icvOctets == 16 {
		gcm, err = gocipher.NewGCM(block)
	} else {
		gcm, err = gocipher.NewGCMWithTagSize(block, icvOctets)
	}
	if err != nil {
		t.Fatalf("build an AES-GCM instance with a %d octet ICV: %v", icvOctets, err)
	}
	nonce := make([]byte, 0, 12)
	nonce = append(nonce, keyWithSalt[len(keyWithSalt)-4:]...)
	nonce = append(nonce, iv...)
	return append(iv, gcm.Seal(nil, nonce, plaintext, aad)...)
}

// RFC requirement: RFC5282-3.2-2 positive -- DecryptIKEAEAD refuses SK payload data whose
// ICV is 13, 14 or 15 octets. Each message is a real AES-GCM sealing at that tag length
// under the correct key, nonce and associated data, so the only reason it is refused is the
// ICV length.
//
// RFC 5282 Section 3.2: "The AES GCM ICV consists solely of the AES GCM Authentication Tag.
// Implementations MUST support a full-length 16 octet ICV, MAY support 8 or 12 octet ICVs,
// and MUST NOT support other ICV lengths."
//
// 13, 14 and 15 are the forbidden lengths this test can construct: the standard library's
// AES-GCM refuses to produce a tag outside 12 to 16 octets, and 8, 12 and 16 are the three
// lengths the sentence allows.
func TestRFC5282AEADRefusesAForbiddenICVLength(t *testing.T) {
	key := aeadKeyWithSalt()
	plaintext := []byte("inner payloads and the pad length")
	aad := bytes.Repeat([]byte{0x11}, 32)

	for _, icvOctets := range []int{13, 14, 15} {
		data := sealSKData(t, key, plaintext, aad, icvOctets)
		got, err := DecryptIKEAEAD(key, data, aad)
		if err == nil {
			t.Fatalf("a %d octet ICV was accepted and answered %q, want a refusal: RFC 5282 "+
				"Section 3.2 forbids supporting an ICV length other than 16, 8 and 12",
				icvOctets, got)
		}
	}
}

// RFC requirement: RFC5282-3.2-2 negative -- the refusals above are specific to the
// forbidden lengths rather than a blanket refusal. The same plaintext, key, nonce and
// associated data sealed at the full-length 16 octet ICV is opened and answers the
// plaintext back.
func TestRFC5282AEADOpensTheFullLengthICV(t *testing.T) {
	key := aeadKeyWithSalt()
	plaintext := []byte("inner payloads and the pad length")
	aad := bytes.Repeat([]byte{0x11}, 32)

	data := sealSKData(t, key, plaintext, aad, 16)
	got, err := DecryptIKEAEAD(key, data, aad)
	if err != nil {
		t.Fatalf("DecryptIKEAEAD over a 16 octet ICV = %v, want the plaintext", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("DecryptIKEAEAD answered %q, want %q", got, plaintext)
	}
}

// aeadSKKeys derives one IKE SA key hierarchy for the given encryption and integrity
// transforms, and answers it beside the prf+ stream it was cut from.
func aeadSKKeys(t *testing.T, enc EncryptionTransform, integ IntegrityTransform) (*SKKeys, []byte) {
	t.Helper()
	seed := bytes.Repeat([]byte{0x5a}, 32)
	ni := bytes.Repeat([]byte{0x01}, 32)
	nr := bytes.Repeat([]byte{0x02}, 32)
	spiI := bytes.Repeat([]byte{0x03}, 8)
	spiR := bytes.Repeat([]byte{0x04}, 8)

	keys, err := DeriveSKKeys(PRF_HMAC_SHA2_256, seed, ni, nr, spiI, spiR, enc, integ)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	total := 32 + 2*int(integ.KeyLength) + 2*encKeyMaterialLen(enc) + 2*32
	stream, err := PRFPlus(PRF_HMAC_SHA2_256, seed,
		bytes.Join([][]byte{ni, nr, spiI, spiR}, nil), total)
	if err != nil {
		t.Fatalf("PRFPlus: %v", err)
	}
	return keys, stream
}

// RFC requirement: RFC5282-7.1-1 positive -- with an AEAD cipher SK_ai and SK_ar are each
// zero octets, and the prf+ stream is cut as though they were: SK_ei starts at the octet
// after SK_d rather than after two integrity keys.
//
// RFC 5282 Section 7.1: "When AES GCM or AES CCM is used with the IKEv2 Encrypted Payload,
// the SK_ai and SK_ar integrity protection keys are not used; each key MUST be treated as
// having a size of zero (0) octets."
//
// The offset assertion is what makes the length assertion mean something. A derivation that
// answered empty slices while still consuming integrity key material from the stream would
// give a peer the wrong SK_ei, and every message would fail to decrypt.
func TestRFC5282AEADIntegrityKeysAreZeroOctets(t *testing.T) {
	enc := NewEncryptionTransform(ENCR_AES_GCM_16, 256)
	keys, stream := aeadSKKeys(t, enc, IntegrityTransform{ID: AUTH_NONE})

	if len(keys.SK_ai) != 0 || len(keys.SK_ar) != 0 {
		t.Fatalf("SK_ai is %d octets and SK_ar is %d octets, want 0 and 0",
			len(keys.SK_ai), len(keys.SK_ar))
	}
	const skDOctets = 32
	encOctets := encKeyMaterialLen(enc)
	if !bytes.Equal(keys.SK_ei, stream[skDOctets:skDOctets+encOctets]) {
		t.Fatalf("SK_ei is not the prf+ octets at offset %d, so the zero-length integrity "+
			"keys did not remove their share of the stream", skDOctets)
	}
	if !bytes.Equal(keys.SK_er, stream[skDOctets+encOctets:skDOctets+2*encOctets]) {
		t.Fatalf("SK_er is not the prf+ octets at offset %d", skDOctets+encOctets)
	}
}

// RFC requirement: RFC5282-7.1-1 negative -- the zero length belongs to the AEAD case. The
// same derivation under AES-CBC with HMAC-SHA2-256-128 gives SK_ai and SK_ar of 32 octets
// each, so an implementation that always answered empty integrity keys would not pass.
func TestRFC5282NonAEADIntegrityKeysAreDerived(t *testing.T) {
	integ, err := LookupIntegrity(hashNameSHA256)
	if err != nil {
		t.Fatalf("LookupIntegrity: %v", err)
	}
	keys, _ := aeadSKKeys(t, NewEncryptionTransform(ENCR_AES_CBC, 256), integ)

	if len(keys.SK_ai) != 32 || len(keys.SK_ar) != 32 {
		t.Fatalf("SK_ai is %d octets and SK_ar is %d octets, want 32 and 32 for AES-CBC "+
			"with HMAC-SHA2-256-128", len(keys.SK_ai), len(keys.SK_ar))
	}
}

// RFC requirement: RFC5282-7.1-2 positive -- SK_ei and SK_er each have the size and format
// of the AES-GCM KEYMAT for the AES key size in use: the cipher key followed by four octets
// of salt, so 20 octets at 128 bits and 36 at 256. The format is proven by USING it, and
// not only by the length: the key is sealed with encryptIKEAEAD and opened by an
// independent AES-GCM instance keyed with the leading octets and given the trailing four as
// the implicit half of the nonce.
//
// RFC 5282 Section 7.1: "The size and format of each of the SK_ei and SK_er encryption keys
// MUST be: For AES GCM, each encryption key has the size and format of the 'KEYMAT
// requested' material specified in Section 8.1 of [RFC4106] for the AES key size being
// used. For example, if the AES key size is 128 bits, each encryption key is 20 octets,
// consisting of a 16-octet AES cipher key followed by 4 octets of salt."
//
// A key of the right length whose salt sat in front of the cipher key would pass a length
// assertion and fail the round trip below, which is why both are here.
func TestRFC5282AEADEncryptionKeysCarryTheirSalt(t *testing.T) {
	for _, tc := range []struct {
		bits   uint16
		octets int
	}{
		{128, 20},
		{256, 36},
	} {
		enc := NewEncryptionTransform(ENCR_AES_GCM_16, tc.bits)
		keys, _ := aeadSKKeys(t, enc, IntegrityTransform{ID: AUTH_NONE})

		for name, key := range map[string][]byte{"SK_ei": keys.SK_ei, "SK_er": keys.SK_er} {
			if len(key) != tc.octets {
				t.Fatalf("AES-GCM-%d: %s is %d octets, want %d: a %d octet cipher key "+
					"followed by 4 octets of salt",
					tc.bits, name, len(key), tc.octets, int(tc.bits)/8)
			}
			plaintext := []byte("the inner payloads")
			aad := bytes.Repeat([]byte{0x33}, 32)
			data, err := encryptIKEAEAD(key, plaintext, aad)
			if err != nil {
				t.Fatalf("AES-GCM-%d: encryptIKEAEAD with %s: %v", tc.bits, name, err)
			}
			block, err := aes.NewCipher(key[:len(key)-4])
			if err != nil {
				t.Fatalf("AES-GCM-%d: %s does not open as a %d octet cipher key: %v",
					tc.bits, name, len(key)-4, err)
			}
			gcm, err := gocipher.NewGCM(block)
			if err != nil {
				t.Fatalf("gocipher.NewGCM: %v", err)
			}
			nonce := append(append([]byte(nil), key[len(key)-4:]...), data[:8]...)
			got, err := gcm.Open(nil, nonce, data[8:], aad)
			if err != nil {
				t.Fatalf("AES-GCM-%d: %s does not split as cipher key then salt: %v",
					tc.bits, name, err)
			}
			if !bytes.Equal(got, plaintext) {
				t.Fatalf("AES-GCM-%d: %s round trip answered %q, want %q",
					tc.bits, name, got, plaintext)
			}
		}
	}
}

// RFC requirement: RFC5282-7.1-2 negative -- the four extra octets belong to the AEAD
// cipher. AES-CBC-256 gives SK_ei and SK_er of exactly 32 octets, so an implementation that
// added a salt to every encryption key would not pass.
func TestRFC5282NonAEADEncryptionKeysCarryNoSalt(t *testing.T) {
	integ, err := LookupIntegrity(hashNameSHA256)
	if err != nil {
		t.Fatalf("LookupIntegrity: %v", err)
	}
	keys, _ := aeadSKKeys(t, NewEncryptionTransform(ENCR_AES_CBC, 256), integ)

	if len(keys.SK_ei) != 32 || len(keys.SK_er) != 32 {
		t.Fatalf("SK_ei is %d octets and SK_er is %d octets, want 32 and 32 for AES-CBC-256",
			len(keys.SK_ei), len(keys.SK_er))
	}
}

// gcmOffer answers one complete AES-GCM IKE proposal at the given Key Length attribute
// value. A value of zero means the peer sent no attribute at all.
func gcmOffer(bits uint16) IKEProposal {
	return IKEProposal{
		Number:     1,
		Encryption: EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: bits, IsAEAD: true},
		PRF:        PRFTransform{ID: PRF_HMAC_SHA2_256},
		Integrity:  IntegrityTransform{ID: AUTH_NONE},
		DHGroup:    DHGroupTransform{ID: DH_ECP_256},
	}
}

// RFC requirement: RFC5282-7.3-1 negative -- an AES-GCM transform arriving with no Key
// Length attribute is refused with ErrKeyLengthMissing, and not negotiated at whatever
// length local policy happens to hold.
//
// RFC 5282 Section 7.3: "Because the AES supports three key lengths, the Key Length
// attribute MUST be specified when any of the identifiers for AES GCM or AES CCM, specified
// in Section 7.2 of this document, is used."
//
// The named error is what makes this the requirement's own branch. Without it the offer
// would still be refused, by the comparison against local policy that any other unequal key
// length fails, and nothing would say the missing attribute was the reason.
func TestRFC5282AEADOfferWithoutAKeyLengthIsRefused(t *testing.T) {
	_, err := NegotiateIKE([]IKEProposal{gcmOffer(0)}, []IKEProposal{gcmOffer(256)})
	if !errors.Is(err, ErrKeyLengthMissing) {
		t.Fatalf("an AES-GCM offer carrying no Key Length attribute answered %v, want %v",
			err, ErrKeyLengthMissing)
	}
}

// RFC requirement: RFC5282-7.3-2 positive -- the three values the attribute may take are
// each accepted, and the selected transform carries the offered value back unchanged.
//
// RFC 5282 Section 7.3: "The Key Length attribute MUST have a value of 128, 192, or 256."
//
// 192 is offered even though no config name reaches it, because the sentence names three
// values and a peer may send any of them.
func TestRFC5282AEADKeyLengthAcceptsTheThreeValues(t *testing.T) {
	for _, bits := range []uint16{128, 192, 256} {
		chosen, err := NegotiateIKE([]IKEProposal{gcmOffer(bits)}, []IKEProposal{gcmOffer(bits)})
		if err != nil {
			t.Fatalf("a Key Length attribute of %d was refused: %v", bits, err)
		}
		if chosen.Encryption.KeyLength != bits {
			t.Fatalf("the selected transform carries a Key Length of %d, want %d",
				chosen.Encryption.KeyLength, bits)
		}
	}
}

// RFC requirement: RFC5282-7.3-2 negative -- a Key Length attribute outside those three
// values is refused, whether local policy names a different length or the same one. The
// second case is what makes this the attribute's own rule rather than the comparison
// against local policy: an offer of 200 bits against a local 200 bits is equal, and is
// still refused because 200 is not one of the three.
func TestRFC5282AEADKeyLengthRefusesEveryOtherValue(t *testing.T) {
	for _, bits := range []uint16{8, 64, 127, 200, 384, 512} {
		if _, err := NegotiateIKE([]IKEProposal{gcmOffer(bits)}, []IKEProposal{gcmOffer(256)}); err == nil {
			t.Fatalf("a Key Length attribute of %d was accepted against a local 256, want a refusal", bits)
		}
		if _, err := NegotiateIKE([]IKEProposal{gcmOffer(bits)}, []IKEProposal{gcmOffer(bits)}); err == nil {
			t.Fatalf("a Key Length attribute of %d was accepted against a local %d, want a refusal",
				bits, bits)
		}
	}
}
