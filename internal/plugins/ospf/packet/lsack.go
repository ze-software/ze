// Design: docs/architecture/ospf/ospf-2-wire.md -- Link State Acknowledgment packet body codec
// RFC 2328 Appendix A.3.6: Link State Acknowledgment packet.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// LSAck is the Link State Acknowledgment packet body. The struct is shared via the types
// leaf (one type across the engine, neighbor FSM, and both wire codecs).
type LSAck = types.LSAck

// DecodeLSAck parses an LS Ack body containing consecutive LSA headers.
func DecodeLSAck(body []byte) (LSAck, error) {
	if len(body)%types.LSAHeaderLen != 0 {
		return LSAck{}, ErrLength
	}
	out := LSAck{Headers: make([]LSAHeader, 0, len(body)/types.LSAHeaderLen)}
	off := 0
	for off < len(body) {
		h, err := DecodeLSAHeader(body[off : off+types.LSAHeaderLen])
		if err != nil {
			return LSAck{}, err
		}
		out.Headers = append(out.Headers, h)
		off += types.LSAHeaderLen
	}
	return out, nil
}

// writeLSAck serializes the OSPFv2 LS Ack body into buf at off (codec-layer wire encode).
func writeLSAck(a LSAck, buf []byte, off int) int {
	for _, h := range a.Headers {
		off = writeLSAHeader(h, buf, off)
	}
	return off
}
