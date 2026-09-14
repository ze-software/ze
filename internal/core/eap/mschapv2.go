// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- MS-CHAPv2 crypto primitives
// RFC: rfc/short/rfc2759.md -- NtPasswordHash, ChallengeResponse, AuthenticatorResponse, MPPE keys

package eap

import (
	"crypto/des"  //nolint:gosec // required by MS-CHAPv2 (RFC 2759)
	"crypto/hkdf"
	"crypto/sha1" //nolint:gosec // required by MS-CHAPv2 (RFC 2759)
	"crypto/sha256"
	"encoding/binary"
	"unicode/utf16"
)

// RFC 2759 Section 8: magic constants for GenerateAuthenticatorResponse.
var (
	magic1 = []byte("Magic server to client signing constant")
	magic2 = []byte("Pad to make it do more than one iteration")
)

// RFC 3079 Section 3: magic constants for GetMasterKey and GetAsymmetricStartKey.
var (
	masterKeyMagic = []byte("This is the MPPE Master Key")
	shsPad1        [40]byte // 40 bytes of 0x00
	shsPad2        [40]byte // 40 bytes of 0xf2
	sendKeyMagic   = []byte("On the client side, this is the send key; on the server side, it is the receive key.")
	recvKeyMagic   = []byte("On the client side, this is the receive key; on the server side, it is the send key.")
)

func init() {
	for i := range shsPad2 {
		shsPad2[i] = 0xf2
	}
}

// ntPasswordHash computes MD4(UTF-16LE(password)).
// RFC 2759 Section 8: NtPasswordHash.
func ntPasswordHash(password string) [16]byte {
	utf16Buf := utf16.Encode([]rune(password))
	raw := make([]byte, len(utf16Buf)*2)
	for i, r := range utf16Buf {
		binary.LittleEndian.PutUint16(raw[i*2:], r)
	}
	out := md4Sum(raw)
	clear(raw)
	return out
}

// hashNtPasswordHash computes MD4(PasswordHash).
// RFC 2759 Section 8: HashNtPasswordHash.
func hashNtPasswordHash(pwHash [16]byte) [16]byte {
	return md4Sum(pwHash[:])
}

// challengeHash computes SHA1(PeerChallenge || AuthChallenge || UserName)[:8].
// RFC 2759 Section 8: ChallengeHash. UserName MUST exclude DOMAIN\ prefix.
func challengeHash(peerChallenge, authChallenge [16]byte, userName string) [8]byte {
	h := sha1.New() //nolint:gosec // required by protocol
	h.Write(peerChallenge[:])
	h.Write(authChallenge[:])
	h.Write([]byte(userName))
	var out [8]byte
	copy(out[:], h.Sum(nil)[:8])
	return out
}

// desEncryptECB encrypts an 8-byte block with a 7-byte key (expanded to 8 with parity).
func desEncryptECB(key7 []byte, data [8]byte) [8]byte {
	key8 := expandDESKey(key7)
	block, _ := des.NewCipher(key8[:]) //nolint:gosec // required by protocol
	var out [8]byte
	block.Encrypt(out[:], data[:])
	return out
}

// expandDESKey expands a 7-byte key to 8 bytes by inserting parity bits.
// RFC 2759: each 7-bit group becomes 8 bits with odd parity in the LSB.
func expandDESKey(key7 []byte) [8]byte {
	var key8 [8]byte
	key8[0] = key7[0] >> 1
	key8[1] = ((key7[0] & 0x01) << 6) | (key7[1] >> 2)
	key8[2] = ((key7[1] & 0x03) << 5) | (key7[2] >> 3)
	key8[3] = ((key7[2] & 0x07) << 4) | (key7[3] >> 4)
	key8[4] = ((key7[3] & 0x0f) << 3) | (key7[4] >> 5)
	key8[5] = ((key7[4] & 0x1f) << 2) | (key7[5] >> 6)
	key8[6] = ((key7[5] & 0x3f) << 1) | (key7[6] >> 7)
	key8[7] = key7[6] & 0x7f
	for i := range key8 {
		key8[i] = (key8[i] << 1) | parityBit(key8[i])
	}
	return key8
}

func parityBit(b byte) byte {
	b ^= b >> 4
	b ^= b >> 2
	b ^= b >> 1
	return ^b & 1
}

// challengeResponse encrypts the 8-byte challenge with the 16-byte password hash,
// producing a 24-byte response. RFC 2759 Section 8: ChallengeResponse.
func challengeResponse(challenge [8]byte, pwHash [16]byte) [24]byte {
	var padded [21]byte
	copy(padded[:16], pwHash[:])

	r1 := desEncryptECB(padded[0:7], challenge)
	r2 := desEncryptECB(padded[7:14], challenge)
	r3 := desEncryptECB(padded[14:21], challenge)

	var out [24]byte
	copy(out[0:8], r1[:])
	copy(out[8:16], r2[:])
	copy(out[16:24], r3[:])
	return out
}

// GenerateNTResponse computes the full NT-Response for an MS-CHAPv2 exchange.
// RFC 2759 Section 8: GenerateNTResponse.
func GenerateNTResponse(authChallenge, peerChallenge [16]byte, userName, password string) [24]byte {
	challenge := challengeHash(peerChallenge, authChallenge, userName)
	pwHash := ntPasswordHash(password)
	return challengeResponse(challenge, pwHash)
}

// GenerateAuthenticatorResponse computes the mutual authentication proof (S= value).
// RFC 2759 Section 8: GenerateAuthenticatorResponse. Returns 20 raw bytes.
func GenerateAuthenticatorResponse(password string, ntResponse [24]byte, peerChallenge, authChallenge [16]byte, userName string) [20]byte {
	pwHash := ntPasswordHash(password)
	pwHashHash := hashNtPasswordHash(pwHash)

	h := sha1.New() //nolint:gosec // required by protocol
	h.Write(pwHashHash[:])
	h.Write(ntResponse[:])
	h.Write(magic1)
	digest := h.Sum(nil)

	challenge := challengeHash(peerChallenge, authChallenge, userName)

	h2 := sha1.New() //nolint:gosec // required by protocol
	h2.Write(digest)
	h2.Write(challenge[:])
	h2.Write(magic2)

	var out [20]byte
	copy(out[:], h2.Sum(nil))
	return out
}

// verifyNTResponse checks whether the received NT-Response matches the expected value.
// Uses constant-time comparison to prevent timing attacks.
func verifyNTResponse(authChallenge, peerChallenge [16]byte, userName, password string, received [24]byte) bool {
	expected := GenerateNTResponse(authChallenge, peerChallenge, userName, password)
	return constantTimeEqual(expected[:], received[:])
}

func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// GetMasterKey derives the 16-byte MPPE master key from MS-CHAPv2 credentials.
// RFC 3079 Section 3: GetMasterKey.
func GetMasterKey(password string, ntResponse [24]byte) [16]byte {
	pwHash := ntPasswordHash(password)
	pwHashHash := hashNtPasswordHash(pwHash)

	h := sha1.New() //nolint:gosec // required by protocol
	h.Write(pwHashHash[:])
	h.Write(ntResponse[:])
	h.Write(masterKeyMagic)

	var out [16]byte
	copy(out[:], h.Sum(nil)[:16])
	return out
}

// GetAsymmetricStartKey derives a session key of the requested length.
// RFC 3079 Section 3: GetAsymmetricStartKey.
func GetAsymmetricStartKey(masterKey [16]byte, keyLen int, isSend, isServer bool) []byte {
	var magic []byte
	switch isSend {
	case isServer:
		magic = sendKeyMagic
	default:
		magic = recvKeyMagic
	}

	h := sha1.New() //nolint:gosec // required by protocol
	h.Write(masterKey[:])
	h.Write(shsPad1[:])
	h.Write(magic)
	h.Write(shsPad2[:])

	return h.Sum(nil)[:keyLen]
}

// DeriveMSK constructs the 64-octet EAP MSK from MS-CHAPv2 credentials.
// RFC 3079 Section 3 + RFC 3748 Section 7.10.
// MSK = MasterReceiveKey(16) || MasterSendKey(16) || zeroPadding(32).
// strongSwan and Windows use zero-padded MSK per draft-kamath-pppext-eap-mschapv2-02.
//
// deriveEMSK below is the other half RFC 3748 Section 7.10 requires. The two are
// SIBLINGS of the MPPE master key rather than parent and child, which is what
// keeps them cryptographically separate.
func DeriveMSK(password string, ntResponse [24]byte) [64]byte {
	masterKey := GetMasterKey(password, ntResponse)

	recvKey := GetAsymmetricStartKey(masterKey, 16, true, true)
	sendKey := GetAsymmetricStartKey(masterKey, 16, false, true)

	var msk [64]byte
	copy(msk[0:16], recvKey)
	copy(msk[16:32], sendKey)
	// Bytes 32-63 remain zero (padding per draft-kamath).
	return msk
}

// mschapv2EMSKInfo separates the EMSK branch of the EAP-MSCHAPv2 key hierarchy
// from every other use of the MPPE master key.
//
// It names ze because no document defines an EAP-MSCHAPv2 EMSK, and nothing
// compares this value with another implementation's. RFC 3748 Section 7.10: "The
// EMSK is not shared with the authenticator or any other third party. The EMSK is
// reserved for future uses that are not defined yet." Section 7.2.1 says the same
// of its use: "Use of the EMSK is reserved." The key never reaches the wire, so
// the label needs no registry and no interop agreement; it needs only to be
// distinct from the two MPPE magic constants above.
const mschapv2EMSKInfo = "ze eap-mschapv2 extended master session key"

// deriveEMSK constructs the 64-octet EAP EMSK from MS-CHAPv2 credentials.
//
// RFC 3748 Section 7.10: "an EAP method supporting key derivation MUST export a
// Master Session Key (MSK) of at least 64 octets, and an Extended Master Session
// Key (EMSK) of at least 64 octets." EAP-MSCHAPv2 supports key derivation
// (TypeDerivesKey, eap.go; draft-kamath-pppext-eap-mschapv2-02 Section 3 states
// "Key derivation:            Yes"), so the sentence binds it and ze owes both.
//
// THE CONSTRAINT THAT SHAPES THIS. RFC 3748 Section 7.10: "Methods supporting key
// derivation MUST demonstrate cryptographic separation between the MSK and EMSK
// branches of the EAP key hierarchy.  Without violating a fundamental
// cryptographic assumption (such as the non-invertibility of a one-way function),
// an attacker recovering the MSK or EMSK MUST NOT be able to recover the other
// quantity with a level of effort less than brute force."
//
// So the EMSK is NOT derived from the MSK. It is derived from the same root the
// MSK is derived from, the MPPE master key, by a different one-way function under
// a label of its own:
//
//	MasterKey = SHA1(MD4(MD4(UTF16LE(password))) || NT-Response || magic)[0:16]
//	MSK       = SHA1(MasterKey || pad1 || recv-magic || pad2)[0:16]
//	         || SHA1(MasterKey || pad1 || send-magic || pad2)[0:16]
//	         || 32 zero octets
//	EMSK      = HKDF-Expand(SHA-256, MasterKey, mschapv2EMSKInfo, 64)
//
// Neither key is a function of the other. Recovering the MSK yields two SHA-1
// digests of MasterKey, each truncated from 20 octets to 16, so reaching MasterKey
// from it means inverting SHA-1 and guessing the discarded octets; recovering the
// EMSK yields HMAC-SHA-256 PRF output, so reaching MasterKey from it means
// inverting that. Either direction costs more than brute force, which is what the
// sentence above requires. Deriving the EMSK from the MSK would have failed it
// outright in one direction, and splitting the existing 64-octet MSK into two
// 32-octet halves fails the 64-octet floor of the same section.
//
// HKDF-Expand rather than HKDF-Extract-then-Expand: MasterKey is already the
// output of a hash and is uniformly distributed, which is the condition RFC 5869
// Section 3.3 names for skipping the extract step. The expansion is what supplies
// the 64 octets SHA-1 cannot.
//
// Substrings: RFC 3748 Section 7.10 also asks that "non-overlapping substrings of
// the EMSK MUST be cryptographically separate from each other, and from
// substrings of the MSK". HKDF-Expand output is a PRF stream keyed by MasterKey,
// so no part of it helps recover another part, or any part of the MSK.
//
// Freshness: MasterKey folds in NT-Response, which folds in both the peer and the
// authenticator challenge, so a fresh exchange yields a fresh EMSK. Section 7.10:
// "EAP methods SHOULD ensure the freshness of the MSK and EMSK". Effective key
// strength is bounded by the password, exactly as it is for the MSK, and
// draft-kamath Section 3 already declares that bound.
func deriveEMSK(password string, ntResponse [24]byte) [64]byte {
	masterKey := GetMasterKey(password, ntResponse)
	defer clear(masterKey[:])

	out, err := hkdf.Expand(sha256.New, masterKey[:], mschapv2EMSKInfo, 64)
	if err != nil {
		// Unreachable, and an assertion rather than an error path for that reason.
		// hkdf.Expand refuses only a keyLength above 255 times the hash size, which
		// is 8160 octets for SHA-256; 64 is a constant in this file. No peer input
		// reaches this argument, so no packet can drive ze here
		// (docs/contributing/ze-go-style.md, "Assertions, in a language that has
		// none").
		panic("BUG: hkdf.Expand refused a 64-octet EMSK under SHA-256: " + err.Error())
	}

	var emsk [64]byte
	copy(emsk[:], out)
	clear(out)
	return emsk
}

// stripDomain removes a DOMAIN\ prefix from a username for ChallengeHash.
// RFC 2759: UserName in ChallengeHash excludes the domain prefix.
func stripDomain(userName string) string {
	for i := range userName {
		if userName[i] == '\\' {
			return userName[i+1:]
		}
	}
	return userName
}
