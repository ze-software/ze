// Design: plan/learned/969-ospfv3-2-wire.md -- OSPFv3 Database Description packet body codec.
// RFC: rfc/short/rfc5340.md (§A.3.3 Database Description packet)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// dbDescFixedLen is the 12-octet fixed prefix before the LSA header list. RFC
// 5340 §A.3.3 reorders the fields versus OSPFv2, widens Options to 24 bits, and
// adds a leading reserved octet plus a reserved octet before the flags.
const dbDescFixedLen = 12

// Database Description flags (RFC 5340 §A.3.3), unchanged from OSPFv2.
const (
	DDFlagInit   = 0x04 // I-bit: this is the first DD in the sequence
	DDFlagMore   = 0x02 // M-bit: more DD packets follow
	DDFlagMaster = 0x01 // MS-bit: the sender is the master
)

// DBDesc body field offsets (RFC 5340 §A.3.3, body-relative).
const (
	dbDescOptionsOff = 1
	dbDescMTUOff     = 4
	dbDescFlagsOff   = 7
	dbDescSeqOff     = 8
)

// DBDesc is the OSPFv3 Database Description packet body.
type DBDesc struct {
	Options      types.Options
	InterfaceMTU uint16
	Flags        uint8
	DDSequence   uint32
	Headers      []LSAHeader
}

// DecodeDBDesc parses a Database Description body. The trailing LSA headers must
// be a whole number of 20-octet headers.
func DecodeDBDesc(body []byte) (DBDesc, error) {
	if len(body) < dbDescFixedLen {
		return DBDesc{}, ErrTruncated
	}
	if (len(body)-dbDescFixedLen)%LSAHeaderLen != 0 {
		return DBDesc{}, ErrLength
	}
	count := (len(body) - dbDescFixedLen) / LSAHeaderLen
	opts, err := types.OptionsFromBytes(body, dbDescOptionsOff)
	if err != nil {
		return DBDesc{}, err
	}
	out := DBDesc{
		Options:      opts,
		InterfaceMTU: readUint16(body, dbDescMTUOff),
		Flags:        body[dbDescFlagsOff],
		DDSequence:   readUint32(body, dbDescSeqOff),
		Headers:      make([]LSAHeader, 0, count),
	}
	off := dbDescFixedLen
	for range count {
		h, err := DecodeLSAHeader(body[off : off+LSAHeaderLen])
		if err != nil {
			return DBDesc{}, err
		}
		out.Headers = append(out.Headers, h)
		off += LSAHeaderLen
	}
	return out, nil
}

// EncodedLen returns the Database Description body length.
func (d DBDesc) EncodedLen() int { return dbDescFixedLen + len(d.Headers)*LSAHeaderLen }

// WriteTo serializes the Database Description body into buf at off. The leading
// reserved octet, the reserved octet before the flags, and the high reserved
// bits of the flags octet are written zero (RFC 5340 §A.3.3).
func (d DBDesc) WriteTo(buf []byte, off int) int {
	buf[off] = 0
	off++
	off += d.Options.WriteTo(buf, off)
	off += writeUint16(buf, off, d.InterfaceMTU)
	buf[off] = 0
	off++
	buf[off] = d.Flags
	off++
	off += writeUint32(buf, off, d.DDSequence)
	for _, h := range d.Headers {
		off = h.WriteTo(buf, off)
	}
	return off
}
