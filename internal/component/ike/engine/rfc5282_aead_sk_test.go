// VALIDATES: the RFC 5282 obligations the IKE engine meets on the Encrypted (SK) payload
// when the negotiated cipher is an authenticated encryption algorithm. The list covers the
// eight-octet IV and its uniqueness (§3.1), the full-length 16 octet ICV (§3.2), the
// salt-then-IV 12 octet nonce (§4), the associated data span and the exclusion of the IV
// and the Ciphertext from it (§5.1), acceptance of any Padding up to 255 octets (§3), the
// mandatory Key Length attribute (§7.3), and the absence of a selected integrity algorithm
// beside an AEAD cipher (§8).
// PREVENTS: a change to the AEAD send or receive path that reuses an IV, truncates the ICV,
// reverses the nonce halves, authenticates the wrong span of the message, or drops the Key
// Length attribute from an AES-GCM transform.
package engine

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	crand "crypto/rand"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// aeadIVOctets is the IV length RFC 5282 Section 3.1 fixes for AES GCM and AES CCM.
	aeadIVOctets = 8
	// aeadICVOctets is the full-length ICV of RFC 5282 Section 3.2, which for AES GCM is
	// the authentication tag.
	aeadICVOctets = 16
	// aeadSaltOctets is the salt SK_ei and SK_er carry beyond the AES key for AES GCM
	// (RFC 5282 Section 7.1), and the implicit half of the nonce (Section 4).
	aeadSaltOctets = 4
)

// establishAEAD runs the in-process PSK handshake under an AES-GCM IKE group and returns
// both established SAs. The pair shares one SK hierarchy, so the initiator's send key is
// the responder's receive key.
//
// It repeats establishPSK (responder_test.go) rather than parameterising it, because the
// group is the one thing the two differ by and establishPSK is the fixture a dozen other
// tests already run: several sessions share this checkout, and a shape change there is a
// change to their fixture rather than to this file.
func establishAEAD(t *testing.T) (ini, resp *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := aeadIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rfc5282-aead")

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, log)

	if ini.State != StateEstablished || resp.State != StateEstablished {
		t.Fatalf("the AES-GCM handshake did not establish: ini=%v resp=%v", ini.State, resp.State)
	}
	if !ini.Proposal.Encryption.IsAEAD {
		t.Fatal("establishAEAD negotiated a cipher that is not AEAD, so no RFC 5282 obligation is under test")
	}
	return ini, resp
}

// aeadSK builds one AEAD SK message the way a peer builds one. Every zero field takes the
// conformant value, so a test names only the octets its own obligation varies.
//
// It exists because the RFC 5282 receive path has to be fed messages ze itself would never
// send: an eleven-octet nonce, a reversed nonce, an ICV of thirteen octets, a payload
// sitting between the fixed header and the Encrypted payload. buildSKMessageAEADWithMsgID
// emits the conformant shape alone.
type aeadSK struct {
	sa        *SA
	inner     []byte              // the inner payload chain, already encoded
	firstType uint8               // the SK generic header's Next Payload
	msgID     uint32              //
	before    []wire.PayloadEntry // unencrypted payloads between the fixed header and SK
	iv        []byte              // nil: eight fresh octets
	padding   []byte              // the Padding field; the Pad Length octet follows it
	nonce     []byte              // nil: salt || iv
	aadEnd    int                 // 0: through the last octet of the SK generic header
	icvOctets int                 // 0: 16
}

// build answers the complete message octets.
func (b aeadSK) build(t *testing.T) []byte {
	t.Helper()
	iv := b.iv
	if iv == nil {
		iv = make([]byte, aeadIVOctets)
		if _, err := crand.Read(iv); err != nil {
			t.Fatalf("crand.Read: %v", err)
		}
	}
	icvOctets := b.icvOctets
	if icvOctets == 0 {
		icvOctets = aeadICVOctets
	}

	// RFC 5282 Section 3, Figure 2: the plaintext is the inner payloads, the Padding,
	// and the one-octet Pad Length.
	plaintext := make([]byte, 0, len(b.inner)+len(b.padding)+1)
	plaintext = append(plaintext, b.inner...)
	plaintext = append(plaintext, b.padding...)
	plaintext = append(plaintext, byte(len(b.padding)))

	prefixOctets := wire.HeaderLen
	for i := range b.before {
		prefixOctets += wire.GenericHeaderLen + b.before[i].Payload.Len()
	}
	prefixOctets += wire.GenericHeaderLen

	totalOctets := prefixOctets + len(iv) + len(plaintext) + icvOctets
	buf := make([]byte, totalOctets)

	firstOuter := wire.PayloadTypeSK
	if len(b.before) > 0 {
		firstOuter = b.before[0].Payload.Type()
	}
	hdr := wire.Header{
		InitiatorSPI: b.sa.InitiatorSPI,
		ResponderSPI: b.sa.ResponderSPI,
		MajorVersion: 2,
		ExchangeType: wire.ExchangeInformational,
		Flags:        initiatorFlag(b.sa),
		MessageID:    b.msgID,
		NextPayload:  firstOuter,
		Length:       uint32(totalOctets),
	}
	hdr.WriteTo(buf, 0)

	off := wire.HeaderLen
	for i := range b.before {
		next := wire.PayloadTypeSK
		if i+1 < len(b.before) {
			next = b.before[i+1].Payload.Type()
		}
		gh := wire.GenericHeader{
			NextPayload: next,
			Length:      uint16(wire.GenericHeaderLen + b.before[i].Payload.Len()),
		}
		gh.WriteTo(buf, off)
		b.before[i].Payload.WriteTo(buf, off+wire.GenericHeaderLen)
		off += wire.GenericHeaderLen + b.before[i].Payload.Len()
	}
	skGH := wire.GenericHeader{NextPayload: b.firstType, Length: uint16(totalOctets - off)}
	skGH.WriteTo(buf, off)
	off += wire.GenericHeaderLen

	copy(buf[off:], iv)

	sendKey := skSendEncKey(b.sa)
	nonce := b.nonce
	if nonce == nil {
		nonce = make([]byte, 0, aeadSaltOctets+len(iv))
		nonce = append(nonce, sendKey[len(sendKey)-aeadSaltOctets:]...)
		nonce = append(nonce, iv...)
	}
	aadEnd := b.aadEnd
	if aadEnd == 0 {
		aadEnd = prefixOctets
	}

	block, err := aes.NewCipher(sendKey[:len(sendKey)-aeadSaltOctets])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm := gcmFor(t, block, len(nonce), icvOctets)
	sealed := gcm.Seal(nil, nonce, plaintext, buf[:aadEnd])
	copy(buf[off+len(iv):], sealed)
	return buf
}

// gcmFor answers the AES-GCM instance one message shape needs. The standard library
// exposes the nonce size and the tag size through separate constructors, and no message
// here varies both at once.
func gcmFor(t *testing.T, block gocipher.Block, nonceOctets, icvOctets int) gocipher.AEAD {
	t.Helper()
	var (
		gcm gocipher.AEAD
		err error
	)
	switch {
	case nonceOctets != 12:
		gcm, err = gocipher.NewGCMWithNonceSize(block, nonceOctets)
	case icvOctets != aeadICVOctets:
		gcm, err = gocipher.NewGCMWithTagSize(block, icvOctets)
	default:
		gcm, err = gocipher.NewGCM(block)
	}
	if err != nil {
		t.Fatalf("build an AES-GCM instance with a %d octet nonce and a %d octet ICV: %v",
			nonceOctets, icvOctets, err)
	}
	return gcm
}

// oneDeletePayload encodes a one-payload inner chain, and answers it with the payload type
// the SK generic header must name.
func oneDeletePayload(t *testing.T) ([]byte, uint8) {
	t.Helper()
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}
	buf := make([]byte, wire.GenericHeaderLen+del.Len())
	gh := wire.GenericHeader{Length: uint16(wire.GenericHeaderLen + del.Len())}
	gh.WriteTo(buf, 0)
	del.WriteTo(buf, wire.GenericHeaderLen)
	return buf, wire.PayloadTypeDelete
}

// expectOneDelete runs the receive path over one message and requires the single Delete
// payload back.
func expectOneDelete(t *testing.T, receiver *SA, raw []byte) {
	t.Helper()
	inner, err := decryptAndParse(receiver, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decryptAndParse = %v, want the message accepted", err)
	}
	if len(inner) != 1 {
		t.Fatalf("recovered %d inner payloads, want 1", len(inner))
	}
	if _, ok := inner[0].Payload.(*wire.PayloadDelete); !ok {
		t.Fatalf("inner payload = %T, want *wire.PayloadDelete", inner[0].Payload)
	}
}

// expectRefused runs the receive path over one message and requires it to be refused.
//
// The outer parse MUST succeed first. A message the wire parser rejects never reaches the
// AEAD open, so counting that as the refusal under test would let a malformed fixture stand
// in for the obligation's own branch (ai/rules/principles.md).
func expectRefused(t *testing.T, receiver *SA, raw []byte, what string) {
	t.Helper()
	msg := parseMsg(t, raw)
	if _, err := decryptAndParse(receiver, msg, raw); err == nil {
		t.Fatalf("%s was accepted, want a refusal", what)
	}
}

// RFC requirement: RFC5282-3-1 positive -- the receive path accepts every Padding length
// from 0 to 255 octets. Each message carries the same single Delete payload followed by
// that many Padding octets and the Pad Length that counts them, and decryptAndParse
// recovers the payload from all 256 of them.
//
// RFC 5282 Section 3: "Pad Length is the number of octets in the Padding field. There are
// no alignment requirements on the length of the Padding field; the recipient MUST accept
// any amount of Padding up to 255 octets."
//
// The Padding octets are non-zero and vary with position, so a receiver that recovered the
// payload by treating trailing zeros as absent would not pass.
func TestRFC5282AEADReceiveAcceptsAnyPaddingTo255(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	for padOctets := range 256 {
		padding := make([]byte, padOctets)
		for i := range padding {
			padding[i] = byte(0xa0 + i%0x50)
		}
		raw := aeadSK{
			sa: ini, inner: inner, firstType: firstType,
			msgID: uint32(padOctets), padding: padding,
		}.build(t)
		inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
		if err != nil {
			t.Fatalf("%d octets of Padding: decryptAndParse = %v, want the message accepted",
				padOctets, err)
		}
		if len(inner) != 1 {
			t.Fatalf("%d octets of Padding: recovered %d inner payloads, want 1",
				padOctets, len(inner))
		}
	}
}

// RFC requirement: RFC5282-3-1 negative -- accepting any Padding is not accepting anything
// after the payload chain. A message whose inner payload declares a length running past the
// recovered plaintext is refused, so the octets after a terminated chain are read as
// Padding only when the chain itself terminated.
//
// Without this polarity a receiver that returned the payloads it had read so far, and
// discarded the rest of the plaintext, would satisfy the positive case while accepting a
// truncated message.
func TestRFC5282AEADReceiveRefusesATruncatedInnerPayload(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	overlong := append([]byte(nil), inner...)
	gh := wire.GenericHeader{Length: uint16(len(inner) + 64)}
	gh.WriteTo(overlong, 0)

	raw := aeadSK{
		sa: ini, inner: overlong, firstType: firstType, msgID: 1,
		padding: bytes.Repeat([]byte{0xcc}, 16),
	}.build(t)
	expectRefused(t, resp, raw, "an inner payload whose declared length runs past the plaintext")
}

// RFC requirement: RFC5282-3.1-1 positive -- every AEAD message ze builds carries an
// Initialization Vector of exactly eight octets. The SK payload's data is the IV, the
// ciphertext and the ICV, so the message length less the prefix, the plaintext and the
// 16 octet ICV is the IV length, and an independent AES-GCM instance opens the message
// with the first eight octets of that data as the explicit half of the nonce.
//
// RFC 5282 Section 3.1: "The Initialization Vector (IV) MUST be eight octets."
//
// The message is the one the established initiator really sent, so the length under
// measurement is the builder's own output and not a fixture's.
func TestRFC5282AEADSendIVIsEightOctets(t *testing.T) {
	ini, _ := establishAEAD(t)
	raw := ini.LastSentMsg
	if len(raw) == 0 {
		t.Fatal("the established initiator sent no message, so there is no IV to measure")
	}
	msg := parseMsg(t, raw)
	sk, ok := msg.Payloads[len(msg.Payloads)-1].Payload.(*wire.PayloadSK)
	if !ok {
		t.Fatalf("last payload = %T, want *wire.PayloadSK", msg.Payloads[len(msg.Payloads)-1].Payload)
	}

	sendKey := skSendEncKey(ini)
	block, err := aes.NewCipher(sendKey[:len(sendKey)-aeadSaltOctets])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := gocipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gocipher.NewGCM: %v", err)
	}
	nonce := make([]byte, 0, aeadSaltOctets+aeadIVOctets)
	nonce = append(nonce, sendKey[len(sendKey)-aeadSaltOctets:]...)
	nonce = append(nonce, sk.CipherText[:aeadIVOctets]...)
	plain, err := gcm.Open(nil, nonce, sk.CipherText[aeadIVOctets:], raw[:sk.DataOffset])
	if err != nil {
		t.Fatalf("opening the message with an eight octet IV failed: %v", err)
	}
	if gotIVOctets := len(sk.CipherText) - (len(plain) + aeadICVOctets); gotIVOctets != aeadIVOctets {
		t.Fatalf("the SK payload carries a %d octet Initialization Vector, want %d",
			gotIVOctets, aeadIVOctets)
	}
}

// RFC requirement: RFC5282-3.1-1 negative -- the eight octets are a fixed field rather than
// whatever the sender chose. The receive path refuses a message whose Initialization Vector
// field is seven octets and one whose field is nine, each sealed under the nonce that field
// length implies, so the split between the IV and the Ciphertext is at octet eight and
// nowhere else.
func TestRFC5282AEADReceiveRefusesAnIVThatIsNotEightOctets(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	for _, ivOctets := range []int{7, 9} {
		iv := bytes.Repeat([]byte{0x5a}, ivOctets)
		sendKey := skSendEncKey(ini)
		nonce := make([]byte, 0, aeadSaltOctets+ivOctets)
		nonce = append(nonce, sendKey[len(sendKey)-aeadSaltOctets:]...)
		nonce = append(nonce, iv...)
		raw := aeadSK{
			sa: ini, inner: inner, firstType: firstType, msgID: uint32(ivOctets),
			iv: iv, nonce: nonce,
		}.build(t)
		expectRefused(t, resp, raw, "a message whose Initialization Vector field is not eight octets")
	}
}

// RFC requirement: RFC5282-3.1-2 positive -- the Initialization Vector of every message ze
// sends under one key is fresh. 512 consecutive AEAD messages built from one SA, and so
// under one SK_ei, carry 512 distinct IVs.
//
// RFC 5282 Section 3.1: "The IV MUST be chosen by the encryptor in a manner that ensures
// that the same IV value is used only once for a given key."
//
// One SA is one key, so the 512 rounds share SK_ei and the repeat would be the failure
// the sentence names.
func TestRFC5282AEADSendIVIsUniquePerKey(t *testing.T) {
	ini, _ := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	const rounds = 512
	seen := make(map[string]int, rounds)
	for round := range rounds {
		raw, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, uint32(round),
			wire.ExchangeInformational, initiatorFlag(ini))
		if err != nil {
			t.Fatalf("round %d: buildSKMessageAEADWithMsgID: %v", round, err)
		}
		iv := string(raw[skIVOffset : skIVOffset+aeadIVOctets])
		if earlier, repeated := seen[iv]; repeated {
			t.Fatalf("round %d repeats the IV of round %d under one key", round, earlier)
		}
		seen[iv] = round
	}
	if len(seen) != rounds {
		t.Fatalf("%d distinct IVs over %d messages under one key", len(seen), rounds)
	}
}

// RFC requirement: RFC5282-3.1-2 negative -- the IV is not derived from what the message
// carries. Two messages built from byte-identical arguments, so the same inner payloads and
// the same Message ID under the same key, still carry different IVs. A retransmission
// therefore cannot repeat one.
//
// Without this polarity an IV computed from the plaintext would satisfy the positive case,
// because 512 rounds there each carry a different Message ID.
func TestRFC5282AEADSendIVDoesNotRepeatForARepeatedMessage(t *testing.T) {
	ini, _ := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	first, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 7,
		wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildSKMessageAEADWithMsgID: %v", err)
	}
	second, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 7,
		wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildSKMessageAEADWithMsgID: %v", err)
	}
	if bytes.Equal(first[skIVOffset:skIVOffset+aeadIVOctets], second[skIVOffset:skIVOffset+aeadIVOctets]) {
		t.Fatalf("two builds of the same message under one key share the IV %x",
			first[skIVOffset:skIVOffset+aeadIVOctets])
	}
}

// RFC requirement: RFC5282-3.2-1 positive -- every AEAD message ze builds carries a
// full-length 16 octet ICV, and the receive path opens it. The message length is the
// 32 octet prefix, the eight octet IV, the plaintext and 16 octets, and an AES-GCM
// instance built with the default 16 octet tag opens the ciphertext.
//
// RFC 5282 Section 3.2: "The AES GCM ICV consists solely of the AES GCM Authentication Tag.
// Implementations MUST support a full-length 16 octet ICV".
func TestRFC5282AEADSendICVIsSixteenOctets(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	raw, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 3,
		wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildSKMessageAEADWithMsgID: %v", err)
	}
	// The builder appends one Pad Length octet of zero to the inner data, so the
	// plaintext is one octet longer than the chain.
	wantOctets := skIVOffset + aeadIVOctets + len(inner) + 1 + aeadICVOctets
	if len(raw) != wantOctets {
		t.Fatalf("the message is %d octets, want %d: a %d octet prefix, an %d octet IV, "+
			"a %d octet plaintext and a %d octet ICV",
			len(raw), wantOctets, skIVOffset, aeadIVOctets, len(inner)+1, aeadICVOctets)
	}
	expectOneDelete(t, resp, raw)
}

// RFC requirement: RFC5282-3.2-1 negative -- the 16 octets are verified rather than carried.
// The same message with its last ICV octet inverted, and the same message with the ICV
// removed altogether, are both refused.
func TestRFC5282AEADReceiveRefusesADamagedICV(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	raw, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 4,
		wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildSKMessageAEADWithMsgID: %v", err)
	}

	flipped := append([]byte(nil), raw...)
	flipped[len(flipped)-1] ^= 0xff
	expectRefused(t, resp, flipped, "a message whose ICV octet was inverted")

	stripped := append([]byte(nil), raw[:len(raw)-aeadICVOctets]...)
	hdr := wire.Header{}
	if err := hdr.ReadFrom(stripped); err != nil {
		t.Fatalf("parse the stripped header: %v", err)
	}
	hdr.Length = uint32(len(stripped))
	hdr.WriteTo(stripped, 0)
	gh := wire.GenericHeader{NextPayload: firstType, Length: uint16(len(stripped) - wire.HeaderLen)}
	gh.WriteTo(stripped, wire.HeaderLen)
	expectRefused(t, resp, stripped, "a message with no ICV at all")
}

// RFC requirement: RFC5282-4-1 positive -- the nonce is the salt concatenated with the IV,
// in that order. A message sealed under salt || IV is accepted by the receive path, and the
// salt is the four octets SK_ei carries beyond the AES key.
//
// RFC 5282 Section 4: "When this default nonce format is used, both the encryptor and
// decryptor construct the nonce by concatenating the salt with the IV, in that order."
//
// The salt is read out of the derived key material rather than named, so a change to the
// key layout moves this test with it.
func TestRFC5282AEADNonceIsSaltThenIV(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x3c}, aeadIVOctets)
	sendKey := skSendEncKey(ini)
	nonce := make([]byte, 0, aeadSaltOctets+aeadIVOctets)
	nonce = append(nonce, sendKey[len(sendKey)-aeadSaltOctets:]...)
	nonce = append(nonce, iv...)

	raw := aeadSK{sa: ini, inner: inner, firstType: firstType, msgID: 5, iv: iv, nonce: nonce}.build(t)
	expectOneDelete(t, resp, raw)
}

// RFC requirement: RFC5282-4-1 negative -- the ORDER of the two halves is load-bearing. The
// same message sealed under IV || salt, which is the same twelve octets the other way
// round, is refused rather than opened. A receiver that built a nonce of the right length
// out of the two halves in either order would not pass.
func TestRFC5282AEADRefusesAReversedNonce(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x3c}, aeadIVOctets)
	sendKey := skSendEncKey(ini)
	reversed := make([]byte, 0, aeadSaltOctets+aeadIVOctets)
	reversed = append(reversed, iv...)
	reversed = append(reversed, sendKey[len(sendKey)-aeadSaltOctets:]...)

	raw := aeadSK{sa: ini, inner: inner, firstType: firstType, msgID: 6, iv: iv, nonce: reversed}.build(t)
	expectRefused(t, resp, raw, "a message sealed under IV || salt rather than salt || IV")
}

// RFC requirement: RFC5282-4-2 positive -- the AES-GCM nonce is twelve octets: the four
// octet salt and the eight octet IV. A message sealed under those twelve octets is
// accepted, and the two lengths are read from the key material and the wire rather than
// asserted.
//
// RFC 5282 Section 4: "For the use of AES GCM with the IKEv2 Encrypted Payload, this
// default nonce format MUST be used and a 12 octet nonce MUST be used."
//
// The twelve is asserted before the message is built, so a salt or an IV of another
// length fails here rather than reaching the AEAD as a shorter nonce.
func TestRFC5282AEADNonceIsTwelveOctets(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x91}, aeadIVOctets)
	sendKey := skSendEncKey(ini)
	nonce := make([]byte, 0, aeadSaltOctets+aeadIVOctets)
	nonce = append(nonce, sendKey[len(sendKey)-aeadSaltOctets:]...)
	nonce = append(nonce, iv...)
	if len(nonce) != 12 {
		t.Fatalf("the salt and the IV give a %d octet nonce, want 12", len(nonce))
	}

	raw := aeadSK{sa: ini, inner: inner, firstType: firstType, msgID: 8, iv: iv, nonce: nonce}.build(t)
	expectOneDelete(t, resp, raw)
}

// RFC requirement: RFC5282-4-2 negative -- twelve is exact rather than a floor. A message
// sealed under an eleven octet nonce, which is the AES CCM length of Section 4 built from a
// three octet salt, is refused, and so is one sealed under a thirteen octet nonce. Both keep
// salt-then-IV order, so only the length differs.
func TestRFC5282AEADRefusesANonceThatIsNotTwelveOctets(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x91}, aeadIVOctets)
	sendKey := skSendEncKey(ini)
	salt := sendKey[len(sendKey)-aeadSaltOctets:]

	for name, nonce := range map[string][]byte{
		"eleven octets":   append(append([]byte(nil), salt[1:]...), iv...),
		"thirteen octets": append(append(append([]byte(nil), byte(0)), salt...), iv...),
	} {
		raw := aeadSK{sa: ini, inner: inner, firstType: firstType, msgID: 9, iv: iv, nonce: nonce}.build(t)
		expectRefused(t, resp, raw, "a message sealed under a nonce of "+name)
	}
}

// RFC requirement: RFC5282-5.1-1 positive -- the associated data runs from the first octet
// of the fixed IKE header through the last octet of the Encrypted payload's own generic
// header, and it includes an unencrypted payload sitting between the two. A message that
// carries a Notify payload in front of the Encrypted payload, sealed under that whole span,
// is accepted.
//
// RFC 5282 Section 5.1: "The associated data (A) MUST consist of the partial contents of
// the IKEv2 message, starting from the first octet of the Fixed IKE Header through the last
// octet of the Payload Header of the Encrypted Payload (i.e., the fourth octet of the
// Encrypted Payload) ... This includes any payloads that are between the Fixed IKE Header
// and the Encrypted Payload."
//
// The Notify payload in front of the Encrypted payload is what puts the last clause under
// test: without it the span is the fixed 32 octets and every implementation agrees.
func TestRFC5282AEADAssociatedDataCoversAnInterveningPayload(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	notify := &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifyCookie,
		NotificationData: bytes.Repeat([]byte{0x77}, 24),
	}
	raw := aeadSK{
		sa: ini, inner: inner, firstType: firstType, msgID: 10,
		before: []wire.PayloadEntry{{Payload: notify}},
	}.build(t)
	if len(raw) <= skIVOffset+aeadIVOctets+len(inner)+1+aeadICVOctets {
		t.Fatal("the message carries no payload in front of the Encrypted payload, so the obligation is not under test")
	}
	expectOneDelete(t, resp, raw)
}

// RFC requirement: RFC5282-5.1-1 negative -- the span really covers the intervening payload
// rather than stopping at the fixed 32 octets an Encrypted payload would sit at on its own.
// One octet of the Notify's data is inverted after sealing, and the message is refused.
//
// Without this polarity a receiver that authenticated the first 32 octets alone would pass
// the positive case, because the sealer and the receiver would then disagree about nothing
// a conformant peer sends.
func TestRFC5282AEADRefusesAnAlteredInterveningPayload(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	notify := &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifyCookie,
		NotificationData: bytes.Repeat([]byte{0x77}, 24),
	}
	raw := aeadSK{
		sa: ini, inner: inner, firstType: firstType, msgID: 11,
		before: []wire.PayloadEntry{{Payload: notify}},
	}.build(t)
	altered := append([]byte(nil), raw...)
	altered[wire.HeaderLen+wire.GenericHeaderLen+6] ^= 0xff
	expectRefused(t, resp, altered, "a message whose intervening Notify payload was altered after sealing")
}

// RFC requirement: RFC5282-5.1-2 positive -- the Initialization Vector and the Ciphertext
// are outside the associated data. A message whose sealer stopped the span at the last
// octet of the Encrypted payload's generic header is accepted, so neither field is part of
// what ze authenticates as associated data.
//
// RFC 5282 Section 5.1: "The Initialization Vector and Ciphertext fields shown in Figure 1
// (above) MUST NOT be included in the associated data."
//
// The sealer here stops the span where the RFC stops it, so the message is accepted only
// by a receiver that stops it in the same place.
func TestRFC5282AEADAssociatedDataExcludesTheIVAndCiphertext(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	raw := aeadSK{
		sa: ini, inner: inner, firstType: firstType, msgID: 12,
		aadEnd: wire.HeaderLen + wire.GenericHeaderLen,
	}.build(t)
	expectOneDelete(t, resp, raw)
}

// RFC requirement: RFC5282-5.1-2 negative -- the exclusion is enforced rather than
// incidental. A message whose sealer extended the associated data over the eight octet
// Initialization Vector is refused, so a receiver that included the IV would disagree with
// this one about which messages are authentic.
func TestRFC5282AEADRefusesAssociatedDataCoveringTheIV(t *testing.T) {
	ini, resp := establishAEAD(t)
	inner, firstType := oneDeletePayload(t)

	raw := aeadSK{
		sa: ini, inner: inner, firstType: firstType, msgID: 13,
		aadEnd: wire.HeaderLen + wire.GenericHeaderLen + aeadIVOctets,
	}.build(t)
	expectRefused(t, resp, raw, "a message whose associated data covered the Initialization Vector")
}

// RFC requirement: RFC5282-7.3-1 positive -- every AES-GCM encryption transform ze puts on
// the wire carries a Key Length attribute, in an IKE proposal and in an ESP proposal alike,
// at both key sizes the config offers.
//
// RFC 5282 Section 7.3: "Because the AES supports three key lengths, the Key Length
// attribute MUST be specified when any of the identifiers for AES GCM or AES CCM, specified
// in Section 7.2 of this document, is used."
//
// Both rails are checked because one function, encAttrs, feeds the IKE proposal and the
// ESP proposal alike, and a peer refuses the transform on either.
func TestRFC5282AEADProposalCarriesTheKeyLengthAttribute(t *testing.T) {
	for _, tc := range []struct {
		algo ipsec.EncryptionAlgo
		bits uint16
	}{
		{ipsec.EncryptionAES128GCM, 128},
		{ipsec.EncryptionAES256GCM, 256},
	} {
		group := aeadIKEGroup()
		group.Proposals[0].Encryption = tc.algo
		ike := buildWireIKEProposals(group)
		if len(ike) != 1 {
			t.Fatalf("%v: buildWireIKEProposals returned %d proposals, want 1", tc.algo, len(ike))
		}
		assertKeyLength(t, "IKE", tc.algo, ike[0], tc.bits)

		esp := espProposalToWire(ipsec.ESPProposal{
			Number: 1, Encryption: tc.algo, Hash: ipsec.HashSHA256,
		}, 0x0a0b0c0d, 1, dhGroupNone)
		assertKeyLength(t, "ESP", tc.algo, esp, tc.bits)
	}
}

// assertKeyLength requires the proposal's encryption transform to carry exactly one Key
// Length attribute of the given value.
func assertKeyLength(t *testing.T, rail string, algo ipsec.EncryptionAlgo, p wire.Proposal, bits uint16) {
	t.Helper()
	found := 0
	for _, transform := range p.Transforms {
		if transform.Type != wire.TransformTypeENCR {
			continue
		}
		if transform.ID != uint16(crypto.ENCR_AES_GCM_16) {
			t.Fatalf("%s %v: encryption Transform ID = %d, want %d",
				rail, algo, transform.ID, crypto.ENCR_AES_GCM_16)
		}
		for _, attr := range transform.Attrs {
			if attr.Type != wire.AttrTypeKeyLength {
				continue
			}
			found++
			if attr.Value != bits {
				t.Fatalf("%s %v: Key Length attribute = %d, want %d", rail, algo, attr.Value, bits)
			}
		}
	}
	if found != 1 {
		t.Fatalf("%s %v: the AES-GCM transform carries %d Key Length attributes, want exactly 1",
			rail, algo, found)
	}
}

// RFC requirement: RFC5282-8-1 positive -- an IKE SA whose selected encryption algorithm is
// an AEAD cipher has no integrity algorithm selected for it. NegotiateIKE over an AES-GCM
// offer answers a proposal whose integrity transform is AUTH_NONE with a zero key length,
// and the established SA derives no SK_ai and no SK_ar.
//
// RFC 5282 Section 8: "This document updates [RFC4306] to require that when an
// authenticated encryption algorithm is selected as the encryption algorithm for any SA
// (IKE or ESP), an integrity algorithm MUST NOT be selected for that SA."
//
// The derived keys are checked beside the selection because a selection that named no
// algorithm while still cutting integrity key material would key the SA differently from
// the peer.
func TestRFC5282AEADSelectionCarriesNoIntegrityAlgorithm(t *testing.T) {
	group := aeadIKEGroup()
	chosen, err := crypto.NegotiateIKE(
		wireProposalsToIKE(buildWireIKEProposals(group)), buildIKEProposals(group))
	if err != nil {
		t.Fatalf("NegotiateIKE over an AES-GCM offer: %v", err)
	}
	if chosen.Integrity.ID != crypto.AUTH_NONE {
		t.Fatalf("the selected integrity algorithm is %v, want none", chosen.Integrity.ID)
	}
	if chosen.Integrity.KeyLength != 0 {
		t.Fatalf("the selected integrity algorithm takes a %d octet key, want 0",
			chosen.Integrity.KeyLength)
	}

	ini, _ := establishAEAD(t)
	if len(ini.SKKeys.SK_ai) != 0 || len(ini.SKKeys.SK_ar) != 0 {
		t.Fatalf("the established AEAD SA holds a %d octet SK_ai and a %d octet SK_ar, want 0 and 0",
			len(ini.SKKeys.SK_ai), len(ini.SKKeys.SK_ar))
	}
}

// RFC requirement: RFC5282-8-1 negative -- the absence is enforced against the peer rather
// than assumed. A peer that offers the same AES-GCM cipher WITH an integrity transform is
// refused, so no integrity algorithm can be selected beside an AEAD cipher.
//
// Without this polarity an implementation that ignored a peer's integrity transform, and
// kept its own AUTH_NONE, would satisfy the positive case while completing a handshake the
// peer believes is integrity protected by that algorithm.
func TestRFC5282AEADOfferWithIntegrityIsRefused(t *testing.T) {
	group := aeadIKEGroup()
	offer := buildWireIKEProposals(group)
	if len(offer) != 1 {
		t.Fatalf("buildWireIKEProposals returned %d proposals, want 1", len(offer))
	}
	offer[0].Transforms = append(offer[0].Transforms, wire.Transform{
		Type: wire.TransformTypeINTG, ID: uint16(crypto.AUTH_HMAC_SHA2_256_128),
	})

	if _, err := crypto.NegotiateIKE(wireProposalsToIKE(offer), buildIKEProposals(group)); err == nil {
		t.Fatal("an AES-GCM offer carrying an integrity transform was accepted, want a refusal")
	}
}
