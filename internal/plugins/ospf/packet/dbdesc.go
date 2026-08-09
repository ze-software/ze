// Design: docs/architecture/ospf/ospf-2-wire.md -- Database Description packet body codec
// RFC 2328 Appendix A.3.3: Database Description packet.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

const (
	dbDescFixedLen = 8
	DDFlagInit     = 0x04
	DDFlagMore     = 0x02
	DDFlagMaster   = 0x01
)

// DBDesc is the Database Description packet body; the struct is shared via the types leaf
// (one type across the engine, neighbor FSM, and both wire codecs).
type DBDesc = types.DBDesc

// DecodeDBDesc parses a Database Description body.
func DecodeDBDesc(body []byte) (DBDesc, error) {
	if len(body) < dbDescFixedLen {
		return DBDesc{}, ErrTruncated
	}
	if (len(body)-dbDescFixedLen)%types.LSAHeaderLen != 0 {
		return DBDesc{}, ErrLength
	}
	count := (len(body) - dbDescFixedLen) / types.LSAHeaderLen
	out := DBDesc{
		InterfaceMTU: readUint16(body, 0),
		Options:      types.Options(body[2]),
		Flags:        body[3],
		DDSequence:   readUint32(body, 4),
		Headers:      make([]LSAHeader, 0, count),
	}
	off := dbDescFixedLen
	for range count {
		h, err := DecodeLSAHeader(body[off : off+types.LSAHeaderLen])
		if err != nil {
			return DBDesc{}, err
		}
		out.Headers = append(out.Headers, h)
		off += types.LSAHeaderLen
	}
	return out, nil
}

// dbDescEncodedLen returns the OSPFv2 Database Description body length.
func dbDescEncodedLen(d DBDesc) int { return dbDescFixedLen + len(d.Headers)*types.LSAHeaderLen }

// writeDBDesc serializes the OSPFv2 Database Description body into buf at off.
func writeDBDesc(d DBDesc, buf []byte, off int) int {
	off += writeUint16(buf, off, d.InterfaceMTU)
	off += d.Options.WriteTo(buf, off)
	buf[off] = d.Flags
	off++
	off += writeUint32(buf, off, d.DDSequence)
	for _, h := range d.Headers {
		off = writeLSAHeader(h, buf, off)
	}
	return off
}
