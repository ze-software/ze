// Design: docs/architecture/wire/buffer-writer.md -- payload chain parsing
// RFC: rfc/short/rfc7296.md -- encrypted payload contents (Section 3.14)
package wire

import "errors"

// ParsePayloadChain parses a chain of IKEv2 payloads from raw data
// (e.g. decrypted SK payload contents). firstType is the payload type
// of the first payload in the chain.
func ParsePayloadChain(data []byte, firstType uint8) ([]PayloadEntry, error) {
	var payloads []PayloadEntry
	nextType := firstType
	off := 0
	for nextType != 0 && off < len(data) {
		if len(payloads) >= MaxPayloads {
			return nil, ErrTooManyPayloads
		}
		// RFC 7296 Section 2.21.2 MUST: a message
		// "where the whole message is malformed (rather than just bad payload contents)"
		// is rejected in its entirety.
		// A chain that ends inside a generic header is malformed.
		// It returns an error rather than the payloads read so far.
		// The partial chain made a malformed message look like a message
		// that is missing a payload.
		// Each caller then took a branch written for an absent payload.
		if off+GenericHeaderLen > len(data) {
			return nil, ErrTruncated
		}
		var gh GenericHeader
		if err := gh.ReadFrom(data[off:]); err != nil {
			return nil, err
		}
		if gh.Length < GenericHeaderLen {
			return nil, ErrPayloadTooShort
		}
		// The same rule as above. A payload whose declared length runs past the end of
		// the chain truncates the message (RFC 7296 Section 2.21.2).
		payloadEnd := off + int(gh.Length)
		if payloadEnd > len(data) {
			return nil, ErrTruncated
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
			//
			// This arm mirrors Message.ReadFrom.
			// Without it the inner chain refused the whole message over one bad
			// proposal. Every SA payload of a CREATE_CHILD_SA or an IKE_AUTH
			// travels in the inner chain.
		case errors.Is(err, ErrUnknownPayload):
			if gh.Critical {
				// RFC 7296 Section 2.5: the answering Notify carries the one-octet
				// payload type, so the error carries it too.
				return nil, &unsupportedCritError{PayloadType: nextType}
			}
			payload = &payloadRaw{PayloadType: nextType, Data: bodyData}
		default:
			return nil, err
		}
		payloads = append(payloads, PayloadEntry{
			Payload:  payload,
			Critical: gh.Critical,
		})
		nextType = gh.NextPayload
		off = payloadEnd
	}
	// The loop also ends when the data runs out while a payload still names a NEXT
	// payload. That chain promised a payload and delivered none.
	// RFC 7296 Section 2.21.2 counts that as a malformed message.
	// It is not a message that is missing a payload.
	//
	// Message.ReadFrom MUST NOT carry this check.
	// An Encrypted payload is the last payload of an outer message
	// (RFC 7296 Section 3.1).
	// Its generic header's Next Payload names the first payload INSIDE the
	// ciphertext. Each well-formed protected message therefore ends the outer
	// chain with a non-zero Next Payload.
	// An inner chain holds no Encrypted payload, so it has no such case.
	if nextType != 0 {
		return nil, ErrTruncated
	}
	return payloads, nil
}
