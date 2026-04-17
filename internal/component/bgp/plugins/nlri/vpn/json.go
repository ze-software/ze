// Design: docs/architecture/wire/nlri.md — VPN in-process JSON writer
//
// AppendJSON writes the VPN NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).

package vpn

import (
	"strconv"
)

// AppendJSON satisfies nlri.JSONAppender.
// Keys alphabetical to match json.Marshal(map[string]any) output.
// Shape: {"labels":[[n],...],"prefix":"...","rd":"..."}.
// path-id is transport-level (RFC 7911) and intentionally NOT emitted, mirroring
// the lossy hex round-trip through Bytes() (path-id is not encoded in the payload).
func (v *VPN) AppendJSON(buf []byte) []byte {
	buf = append(buf, '{')
	first := true

	if len(v.labels) > 0 {
		buf = append(buf, `"labels":[`...)
		for i, l := range v.labels {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '[')
			buf = strconv.AppendUint(buf, uint64(l), 10)
			buf = append(buf, ']')
		}
		buf = append(buf, ']')
		first = false
	}

	if !first {
		buf = append(buf, ',')
	}
	buf = append(buf, `"prefix":"`...)
	var pfxBuf [44]byte
	buf = append(buf, v.prefix.AppendTo(pfxBuf[:0])...)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, v.rd.String()...)
	buf = append(buf, `"}`...)
	return buf
}
