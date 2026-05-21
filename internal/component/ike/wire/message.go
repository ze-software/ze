// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — IKEv2 message structure (Section 3)
package wire

import "errors"

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
		if err != nil {
			if !errors.Is(err, ErrUnknownPayload) {
				return err
			}
			// RFC 7296 Section 3.2: unknown type with critical bit set must reject
			if gh.Critical {
				return ErrUnsupportedCrit
			}
			payload = &PayloadRaw{PayloadType: nextType, Data: bodyData}
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
