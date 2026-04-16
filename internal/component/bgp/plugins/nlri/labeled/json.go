// Design: docs/architecture/wire/nlri.md — Labeled Unicast in-process JSON writer
//
// AppendJSON writes the labeled unicast NLRI's JSON representation directly
// into a strings.Builder, bypassing the wire-encode / hex / re-parse
// round-trip used by the RPC decoder path (DecodeNLRIHex).

package labeled

import (
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter.
// Matches DecodeNLRIHex output: {"labels":[n,...],"prefix":"..."}.
// Note: labels is a flat array here (not nested like VPN).
func (l *LabeledUnicast) AppendJSON(sb *strings.Builder) {
	sb.WriteByte('{')
	if len(l.labels) > 0 {
		sb.WriteString(`"labels":[`)
		for i, lab := range l.labels {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatUint(uint64(lab), 10))
		}
		sb.WriteString(`],`)
	}
	sb.WriteString(`"prefix":"`)
	var pfxBuf [44]byte
	sb.Write(l.prefix.AppendTo(pfxBuf[:0]))
	sb.WriteString(`"}`)
}
