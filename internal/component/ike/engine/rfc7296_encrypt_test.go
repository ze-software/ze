// VALIDATES: the RFC 7296 obligations on the Encrypted (SK) payload the IKE engine builds and
// consumes. The list covers the terminal position of that payload and its non-nesting (§3.1).
// It covers a fresh unpredictable IV for each message, and acceptance of any IV on receipt
// (§3.14). It covers padding that aligns to the block size, and acceptance of any aligning
// Pad Length (§3.14). It covers the integrity checksum computed over the encrypted message
// (§3.14), and the nonce sized against the negotiated PRF (§2.10). Each test carries an
// `RFC requirement:` tag binding it to its checklist id.
// PREVENTS: an SK-builder change that reuses an IV, mis-sizes the padding, moves the checksum
// off the encrypted bytes, or lets a payload follow the Encrypted payload.
package engine

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// skIVOffset is where the initialization vector starts in an SK message: after the fixed IKE
// header and the SK payload's generic header (auth.go:637).
const skIVOffset = wire.HeaderLen + wire.GenericHeaderLen

// buildCBCSKWithIVAndPad builds a CBC-mode SK message exactly as
// buildSKMessageCBCWithMsgID does, except that the caller chooses the IV and the number of
// padding octets. It feeds the RECEIVE path values a conforming peer CAN legally send.
func buildCBCSKWithIVAndPad(t *testing.T, sa *SA, innerData []byte, msgID uint32, iv []byte, padLen int) []byte {
	t.Helper()
	const firstType = wire.PayloadTypeDelete
	const blockSize = 16
	if len(iv) != blockSize {
		t.Fatalf("iv is %d octets, want %d", len(iv), blockSize)
	}
	contentLen := len(innerData)
	paddedLen := contentLen + padLen + 1
	if paddedLen%blockSize != 0 {
		t.Fatalf("content %d + pad %d + 1 = %d is not a multiple of %d", contentLen, padLen, paddedLen, blockSize)
	}
	integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}
	totalLen := wire.HeaderLen + wire.GenericHeaderLen + blockSize + paddedLen + integTrunc
	buf := make([]byte, totalLen)
	writeAuthHeaderWithMsgID(buf, sa, firstType, uint32(totalLen), msgID,
		wire.ExchangeInformational, initiatorFlag(sa))
	copy(buf[skIVOffset:], iv)

	dataOff := skIVOffset + blockSize
	copy(buf[dataOff:], innerData)
	// Padding MAY contain any value. Use a recognizable non-zero fill.
	for i := range padLen {
		buf[dataOff+contentLen+i] = byte(0xa0 + i%16)
	}
	buf[dataOff+contentLen+padLen] = byte(padLen)

	block, err := aes.NewCipher(skSendEncKey(sa))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gocipher.NewCBCEncrypter(block, buf[skIVOffset:skIVOffset+blockSize]).
		CryptBlocks(buf[dataOff:dataOff+paddedLen], buf[dataOff:dataOff+paddedLen])

	mac, err := ikecrypto.ComputeIntegrity(sa.Proposal.Integrity.ID, skSendIntegKey(sa), buf[:totalLen-integTrunc])
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}
	copy(buf[totalLen-integTrunc:], mac)
	return buf
}

// cbcSKPair returns an established initiator/responder pair that shares one SK hierarchy
// under a CBC cipher. The sender builds the message. The receiver verifies and decrypts it,
// exactly as the two ends of a real exchange do (skSendEncKey/skRecvEncKey, auth.go:503-530).
func cbcSKPair(t *testing.T) (sender, receiver *SA) {
	t.Helper()
	ini, resp, _ := establishPSK(t)
	if ini.Proposal.Encryption.IsAEAD {
		t.Fatal("cbcSKPair needs a CBC proposal; the test IKE group negotiated an AEAD cipher")
	}
	return ini, resp
}

// RFC requirement: RFC7296-3.1-1 positive -- the Encrypted payload is the last payload in every
// message ze sends. writeAuthHeaderWithMsgID writes the fixed header with NextPayload SK, and
// then exactly one SK generic header (auth.go:618-635). An SK message therefore parses to a
// single payload with nothing after it.
// RFC requirement: RFC7296-3.1-2 positive -- an Encrypted payload never contains another. The SK
// generic header's Next Payload names the FIRST INNER payload type. The inner chain that
// ParsePayloadChain recovers holds no SK payload (chain.go:10).
//
// RFC requirement: RFC7296-3.1-1 negative -- the single-payload shape belongs to the builder, not to
// the parser. An unencrypted IKE_SA_INIT built by the same engine DOES carry several payloads,
// so the SK message's payload count of one is meaningful.
// RFC requirement: RFC7296-3.1-2 negative -- the inner chain is a real chain. It holds several
// payloads of other types, so the absence of a nested SK is not the absence of inner content.
func TestSKIsLastAndNeverNested(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	for _, tc := range []struct {
		name string
		raw  []byte
		peer *SA
	}{
		{"IKE_AUTH request", ini.LastSentMsg, resp},
		{"IKE_AUTH response", resp.LastSentMsg, ini},
	} {
		msg := parseMsg(t, tc.raw)
		if len(msg.Payloads) != 1 {
			t.Errorf("%s: outer chain has %d payloads, want 1 (the Encrypted payload is last)",
				tc.name, len(msg.Payloads))
			continue
		}
		if _, ok := msg.Payloads[0].Payload.(*wire.PayloadSK); !ok {
			t.Errorf("%s: sole payload = %T, want *wire.PayloadSK", tc.name, msg.Payloads[0].Payload)
			continue
		}
		if msg.Header.NextPayload != wire.PayloadTypeSK {
			t.Errorf("%s: header Next Payload = %d, want %d", tc.name, msg.Header.NextPayload, wire.PayloadTypeSK)
		}
		inner, err := decryptAndParse(tc.peer, msg, tc.raw)
		if err != nil {
			t.Errorf("%s: decrypt: %v", tc.name, err)
			continue
		}
		if len(inner) == 0 {
			t.Errorf("%s: the inner chain is empty, so a missing nested SK proves nothing", tc.name)
		}
		for i := range inner {
			if _, ok := inner[i].Payload.(*wire.PayloadSK); ok {
				t.Errorf("%s: inner payload %d is an Encrypted payload; an Encrypted payload "+
					"must not contain another", tc.name, i)
			}
		}
	}

	// Negative for 3.1-1: an unencrypted message the same engine builds does carry several
	// payloads, so "one payload" is a property of the SK shape.
	initMsg := parseMsg(t, ini.InitiatorSAInitMsg)
	if len(initMsg.Payloads) < 2 {
		t.Errorf("the IKE_SA_INIT request carries %d payloads; a multi-payload message must be "+
			"possible or the SK single-payload assertion is vacuous", len(initMsg.Payloads))
	}
}

// RFC requirement: RFC7296-3.14-2 positive -- Ze selects a new unpredictable IV for every message.
// buildSKMessageCBCWithMsgID fills the IV region from crypto/rand on every call
// (auth.go:553-556). Many messages built from the same SA and the same plaintext therefore
// carry different IVs and different ciphertext.
// RFC requirement: RFC7296-3.14-2 negative -- the IV is not a counter, and it is not a copy of a
// neighboring field. It is never equal to the previous message's final ciphertext block, which
// RFC 7296 names explicitly as not unpredictable.
func TestSKSelectsFreshIVPerMessage(t *testing.T) {
	sa := testSAWithKeys(t)
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}}

	const rounds = 32
	seenIV := make(map[string]bool, rounds)
	var prevFinalBlock []byte
	for i := range rounds {
		raw, err := buildEncryptedMessageEx(sa, inner, uint32(i), wire.ExchangeInformational, wire.FlagInitiator)
		if err != nil {
			t.Fatalf("buildEncryptedMessageEx: %v", err)
		}
		if len(raw) < skIVOffset+16 {
			t.Fatalf("message is %d octets, too short to hold an IV", len(raw))
		}
		iv := raw[skIVOffset : skIVOffset+16]
		key := string(iv)
		if seenIV[key] {
			t.Fatalf("IV %x repeated at message %d; a new IV is required for every message", iv, i)
		}
		seenIV[key] = true

		if prevFinalBlock != nil && bytes.Equal(iv, prevFinalBlock) {
			t.Errorf("message %d reused the previous message's final ciphertext block as its "+
				"IV; the RFC names that as not unpredictable", i)
		}
		integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
		if integTrunc == 0 {
			integTrunc = 16
		}
		ctEnd := len(raw) - integTrunc
		prevFinalBlock = append([]byte(nil), raw[ctEnd-16:ctEnd]...)
	}
	if len(seenIV) != rounds {
		t.Errorf("%d distinct IVs over %d messages", len(seenIV), rounds)
	}
}

// RFC requirement: RFC7296-3.14-3 positive -- the receive path accepts any IV value. decryptSKPayload
// takes the IV from the message rather than from local state (auth.go:639-682). A peer that
// sends an all-zero IV, an all-ones IV, or a repeat of an earlier IV is decrypted correctly.
// RFC requirement: RFC7296-3.14-3 negative -- tolerance of any IV is not tolerance of anything. The
// same message with a corrupted integrity checksum is refused, so the IV tolerance does not
// weaken authentication.
func TestSKAcceptsAnyIVOnReceipt(t *testing.T) {
	sa, peer := cbcSKPair(t)
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}
	innerBuf := make([]byte, wire.GenericHeaderLen+del.Len())
	gh := wire.GenericHeader{Length: uint16(wire.GenericHeaderLen + del.Len())}
	gh.WriteTo(innerBuf, 0)
	del.WriteTo(innerBuf, wire.GenericHeaderLen)

	for _, tc := range []struct {
		name string
		iv   []byte
	}{
		{"all zero", make([]byte, 16)},
		{"all ones", bytes.Repeat([]byte{0xff}, 16)},
		{"ascending", []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}},
	} {
		padLen := 16 - ((len(innerBuf) + 1) % 16)
		if padLen == 16 {
			padLen = 0
		}
		raw := buildCBCSKWithIVAndPad(t, sa, innerBuf, 3, tc.iv, padLen)
		msg := parseMsg(t, raw)
		inner, err := decryptAndParse(peer, msg, raw)
		if err != nil {
			t.Errorf("%s IV: decryptAndParse = %v, want the message accepted", tc.name, err)
			continue
		}
		if len(inner) != 1 {
			t.Errorf("%s IV: recovered %d inner payloads, want 1", tc.name, len(inner))
			continue
		}
		if _, ok := inner[0].Payload.(*wire.PayloadDelete); !ok {
			t.Errorf("%s IV: inner payload = %T, want *wire.PayloadDelete", tc.name, inner[0].Payload)
		}
	}

	// Negative: a corrupted checksum is refused despite a legal IV.
	padLen := 16 - ((len(innerBuf) + 1) % 16)
	if padLen == 16 {
		padLen = 0
	}
	raw := buildCBCSKWithIVAndPad(t, sa, innerBuf, 4,
		bytes.Repeat([]byte{0x11}, 16), padLen)
	raw[len(raw)-1] ^= 0xff
	if _, err := decryptAndParse(peer, parseMsg(t, raw), raw); err == nil {
		t.Error("a message with a corrupted integrity checksum was accepted; IV tolerance " +
			"must not weaken authentication")
	}
}

// RFC requirement: RFC7296-3.14-4 positive -- the Padding makes the payloads, the Padding and the Pad
// Length a multiple of the encryption block size. buildSKMessageCBCWithMsgID computes padLen
// from (contentLen+1) mod 16 (auth.go:536-541). The encrypted region of every message is
// therefore block-aligned, whatever the inner length.
// RFC requirement: RFC7296-3.14-6 positive -- Ze computes the checksum over the encrypted message. The
// integrity MAC is taken over buf[:totalLen-integTrunc] AFTER CryptBlocks has replaced the
// plaintext with ciphertext (auth.go:562-568). It therefore covers the encrypted bytes and the
// header, never the plaintext.
//
// RFC requirement: RFC7296-3.14-4 negative -- the alignment comes from the padding, and not from a
// restriction on the input. Inner lengths across a whole block are all accepted, and the pad
// length varies to suit each one.
// RFC requirement: RFC7296-3.14-6 negative -- the checksum is over the ENCRYPTED bytes. One flipped
// ciphertext octet fails the check, so the MAC is not computed over a plaintext copy. A copy of
// that kind would stay untouched by a ciphertext change.
func TestSKPaddingAlignsAndChecksumCoversCiphertext(t *testing.T) {
	sa, peer := cbcSKPair(t)
	integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}

	padLens := make(map[int]bool, 16)
	for n := range 20 {
		// Vary the inner length across more than one block by padding the Delete SPI list.
		spis := bytes.Repeat([]byte{9}, 4*n)
		del := &wire.PayloadDelete{ProtocolID: wire.ProtocolESP, SPISize: 4, NumSPIs: uint16(n), SPIs: spis}
		raw, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: del}},
			uint32(n), wire.ExchangeInformational, wire.FlagInitiator)
		if err != nil {
			t.Fatalf("buildEncryptedMessageEx(n=%d): %v", n, err)
		}
		encRegion := len(raw) - skIVOffset - 16 - integTrunc
		if encRegion%16 != 0 {
			t.Errorf("n=%d: encrypted region is %d octets, not a multiple of the 16-octet block "+
				"size; the Padding length must make it align", n, encRegion)
		}
		// Recover the pad length the builder chose.
		inner, err := decryptAndParse(peer, parseMsg(t, raw), raw)
		if err != nil {
			t.Fatalf("n=%d: decryptAndParse: %v", n, err)
		}
		if len(inner) != 1 {
			t.Fatalf("n=%d: recovered %d inner payloads, want 1", n, len(inner))
		}
		innerLen := wire.GenericHeaderLen + del.Len()
		pad := 16 - ((innerLen + 1) % 16)
		if pad == 16 {
			pad = 0
		}
		padLens[pad] = true
	}
	if len(padLens) < 4 {
		t.Errorf("only %d distinct pad lengths were exercised; the alignment assertion needs "+
			"the pad length to vary", len(padLens))
	}

	// The checksum covers the encrypted bytes: flipping one ciphertext octet fails verification.
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}
	raw, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: del}},
		99, wire.ExchangeInformational, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	if _, err := decryptAndParse(peer, parseMsg(t, raw), raw); err != nil {
		t.Fatalf("the unmodified message did not verify: %v", err)
	}
	ctOff := skIVOffset + 16
	tampered := append([]byte(nil), raw...)
	tampered[ctOff] ^= 0x01
	if _, err := decryptAndParse(peer, parseMsg(t, tampered), tampered); err == nil {
		t.Error("a flipped ciphertext octet still verified; the checksum must be computed over " +
			"the encrypted message")
	}
	// The IV is inside the checksummed region too, since it precedes the Pad Length.
	ivTampered := append([]byte(nil), raw...)
	ivTampered[skIVOffset] ^= 0x01
	if _, err := decryptAndParse(peer, parseMsg(t, ivTampered), ivTampered); err == nil {
		t.Error("a flipped IV octet still verified; the checksum runs from the fixed header " +
			"through the Pad Length")
	}
}

// RFC requirement: RFC7296-3.14-5 positive -- the recipient accepts any Pad Length that results in
// proper alignment. decryptSKPayload reads the trailing Pad Length octet and slices it off
// (auth.go:676-681). A peer that pads with a full extra block rather than the minimum is
// decrypted correctly.
// RFC requirement: RFC7296-3.14-5 negative -- Ze refuses a Pad Length that does NOT result in proper
// alignment. It does not mis-parse the message, and a value that reaches back past the start
// of the plaintext yields errInvalidMessage.
func TestSKAcceptsAnyAligningPadLength(t *testing.T) {
	sa, peer := cbcSKPair(t)
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}
	innerBuf := make([]byte, wire.GenericHeaderLen+del.Len())
	gh := wire.GenericHeader{Length: uint16(wire.GenericHeaderLen + del.Len())}
	gh.WriteTo(innerBuf, 0)
	del.WriteTo(innerBuf, wire.GenericHeaderLen)

	minPad := 16 - ((len(innerBuf) + 1) % 16)
	if minPad == 16 {
		minPad = 0
	}
	// The minimum, and the minimum plus one and two whole extra blocks: all align.
	for _, extra := range []int{0, 16, 32} {
		padLen := minPad + extra
		raw := buildCBCSKWithIVAndPad(t, sa, innerBuf, uint32(extra),
			bytes.Repeat([]byte{0x22}, 16), padLen)
		inner, err := decryptAndParse(peer, parseMsg(t, raw), raw)
		if err != nil {
			t.Errorf("pad length %d: decryptAndParse = %v, want the message accepted", padLen, err)
			continue
		}
		if len(inner) != 1 {
			t.Errorf("pad length %d: recovered %d inner payloads, want 1", padLen, len(inner))
			continue
		}
		if _, ok := inner[0].Payload.(*wire.PayloadDelete); !ok {
			t.Errorf("pad length %d: inner payload = %T, want *wire.PayloadDelete", padLen, inner[0].Payload)
		}
	}

	// Negative: a Pad Length longer than the plaintext is refused.
	raw := buildCBCSKWithIVAndPad(t, sa, innerBuf, 7,
		bytes.Repeat([]byte{0x33}, 16), minPad)
	msg := parseMsg(t, raw)
	sk, ok := msg.Payloads[0].Payload.(*wire.PayloadSK)
	if !ok {
		t.Fatalf("sole payload = %T, want *wire.PayloadSK", msg.Payloads[0].Payload)
	}
	// Re-encrypt with an over-long Pad Length so the MAC stays valid and only the pad is wrong.
	overlong := forgePadLength(t, sa, peer, raw, sk, 0xff)
	if _, err := decryptAndParse(peer, parseMsg(t, overlong), overlong); err == nil {
		t.Error("a Pad Length reaching past the start of the plaintext was accepted; only an " +
			"aligning Pad Length may be")
	}
}

// forgePadLength rewrites the final plaintext octet of a CBC SK message to padLen. It then
// re-encrypts and re-MACs the message. The receive path therefore sees a well-authenticated
// message whose Pad Length is the only thing wrong.
func forgePadLength(t *testing.T, sa, peer *SA, raw []byte, sk *wire.PayloadSK, padLen byte) []byte {
	t.Helper()
	integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}
	ct := sk.CipherText[:len(sk.CipherText)-integTrunc]
	plain, err := ikecrypto.DecryptAESCBCRaw(skRecvEncKey(peer), ct)
	if err != nil {
		t.Fatalf("DecryptAESCBCRaw: %v", err)
	}
	plain[len(plain)-1] = padLen

	out := append([]byte(nil), raw...)
	dataOff := skIVOffset + 16
	block, err := aes.NewCipher(skSendEncKey(sa))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	copy(out[dataOff:], plain)
	gocipher.NewCBCEncrypter(block, out[skIVOffset:skIVOffset+16]).
		CryptBlocks(out[dataOff:dataOff+len(plain)], out[dataOff:dataOff+len(plain)])

	mac, err := ikecrypto.ComputeIntegrity(sa.Proposal.Integrity.ID, skSendIntegKey(sa), out[:len(out)-integTrunc])
	if err != nil {
		t.Fatalf("ComputeIntegrity: %v", err)
	}
	copy(out[len(out)-integTrunc:], mac)
	return out
}

// RFC requirement: RFC7296-2.10-3 positive -- the nonce is at least half the key size of the
// negotiated PRF. nonceLen is 32 octets (fsm.go:33), and every PRF this implementation can
// negotiate has a preferred key size of at most 64 octets (crypto/transform.go:126-130). So
// 32 >= KeyLength/2 for each of them. newInitiatorSA, newResponderSA and every rekey nonce
// all use that same nonceLen (initiator.go:24, responder.go:30, rekey.go:55, :158, :296, :469).
// RFC requirement: RFC7296-2.10-3 negative -- the bound is against the PRF, not against the 16-octet
// wire minimum. For the largest PRF this implementation supports the bound is exactly met, so
// a nonce one octet shorter would violate it.
func TestNonceMeetsHalfPRFKeySize(t *testing.T) {
	for _, name := range []string{"sha256", "sha384", "sha512"} {
		prf, err := ikecrypto.LookupPRF(name)
		if err != nil {
			t.Fatalf("LookupPRF(%q): %v", name, err)
		}
		half := int(prf.KeyLength) / 2
		if nonceLen < half {
			t.Errorf("nonceLen = %d octets but PRF %s needs at least %d (half of its %d-octet "+
				"key)", nonceLen, name, half, prf.KeyLength)
		}
	}

	// The bound is tight for the largest supported PRF, so it is a real constraint.
	sha512, err := ikecrypto.LookupPRF("sha512")
	if err != nil {
		t.Fatalf("LookupPRF: %v", err)
	}
	if nonceLen != int(sha512.KeyLength)/2 {
		t.Logf("nonceLen %d exceeds half the sha512 key size %d; the bound holds with slack",
			nonceLen, int(sha512.KeyLength)/2)
	}
	if nonceLen-1 >= int(sha512.KeyLength)/2 {
		t.Errorf("a nonce of %d octets would still satisfy the sha512 bound, so nonceLen is "+
			"not sized against the PRF", nonceLen-1)
	}

	// Every SA the engine creates really uses nonceLen.
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "nonce-psk")
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	if len(ini.LocalNonce) != nonceLen {
		t.Errorf("initiator nonce = %d octets, want %d", len(ini.LocalNonce), nonceLen)
	}
	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	if len(resp.LocalNonce) != nonceLen {
		t.Errorf("responder nonce = %d octets, want %d", len(resp.LocalNonce), nonceLen)
	}
}
