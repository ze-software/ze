// Design: docs/architecture/wire/isis.md -- ISO 9542 ISH for point-to-point discovery.
// RFC 1195 sections 4.4 and 5.3.10 extend ISH with Protocols Supported.
// RFC 995 sections 8.2 and 8.7 publish the ES-IS fixed header and ISH layout.

package packet

import (
	"encoding/binary"
	"errors"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// ISH offsets, relative to the start of the PDU (ISO 9542 section 8.7):
//
//	0      | NLPID = 0x82                    |
//	1      | total PDU length                |
//	2      | version = 1                     |
//	3      | reserved = 0                    |
//	4      | 000 | PDU type = 4              |
//	5..6   | holding time                    |
//	7..8   | Fletcher checksum               |
//	9      | NET length                     |
//	10..   | NET, then optional TLVs         |
const (
	ESISProtocolDiscriminator = 0x82
	ishPDUType                = 4
	ishNETOffset              = 10
	ishChecksumOffset         = 7
	ishLengthMax              = 254
)

// ErrISHChecksum reports corruption of an ISH whose checksum is enabled.
var ErrISHChecksum = errors.New("isis packet: invalid ISH checksum")

// ISH is an ISO 9542 Intermediate System Hello. Its NET identifies the neighbor
// before IS-IS Hellos start. Decoded TLV values alias the source PDU; callers MUST
// keep that buffer stable until done and MUST call ReleaseTLVs after use.
type ISH struct {
	NET         types.NET
	HoldingTime uint16
	TLVs        []TLV
}

// EncodedLen returns the entire ISH length, including the NET and options.
func (h *ISH) EncodedLen() int {
	return ishNETOffset + h.NET.Len() + tlvsEncodedLen(h.TLVs)
}

// WriteTo emits an ISH and returns the new offset. The caller MUST provide room
// for a valid NET and TLVs, and MUST limit the whole PDU to 254 octets. The length
// and checksum are backfilled after the NET and options have been written.
func (h *ISH) WriteTo(buf []byte, off int) int {
	start := off
	buf[off] = ESISProtocolDiscriminator
	buf[off+1] = 0
	buf[off+2] = 1
	buf[off+3] = 0
	buf[off+4] = ishPDUType
	binary.BigEndian.PutUint16(buf[off+5:off+7], h.HoldingTime)
	buf[off+7] = 0
	buf[off+8] = 0
	buf[off+9] = byte(h.NET.Len())
	off += ishNETOffset
	off += h.NET.WriteTo(buf, off)
	off = writeTLVs(buf, off, h.TLVs)
	buf[start+1] = byte(off - start)
	hi, lo := Checksum(buf[start:off], ishChecksumOffset)
	buf[start+ishChecksumOffset] = hi
	buf[start+ishChecksumOffset+1] = lo
	return off
}

// DecodeISH parses one complete ISH and ignores link-layer padding beyond its
// length indicator. It accepts a disabled checksum (zero), but rejects a bad
// nonzero checksum. On success the caller MUST call ReleaseTLVs on the result.
func DecodeISH(pdu []byte) (ISH, error) {
	if len(pdu) < ishNETOffset {
		return ISH{}, ErrTruncated
	}
	if pdu[0] != ESISProtocolDiscriminator {
		return ISH{}, ErrBadDiscriminator
	}
	if pdu[2] != 1 {
		return ISH{}, ErrBadVersion
	}
	if pdu[4]&pduTypeMask != ishPDUType {
		return ISH{}, ErrUnknownPDUType
	}
	length := int(pdu[1])
	if length > ishLengthMax {
		return ISH{}, ErrLength
	}
	if length < ishNETOffset {
		return ISH{}, ErrLength
	}
	if length > len(pdu) {
		return ISH{}, ErrTruncated
	}
	pdu = pdu[:length]
	// RFC 995 section 8.2.7: "A non-zero value indicates that the checksum
	// must be processed. If the checksum calculation fails, the PDU must
	// be discarded."
	if binary.BigEndian.Uint16(pdu[7:9]) != 0 {
		if !VerifyChecksum(pdu) {
			return ISH{}, ErrISHChecksum
		}
	}
	netEnd := ishNETOffset + int(pdu[9])
	if netEnd > len(pdu) {
		return ISH{}, ErrTruncated
	}
	net, err := types.NETFromBytes(pdu[ishNETOffset:netEnd])
	if err != nil {
		return ISH{}, err
	}
	tlvs, err := DecodeTLVs(pdu[netEnd:])
	if err != nil {
		ReleaseTLVs(tlvs)
		return ISH{}, err
	}
	return ISH{
		NET:         net,
		HoldingTime: binary.BigEndian.Uint16(pdu[5:7]),
		TLVs:        tlvs,
	}, nil
}
