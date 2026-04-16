// Design: docs/architecture/wire/nlri-evpn.md — EVPN in-process JSON writer
//
// AppendJSON writes each EVPN NLRI type's JSON representation directly into a
// strings.Builder, bypassing the wire-encode / hex / re-parse / map-marshal
// round-trip used by the RPC decoder path (DecodeNLRIHex).
//
// Output shape MUST match evpnToJSON for the same NLRI. Key order is
// alphabetical to match json.Marshal(map[string]any).

package evpn

import (
	"strconv"
	"strings"
)

// hexUpper is the uppercase hex alphabet, matching fmt.Sprintf("%X", ...).
const hexUpper = "0123456789ABCDEF"

// writeHexUpper encodes b into sb as uppercase hex with no allocations.
func writeHexUpper(sb *strings.Builder, b []byte) {
	for _, x := range b {
		sb.WriteByte(hexUpper[x>>4])
		sb.WriteByte(hexUpper[x&0x0F])
	}
}

// appendRawField writes the NLRI payload (the bytes after the 2-byte
// type+length header) as an uppercase hex string for the "raw" JSON field.
// Uses a stack-allocated scratch buffer sized for the largest EVPN type.
func appendRawField(sb *strings.Builder, e EVPN) {
	var scratch [256]byte
	n := e.WriteTo(scratch[:], 0)
	if n < 2 {
		return
	}
	writeHexUpper(sb, scratch[2:n])
}

// writeCommonPrefix writes "code", "esi" (if present), fields up to the
// point where type-specific fields diverge. Called by each EVPNTypeN AppendJSON.
// Returns true if more keys should follow (always true here).

// AppendJSON for EVPNType1 (Ethernet Auto-Discovery).
// Keys: code, esi, ethernet-tag, label, name, parsed, raw, rd.
func (e *EVPNType1) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`[{"code":1,"esi":"`)
	sb.WriteString(formatESIForJSON(e.ESI()))
	sb.WriteString(`","ethernet-tag":`)
	sb.WriteString(strconv.FormatUint(uint64(e.EthernetTag()), 10))
	sb.WriteString(`,"label":`)
	appendLabelsNested(sb, e.Labels())
	sb.WriteString(`,"name":"Ethernet Auto-Discovery","parsed":true,"raw":"`)
	appendRawField(sb, e)
	sb.WriteString(`","rd":"`)
	sb.WriteString(e.RD().String())
	sb.WriteString(`"}]`)
}

// AppendJSON for EVPNType2 (MAC/IP advertisement).
// Keys: code, esi, ethernet-tag, ip (optional), label, mac, name, parsed, raw, rd.
func (e *EVPNType2) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`[{"code":2,"esi":"`)
	sb.WriteString(formatESIForJSON(e.ESI()))
	sb.WriteString(`","ethernet-tag":`)
	sb.WriteString(strconv.FormatUint(uint64(e.EthernetTag()), 10))
	if ip := e.IP(); ip.IsValid() {
		sb.WriteString(`,"ip":"`)
		sb.WriteString(ip.String())
		sb.WriteString(`"`)
	}
	sb.WriteString(`,"label":`)
	appendLabelsNested(sb, e.Labels())
	sb.WriteString(`,"mac":"`)
	sb.WriteString(formatMACUpper(e.MAC()))
	sb.WriteString(`","name":"MAC/IP advertisement","parsed":true,"raw":"`)
	appendRawField(sb, e)
	sb.WriteString(`","rd":"`)
	sb.WriteString(e.RD().String())
	sb.WriteString(`"}]`)
}

// AppendJSON for EVPNType3 (Inclusive Multicast Ethernet Tag).
// Keys: code, ethernet-tag, name, originator, parsed, raw, rd.
func (e *EVPNType3) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`[{"code":3,"ethernet-tag":`)
	sb.WriteString(strconv.FormatUint(uint64(e.EthernetTag()), 10))
	sb.WriteString(`,"name":"Inclusive Multicast","originator":"`)
	sb.WriteString(e.OriginatorIP().String())
	sb.WriteString(`","parsed":true,"raw":"`)
	appendRawField(sb, e)
	sb.WriteString(`","rd":"`)
	sb.WriteString(e.RD().String())
	sb.WriteString(`"}]`)
}

// AppendJSON for EVPNType4 (Ethernet Segment).
// Keys: code, esi, name, originator, parsed, raw, rd.
func (e *EVPNType4) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`[{"code":4,"esi":"`)
	sb.WriteString(formatESIForJSON(e.ESI()))
	sb.WriteString(`","name":"Ethernet Segment","originator":"`)
	sb.WriteString(e.OriginatorIP().String())
	sb.WriteString(`","parsed":true,"raw":"`)
	appendRawField(sb, e)
	sb.WriteString(`","rd":"`)
	sb.WriteString(e.RD().String())
	sb.WriteString(`"}]`)
}

// AppendJSON for EVPNType5 (IP Prefix).
// Keys: code, esi, ethernet-tag, gateway (optional), label, name, parsed, prefix, raw, rd.
func (e *EVPNType5) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`[{"code":5,"esi":"`)
	sb.WriteString(formatESIForJSON(e.ESI()))
	sb.WriteString(`","ethernet-tag":`)
	sb.WriteString(strconv.FormatUint(uint64(e.EthernetTag()), 10))
	if gw := e.Gateway(); gw.IsValid() && !gw.IsUnspecified() {
		sb.WriteString(`,"gateway":"`)
		sb.WriteString(gw.String())
		sb.WriteString(`"`)
	}
	sb.WriteString(`,"label":`)
	appendLabelsNested(sb, e.Labels())
	sb.WriteString(`,"name":"IP Prefix","parsed":true,"prefix":"`)
	sb.WriteString(e.Prefix().String())
	sb.WriteString(`","raw":"`)
	appendRawField(sb, e)
	sb.WriteString(`","rd":"`)
	sb.WriteString(e.RD().String())
	sb.WriteString(`"}]`)
}

// AppendJSON for EVPNGeneric (unparsed / unknown route type).
// Keys: code, parsed, raw.
func (e *EVPNGeneric) AppendJSON(sb *strings.Builder) {
	sb.WriteString(`[{"code":`)
	sb.WriteString(strconv.FormatUint(uint64(e.RouteType()), 10))
	sb.WriteString(`,"parsed":false,"raw":"`)
	appendRawField(sb, e)
	sb.WriteString(`"}]`)
}

// appendLabelsNested writes labels as [[n1],[n2],...] matching formatLabelsForJSON.
// Empty labels render as [[0]] (per existing behavior).
func appendLabelsNested(sb *strings.Builder, labels []uint32) {
	if len(labels) == 0 {
		sb.WriteString(`[[0]]`)
		return
	}
	sb.WriteByte('[')
	for i, l := range labels {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('[')
		sb.WriteString(strconv.FormatUint(uint64(l), 10))
		sb.WriteByte(']')
	}
	sb.WriteByte(']')
}
