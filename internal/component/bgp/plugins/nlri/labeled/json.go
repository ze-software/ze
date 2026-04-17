// Design: docs/architecture/wire/nlri.md — Labeled Unicast in-process JSON writer
//
// AppendJSON writes the labeled unicast NLRI's JSON representation directly
// into a caller-provided []byte, bypassing the wire-encode / hex / re-parse
// round-trip used by the RPC decoder path (DecodeNLRIHex).

package labeled

import (
	"strconv"
)

// AppendJSON satisfies nlri.JSONAppender.
// Matches DecodeNLRIHex output: {"labels":[n,...],"prefix":"..."}.
// Note: labels is a flat array here (not nested like VPN).
func (l *LabeledUnicast) AppendJSON(buf []byte) []byte {
	buf = append(buf, '{')
	if len(l.labels) > 0 {
		buf = append(buf, `"labels":[`...)
		for i, lab := range l.labels {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = strconv.AppendUint(buf, uint64(lab), 10)
		}
		buf = append(buf, `],`...)
	}
	buf = append(buf, `"prefix":"`...)
	var pfxBuf [44]byte
	buf = append(buf, l.prefix.AppendTo(pfxBuf[:0])...)
	buf = append(buf, `"}`...)
	return buf
}
