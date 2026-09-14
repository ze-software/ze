// VALIDATES: the RFC 5282 obligations the IKE engine meets on the Encrypted (SK) payload
// when the negotiated cipher is AES CCM. The list covers the ICV sizes an AES CCM
// implementation must support (§3.2) and the default eleven octet nonce built from the
// salt and the IV (§4).
// PREVENTS: an AES CCM send path that sizes the message with another transform's ICV, a
// receive path that carries the ICV without verifying it, a nonce built IV-then-salt, and
// a nonce of the twelve octets AES GCM takes rather than the eleven AES CCM takes.
package engine

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	crand "crypto/rand"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/ccm"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ccmSaltOctets is the salt SK_ei and SK_er carry beyond the AES key for AES CCM, and the
// implicit half of the nonce. RFC 4309 Section 7.1: "The size of KEYMAT MUST be three
// octets longer than is needed for the associated AES key." RFC 5282 Section 7.1 carries
// that layout into the IKE SA.
const ccmSaltOctets = 3

// ccmNonceOctets is the nonce length RFC 5282 Section 4 fixes for AES CCM. It is written
// as the sum of its two halves, because that is what the section defines it as.
const ccmNonceOctets = ccmSaltOctets + aeadIVOctets

// ccmAlgos names the AES CCM algorithm ze offers for each ICV size RFC 5282 Section 7.2
// assigns a Transform ID, with the ICV that identifier carries.
var ccmAlgos = map[ipsec.EncryptionAlgo]int{
	ipsec.EncryptionAES256CCM8:  8,
	ipsec.EncryptionAES256CCM12: 12,
	ipsec.EncryptionAES256CCM16: 16,
}

// ccmIKEGroup answers an IKE group whose one proposal offers this AES CCM algorithm.
func ccmIKEGroup(algo ipsec.EncryptionAlgo) ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike-ccm",
		Proposals: []ipsec.IKEProposal{{
			Number:     1,
			Encryption: algo,
			Hash:       ipsec.HashSHA256,
			DHGroup:    14,
		}},
	}
}

// establishCCM runs the in-process PSK handshake under an AES CCM IKE group and returns
// both established SAs. The pair shares one SK hierarchy, so the initiator's send key is
// the responder's receive key.
//
// The handshake is the real entry point: IKE_SA_INIT negotiates the transform off the
// wire, DeriveSKKeys cuts SK_ei and SK_er at the AES CCM KEYMAT length, and IKE_AUTH is
// carried in an Encrypted payload this cipher sealed. A test that keyed the cipher by hand
// would prove the mode and not the daemon (ai/rules/principles.md).
func establishCCM(t *testing.T, algo ipsec.EncryptionAlgo) (ini, resp *SA) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := ccmIKEGroup(algo)
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rfc5282-ccm")

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
		t.Fatalf("the %s handshake did not establish: ini=%v resp=%v", algo, ini.State, resp.State)
	}
	if !ini.Proposal.Encryption.IsAEAD {
		t.Fatalf("%s negotiated a cipher that is not AEAD, so no RFC 5282 obligation is under test", algo)
	}
	return ini, resp
}

// ccmSK builds one AES CCM SK message the way a peer builds one, so the receive path can
// be fed the shapes ze itself never sends: a reversed nonce, a nonce of another length,
// an ICV with a flipped bit.
type ccmSK struct {
	sa        *SA
	inner     []byte // the inner payload chain, already encoded
	firstType uint8  // the SK generic header's Next Payload
	msgID     uint32
	icvOctets int    // the ICV this message carries
	iv        []byte // nil: eight fresh octets
	nonce     []byte // nil: salt || iv
}

// build answers the complete message octets.
func (b ccmSK) build(t *testing.T) []byte {
	t.Helper()
	iv := b.iv
	if iv == nil {
		iv = make([]byte, aeadIVOctets)
		if _, err := crand.Read(iv); err != nil {
			t.Fatalf("crand.Read: %v", err)
		}
	}

	// RFC 5282 Section 3, Figure 2: the plaintext is the inner payloads, the Padding,
	// and the one octet Pad Length. This builder sends no Padding.
	plaintext := make([]byte, len(b.inner)+1)
	copy(plaintext, b.inner)

	prefixOctets := wire.HeaderLen + wire.GenericHeaderLen
	totalOctets := prefixOctets + len(iv) + len(plaintext) + b.icvOctets
	buf := make([]byte, totalOctets)

	hdr := wire.Header{
		InitiatorSPI: b.sa.InitiatorSPI,
		ResponderSPI: b.sa.ResponderSPI,
		MajorVersion: 2,
		ExchangeType: wire.ExchangeInformational,
		Flags:        initiatorFlag(b.sa),
		MessageID:    b.msgID,
		NextPayload:  wire.PayloadTypeSK,
		Length:       uint32(totalOctets),
	}
	hdr.WriteTo(buf, 0)
	skGH := wire.GenericHeader{
		NextPayload: b.firstType,
		Length:      uint16(totalOctets - wire.HeaderLen),
	}
	skGH.WriteTo(buf, wire.HeaderLen)
	copy(buf[prefixOctets:], iv)

	sendKey := skSendEncKey(b.sa)
	nonce := b.nonce
	if nonce == nil {
		nonce = ccmNonce(sendKey, iv)
	}
	mode := ccmFor(t, sendKey, b.icvOctets, len(nonce))
	sealed := mode.Seal(nil, nonce, plaintext, buf[:prefixOctets])
	copy(buf[prefixOctets+len(iv):], sealed)
	return buf
}

// ccmNonce builds the default nonce format of RFC 5282 Section 4 from one direction's key
// material and the IV on the wire: the salt, then the IV.
func ccmNonce(keyWithSalt, iv []byte) []byte {
	nonce := make([]byte, 0, ccmSaltOctets+len(iv))
	nonce = append(nonce, keyWithSalt[len(keyWithSalt)-ccmSaltOctets:]...)
	return append(nonce, iv...)
}

// ccmFor answers an AES CCM instance keyed independently of the daemon's own, from the
// leading octets of the key material.
func ccmFor(t *testing.T, keyWithSalt []byte, icvOctets, nonceOctets int) gocipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(keyWithSalt[:len(keyWithSalt)-ccmSaltOctets])
	if err != nil {
		t.Fatalf("aes.NewCipher over the leading octets of the key material: %v", err)
	}
	mode, err := ccm.New(block, icvOctets, nonceOctets)
	if err != nil {
		t.Fatalf("build an AES CCM instance with a %d octet ICV and a %d octet nonce: %v",
			icvOctets, nonceOctets, err)
	}
	return mode
}

// skOf answers the Encrypted payload of a raw message.
func skOf(t *testing.T, raw []byte) *wire.PayloadSK {
	t.Helper()
	msg := parseMsg(t, raw)
	sk, ok := msg.Payloads[len(msg.Payloads)-1].Payload.(*wire.PayloadSK)
	if !ok {
		t.Fatalf("last payload = %T, want *wire.PayloadSK", msg.Payloads[len(msg.Payloads)-1].Payload)
	}
	return sk
}

// RFC requirement: RFC5282-3.2-3 positive -- ze supports an AES CCM ICV of eight octets and
// one of sixteen. Under each, a full handshake establishes, the message ze builds is longer
// than its plaintext by exactly that many octets, and the peer's receive path recovers the
// inner payload from it.
//
// RFC 5282 Section 3.2: "AES CCM provides an encrypted ICV. Implementations MUST support
// ICV sizes of 8 octets and 16 octets."
//
// The ICV length is measured rather than named: it is what is left of the Encrypted
// payload once the eight octet IV and the recovered plaintext are taken off, so a builder
// that appended the wrong number of octets fails here.
func TestRFC5282CCMSupportsTheEightAndSixteenOctetICV(t *testing.T) {
	for _, algo := range []ipsec.EncryptionAlgo{ipsec.EncryptionAES256CCM8, ipsec.EncryptionAES256CCM16} {
		icvOctets := ccmAlgos[algo]
		ini, resp := establishCCM(t, algo)
		inner, firstType := oneDeletePayload(t)

		raw, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 3,
			wire.ExchangeInformational, initiatorFlag(ini))
		if err != nil {
			t.Fatalf("%s: buildSKMessageAEADWithMsgID: %v", algo, err)
		}
		sk := skOf(t, raw)
		plaintextOctets := len(inner) + 1 // the inner chain and the Pad Length octet
		got := len(sk.CipherText) - aeadIVOctets - plaintextOctets
		if got != icvOctets {
			t.Errorf("%s: the Encrypted payload carries a %d octet ICV, want %d",
				algo, got, icvOctets)
		}
		expectOneDelete(t, resp, raw)
	}
}

// RFC requirement: RFC5282-3.2-3 negative -- the eight and sixteen octet ICVs are VERIFIED
// rather than carried. At each size, a message whose ICV has one flipped bit is refused by
// the receive path, and so is one whose ciphertext has one.
//
// Without this polarity a receiver that split the ICV off and threw it away would satisfy
// the positive case, because the length would still be right and the plaintext would still
// come back.
func TestRFC5282CCMVerifiesTheEightAndSixteenOctetICV(t *testing.T) {
	for _, algo := range []ipsec.EncryptionAlgo{ipsec.EncryptionAES256CCM8, ipsec.EncryptionAES256CCM16} {
		ini, resp := establishCCM(t, algo)
		inner, firstType := oneDeletePayload(t)

		raw, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 4,
			wire.ExchangeInformational, initiatorFlag(ini))
		if err != nil {
			t.Fatalf("%s: buildSKMessageAEADWithMsgID: %v", algo, err)
		}

		damagedICV := bytes.Clone(raw)
		damagedICV[len(damagedICV)-1] ^= 0x01
		expectRefused(t, resp, damagedICV, algo.String()+" with a flipped bit in its ICV")

		damagedText := bytes.Clone(raw)
		damagedText[wire.HeaderLen+wire.GenericHeaderLen+aeadIVOctets] ^= 0x01
		expectRefused(t, resp, damagedText, algo.String()+" with a flipped bit in its ciphertext")
	}
}

// RFC requirement: RFC5282-4-3 positive -- for AES CCM the nonce is the salt concatenated
// with the IV, in that order. The message ze really sent is opened by an AES CCM instance
// keyed independently, under a nonce built as the three salt octets of SK_ei followed by
// the eight IV octets read off the wire.
//
// RFC 5282 Section 4: "When this default nonce format is used, both the encryptor and
// decryptor construct the nonce by concatenating the salt with the IV, in that order. ...
// For the use of AES CCM with the IKEv2 Encrypted Payload, this default nonce format MUST
// be used and an 11 octet nonce MUST be used."
//
// The salt is cut out of the derived key material rather than named, so a change to the
// KEYMAT layout moves this test with it.
func TestRFC5282CCMNonceIsSaltThenIV(t *testing.T) {
	ini, _ := establishCCM(t, ipsec.EncryptionAES256CCM16)
	inner, firstType := oneDeletePayload(t)

	raw, err := buildSKMessageAEADWithMsgID(ini, inner, firstType, 5,
		wire.ExchangeInformational, initiatorFlag(ini))
	if err != nil {
		t.Fatalf("buildSKMessageAEADWithMsgID: %v", err)
	}
	sk := skOf(t, raw)
	sendKey := skSendEncKey(ini)

	mode := ccmFor(t, sendKey, ccmAlgos[ipsec.EncryptionAES256CCM16], ccmNonceOctets)
	nonce := ccmNonce(sendKey, sk.CipherText[:aeadIVOctets])
	got, err := mode.Open(nil, nonce, sk.CipherText[aeadIVOctets:], raw[:sk.DataOffset])
	if err != nil {
		t.Fatalf("opening ze's own message under salt || IV failed: %v", err)
	}
	if !bytes.HasPrefix(got, inner) {
		t.Fatalf("the recovered plaintext %x does not start with the inner chain %x", got, inner)
	}
}

// RFC requirement: RFC5282-4-3 negative -- the ORDER of the two halves is load-bearing. A
// message sealed under IV || salt, which is the same eleven octets the other way round, is
// refused rather than opened. A receiver that built a nonce of the right length out of the
// two halves in either order would not pass.
func TestRFC5282CCMRefusesAReversedNonce(t *testing.T) {
	ini, resp := establishCCM(t, ipsec.EncryptionAES256CCM16)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x3c}, aeadIVOctets)
	sendKey := skSendEncKey(ini)
	reversed := make([]byte, 0, ccmNonceOctets)
	reversed = append(reversed, iv...)
	reversed = append(reversed, sendKey[len(sendKey)-ccmSaltOctets:]...)

	raw := ccmSK{
		sa: ini, inner: inner, firstType: firstType, msgID: 6,
		icvOctets: ccmAlgos[ipsec.EncryptionAES256CCM16], iv: iv, nonce: reversed,
	}.build(t)
	expectRefused(t, resp, raw, "a message sealed under IV || salt rather than salt || IV")
}

// RFC requirement: RFC5282-4-4 positive -- the AES CCM nonce is eleven octets: the three
// octet salt and the eight octet IV. A message sealed under those eleven octets is accepted
// by the receive path, and both lengths are read from the key material and the wire rather
// than asserted.
//
// RFC 5282 Section 4: "For the use of AES CCM with the IKEv2 Encrypted Payload, this
// default nonce format MUST be used and an 11 octet nonce MUST be used."
//
// The eleven is checked before the message is built, so a salt or an IV of another length
// fails here rather than reaching the cipher as a shorter nonce.
func TestRFC5282CCMNonceIsElevenOctets(t *testing.T) {
	ini, resp := establishCCM(t, ipsec.EncryptionAES256CCM16)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x91}, aeadIVOctets)
	nonce := ccmNonce(skSendEncKey(ini), iv)
	if len(nonce) != 11 {
		t.Fatalf("the salt and the IV give a %d octet nonce, want 11", len(nonce))
	}

	raw := ccmSK{
		sa: ini, inner: inner, firstType: firstType, msgID: 7,
		icvOctets: ccmAlgos[ipsec.EncryptionAES256CCM16], iv: iv, nonce: nonce,
	}.build(t)
	expectOneDelete(t, resp, raw)
}

// RFC requirement: RFC5282-4-4 negative -- eleven is exact rather than a floor. A message
// sealed under a ten octet nonce is refused, and so is one sealed under the twelve octets
// AES GCM takes. Both keep salt-then-IV order, so only the length differs.
//
// Twelve is the interesting one: it is the nonce of Section 4's other sentence, so a
// receiver that built one nonce format for every AEAD cipher would accept it.
func TestRFC5282CCMRefusesANonceThatIsNotElevenOctets(t *testing.T) {
	ini, resp := establishCCM(t, ipsec.EncryptionAES256CCM16)
	inner, firstType := oneDeletePayload(t)

	iv := bytes.Repeat([]byte{0x91}, aeadIVOctets)
	sendKey := skSendEncKey(ini)
	salt := sendKey[len(sendKey)-ccmSaltOctets:]

	for name, nonce := range map[string][]byte{
		"ten octets":    append(append([]byte(nil), salt[1:]...), iv...),
		"twelve octets": append(append(append([]byte(nil), byte(0)), salt...), iv...),
	} {
		raw := ccmSK{
			sa: ini, inner: inner, firstType: firstType, msgID: 8,
			icvOctets: ccmAlgos[ipsec.EncryptionAES256CCM16], iv: iv, nonce: nonce,
		}.build(t)
		expectRefused(t, resp, raw, "a message sealed under a nonce of "+name)
	}
}
