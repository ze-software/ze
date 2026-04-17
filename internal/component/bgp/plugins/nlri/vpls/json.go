// Design: docs/architecture/wire/nlri.md — VPLS in-process JSON writer
//
// AppendJSON writes the VPLS NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).

package vpls

import (
	"strconv"
)

// AppendJSON satisfies nlri.JSONAppender.
// Keys alphabetical to match json.Marshal(map[string]any) output.
func (v *VPLS) AppendJSON(buf []byte) []byte {
	buf = append(buf, `{"label-base":`...)
	buf = strconv.AppendUint(buf, uint64(v.labelBase), 10)
	buf = append(buf, `,"rd":"`...)
	buf = append(buf, v.rd.String()...)
	buf = append(buf, `","ve-block-offset":`...)
	buf = strconv.AppendUint(buf, uint64(v.veBlockOffset), 10)
	buf = append(buf, `,"ve-block-size":`...)
	buf = strconv.AppendUint(buf, uint64(v.veBlockSize), 10)
	buf = append(buf, `,"ve-id":`...)
	buf = strconv.AppendUint(buf, uint64(v.veID), 10)
	buf = append(buf, '}')
	return buf
}
