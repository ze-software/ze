// Design: docs/architecture/wire/nlri.md — VPN in-process JSON writer
//
// AppendJSON writes the VPN NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).

package vpn

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// AppendJSON satisfies nlri.JSONAppender.
// Keys alphabetical to match json.Marshal(map[string]any) output.
// Shape: {"labels":[[label,entry],...],"prefix":"...","rd":"..."}.
//
// Each member is the pair RFC 8277 Section 2.1 puts on the wire: the 20-bit
// label a reader matches on, and the whole 3-octet entry it came from, which
// carries the traffic class and the bottom-of-stack bit. Writing the label
// alone left no way to tell a one-label stack from the first entry of a longer
// one. The entry is omitted when it is zero, which is the one case it says
// nothing the label did not.
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
			buf = strconv.AppendUint(buf, uint64(nlri.LabelValue(l)), 10)
			if l != 0 {
				buf = append(buf, ',')
				buf = strconv.AppendUint(buf, uint64(l), 10)
			}
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
