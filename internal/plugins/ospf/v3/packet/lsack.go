// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Link State Acknowledgment packet body codec.
// RFC: rfc/short/rfc5340.md (§A.3.6 Link State Acknowledgment packet)

package packet

// LSAck is the OSPFv3 Link State Acknowledgment packet body: a list of 20-octet
// LSA headers (RFC 5340 §A.3.6).
type LSAck struct {
	Headers []LSAHeader
}

// DecodeLSAck parses an LS Ack body containing consecutive LSA headers.
func DecodeLSAck(body []byte) (LSAck, error) {
	if len(body)%LSAHeaderLen != 0 {
		return LSAck{}, ErrLength
	}
	out := LSAck{Headers: make([]LSAHeader, 0, len(body)/LSAHeaderLen)}
	off := 0
	for off < len(body) {
		h, err := DecodeLSAHeader(body[off : off+LSAHeaderLen])
		if err != nil {
			return LSAck{}, err
		}
		out.Headers = append(out.Headers, h)
		off += LSAHeaderLen
	}
	return out, nil
}

// EncodedLen returns the LS Ack body length.
func (a LSAck) EncodedLen() int { return len(a.Headers) * LSAHeaderLen }

// WriteTo serializes the LS Ack body into buf at off.
func (a LSAck) WriteTo(buf []byte, off int) int {
	for _, h := range a.Headers {
		off = h.WriteTo(buf, off)
	}
	return off
}
