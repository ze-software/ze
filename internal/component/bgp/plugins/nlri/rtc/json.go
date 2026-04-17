// Design: docs/architecture/wire/nlri.md — RTC in-process JSON writer
//
// AppendJSON writes the RTC NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).
//
// Output shape MUST match DecodeNLRIHex (keys alphabetical, RFC-8259 JSON).

package rtc

import (
	"strconv"
)

// AppendJSON satisfies nlri.JSONAppender.
// Format: {"is-default":bool,"origin-as":N,"route-target":"..."}.
func (r *RTC) AppendJSON(buf []byte) []byte {
	buf = append(buf, `{"is-default":`...)
	if r.IsDefault() {
		buf = append(buf, "true"...)
	} else {
		buf = append(buf, "false"...)
	}
	buf = append(buf, `,"origin-as":`...)
	buf = strconv.AppendUint(buf, uint64(r.originAS), 10)
	buf = append(buf, `,"route-target":"`...)
	buf = append(buf, r.routeTarget.String()...)
	buf = append(buf, `"}`...)
	return buf
}
