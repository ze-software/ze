// VALIDATES: the SENDER half of the RFC 5282 obligations whose receive half
// rfc5282_aead_sk_test.go proves: the nonce ze seals under is the salt then the IV
// (Section 4), and the associated data ze seals under runs from the fixed header through
// the Encrypted payload's generic header and stops before the IV (Section 5.1). It also
// proves the ESP half of Section 8 and the AES CCM half of Section 7.3.
// PREVENTS: a sender that swaps the nonce halves or cuts the associated data at another
// octet while the receive path stays conformant, an ESP proposal or selection that carries
// an integrity algorithm beside an AEAD cipher, and an AES CCM transform sent without its
// Key Length attribute.
// Related: auth.go -- buildSKMessageAEADWithMsgID; initiator.go -- espProposalToWire, encAttrs
package engine

import (
	"crypto/aes"
	gocipher "crypto/cipher"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// sentAEADMessage is one message an established AEAD SA really built, cut into the parts
// an independent AES-GCM instance needs to open it.
type sentAEADMessage struct {
	raw        []byte
	dataOffset int    // where the Encrypted payload's data (the IV) starts
	data       []byte // IV || ciphertext || ICV
	salt       []byte
	gcm        gocipher.AEAD
}

// sentAEAD answers the last message the SA sent. It requires the Encrypted payload to be
// the only payload of the message, so the RFC 5282 Section 5.1 span is the fixed header
// and the SK generic header and nothing else.
func sentAEAD(t *testing.T, sa *SA, who string) sentAEADMessage {
	t.Helper()
	raw := sa.LastSentMsg
	if len(raw) == 0 {
		t.Fatalf("%s sent no message, so there is nothing to open", who)
	}
	msg := parseMsg(t, raw)
	if len(msg.Payloads) != 1 {
		t.Fatalf("%s's message carries %d outer payloads, want the Encrypted payload alone", who, len(msg.Payloads))
	}
	sk, ok := msg.Payloads[0].Payload.(*wire.PayloadSK)
	if !ok {
		t.Fatalf("%s's outer payload = %T, want *wire.PayloadSK", who, msg.Payloads[0].Payload)
	}
	sendKey := skSendEncKey(sa)
	block, err := aes.NewCipher(sendKey[:len(sendKey)-aeadSaltOctets])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := gocipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gocipher.NewGCM: %v", err)
	}
	return sentAEADMessage{
		raw:        raw,
		dataOffset: sk.DataOffset,
		data:       sk.CipherText,
		salt:       sendKey[len(sendKey)-aeadSaltOctets:],
		gcm:        gcm,
	}
}

// open tries the message under one nonce and one associated-data span.
func (m sentAEADMessage) open(nonce, aad []byte) error {
	_, err := m.gcm.Open(nil, nonce, m.data[aeadIVOctets:], aad)
	return err
}

// saltThenIV is the RFC 5282 Section 4 nonce for this message.
func (m sentAEADMessage) saltThenIV() []byte {
	nonce := make([]byte, 0, aeadSaltOctets+aeadIVOctets)
	nonce = append(nonce, m.salt...)
	return append(nonce, m.data[:aeadIVOctets]...)
}

// bothSenders answers the last message of each end of one AEAD handshake: the initiator's
// IKE_AUTH request and the responder's IKE_AUTH response.
func bothSenders(t *testing.T) map[string]sentAEADMessage {
	t.Helper()
	ini, resp := establishAEAD(t)
	return map[string]sentAEADMessage{
		"initiator": sentAEAD(t, ini, "the initiator"),
		"responder": sentAEAD(t, resp, "the responder"),
	}
}

// RFC requirement: RFC5282-4-1 positive -- the ENCRYPTOR half: every AES-GCM message ze
// builds, as initiator and as responder, opens under the nonce made of the salt followed
// by the IV, and does not open under the IV followed by the salt, so ze seals under the
// default nonce format in that order.
//
// RFC 5282 Section 4: "When this default nonce format is used, both the encryptor and
// decryptor construct the nonce by concatenating the salt with the IV, in that order. For
// the use of AES GCM with the IKEv2 Encrypted Payload, this default nonce format MUST be
// used".
//
// The reversed nonce has the same length, so the refusal is about the order alone.
func TestRFC5282AEADSendSealsUnderSaltThenIV(t *testing.T) {
	for who, m := range bothSenders(t) {
		aad := m.raw[:m.dataOffset]
		if err := m.open(m.saltThenIV(), aad); err != nil {
			t.Fatalf("%s: the message does not open under salt || IV: %v", who, err)
		}
		reversed := make([]byte, 0, aeadSaltOctets+aeadIVOctets)
		reversed = append(reversed, m.data[:aeadIVOctets]...)
		reversed = append(reversed, m.salt...)
		if err := m.open(reversed, aad); err == nil {
			t.Fatalf("%s: the message also opens under IV || salt, so the order is not what was sealed", who)
		}
	}
}

// RFC requirement: RFC5282-5.1-1 positive -- the SENDER half: every AES-GCM message ze
// builds, as initiator and as responder, is sealed over associated data that starts at the
// first octet of the fixed IKE header and ends at the last octet of the Encrypted payload's
// generic header. The span is 32 octets because the Encrypted payload is the only outer
// payload, the message opens under exactly that span, and it opens under neither the
// span one octet short nor the fixed header alone.
//
// RFC 5282 Section 5.1: "The associated data (A) MUST consist of the partial contents of
// the IKEv2 message, starting from the first octet of the Fixed IKE Header through the last
// octet of the Payload Header of the Encrypted Payload (i.e., the fourth octet of the
// Encrypted Payload), as shown in Figure 3."
//
// The receive half is TestRFC5282AEADAssociatedDataCoversAnInterveningPayload.
func TestRFC5282AEADSendAssociatedDataRunsThroughTheSKHeader(t *testing.T) {
	for who, m := range bothSenders(t) {
		if m.dataOffset != wire.HeaderLen+wire.GenericHeaderLen {
			t.Fatalf("%s: the Encrypted payload's data starts at octet %d, want %d",
				who, m.dataOffset, wire.HeaderLen+wire.GenericHeaderLen)
		}
		nonce := m.saltThenIV()
		if err := m.open(nonce, m.raw[:m.dataOffset]); err != nil {
			t.Fatalf("%s: the message does not open over the fixed header through the SK generic header: %v", who, err)
		}
		if err := m.open(nonce, m.raw[:m.dataOffset-1]); err == nil {
			t.Fatalf("%s: the message also opens without the last octet of the SK generic header", who)
		}
		if err := m.open(nonce, m.raw[:wire.HeaderLen]); err == nil {
			t.Fatalf("%s: the message also opens over the fixed header alone", who)
		}
	}
}

// RFC requirement: RFC5282-5.1-2 positive -- the SENDER half: no AES-GCM message ze
// builds, as initiator or as responder, opens under associated data that runs into the
// Initialization Vector or through the Ciphertext, while each opens under the span that
// stops before the IV. So neither field is in the associated data ze seals under.
//
// RFC 5282 Section 5.1: "The Initialization Vector and Ciphertext fields shown in Figure 1
// (above) MUST NOT be included in the associated data."
//
// The receive half is TestRFC5282AEADAssociatedDataExcludesTheIVAndCiphertext.
func TestRFC5282AEADSendAssociatedDataExcludesTheIVAndCiphertext(t *testing.T) {
	for who, m := range bothSenders(t) {
		nonce := m.saltThenIV()
		if err := m.open(nonce, m.raw[:m.dataOffset]); err != nil {
			t.Fatalf("%s: the message does not open over the span that stops before the IV: %v", who, err)
		}
		if err := m.open(nonce, m.raw[:m.dataOffset+aeadIVOctets]); err == nil {
			t.Fatalf("%s: the message opens over associated data that includes the IV", who)
		}
		if err := m.open(nonce, m.raw[:len(m.raw)-aeadICVOctets]); err == nil {
			t.Fatalf("%s: the message opens over associated data that includes the IV and the Ciphertext", who)
		}
	}
}

// aeadESPGroup offers one AES-GCM ESP proposal. Hash is still set, so a builder or a
// selection that ignores the AEAD property has an integrity algorithm available.
func aeadESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name: "test-esp-aead",
		Proposals: []ipsec.ESPProposal{{
			Number: 1, Encryption: ipsec.EncryptionAES256GCM, Hash: ipsec.HashSHA256,
		}},
	}
}

// RFC requirement: RFC5282-8-2 positive -- the ESP rail: an ESP proposal whose only
// encryption algorithm is AES GCM goes on the wire with no Transform Type 3, and keeps its
// encryption and Extended Sequence Numbers transforms.
//
// RFC 5282 Section 8: "This document further updates [RFC4306] to require that if all of
// the encryption algorithms in any proposal are authenticated encryption algorithms, then
// the proposal MUST NOT propose any integrity transforms."
//
// The IKE rail is TestRFC5282AEADIKEProposalCarriesNoIntegrityTransform.
func TestRFC5282AEADESPProposalCarriesNoIntegrityTransform(t *testing.T) {
	p := espProposalToWire(aeadESPGroup().Proposals[0], 0x0a0b0c0d, 1, dhGroupNone)
	if got := integrityTransformCount(p); got != 0 {
		t.Fatalf("the AES-GCM ESP proposal carries %d Transform Type 3, want 0", got)
	}
	want := map[uint8]bool{wire.TransformTypeENCR: false, wire.TransformTypeESN: false}
	for _, transform := range p.Transforms {
		if _, ok := want[transform.Type]; ok {
			want[transform.Type] = true
		}
	}
	for transformType, present := range want {
		if !present {
			t.Fatalf("the AES-GCM ESP proposal lost Transform Type %d; the omission must be the integrity transform alone", transformType)
		}
	}
}

// RFC requirement: RFC5282-8-2 negative -- the ESP rail: an ESP proposal whose encryption
// algorithm is AES CBC carries exactly one Transform Type 3, so the omission above is
// conditional on the AEAD property.
func TestRFC5282NonAEADESPProposalKeepsItsIntegrityTransform(t *testing.T) {
	p := espProposalToWire(ipsec.ESPProposal{
		Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256,
	}, 0x0a0b0c0d, 1, dhGroupNone)
	if got := integrityTransformCount(p); got != 1 {
		t.Fatalf("the AES-CBC ESP proposal carries %d Transform Type 3, want exactly 1", got)
	}
}

// RFC requirement: RFC5282-8-1 positive -- the ESP SA: NegotiateESP over ze's own AES-GCM
// ESP offer selects AES GCM with AUTH_NONE as the integrity algorithm, although the
// configured proposal names a hash.
//
// RFC 5282 Section 8: "This document updates [RFC4306] to require that when an
// authenticated encryption algorithm is selected as the encryption algorithm for any SA
// (IKE or ESP), an integrity algorithm MUST NOT be selected for that SA."
//
// The IKE SA is TestRFC5282AEADSelectionCarriesNoIntegrityAlgorithm.
func TestRFC5282AEADESPSelectionCarriesNoIntegrityAlgorithm(t *testing.T) {
	group := aeadESPGroup()
	offer := buildWireESPProposals(group, 0x0a0b0c0d, dhGroupNone)
	chosen, err := crypto.NegotiateESP(wireProposalsToESP(offer), buildESPProposals(group))
	if err != nil {
		t.Fatalf("NegotiateESP over an AES-GCM ESP offer: %v", err)
	}
	if chosen.Encryption.ID != crypto.ENCR_AES_GCM_16 {
		t.Fatalf("the selected ESP cipher is %v, want AES GCM", chosen.Encryption.ID)
	}
	if chosen.Integrity.ID != crypto.AUTH_NONE {
		t.Fatalf("the selected ESP integrity algorithm is %v, want none", chosen.Integrity.ID)
	}
}

// RFC requirement: RFC5282-8-1 negative -- the ESP SA: a peer's ESP offer that carries an
// integrity transform beside AES GCM is refused, so no integrity algorithm can be selected
// for an ESP SA whose cipher is AEAD.
func TestRFC5282AEADESPOfferWithIntegrityIsRefused(t *testing.T) {
	group := aeadESPGroup()
	offer := buildWireESPProposals(group, 0x0a0b0c0d, dhGroupNone)
	if len(offer) != 1 {
		t.Fatalf("buildWireESPProposals returned %d proposals, want 1", len(offer))
	}
	offer[0].Transforms = append(offer[0].Transforms, wire.Transform{
		Type: wire.TransformTypeINTG, ID: uint16(crypto.AUTH_HMAC_SHA2_256_128),
	})
	if _, err := crypto.NegotiateESP(wireProposalsToESP(offer), buildESPProposals(group)); err == nil {
		t.Fatal("an AES-GCM ESP offer carrying an integrity transform was accepted, want a refusal")
	}
}

// RFC requirement: RFC5282-7.3-1 positive -- the AES CCM identifiers: every AES CCM
// encryption transform ze puts in an IKE proposal, at each ICV length and both key sizes,
// carries exactly one Key Length attribute naming the key size.
//
// RFC 5282 Section 7.3: "Because the AES supports three key lengths, the Key Length
// attribute MUST be specified when any of the identifiers for AES GCM or AES CCM, specified
// in Section 7.2 of this document, is used."
//
// The AES GCM identifiers are TestRFC5282AEADProposalCarriesTheKeyLengthAttribute.
func TestRFC5282AEADCCMProposalCarriesTheKeyLengthAttribute(t *testing.T) {
	for _, tc := range []struct {
		algo ipsec.EncryptionAlgo
		id   crypto.EncryptionID
		bits uint16
	}{
		{ipsec.EncryptionAES128CCM8, crypto.ENCR_AES_CCM_8, 128},
		{ipsec.EncryptionAES256CCM8, crypto.ENCR_AES_CCM_8, 256},
		{ipsec.EncryptionAES128CCM12, crypto.ENCR_AES_CCM_12, 128},
		{ipsec.EncryptionAES256CCM12, crypto.ENCR_AES_CCM_12, 256},
		{ipsec.EncryptionAES128CCM16, crypto.ENCR_AES_CCM_16, 128},
		{ipsec.EncryptionAES256CCM16, crypto.ENCR_AES_CCM_16, 256},
	} {
		group := aeadIKEGroup()
		group.Proposals[0].Encryption = tc.algo
		proposals := buildWireIKEProposals(group)
		if len(proposals) != 1 {
			t.Fatalf("%v: buildWireIKEProposals returned %d proposals, want 1", tc.algo, len(proposals))
		}
		found := 0
		for _, transform := range proposals[0].Transforms {
			if transform.Type != wire.TransformTypeENCR {
				continue
			}
			if transform.ID != uint16(tc.id) {
				t.Fatalf("%v: encryption Transform ID = %d, want %d", tc.algo, transform.ID, tc.id)
			}
			for _, attr := range transform.Attrs {
				if attr.Type != wire.AttrTypeKeyLength {
					continue
				}
				found++
				if attr.Value != tc.bits {
					t.Fatalf("%v: Key Length attribute = %d, want %d", tc.algo, attr.Value, tc.bits)
				}
			}
		}
		if found != 1 {
			t.Fatalf("%v: the AES CCM transform carries %d Key Length attributes, want exactly 1", tc.algo, found)
		}
	}
}
