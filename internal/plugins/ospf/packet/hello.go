// Design: plan/learned/956-ospf-2-wire.md -- Hello packet body codec
// RFC 2328 Appendix A.3.2: Hello packet.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

const helloFixedLen = 20

// Hello is the OSPF Hello packet body; the struct is shared via the types leaf (a superset
// of the v2/v6 fields). The OSPFv2 wire encode/decode below is version-specific.
type Hello = types.Hello

// DecodeHello parses a Hello body.
func DecodeHello(body []byte) (Hello, error) {
	if len(body) < helloFixedLen {
		return Hello{}, ErrTruncated
	}
	if (len(body)-helloFixedLen)%types.RouterIDLen != 0 {
		return Hello{}, ErrLength
	}
	count := (len(body) - helloFixedLen) / types.RouterIDLen
	out := Hello{
		NetworkMask:   readIPv4(body, 0),
		HelloInterval: readUint16(body, 4),
		Options:       types.Options(body[6]),
		Priority:      body[7],
		DeadInterval:  readUint32(body, 8),
		DR:            readIPv4(body, 12),
		BDR:           readIPv4(body, 16),
		Neighbors:     make([]types.RouterID, 0, count),
	}
	off := helloFixedLen
	for range count {
		id, err := types.RouterIDFromBytes(body[off : off+types.RouterIDLen])
		if err != nil {
			return Hello{}, err
		}
		out.Neighbors = append(out.Neighbors, id)
		off += types.RouterIDLen
	}
	return out, nil
}

// helloEncodedLen returns the OSPFv2 Hello body length.
func helloEncodedLen(h Hello) int { return helloFixedLen + len(h.Neighbors)*types.RouterIDLen }

// writeHello serializes the OSPFv2 Hello body into buf at off (codec-layer wire encode).
func writeHello(h Hello, buf []byte, off int) int {
	off += writeIPv4(buf, off, h.NetworkMask)
	off += writeUint16(buf, off, h.HelloInterval)
	off += h.Options.WriteTo(buf, off)
	buf[off] = h.Priority
	off++
	off += writeUint32(buf, off, h.DeadInterval)
	off += writeIPv4(buf, off, h.DR)
	off += writeIPv4(buf, off, h.BDR)
	for _, n := range h.Neighbors {
		off += n.WriteTo(buf, off)
	}
	return off
}
