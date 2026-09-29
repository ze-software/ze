package engine

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// chkscopeICVOctets is the checksum length of HMAC-SHA2-256-128, the integrity
// algorithm cbcSKPair negotiates (RFC 4868 Section 2.3: the output is truncated to
// 128 bits).
const chkscopeICVOctets = 16

// chkscopeSealed returns a CBC SK message Ze sealed under sa: one Delete payload.
func chkscopeSealed(t *testing.T, sa *SA) []byte {
	t.Helper()
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolESP, SPISize: 4, NumSPIs: 2, SPIs: bytes.Repeat([]byte{9}, 8)}
	raw, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: del}}, 1, wire.ExchangeInformational, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	if int(sa.Proposal.Integrity.TruncatedLength) != chkscopeICVOctets {
		t.Fatalf("integrity truncates to %d octets, want %d", sa.Proposal.Integrity.TruncatedLength, chkscopeICVOctets)
	}
	return raw
}

// chkscopeHMAC computes HMAC-SHA2-256-128 under key over the concatenation of parts,
// with crypto/hmac directly and none of Ze's integrity code.
func chkscopeHMAC(key []byte, parts ...[]byte) []byte {
	mac := hmac.New(sha256.New, key)
	for _, part := range parts {
		mac.Write(part)
	}
	return mac.Sum(nil)[:chkscopeICVOctets]
}

// VALIDATES: the Integrity Checksum Data of a message Ze sends is the HMAC of every
// octet from the IKE header through the Pad Length, as they stand AFTER encryption.
//
// METHOD: the checksum is computed independently, with crypto/hmac under the sender's
// SK_a, over the sealed message minus its checksum, and compared with the checksum Ze
// wrote. The receiver accepts the message.
//
// RFC requirement: RFC7296-3.14-6 positive -- the checksum Ze writes equals HMAC-SHA2-256-128 under SK_ai over the header, the SK generic header, the IV and the ciphertext, computed outside Ze, and the peer accepts the message.
func TestRFC7296ChecksumIsTheHMACOfTheEncryptedMessage(t *testing.T) {
	sa, peer := cbcSKPair(t)
	raw := chkscopeSealed(t, sa)
	covered := raw[:len(raw)-chkscopeICVOctets]

	want := chkscopeHMAC(skSendIntegKey(sa), covered)
	if got := raw[len(raw)-chkscopeICVOctets:]; !bytes.Equal(got, want) {
		t.Fatalf("checksum = %x, want the HMAC of the encrypted message %x", got, want)
	}
	if _, err := decryptAndParse(peer, parseMsg(t, raw), raw); err != nil {
		t.Fatalf("the peer refused Ze's message: %v", err)
	}
}

// VALIDATES: the receiver refuses a message whose checksum was computed over the
// plaintext rather than over the encrypted message.
//
// METHOD: Ze seals a message. Its ciphertext is decrypted outside Ze, and a checksum
// is computed over the same header, SK header and IV followed by the PLAINTEXT. That
// checksum replaces Ze's. Every octet on the wire other than the checksum is
// unchanged, so the one difference is the MAC input: plaintext instead of
// ciphertext. The control message carries the checksum computed over the ciphertext
// the same way, and it is accepted.
//
// RFC requirement: RFC7296-3.14-6 negative -- a checksum taken over the header, IV and plaintext is refused by decryptAndParse, while the same construction over the ciphertext is accepted.
func TestRFC7296ChecksumOverThePlaintextIsRefused(t *testing.T) {
	sa, peer := cbcSKPair(t)
	raw := chkscopeSealed(t, sa)
	const blockOctets = aes.BlockSize
	ivEnd := skIVOffset + blockOctets
	ciphertext := raw[ivEnd : len(raw)-chkscopeICVOctets]

	block, err := aes.NewCipher(skSendEncKey(sa))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, raw[skIVOffset:ivEnd]).CryptBlocks(plaintext, ciphertext)
	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("the plaintext equals the ciphertext, so the two MAC inputs would not differ")
	}

	overPlaintext := bytes.Clone(raw)
	copy(overPlaintext[len(raw)-chkscopeICVOctets:], chkscopeHMAC(skSendIntegKey(sa), raw[:ivEnd], plaintext))
	if _, err := decryptAndParse(peer, parseMsg(t, overPlaintext), overPlaintext); err == nil {
		t.Fatal("the peer accepted a checksum computed over the plaintext")
	}

	overCiphertext := bytes.Clone(raw)
	copy(overCiphertext[len(raw)-chkscopeICVOctets:], chkscopeHMAC(skSendIntegKey(sa), raw[:ivEnd], ciphertext))
	if _, err := decryptAndParse(peer, parseMsg(t, overCiphertext), overCiphertext); err != nil {
		t.Fatalf("the peer refused a checksum computed over the ciphertext: %v", err)
	}
}
