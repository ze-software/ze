// Design: docs/architecture/wire/buffer-writer.md -- buffer-first encoding
// RFC: rfc/short/rfc7296.md -- Encrypted payload (Section 3.14)
package wire

// PayloadSK is the Encrypted (SK) payload (type 46).
// Stores raw ciphertext; decryption is handled by the crypto layer.
// InnerNextPayload carries the first inner payload type which RFC 7296
// Section 3.14 places in the SK generic header's Next Payload field
// (an exception to the normal next-in-chain semantics).
// DataOffset is where this payload's data starts in the message it was READ
// from, so it is one past the last octet of the Encrypted payload's own generic
// header. RFC 5282 Section 5.1 makes that boundary the end of the associated
// data an AEAD cipher authenticates: "The associated data (A) MUST consist of
// the partial contents of the IKEv2 message, starting from the first octet of
// the Fixed IKE Header through the last octet of the Payload Header of the
// Encrypted Payload (i.e., the fourth octet of the Encrypted Payload) ... This
// includes any payloads that are between the Fixed IKE Header and the Encrypted
// Payload."
//
// A constant 32 octets is that span only while no payload sits between the
// header and the Encrypted payload. Message.ReadFrom is the one reader that
// knows where this payload really begins, so it records the offset rather than
// leave every consumer to derive it.
//
// Zero means this payload was BUILT rather than read, and a decryptor MUST
// refuse that value rather than authenticate an empty prefix: a zero offset is
// indistinguishable from a message whose parse never ran (ai/rules/principles.md).
type PayloadSK struct {
	CipherText       []byte
	InnerNextPayload uint8
	DataOffset       int
}

func (p *PayloadSK) Type() uint8 { return PayloadTypeSK }

func (p *PayloadSK) WriteTo(buf []byte, off int) int {
	copy(buf[off:], p.CipherText)
	return len(p.CipherText)
}

func (p *PayloadSK) Len() int { return len(p.CipherText) }

func (p *PayloadSK) ReadFrom(data []byte) error {
	p.CipherText = make([]byte, len(data))
	copy(p.CipherText, data)
	return nil
}
