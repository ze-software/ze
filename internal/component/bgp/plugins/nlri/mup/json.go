// Design: docs/architecture/wire/nlri.md — MUP in-process JSON writer
//
// AppendJSON writes the MUP NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).

package mup

import (
	"strconv"
)

// AppendJSON satisfies nlri.JSONAppender.
// Keys alphabetical to match json.Marshal(map[string]any) output.
func (m *MUP) AppendJSON(buf []byte) []byte {
	buf = append(buf, `{"arch-type":`...)
	buf = strconv.AppendUint(buf, uint64(m.archType), 10)
	buf = append(buf, `,"rd":"`...)
	buf = append(buf, m.rd.String()...)
	buf = append(buf, `","route-type":`...)
	buf = strconv.AppendUint(buf, uint64(m.routeType), 10)
	buf = append(buf, '}')
	return buf
}
