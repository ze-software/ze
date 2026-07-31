// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — IKEv2 message structure (Section 3)
package wire

import (
	"errors"
	"fmt"
)

// Message is a complete IKEv2 message: header + payload chain.
type Message struct {
	Header   Header
	Payloads []PayloadEntry
}

// PayloadEntry pairs a payload with its generic header metadata.
type PayloadEntry struct {
	Payload  Payload
	Critical bool
}

// WriteTo writes the full message into buf at off. Returns bytes written.
// The header's NextPayload and Length fields are set automatically.
func (m *Message) WriteTo(buf []byte, off int) int {
	start := off
	// Skip header, backfill after payloads are written
	off += HeaderLen
	for i := range m.Payloads {
		var gh GenericHeader
		gh.Critical = m.Payloads[i].Critical
		if sk, ok := m.Payloads[i].Payload.(*PayloadSK); ok && sk.InnerNextPayload != 0 {
			gh.NextPayload = sk.InnerNextPayload
		} else if i+1 < len(m.Payloads) {
			gh.NextPayload = m.Payloads[i+1].Payload.Type()
		}
		// Skip generic header, backfill length
		ghOff := off
		off += GenericHeaderLen
		bodyLen := m.Payloads[i].Payload.WriteTo(buf, off)
		off += bodyLen
		// RFC 7296 Section 3.2: payload length includes the 4-byte generic header
		gh.Length = uint16(GenericHeaderLen + bodyLen)
		gh.WriteTo(buf, ghOff)
	}
	// Set header fields
	m.Header.Length = uint32(off - start)
	if len(m.Payloads) > 0 {
		m.Header.NextPayload = m.Payloads[0].Payload.Type()
	} else {
		m.Header.NextPayload = 0
	}
	m.Header.WriteTo(buf, start)
	return off - start
}

// Len reports the exact number of bytes WriteTo will write for this message:
// the fixed header, plus a generic header and body for each payload. It must
// match WriteTo's return value so CheckedWriteTo can reject an undersized
// buffer before any index write panics (RFC 7296 Section 3.1 length field).
func (m *Message) Len() int {
	total := HeaderLen
	for i := range m.Payloads {
		total += GenericHeaderLen + m.Payloads[i].Payload.Len()
	}
	return total
}

// CheckedWriteTo validates that buf has room for the whole message before
// writing. WriteTo indexes buf directly (buffer-first contract), so an
// oversized message would panic with a slice-bounds error partway through the
// encode; this guard converts that latent panic into a returned error naming
// the required length. On error nothing is written and the caller must skip
// the send rather than transmit a truncated (malformed) IKE message.
func (m *Message) CheckedWriteTo(buf []byte, off int) (int, error) {
	need := m.Len()
	avail := len(buf) - off
	if off < 0 || avail < need {
		return 0, fmt.Errorf("ike: message needs %d bytes but buffer has %d", need, avail)
	}
	return m.WriteTo(buf, off), nil
}

// ReadFrom parses a complete IKEv2 message from data.
func (m *Message) ReadFrom(data []byte) error {
	if err := m.Header.ReadFrom(data); err != nil {
		return err
	}
	if int(m.Header.Length) > len(data) {
		return ErrLengthMismatch
	}
	if m.Header.Length < HeaderLen {
		return ErrHeaderTooShort
	}
	m.Payloads = nil
	nextType := m.Header.NextPayload
	off := HeaderLen
	end := int(m.Header.Length)
	for nextType != 0 && off < end {
		if len(m.Payloads) >= MaxPayloads {
			return ErrTooManyPayloads
		}
		if off+GenericHeaderLen > end {
			return ErrTruncated
		}
		var gh GenericHeader
		if err := gh.ReadFrom(data[off:]); err != nil {
			return err
		}
		if gh.Length < GenericHeaderLen {
			return ErrPayloadTooShort
		}
		payloadEnd := off + int(gh.Length)
		if payloadEnd > end {
			return ErrTruncated
		}
		bodyData := data[off+GenericHeaderLen : payloadEnd]
		payload, err := decodePayload(nextType, bodyData)
		switch {
		case err == nil:
			// The payload parsed whole.
		case isItemRejected(err):
			// RFC 7296 Section 3.3.6: one refused proposal or transform is dropped, and
			// the other proposals and transforms are processed as usual. The payload
			// carries what survived, so the message stands.
		case errors.Is(err, ErrUnknownPayload):
			// RFC 7296 Section 3.2: unknown type with critical bit set must reject.
			// Section 2.5 puts the one-octet payload type in the answering Notify, so
			// the error carries it out to the caller that builds that Notify.
			if gh.Critical {
				return &UnsupportedCritError{PayloadType: nextType}
			}
			payload = &PayloadRaw{PayloadType: nextType, Data: bodyData}
		default:
			return err
		}
		if sk, ok := payload.(*PayloadSK); ok {
			sk.InnerNextPayload = gh.NextPayload
		}
		m.Payloads = append(m.Payloads, PayloadEntry{
			Payload:  payload,
			Critical: gh.Critical,
		})
		nextType = gh.NextPayload
		off = payloadEnd
	}
	return nil
}
