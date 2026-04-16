// Design: docs/architecture/wire/nlri.md — VPN in-process JSON writer
//
// AppendJSON writes the VPN NLRI's JSON representation directly into a
// strings.Builder, bypassing the wire-encode / hex / re-parse / map-marshal
// round-trip used by the RPC decoder path (DecodeNLRIHex).

package vpn

import (
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter.
// Keys alphabetical to match json.Marshal(map[string]any) output.
// Shape: {"labels":[[n],...],"prefix":"...","rd":"..."}.
// path-id is transport-level (RFC 7911) and intentionally NOT emitted, mirroring
// the lossy hex round-trip through Bytes() (path-id is not encoded in the payload).
func (v *VPN) AppendJSON(sb *strings.Builder) {
	sb.WriteByte('{')
	first := true

	if len(v.labels) > 0 {
		sb.WriteString(`"labels":[`)
		for i, l := range v.labels {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteByte('[')
			sb.WriteString(strconv.FormatUint(uint64(l), 10))
			sb.WriteByte(']')
		}
		sb.WriteByte(']')
		first = false
	}

	if !first {
		sb.WriteByte(',')
	}
	sb.WriteString(`"prefix":"`)
	var pfxBuf [44]byte
	sb.Write(v.prefix.AppendTo(pfxBuf[:0]))
	sb.WriteString(`","rd":"`)
	sb.WriteString(v.rd.String())
	sb.WriteString(`"}`)
}
