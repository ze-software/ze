// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- the AEAD transforms of the IKEv2 Encrypted payload
// Related: cipher.go -- the AES-CBC pair and the HMAC integrity helpers
// Related: internal/core/ccm/ccm.go -- the CCM mode an AES CCM transform keys

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"github.com/ze-software/ze/internal/core/ccm"
)

// ikeIVOctets is the explicit half of the nonce, the part that travels in the Encrypted
// payload. RFC 5282 Section 3.1: "The Initialization Vector (IV) MUST be eight octets."
// The same length serves AES GCM and AES CCM, so it is written once.
const ikeIVOctets = 8

// aeadTransform carries the three facts that follow from choosing one AEAD encryption
// transform: the salt it cuts from the key material, the ICV it appends to the
// ciphertext, and the mode that produces both.
type aeadTransform struct {
	saltOctets int
	icvOctets  int
	mode       func(block cipher.Block, icvOctets, nonceOctets int) (cipher.AEAD, error)
}

// aeadTransforms names every encryption transform that combines integrity with
// encryption. It is the one place these properties are written down for a wire
// Transform ID. Every site that decides the AEAD property asks EncryptionID.IsAEAD, and
// the tests in aead_predicate_test.go enumerate this map to prove it.
//
// The comment above once said the same thing. ikeProposalComplete kept a private copy
// that compared against ENCR_AES_GCM_16 alone. An entry added here therefore produced a
// cipher that keyed correctly, and negotiation then refused it with
// ErrProposalIncomplete.
//
// Two neighboring maps are NOT this property, and neither is derived from it. A new
// AEAD cipher needs an entry in each. specifiedEncryption (proposal.go) lists the IDs
// this build accepts off the wire. encryptionRegistry (transform.go) maps a config name
// to a transform. ipsec.EncryptionAlgo.IsAEAD answers the same question over the config
// enum rather than the wire ID. A test in that package binds the two.
//
// The salt is per transform and never one number shared by every AEAD. That constant
// was correct while AES GCM was the only entry, and it gives a wrong key length
// silently for the first cipher that differs, which AES CCM is.
//
// The nonce length is NOT a field here. RFC 5282 Section 4 derives it: the nonce is the
// salt concatenated with the IV, so it is saltOctets plus ikeIVOctets. That gives AES
// GCM twelve octets and AES CCM eleven, which is what the same section requires of
// each, and a stored length could disagree with the salt beside it.
var aeadTransforms = map[EncryptionID]aeadTransform{
	// RFC 4106 Section 8.1 gives AES GCM a four octet salt, and RFC 5282 Section 3.2
	// gives the AES GCM ICV its full sixteen octets.
	ENCR_AES_GCM_16: {saltOctets: 4, icvOctets: 16, mode: newAESGCM},
	// RFC 4309 Section 7.1: "The KEYMAT requested for each AES CCM key is 19 octets.
	// The first 16 octets are the 128-bit AES key, and the remaining three octets are
	// used as the salt value in the counter block." RFC 5282 Section 7.1 carries that
	// layout into the IKE SA, and Section 7.2 assigns one Transform ID per ICV size:
	// "14 for AES CCM with an 8-octet ICV; 15 for AES CCM with a 12-octet ICV; 16 for
	// AES CCM with a 16-octet ICV."
	ENCR_AES_CCM_8:  {saltOctets: 3, icvOctets: 8, mode: newAESCCM},
	ENCR_AES_CCM_12: {saltOctets: 3, icvOctets: 12, mode: newAESCCM},
	ENCR_AES_CCM_16: {saltOctets: 3, icvOctets: 16, mode: newAESCCM},
}

// newAESGCM builds the AES-GCM mode for an IKEv2 Encrypted payload.
//
// cipher.NewGCM answers a twelve octet nonce and a sixteen octet ICV, which is exactly
// what RFC 5282 Section 4 and Section 3.2 require of ENCR_AES_GCM_16, the one AES GCM
// transform ze specifies. Both sizes are still compared against the table rather than
// assumed, so a table entry that disagrees with the mode is refused instead of keyed
// silently at another length.
func newAESGCM(block cipher.Block, icvOctets, nonceOctets int) (cipher.AEAD, error) {
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if gcm.Overhead() != icvOctets || gcm.NonceSize() != nonceOctets {
		return nil, ErrUnsupportedAlgorithm
	}
	return gcm, nil
}

// newAESCCM builds the AES-CCM mode for an IKEv2 Encrypted payload.
//
// RFC 5282 Section 4: "For the use of AES CCM with the IKEv2 Encrypted Payload, this
// default nonce format MUST be used and an 11 octet nonce MUST be used." The caller
// derives that eleven from the three octet salt and the eight octet IV, and ccm.New
// refuses any other length rather than clamping it.
func newAESCCM(block cipher.Block, icvOctets, nonceOctets int) (cipher.AEAD, error) {
	return ccm.New(block, icvOctets, nonceOctets)
}

// AEADICVOctets answers the ICV this AEAD encryption transform appends to the
// ciphertext, and refuses a transform ze does not specify.
//
// RFC 5282 Section 3.2 fixes the permitted sizes and forbids the rest: "The AES GCM ICV
// consists solely of the AES GCM Authentication Tag. Implementations MUST support a
// full-length 16 octet ICV, MAY support 8 or 12 octet ICVs, and MUST NOT support other
// ICV lengths. AES CCM provides an encrypted ICV. Implementations MUST support ICV
// sizes of 8 octets and 16 octets. Implementations MAY also support 12 octet ICVs and
// MUST NOT support other ICV lengths."
//
// aeadTransforms is how ze supports a size: an ICV length exists only where a Transform
// ID names it, Section 7.2 assigns one ID per AES CCM ICV size, and the map holds those
// three and no others. So an ID outside the map gets an error rather than a length. A
// zero would read as a valid "no ICV" answer and size a message without one
// (ai/rules/evidence.md).
func AEADICVOctets(id EncryptionID) (int, error) {
	t, ok := aeadTransforms[id]
	if !ok {
		return 0, ErrUnsupportedAlgorithm
	}
	return t.icvOctets, nil
}

// SealIKEAEAD encrypts one IKEv2 Encrypted payload and answers the payload data: the
// eight octet IV followed by the ciphertext, whose ICV is incorporated (RFC 5282
// Section 3).
//
// keyWithSalt is SK_ei or SK_er for the sending direction, the AES cipher key followed
// by this transform's salt (Section 7.1). aad is the associated data the caller cuts
// from the message prefix (Section 5.1).
func SealIKEAEAD(id EncryptionID, keyWithSalt, plaintext, aad []byte) ([]byte, error) {
	aead, salt, err := newIKEAEAD(id, keyWithSalt)
	if err != nil {
		return nil, err
	}

	// RFC 5282 Section 3.1: "The IV MUST be chosen by the encryptor in a manner that
	// ensures that the same IV value is used only once for a given key." Eight fresh
	// octets from the system CSPRNG are what "The encryptor MAY generate the IV in any
	// manner that ensures uniqueness" permits.
	out := make([]byte, ikeIVOctets, ikeIVOctets+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(out); err != nil {
		return nil, err
	}
	return aead.Seal(out, ikeAEADNonce(salt, out), plaintext, aad), nil
}

// OpenIKEAEAD decrypts one IKEv2 Encrypted payload. data is the payload data as it
// arrived: the eight octet IV followed by the ciphertext.
//
// Every failure answers ErrDecryptionFailed and nothing else, so a peer learns that the
// message was refused and never why.
func OpenIKEAEAD(id EncryptionID, keyWithSalt, data, aad []byte) ([]byte, error) {
	aead, salt, err := newIKEAEAD(id, keyWithSalt)
	if err != nil {
		return nil, err
	}
	if len(data) < ikeIVOctets {
		return nil, ErrDecryptionFailed
	}
	plaintext, err := aead.Open(nil, ikeAEADNonce(salt, data[:ikeIVOctets]), data[ikeIVOctets:], aad)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// ikeAEADNonce builds the default nonce format of RFC 5282 Section 4: "When this
// default nonce format is used, both the encryptor and decryptor construct the nonce by
// concatenating the salt with the IV, in that order."
//
// The length follows from the two halves rather than from a constant, so AES GCM
// answers twelve octets and AES CCM answers eleven, which is what Section 4 requires of
// each.
func ikeAEADNonce(salt, iv []byte) []byte {
	nonce := make([]byte, 0, len(salt)+len(iv))
	nonce = append(nonce, salt...)
	return append(nonce, iv...)
}

// newIKEAEAD builds the mode for this transform and cuts the key material into the AES
// cipher key and the nonce salt.
//
// RFC 5282 Section 7.1 states the layout: "if the AES key size is 128 bits, each
// encryption key is 20 octets, consisting of a 16-octet AES cipher key followed by 4
// octets of salt" for AES GCM, and nineteen octets with three of salt for AES CCM. The
// cut is therefore taken from the END, which is what makes one function serve both.
func newIKEAEAD(id EncryptionID, keyWithSalt []byte) (cipher.AEAD, []byte, error) {
	t, ok := aeadTransforms[id]
	if !ok {
		return nil, nil, ErrUnsupportedAlgorithm
	}
	if len(keyWithSalt) <= t.saltOctets {
		return nil, nil, ErrInvalidKeyLength
	}
	cut := len(keyWithSalt) - t.saltOctets

	block, err := aes.NewCipher(keyWithSalt[:cut])
	if err != nil {
		return nil, nil, err
	}
	aead, err := t.mode(block, t.icvOctets, t.saltOctets+ikeIVOctets)
	if err != nil {
		return nil, nil, err
	}
	return aead, keyWithSalt[cut:], nil
}
