// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the handshake, both roles
// Related: protected_input.go -- openProtected, which refuses both shapes on reception
// Related: auth.go -- buildEncryptedMessageEx, which builds the shape these tests read
//
// VALIDATES: the shape of the Encrypted (SK) payload buildEncryptedMessageEx writes, read
// from the octets it returns rather than through a handshake. RFC 7296 Section 3.1: "An
// Encrypted payload MUST be the last payload in a packet and an Encrypted payload MUST NOT
// contain another Encrypted payload."
//
// METHOD: the positives call the builder directly, for the CBC and the GCM key sets: one
// reads the fixed header and the SK generic header at their offsets, the other asserts the
// builder's refusal of a nested Encrypted payload. A built inner chain is opened through
// decryptAndParse by an SA holding the opposite role over the same keys. The negatives hand a
// receiver the message a non-conforming peer sends, sealed by hand, and read openProtected.
//
// PREVENTS: a builder change that leaves octets after the Encrypted payload, or that seals
// an inner chain holding an Encrypted payload when a caller hands it one. A one-line break of either shape is caught
// here, where the handshake test in rfc7296_encrypt_test.go never reaches its assertion
// because the peer refuses the message first.
package engine

import (
	"encoding/binary"
	"errors"
	"testing"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// skShapeInner is the inner chain the two positives seal: two payloads of different types, so the
// first inner type is known and the inner chain has a payload after its first one.
func skShapeInner() []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}},
		{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}},
	}
}

// skShapeKeySets lists the two SK sealing paths buildEncryptedMessageEx selects between.
var skShapeKeySets = []struct {
	name string
	mk   func(t *testing.T) *SA
}{
	{"cbc", testSAWithKeys},
	{"gcm", testSAWithGCMKeys},
}

// RFC requirement: RFC7296-3.1-1 positive -- a message buildEncryptedMessageEx builds, under the
// CBC and the GCM key sets, names SK in the fixed header's Next Payload, carries a generic
// header whose Next Payload names the first inner payload, and whose Length runs exactly to
// the end of the message, and its outer chain parses to that single Encrypted payload.
//
// The Length check is the one that says nothing follows: an SK Length shorter than the rest
// of the message leaves octets that a receiver would walk as another outer payload, typed by
// the SK's Next Payload.
func TestSKBuiltIsLastOuterPayload(t *testing.T) {
	for _, ks := range skShapeKeySets {
		t.Run(ks.name, func(t *testing.T) {
			sa := ks.mk(t)
			inner := skShapeInner()
			raw, err := buildEncryptedMessageEx(sa, inner, 1, wire.ExchangeInformational, wire.FlagInitiator)
			if err != nil {
				t.Fatalf("buildEncryptedMessageEx: %v", err)
			}
			if len(raw) < wire.HeaderLen+wire.GenericHeaderLen {
				t.Fatalf("message is %d octets, too short for a header and an SK payload header", len(raw))
			}

			// Fixed header: Next Payload at octet 16, Length at octets 24-27 (RFC 7296 Section 3.1).
			if got := raw[16]; got != wire.PayloadTypeSK {
				t.Errorf("header Next Payload = %d, want %d (SK)", got, wire.PayloadTypeSK)
			}
			if got := binary.BigEndian.Uint32(raw[24:28]); int(got) != len(raw) {
				t.Errorf("header Length = %d, message is %d octets", got, len(raw))
			}

			// SK generic header at octet 28: Next Payload, flags, then a two-octet Length
			// (RFC 7296 Section 3.2). Section 3.14 makes Next Payload name the first inner
			// payload.
			skHeader := raw[wire.HeaderLen : wire.HeaderLen+wire.GenericHeaderLen]
			if got, want := skHeader[0], inner[0].Payload.Type(); got != want {
				t.Errorf("SK Next Payload = %d, want %d (the first inner payload)", got, want)
			}
			skLength := int(binary.BigEndian.Uint16(skHeader[2:4]))
			if want := len(raw) - wire.HeaderLen; skLength != want {
				t.Errorf("SK Length = %d, want %d: %d octets follow the Encrypted payload",
					skLength, want, want-skLength)
				return
			}

			var msg wire.Message
			if err := msg.ReadFrom(raw); err != nil {
				t.Fatalf("parse the built message: %v", err)
			}
			if len(msg.Payloads) != 1 {
				t.Fatalf("outer chain holds %d payloads, want 1 (the Encrypted payload is last)",
					len(msg.Payloads))
			}
			if _, ok := msg.Payloads[0].Payload.(*wire.PayloadSK); !ok {
				t.Errorf("sole outer payload is %T, want *wire.PayloadSK", msg.Payloads[0].Payload)
			}
		})
	}
}

// RFC requirement: RFC7296-3.1-2 positive -- buildEncryptedMessageEx, under the CBC and the
// GCM key sets, refuses with errNestedSKBuild an inner chain that holds an Encrypted payload
// after a Delete, and returns no octets; a Delete then a Notify, with no Encrypted payload,
// is sealed and opens to exactly those two payloads, none of them an Encrypted payload.
//
// The refusal is the test that matters for a MUST NOT: a builder that never meets a nested
// Encrypted payload in its input would pass a test that only reads what it built. The
// control keeps the refusal tied to the nested payload rather than to the keys or the
// exchange, and its second payload makes the builder chain one inner payload to the next,
// so a builder that types a link as SK on its own is caught too.
func TestSKBuildRefusesANestedEncryptedPayload(t *testing.T) {
	for _, ks := range skShapeKeySets {
		t.Run(ks.name, func(t *testing.T) {
			sender := ks.mk(t)
			sender.IsInitiator = true
			receiver := *sender // the same keys, the opposite role (role_direction_test.go)
			receiver.IsInitiator = false

			raw, err := buildEncryptedMessageEx(sender, skNestedInner(), 1, wire.ExchangeInformational,
				wire.FlagInitiator)
			if !errors.Is(err, errNestedSKBuild) {
				t.Errorf("buildEncryptedMessageEx(Delete, SK) = %d octets, err %v; want errNestedSKBuild: "+
					"an Encrypted payload MUST NOT contain another Encrypted payload", len(raw), err)
			}
			if raw != nil {
				t.Errorf("buildEncryptedMessageEx returned %d octets with its refusal", len(raw))
			}

			control := skShapeInner()
			raw, err = buildEncryptedMessageEx(sender, control, 1, wire.ExchangeInformational, wire.FlagInitiator)
			if err != nil {
				t.Fatalf("control: buildEncryptedMessageEx(Delete, Notify) = %v", err)
			}
			opened, err := decryptAndParse(&receiver, parseMsg(t, raw), raw)
			if err != nil {
				t.Fatalf("control: open the built message: %v", err)
			}
			for i := range opened {
				if _, ok := opened[i].Payload.(*wire.PayloadSK); ok {
					t.Errorf("control: inner payload %d is an Encrypted payload; an Encrypted "+
						"payload must not contain another", i)
				}
			}
			if len(opened) != 2 {
				t.Fatalf("control: inner chain holds %d payloads, want 2 (Delete, Notify)", len(opened))
			}
			if _, ok := opened[0].Payload.(*wire.PayloadDelete); !ok {
				t.Errorf("control: inner payload 0 is %T, want *wire.PayloadDelete", opened[0].Payload)
			}
			if _, ok := opened[1].Payload.(*wire.PayloadNotify); !ok {
				t.Errorf("control: inner payload 1 is %T, want *wire.PayloadNotify", opened[1].Payload)
			}
		})
	}
}

// skNestedInner is the inner chain a peer that nests SK would seal: a Delete, then an
// Encrypted payload.
func skNestedInner() []wire.PayloadEntry {
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}},
		{Payload: &wire.PayloadSK{CipherText: []byte{0xde, 0xad, 0xbe, 0xef}}},
	}
}

// skSealInnerUnchecked seals inner under sender's keys the way buildEncryptedMessageEx does,
// without its refusal of a nested Encrypted payload, so a test can build the message a
// non-conforming peer sends. It chains the generic headers as the builder does and hands the
// octets to the same CBC or AEAD sealer.
func skSealInnerUnchecked(t *testing.T, sender *SA, inner []wire.PayloadEntry) []byte {
	t.Helper()
	buf := make([]byte, 4096)
	off := 0
	for i := range inner {
		var gh wire.GenericHeader
		if i+1 < len(inner) {
			gh.NextPayload = inner[i+1].Payload.Type()
		}
		ghOff := off
		off += wire.GenericHeaderLen
		bodyLen := inner[i].Payload.WriteTo(buf, off)
		off += bodyLen
		gh.Length = uint16(wire.GenericHeaderLen + bodyLen)
		gh.WriteTo(buf, ghOff)
	}
	seal := buildSKMessageCBCWithMsgID
	if sender.Proposal.Encryption.IsAEAD {
		seal = buildSKMessageAEADWithMsgID
	}
	raw, err := seal(sender, buf[:off], inner[0].Payload.Type(), 1, wire.ExchangeInformational, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("seal the inner chain: %v", err)
	}
	return raw
}

// skTrailedGCMMessage seals, under sender's GCM keys, a message whose Encrypted payload is
// followed by one more outer payload: a Notify the SK's Next Payload types, since the outer
// walk reads the octets after SK as the payload SK names. The fixed header's Length counts
// the trailer and the SK Length does not, which is the message a peer that puts a payload
// after SK sends. RFC 5282 Section 5.1 ends the associated data at the SK payload header, so
// the tag verifies and only the shape is wrong.
func skTrailedGCMMessage(t *testing.T, sender *SA) []byte {
	t.Helper()
	notify := &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}
	innerLen := wire.GenericHeaderLen + notify.Len()
	plaintext := make([]byte, innerLen+1) // the chain, then a Pad Length of 0
	innerHeader := wire.GenericHeader{Length: uint16(innerLen)}
	innerHeader.WriteTo(plaintext, 0)
	notify.WriteTo(plaintext, wire.GenericHeaderLen)

	sealedLen := 8 + len(plaintext) + 16 // IV, ciphertext, 16-octet ICV of AES-GCM-16
	skLen := wire.GenericHeaderLen + sealedLen
	trailerLen := wire.GenericHeaderLen + notify.Len()
	total := wire.HeaderLen + skLen + trailerLen
	buf := make([]byte, total)
	writeAuthHeaderWithMsgID(buf, sender, notify.Type(), uint32(total), 1, wire.ExchangeInformational, wire.FlagInitiator)
	binary.BigEndian.PutUint16(buf[wire.HeaderLen+2:], uint16(skLen))

	sealed, err := ikecrypto.SealIKEAEAD(sender.Proposal.Encryption.ID, skSendEncKey(sender), plaintext,
		buf[:wire.HeaderLen+wire.GenericHeaderLen])
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if len(sealed) != sealedLen {
		t.Fatalf("sealed %d octets, want %d", len(sealed), sealedLen)
	}
	copy(buf[wire.HeaderLen+wire.GenericHeaderLen:], sealed)
	trailer := buf[wire.HeaderLen+skLen:]
	trailerHeader := wire.GenericHeader{Length: uint16(trailerLen)}
	trailerHeader.WriteTo(trailer, 0)
	notify.WriteTo(trailer, wire.GenericHeaderLen)
	return buf
}

// RFC requirement: RFC7296-3.1-1 negative -- a GCM-sealed message whose Encrypted payload is
// followed by another outer payload, and whose tag verifies, is dropped by openProtected
// rather than opened: the outer chain parses to SK then the trailing payload, and the opener
// answers dropped with no inner chain.
//
// The control is the trailer itself: the same keys open the same Notify sealed with nothing
// after the Encrypted payload, so the drop is the shape and not the seal.
func TestSKFollowedByAnotherPayloadIsDropped(t *testing.T) {
	sender := testSAWithGCMKeys(t)
	sender.IsInitiator = true
	receiver := *sender
	receiver.IsInitiator = false

	raw := skTrailedGCMMessage(t, sender)
	msg := parseMsg(t, raw)
	if len(msg.Payloads) != 2 {
		t.Fatalf("outer chain holds %d payloads, want 2 (SK, then the trailer)", len(msg.Payloads))
	}
	inner, dropped, err := openProtected(&receiver, msg, raw, testReaderReason, slogutil.DiscardLogger())
	if !dropped || err != nil || inner != nil {
		t.Errorf("openProtected = (%d inner, dropped %v, err %v), want dropped: an Encrypted payload "+
			"MUST be the last payload in a packet", len(inner), dropped, err)
	}

	// The fixed header is in the associated data, so the control is sealed afresh with
	// nothing after the Encrypted payload rather than cut from the trailed message.
	resealed := skTrailedGCMMessageWithoutTrailer(t, sender)
	if _, dropped, err := openProtected(&receiver, parseMsg(t, resealed), resealed, testReaderReason,
		slogutil.DiscardLogger()); dropped || err != nil {
		t.Fatalf("control: the same keys refuse the untrailed message (dropped %v, err %v)", dropped, err)
	}
}

// skTrailedGCMMessageWithoutTrailer is the control message for the test above: the same
// Notify sealed the same way with nothing after the Encrypted payload.
func skTrailedGCMMessageWithoutTrailer(t *testing.T, sender *SA) []byte {
	t.Helper()
	raw, err := buildEncryptedMessageEx(sender,
		[]wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyInitialContact}}},
		1, wire.ExchangeInformational, wire.FlagInitiator)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}
	return raw
}

// RFC requirement: RFC7296-3.1-2 negative -- a message whose decrypted inner chain carries an
// Encrypted payload after a Delete, under the CBC and the GCM key sets, is refused by
// openProtected as an authenticated chain that does not parse (errInnerChain), not opened
// and not dropped as unauthenticated. The message is sealed by skSealInnerUnchecked, since
// Ze's own builder refuses to write it; the same sealer's Delete alone opens.
func TestSKNestedInTheInnerChainIsRefused(t *testing.T) {
	for _, ks := range skShapeKeySets {
		t.Run(ks.name, func(t *testing.T) {
			sender := ks.mk(t)
			sender.IsInitiator = true
			receiver := *sender
			receiver.IsInitiator = false

			raw := skSealInnerUnchecked(t, sender, skNestedInner())
			inner, dropped, err := openProtected(&receiver, parseMsg(t, raw), raw, testReaderReason,
				slogutil.DiscardLogger())
			if dropped {
				t.Fatalf("openProtected dropped the message as unauthenticated; its seal verifies")
			}
			if !errors.Is(err, errInnerChain) {
				t.Errorf("openProtected = (%d inner, err %v), want errInnerChain: an Encrypted payload "+
					"MUST NOT contain another Encrypted payload", len(inner), err)
			}

			control := skSealInnerUnchecked(t, sender, skNestedInner()[:1])
			if opened, dropped, err := openProtected(&receiver, parseMsg(t, control), control,
				testReaderReason, slogutil.DiscardLogger()); dropped || err != nil || len(opened) != 1 {
				t.Fatalf("control: the same sealer's Delete alone = (%d inner, dropped %v, err %v), "+
					"want it opened", len(opened), dropped, err)
			}
		})
	}
}
