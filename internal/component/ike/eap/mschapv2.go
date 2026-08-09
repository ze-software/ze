// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- MS-CHAPv2 crypto primitives
// RFC: rfc/short/rfc2759.md -- NtPasswordHash, ChallengeResponse, AuthenticatorResponse, MPPE keys

package eap

import (
	"crypto/des"  //nolint:gosec // required by MS-CHAPv2 (RFC 2759)
	"crypto/sha1" //nolint:gosec // required by MS-CHAPv2 (RFC 2759)
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

// VerifyNTResponse checks whether the received NT-Response matches the expected value.
// Uses constant-time comparison to prevent timing attacks.
func VerifyNTResponse(authChallenge, peerChallenge [16]byte, userName, password string, received [24]byte) bool {
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

// StripDomain removes a DOMAIN\ prefix from a username for ChallengeHash.
// RFC 2759: UserName in ChallengeHash excludes the domain prefix.
func StripDomain(userName string) string {
	for i := range userName {
		if userName[i] == '\\' {
			return userName[i+1:]
		}
	}
	return userName
}
