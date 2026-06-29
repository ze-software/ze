// Design: docs/research/l2tpv2-implementation-guide.md -- LCP Echo (RFC 1661 §5.8)
// Related: lcp.go -- packet codec used to build/parse Echo packets

package ppp

import (
	"encoding/binary"
	"errors"
)

// errLCPEchoTooShort is returned when an Echo-Request/Reply payload is
// shorter than the 4-byte Magic-Number field.
var errLCPEchoTooShort = errors.New("ppp: LCP Echo packet shorter than 4-byte Magic-Number")

// ParseLCPEchoMagic extracts the Magic-Number from an Echo-Request,
// Echo-Reply, or Discard-Request payload. RFC 1661 §5.8 specifies a
// 4-byte Magic-Number aiding loopback detection.
//
// The remaining payload (after the 4-byte Magic-Number) is opaque
// per RFC 1661 -- "Data" field, ignored on receive.
func ParseLCPEchoMagic(data []byte) (magic uint32, err error) {
	if len(data) < 4 {
		return 0, errLCPEchoTooShort
	}
	return binary.BigEndian.Uint32(data[:4]), nil
}

// WriteLCPEcho builds an LCP Echo-Request or Echo-Reply at buf[off:].
// Caller passes the LCP code (LCPEchoRequest or LCPEchoReply), the
// Identifier (echoes the request's identifier on a reply; monotonic
// counter on a request), ze's local Magic-Number, and any extra Data
// bytes that follow the Magic-Number per RFC 1661 §5.8.
//
// extraData may be nil for a bare Echo with just Magic-Number. On a
// reply built via BuildLCPEchoReply, extraData carries the request's
// post-Magic bytes verbatim so the peer's "Data" field is mirrored.
//
// Returns total bytes written (4 LCP header + 4 Magic-Number +
// len(extraData)). The caller MUST ensure buf[off:] has the capacity.
func WriteLCPEcho(buf []byte, off int, code, identifier uint8, magic uint32, extraData []byte) int {
	// Magic-Number is the first 4 bytes of LCP Data; the rest is the
	// optional Data field. Both go inline in buf[off+lcpHeaderLen:].
	binary.BigEndian.PutUint32(buf[off+lcpHeaderLen:], magic)
	n := copy(buf[off+lcpHeaderLen+4:], extraData)
	dataLen := 4 + n
	// Backfill the LCP header in place. Cannot use WriteLCPPacket here
	// because that would copy the data slice over itself (no-op but
	// confusing); inline the header writes instead.
	buf[off] = code
	buf[off+1] = identifier
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(lcpHeaderLen+dataLen))
	return lcpHeaderLen + dataLen
}

// BuildLCPEchoReply writes the Echo-Reply for a received Echo-Request.
// Identifier MUST echo the request's identifier (RFC 1661 §5.8).
//
// magic is ze's local Magic-Number, NOT the peer's value from the
// request. requestData is the LCP Data of the received Echo-Request;
// per RFC 1661 §5.8 the Reply mirrors any post-Magic-Number bytes
// (call requestData == pkt.Data, where pkt is the parsed request --
// the first 4 bytes are the peer's magic and are dropped, the rest
// is mirrored).
func BuildLCPEchoReply(buf []byte, off int, requestID uint8, magic uint32, requestData []byte) int {
	var extra []byte
	if len(requestData) > 4 {
		extra = requestData[4:]
	}
	return WriteLCPEcho(buf, off, LCPEchoReply, requestID, magic, extra)
}

// IsLCPLoopback reports whether a received Echo-Reply's payload
// contains ze's own Magic-Number, indicating a looped link. RFC 1661
// §6.4 documents this as a permitted loopback detection mechanism.
//
// This is a defensive check; ze does not currently take action on a
// loopback detection beyond logging (deferred to operations).
func IsLCPLoopback(payload []byte, localMagic uint32) bool {
	m, err := ParseLCPEchoMagic(payload)
	if err != nil {
		return false
	}
	return m == localMagic
}
