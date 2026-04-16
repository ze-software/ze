// Design: docs/architecture/wire/nlri.md — RTC in-process JSON writer
//
// AppendJSON writes the RTC NLRI's JSON representation directly into a
// strings.Builder, bypassing the wire-encode / hex / re-parse / map-marshal
// round-trip used by the RPC decoder path (DecodeNLRIHex).
//
// Output shape MUST match DecodeNLRIHex (keys alphabetical, RFC-8259 JSON).

package rtc

import (
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter.
// Format: {"is-default":bool,"origin-as":N,"route-target":"..."}.
func (r *RTC) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`{"is-default":`)
	if r.IsDefault() {
		sb.WriteString("true")
	} else {
		sb.WriteString("false")
	}
	sb.WriteString(`,"origin-as":`)
	sb.WriteString(strconv.FormatUint(uint64(r.originAS), 10))
	sb.WriteString(`,"route-target":"`)
	sb.WriteString(r.routeTarget.String())
	sb.WriteString(`"}`)
}
