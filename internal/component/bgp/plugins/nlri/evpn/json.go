// Design: docs/architecture/wire/nlri-evpn.md — EVPN in-process JSON writer
//
// AppendJSON writes each EVPN NLRI type's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).
//
// Output shape MUST match evpnToJSON for the same NLRI. Key order is
// alphabetical to match json.Marshal(map[string]any).

package evpn

import (
	"strconv"
)

// hexUpper is the uppercase hex alphabet, matching fmt.Sprintf("%X", ...).
const hexUpper = "0123456789ABCDEF"

// appendHexUpper encodes b into buf as uppercase hex with no allocations.
func appendHexUpper(buf, b []byte) []byte {
	for _, x := range b {
		buf = append(buf, hexUpper[x>>4], hexUpper[x&0x0F])
	}
	return buf
}

// appendRawField writes the NLRI payload (the bytes after the 2-byte
// type+length header) as an uppercase hex string for the "raw" JSON field.
// Uses a stack-allocated scratch buffer sized for the largest EVPN type.
func appendRawField(buf []byte, e EVPN) []byte {
	var scratch [256]byte
	n := e.WriteTo(scratch[:], 0)
	if n < 2 {
		return buf
	}
	return appendHexUpper(buf, scratch[2:n])
}

// AppendJSON for EVPNType1 (Ethernet Auto-Discovery).
// Keys: code, esi, ethernet-tag, label, name, parsed, raw, rd.
func (e *EVPNType1) AppendJSON(buf []byte) []byte {
	buf = append(buf, `[{"code":1,"esi":"`...)
	buf = append(buf, formatESIForJSON(e.ESI())...)
	buf = append(buf, `","ethernet-tag":`...)
	buf = strconv.AppendUint(buf, uint64(e.EthernetTag()), 10)
	buf = append(buf, `,"label":`...)
	buf = appendLabelsNested(buf, e.Labels())
	buf = append(buf, `,"name":"Ethernet Auto-Discovery","parsed":true,"raw":"`...)
	buf = appendRawField(buf, e)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, e.RD().String()...)
	buf = append(buf, `"}]`...)
	return buf
}

// AppendJSON for EVPNType2 (MAC/IP advertisement).
// Keys: code, esi, ethernet-tag, ip (optional), label, mac, name, parsed, raw, rd.
func (e *EVPNType2) AppendJSON(buf []byte) []byte {
	buf = append(buf, `[{"code":2,"esi":"`...)
	buf = append(buf, formatESIForJSON(e.ESI())...)
	buf = append(buf, `","ethernet-tag":`...)
	buf = strconv.AppendUint(buf, uint64(e.EthernetTag()), 10)
	if ip := e.IP(); ip.IsValid() {
		buf = append(buf, `,"ip":"`...)
		buf = append(buf, ip.String()...)
		buf = append(buf, '"')
	}
	buf = append(buf, `,"label":`...)
	buf = appendLabelsNested(buf, e.Labels())
	buf = append(buf, `,"mac":"`...)
	buf = append(buf, formatMACUpper(e.MAC())...)
	buf = append(buf, `","name":"MAC/IP advertisement","parsed":true,"raw":"`...)
	buf = appendRawField(buf, e)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, e.RD().String()...)
	buf = append(buf, `"}]`...)
	return buf
}

// AppendJSON for EVPNType3 (Inclusive Multicast Ethernet Tag).
// Keys: code, ethernet-tag, name, originator, parsed, raw, rd.
func (e *EVPNType3) AppendJSON(buf []byte) []byte {
	buf = append(buf, `[{"code":3,"ethernet-tag":`...)
	buf = strconv.AppendUint(buf, uint64(e.EthernetTag()), 10)
	buf = append(buf, `,"name":"Inclusive Multicast","originator":"`...)
	buf = append(buf, e.OriginatorIP().String()...)
	buf = append(buf, `","parsed":true,"raw":"`...)
	buf = appendRawField(buf, e)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, e.RD().String()...)
	buf = append(buf, `"}]`...)
	return buf
}

// AppendJSON for EVPNType4 (Ethernet Segment).
// Keys: code, esi, name, originator, parsed, raw, rd.
func (e *EVPNType4) AppendJSON(buf []byte) []byte {
	buf = append(buf, `[{"code":4,"esi":"`...)
	buf = append(buf, formatESIForJSON(e.ESI())...)
	buf = append(buf, `","name":"Ethernet Segment","originator":"`...)
	buf = append(buf, e.OriginatorIP().String()...)
	buf = append(buf, `","parsed":true,"raw":"`...)
	buf = appendRawField(buf, e)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, e.RD().String()...)
	buf = append(buf, `"}]`...)
	return buf
}

// AppendJSON for EVPNType5 (IP Prefix).
// Keys: code, esi, ethernet-tag, gateway (optional), label, name, parsed, prefix, raw, rd.
func (e *EVPNType5) AppendJSON(buf []byte) []byte {
	buf = append(buf, `[{"code":5,"esi":"`...)
	buf = append(buf, formatESIForJSON(e.ESI())...)
	buf = append(buf, `","ethernet-tag":`...)
	buf = strconv.AppendUint(buf, uint64(e.EthernetTag()), 10)
	if gw := e.Gateway(); gw.IsValid() && !gw.IsUnspecified() {
		buf = append(buf, `,"gateway":"`...)
		buf = append(buf, gw.String()...)
		buf = append(buf, '"')
	}
	buf = append(buf, `,"label":`...)
	buf = appendLabelsNested(buf, e.Labels())
	buf = append(buf, `,"name":"IP Prefix","parsed":true,"prefix":"`...)
	buf = append(buf, e.Prefix().String()...)
	buf = append(buf, `","raw":"`...)
	buf = appendRawField(buf, e)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, e.RD().String()...)
	buf = append(buf, `"}]`...)
	return buf
}

// AppendJSON for EVPNGeneric (unparsed / unknown route type).
// Keys: code, parsed, raw.
func (e *EVPNGeneric) AppendJSON(buf []byte) []byte {
	buf = append(buf, `[{"code":`...)
	buf = strconv.AppendUint(buf, uint64(e.RouteType()), 10)
	buf = append(buf, `,"parsed":false,"raw":"`...)
	buf = appendRawField(buf, e)
	buf = append(buf, `"}]`...)
	return buf
}

// appendLabelsNested writes labels as [[n1],[n2],...] matching formatLabelsForJSON.
// Empty labels render as [[0]] (per existing behavior).
func appendLabelsNested(buf []byte, labels []uint32) []byte {
	if len(labels) == 0 {
		return append(buf, `[[0]]`...)
	}
	buf = append(buf, '[')
	for i, l := range labels {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		buf = strconv.AppendUint(buf, uint64(l), 10)
		buf = append(buf, ']')
	}
	buf = append(buf, ']')
	return buf
}
