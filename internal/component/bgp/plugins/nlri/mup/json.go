// Design: docs/architecture/wire/nlri.md — MUP in-process JSON writer
//
// AppendJSON writes the MUP NLRI's JSON representation directly into a
// strings.Builder, bypassing the wire-encode / hex / re-parse / map-marshal
// round-trip used by the RPC decoder path (DecodeNLRIHex).

package mup

import (
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter.
// Keys alphabetical to match json.Marshal(map[string]any) output.
func (m *MUP) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`{"arch-type":`)
	sb.WriteString(strconv.FormatUint(uint64(m.archType), 10))
	sb.WriteString(`,"rd":"`)
	sb.WriteString(m.rd.String())
	sb.WriteString(`","route-type":`)
	sb.WriteString(strconv.FormatUint(uint64(m.routeType), 10))
	sb.WriteString("}")
}
