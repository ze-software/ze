// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec in-process JSON writer
//
// AppendJSON writes the FlowSpec NLRI's JSON representation directly into a
// strings.Builder, bypassing the wire-encode / hex / re-parse round-trip
// used by the RPC decoder path (DecodeNLRIHex).
//
// This implementation still builds an intermediate map and calls json.Marshal
// because FlowSpec JSON is highly variable (any of ~13 component types, with
// numeric operators, bitmask flags, nested OR/AND groupings). Hand-streaming
// that shape would duplicate every branch in componentToJSON. The savings vs
// the hex path are the four allocations from Bytes() + hex.Encode + hex.Decode
// + ParseFlowSpec that we skip by passing the already-parsed FlowSpec directly.

package flowspec

import (
	"encoding/json"
	"strings"
)

// AppendJSON satisfies nlri.JSONWriter for FlowSpec (SAFI 133).
func (f *FlowSpec) AppendJSON(sb *strings.Builder) {
	result := flowSpecToJSON(f, f.family.String(), nil)
	b, err := json.Marshal(result)
	if err != nil {
		sb.WriteString(`{"error":"flowspec json encode failed"}`)
		return
	}
	sb.Write(b)
}

// AppendJSON satisfies nlri.JSONWriter for FlowSpecVPN (SAFI 134).
func (f *FlowSpecVPN) AppendJSON(sb *strings.Builder) {
	rd := f.rd
	result := flowSpecToJSON(f.flowSpec, f.Family().String(), &rd)
	b, err := json.Marshal(result)
	if err != nil {
		sb.WriteString(`{"error":"flowspec-vpn json encode failed"}`)
		return
	}
	sb.Write(b)
}
