// Design: docs/architecture/wire/nlri.md — Labeled Unicast in-process JSON writer
//
// AppendJSON writes the labeled unicast NLRI's JSON representation directly
// into a caller-provided []byte, bypassing the wire-encode / hex / re-parse
// round-trip used by the RPC decoder path (DecodeNLRIHex).

package labeled

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// AppendJSON satisfies nlri.JSONAppender.
// Matches DecodeNLRIHex output, its keys in the order json.Marshal sorts
// them: {"labels":[[label,entry],...],"path-id":N,"prefix":"..."}, path-id
// present only when the NLRI carries a Path Identifier.
//
// Each member is the pair RFC 8277 Section 2.1 puts on the wire: the 20-bit
// label a reader matches on, and the 3-octet entry it came from, which carries
// the traffic class and the bottom-of-stack bit.
func (l *LabeledUnicast) AppendJSON(buf []byte) []byte {
	buf = append(buf, '{')
	if len(l.labels) > 0 {
		buf = append(buf, `"labels":[`...)
		for i, entry := range l.labels {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '[')
			buf = strconv.AppendUint(buf, uint64(nlri.LabelValue(entry)), 10)
			if entry != 0 {
				buf = append(buf, ',')
				buf = strconv.AppendUint(buf, uint64(entry), 10)
			}
			buf = append(buf, ']')
		}
		buf = append(buf, `],`...)
	}
	// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
	// message, the NLRI encoding MUST be extended by prepending the Path
	// Identifier field, which is of four octets."
	//
	// No value is reserved, so presence follows hasPath, never the value: an
	// identifier of zero is written.
	if l.hasPath {
		buf = append(buf, `"path-id":`...)
		buf = strconv.AppendUint(buf, uint64(l.pathID), 10)
		buf = append(buf, ',')
	}
	buf = append(buf, `"prefix":"`...)
	var pfxBuf [44]byte
	buf = append(buf, l.prefix.AppendTo(pfxBuf[:0])...)
	buf = append(buf, `"}`...)
	return buf
}
