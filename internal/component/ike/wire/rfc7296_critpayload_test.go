package wire

import (
	"errors"
	"testing"
)

// critUnknownType is a payload type no IKEv2 document defines, so decodePayload always
// refuses it. 199 sits above every type in RFC 7296 Section 3.2.
const critUnknownType uint8 = 199

// critChain builds a payload chain holding one payload of the named type, with the
// critical bit set or clear. The body is four octets of filler.
func critChain(critical bool) []byte {
	body := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	out := make([]byte, 0, GenericHeaderLen+len(body))
	flags := byte(0)
	if critical {
		flags = 0x80
	}
	total := uint16(GenericHeaderLen + len(body))
	out = append(out, 0, flags, byte(total>>8), byte(total))
	return append(out, body...)
}

// critMessage wraps a payload chain in a complete IKE message header whose
// NextPayload names the first payload.
func critMessage(firstType uint8, chain []byte) []byte {
	hdr := Header{
		InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		NextPayload:  firstType,
		MajorVersion: 2,
		ExchangeType: ExchangeInformational,
		MessageID:    7,
		Length:       uint32(HeaderLen + len(chain)),
	}
	buf := make([]byte, HeaderLen+len(chain))
	hdr.WriteTo(buf, 0)
	copy(buf[HeaderLen:], chain)
	return buf
}

// VALIDATES: an unrecognized payload carrying the critical bit is refused, and the
// refusal names the one-octet payload type the answering Notify must carry.
// RFC requirement: RFC7296-2.5-18 positive -- both producers return an
// *UnsupportedCritError holding the type. Message.ReadFrom returns it from the outer
// chain and ParsePayloadChain returns it from the inner chain, so CriticalPayloadType
// reads 199 back in either case. Without the type the Notification Data clause of
// Section 2.5 is unreachable.
// RFC requirement: RFC7296-2.5-18 negative -- the same payload with the critical bit
// CLEAR is not refused at all. It is demoted to PayloadRaw and the message stands, so
// the refusal is caused by the critical bit and not by the unknown type.
func TestCritUnknownCriticalPayloadNamesItsType(t *testing.T) {
	t.Run("outer chain refuses and names the type", func(t *testing.T) {
		var msg Message
		err := msg.ReadFrom(critMessage(critUnknownType, critChain(true)))
		if err == nil {
			t.Fatal("an unknown critical payload was accepted")
		}
		if !errors.Is(err, ErrUnsupportedCrit) {
			t.Fatalf("error = %v, want it to match ErrUnsupportedCrit", err)
		}
		ptype, ok := CriticalPayloadType(err)
		if !ok {
			t.Fatal("the error carries no payload type, so the Notify has no Notification Data")
		}
		if ptype != critUnknownType {
			t.Errorf("payload type = %d, want %d", ptype, critUnknownType)
		}
	})

	t.Run("inner chain refuses and names the type", func(t *testing.T) {
		_, err := ParsePayloadChain(critChain(true), critUnknownType)
		if err == nil {
			t.Fatal("an unknown critical payload was accepted in the inner chain")
		}
		if !errors.Is(err, ErrUnsupportedCrit) {
			t.Fatalf("error = %v, want it to match ErrUnsupportedCrit", err)
		}
		ptype, ok := CriticalPayloadType(err)
		if !ok || ptype != critUnknownType {
			t.Errorf("payload type = %d ok=%v, want %d true", ptype, ok, critUnknownType)
		}
	})

	t.Run("the critical bit clear is accepted", func(t *testing.T) {
		var msg Message
		if err := msg.ReadFrom(critMessage(critUnknownType, critChain(false))); err != nil {
			t.Fatalf("an unknown NON-critical payload was refused: %v", err)
		}
		if len(msg.Payloads) != 1 {
			t.Fatalf("payload count = %d, want 1", len(msg.Payloads))
		}
		raw, ok := msg.Payloads[0].Payload.(*PayloadRaw)
		if !ok {
			t.Fatalf("payload type = %T, want *PayloadRaw", msg.Payloads[0].Payload)
		}
		if raw.PayloadType != critUnknownType {
			t.Errorf("raw payload type = %d, want %d", raw.PayloadType, critUnknownType)
		}
		inner, err := ParsePayloadChain(critChain(false), critUnknownType)
		if err != nil {
			t.Fatalf("the inner chain refused a non-critical unknown payload: %v", err)
		}
		if len(inner) != 1 {
			t.Errorf("inner payload count = %d, want 1", len(inner))
		}
	})

	t.Run("no other error names a payload type", func(t *testing.T) {
		if _, ok := CriticalPayloadType(nil); ok {
			t.Error("a nil error reported a payload type")
		}
		if _, ok := CriticalPayloadType(ErrTruncated); ok {
			t.Error("a truncation error reported a payload type")
		}
	})
}

// VALIDATES: a payload chain that ends inside a payload is reported as malformed, and
// a chain whose payload CONTENTS are merely unacceptable is not.
// RFC requirement: RFC7296-2.21.2-1 positive -- ParsePayloadChain returns ErrTruncated
// for a chain that stops inside a generic header, and for one whose declared payload
// length runs past the end. The payloads read so far made a malformed message look
// like a message that was missing a payload.
// Each caller then took a branch written for an absent payload.
// No caller rejected the message.
// RFC requirement: RFC7296-2.21.2-1 negative -- a well-formed chain carrying a payload
// with unacceptable CONTENTS is NOT a malformed message. RFC 7296 draws that line
// explicitly ("rather than just bad payload contents"), and Section 3.3.6 keeps such a
// payload. Over-rejecting here would refuse messages every conforming peer sends.
func TestCritChainReportsTruncationButNotBadContents(t *testing.T) {
	full := critChain(false)

	t.Run("a chain ending inside a generic header is malformed", func(t *testing.T) {
		// Two octets is less than the four-octet generic header.
		_, err := ParsePayloadChain(full[:2], PayloadTypeNonce)
		if !errors.Is(err, ErrTruncated) {
			t.Fatalf("error = %v, want ErrTruncated", err)
		}
	})

	t.Run("a payload longer than the chain is malformed", func(t *testing.T) {
		// The generic header declares eight octets and only six are present.
		_, err := ParsePayloadChain(full[:len(full)-2], PayloadTypeNonce)
		if !errors.Is(err, ErrTruncated) {
			t.Fatalf("error = %v, want ErrTruncated", err)
		}
	})

	t.Run("a well-formed chain parses", func(t *testing.T) {
		out, err := ParsePayloadChain(full, PayloadTypeNonce)
		if err == nil && len(out) != 1 {
			t.Fatalf("payload count = %d, want 1", len(out))
		}
		if err != nil && errors.Is(err, ErrTruncated) {
			t.Fatalf("a complete chain was reported as truncated: %v", err)
		}
	})

	t.Run("bad payload contents are not a malformed message", func(t *testing.T) {
		// rfc-test-change-approved: 2026-07-31 owner standing approval for
		// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
		// The first fixture used an unknown transform TYPE, which decodePayload
		// accepts. The subtest never reached the rejection path, and no mutation
		// can kill it. This fixture trips a real rejection.
		//
		// An SA payload carrying a Key Length attribute on a fixed-length-key cipher.
		// RFC 7296 Section 3.3.5 forbids that attribute there, so the transform is
		// refused. Section 3.3.6 keeps the payload with what survived.
		// The chain parser must agree with Message.ReadFrom here.
		// An SA payload of a CREATE_CHILD_SA or an IKE_AUTH only ever travels in the
		// INNER chain. A stricter inner parser refuses messages the outer parser
		// accepts.
		sa := &PayloadSA{Proposals: []Proposal{{
			Number:     1,
			ProtocolID: ProtocolESP,
			SPISize:    4,
			SPI:        []byte{1, 2, 3, 4},
			Transforms: []Transform{
				{Type: TransformTypeENCR, ID: encrDES, Attrs: []TransformAttr{
					{Type: AttrTypeKeyLength, Value: 128},
				}},
				{Type: TransformTypeENCR, ID: 12},
			},
		}}}
		body := make([]byte, sa.Len())
		n := sa.WriteTo(body, 0)
		chain := make([]byte, 0, GenericHeaderLen+n)
		total := uint16(GenericHeaderLen + n)
		chain = append(chain, 0, 0, byte(total>>8), byte(total))
		chain = append(chain, body[:n]...)

		out, err := ParsePayloadChain(chain, PayloadTypeSA)
		if errors.Is(err, ErrTruncated) {
			t.Fatalf("a message with bad payload contents was reported as truncated: %v", err)
		}
		if err != nil {
			t.Fatalf("a message with bad payload contents was refused: %v", err)
		}
		if len(out) != 1 {
			t.Errorf("payload count = %d, want the payload kept", len(out))
		}
	})
}

// VALIDATES: notify message types split into errors and status at the RFC boundary,
// and recognition fails closed.
// RFC requirement: RFC7296-3.10.1-2 positive -- NotifyIsError puts every type below
// 16384 in the error half and every type at or above it in the status half, which is
// the split RFC 7296 Section 3.10.1 draws. NotifyTypeRecognized reports true only for
// a type the registry holds.
// RFC requirement: RFC7296-3.10.1-2 negative -- an unregistered type reads NOT
// recognized in both halves of the number space, so the classifier can never pass an
// unknown value off as understood.
func TestCritNotifyTypeClassification(t *testing.T) {
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/spec-rfcgate-1b-rfc7296-pilot.md, strengthening only.
	// The boundary is now asserted against the LITERAL RFC 7296 Section 3.10.1 value.
	// Phrased in terms of NotifyStatusFloor it moved with the constant.
	// A split moved to 8192 then left the test green.
	// It proved nothing about where the RFC puts the split.
	if NotifyStatusFloor != 16384 {
		t.Errorf("NotifyStatusFloor = %d, want the RFC 7296 Section 3.10.1 value 16384", NotifyStatusFloor)
	}
	if !NotifyIsError(NotifyNoProposalChosen) {
		t.Error("NO_PROPOSAL_CHOSEN is not classed as an error")
	}
	if !NotifyIsError(16383) {
		t.Error("16383, the last error type, is not classed as an error")
	}
	if NotifyIsError(16384) {
		t.Error("16384, the first status type, is classed as an error")
	}
	if NotifyIsError(NotifyInitialContact) {
		t.Error("INITIAL_CONTACT is classed as an error")
	}

	for _, known := range []uint16{
		NotifyUnsupportedCriticalPayload, NotifyInvalidIKESPI, NotifyInvalidSyntax,
		NotifyNoProposalChosen, NotifyTemporaryFailure, NotifyInitialContact,
		NotifyRekeySA, NotifySignatureHashAlgorithms,
	} {
		if !NotifyTypeRecognized(known) {
			t.Errorf("registered type %d reads unrecognized", known)
		}
		if NotifyTypeName(known) == "UNRECOGNIZED" {
			t.Errorf("registered type %d has no name", known)
		}
	}

	// Negative. Two unassigned values, one in each half of the number space.
	for _, unknown := range []uint16{9999, 40961} {
		if NotifyTypeRecognized(unknown) {
			t.Errorf("unassigned type %d reads recognized", unknown)
		}
		if NotifyTypeName(unknown) != "UNRECOGNIZED" {
			t.Errorf("unassigned type %d has a name", unknown)
		}
	}
}
