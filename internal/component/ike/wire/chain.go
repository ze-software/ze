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
		if off+GenericHeaderLen > len(data) {
			break
		}
		var gh GenericHeader
		if err := gh.ReadFrom(data[off:]); err != nil {
			return nil, err
		}
		if gh.Length < GenericHeaderLen {
			return nil, ErrPayloadTooShort
		}
		payloadEnd := off + int(gh.Length)
		if payloadEnd > len(data) {
			break
		}
		bodyData := data[off+GenericHeaderLen : payloadEnd]
		payload, err := decodePayload(nextType, bodyData)
		if err != nil {
			if !errors.Is(err, ErrUnknownPayload) {
				return nil, err
			}
			if gh.Critical {
				return nil, ErrUnsupportedCrit
			}
			payload = &PayloadRaw{PayloadType: nextType, Data: bodyData}
		}
		payloads = append(payloads, PayloadEntry{
			Payload:  payload,
			Critical: gh.Critical,
		})
		nextType = gh.NextPayload
		off = payloadEnd
	}
	return payloads, nil
}
