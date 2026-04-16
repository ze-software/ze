// Design: docs/architecture/wire/nlri.md — VPLS in-process JSON writer
//
// AppendJSON writes the VPLS NLRI's JSON representation directly into a
// strings.Builder, bypassing the wire-encode / hex / re-parse / map-marshal
// round-trip used by the RPC decoder path (DecodeNLRIHex).

package vpls

import (
	"strconv"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter.
// Keys alphabetical to match json.Marshal(map[string]any) output.
func (v *VPLS) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`{"label-base":`)
	sb.WriteString(strconv.FormatUint(uint64(v.labelBase), 10))
	sb.WriteString(`,"rd":"`)
	sb.WriteString(v.rd.String())
	sb.WriteString(`","ve-block-offset":`)
	sb.WriteString(strconv.FormatUint(uint64(v.veBlockOffset), 10))
	sb.WriteString(`,"ve-block-size":`)
	sb.WriteString(strconv.FormatUint(uint64(v.veBlockSize), 10))
	sb.WriteString(`,"ve-id":`)
	sb.WriteString(strconv.FormatUint(uint64(v.veID), 10))
	sb.WriteString("}")
}
