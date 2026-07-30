// VALIDATES: RFC 7296 Section 3.16 EAP message format on the wire. The EAP Length
// field is four less than the Payload Length of the generic payload header that
// carries it.
// PREVENTS: an EAP payload whose declared length disagrees with the payload around
// it. A peer reads such a message as truncated or as over-long.
package wire

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// eapfmtEncodeEAPMessage encodes one IKE_AUTH message that carries a single EAP
// payload and returns the wire bytes.
func eapfmtEncodeEAPMessage(t *testing.T, p *PayloadEAP) []byte {
	t.Helper()
	msg := Message{
		Header: Header{
			InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			MajorVersion: 2,
			ExchangeType: ExchangeIKEAuth,
			Flags:        FlagInitiator,
		},
		Payloads: []PayloadEntry{{Payload: p}},
	}
	buf := make([]byte, msg.Len()+64)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("CheckedWriteTo: %v", err)
	}
	return buf[:n]
}

// eapfmtLengths reads the encapsulating Payload Length and the EAP Length out of an
// encoded message. The generic payload header follows the 28-octet IKE header.
func eapfmtLengths(t *testing.T, raw []byte) (payloadLen, eapLen int) {
	t.Helper()
	if len(raw) < HeaderLen+GenericHeaderLen+4 {
		t.Fatalf("message holds only %d octets", len(raw))
	}
	payloadLen = int(binary.BigEndian.Uint16(raw[HeaderLen+2:]))
	eapLen = int(binary.BigEndian.Uint16(raw[HeaderLen+GenericHeaderLen+2:]))
	return payloadLen, eapLen
}

// RFC requirement: RFC7296-3.16-2 positive -- PayloadEAP.WriteTo writes the EAP Length as
// four plus the body size (payload_eap.go:27-28), and it returns that same count
// (payload_eap.go:30). Message.WriteTo adds the 4-octet generic header to the
// returned count (message.go:39-42). The wire Payload Length is therefore the EAP
// Length plus four.
// RFC requirement: RFC7296-3.16-2 negative -- the relation is not an artifact of one body
// size. The sweep below covers five sizes and refuses a repeated EAP Length.
func TestEapfmtEAPLengthIsFourLessThanPayloadLength(t *testing.T) {
	seen := make(map[int]bool)
	for _, size := range []int{0, 1, 5, 60, 255} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i + 1)
		}
		raw := eapfmtEncodeEAPMessage(t, &PayloadEAP{
			Code:       EAPCodeRequest,
			Identifier: 7,
			EAPData:    data,
		})
		payloadLen, eapLen := eapfmtLengths(t, raw)

		// The obligation itself.
		if eapLen != payloadLen-4 {
			t.Fatalf("size %d: EAP Length %d, want Payload Length %d less four", size, eapLen, payloadLen)
		}
		if eapLen != 4+size {
			t.Fatalf("size %d: EAP Length %d, want %d", size, eapLen, 4+size)
		}

		// A constant length, or a copy of the Payload Length, would pass the check
		// above for one body size alone.
		if seen[eapLen] {
			t.Fatalf("size %d: EAP Length %d repeats an earlier body size", size, eapLen)
		}
		seen[eapLen] = true
		if eapLen == payloadLen {
			t.Fatalf("size %d: EAP Length copies the Payload Length %d", size, payloadLen)
		}

		// The declared length delimits real content, so the decoder recovers every
		// data octet from the same bytes.
		var back Message
		if err := back.ReadFrom(raw); err != nil {
			t.Fatalf("size %d: ReadFrom: %v", size, err)
		}
		if len(back.Payloads) != 1 {
			t.Fatalf("size %d: decoded %d payloads, want 1", size, len(back.Payloads))
		}
		got, ok := back.Payloads[0].Payload.(*PayloadEAP)
		if !ok {
			t.Fatalf("size %d: decoded payload type %T, want *PayloadEAP", size, back.Payloads[0].Payload)
		}
		if !bytes.Equal(got.EAPData, data) {
			t.Fatalf("size %d: decoded body %x, want %x", size, got.EAPData, data)
		}
	}
}
