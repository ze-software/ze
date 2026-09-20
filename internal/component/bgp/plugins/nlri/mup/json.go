// Design: docs/architecture/wire/nlri.md — MUP in-process JSON writer
// Related: types.go -- the parse that fills the fields written here
//
// AppendJSON writes the MUP NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).
//
// The two writers MUST produce the same members with the same values for the
// same NLRI: a reader cannot tell which path served it. AppendJSON writes its
// keys in alphabetical order so its bytes equal json.Marshal of the map, which
// is what TestMUPJSONPathsAgree compares.
//
// The member NAMES are ExaBGP's, not ze's usual kebab-case. MUP is the one
// family the ExaBGP bridge renames nothing for (exabgpNLRIMembers,
// internal/exabgp/bridge/bridge_event.go), so these names are the compatibility
// contract a script keying on `prefix_ip_len` or `endpoint_ip` reads, and
// test/exabgp-compat/encoding/conf-srv6-mup.ci pins them.

package mup

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Route type names, spelled as ExaBGP's MUP classes name them (isd.py, dsd.py,
// t1st.py, t2st.py). They reach an operator through the `name` member of a
// decoded route.
const (
	nameISD  = "InterworkSegmentDiscoveryRoute"
	nameDSD  = "DirectSegmentDiscoveryRoute"
	nameT1ST = "Type1SessionTransformedRoute"
	nameT2ST = "Type2SessionTransformedRoute"
)

// sourceIPAbsent is what the source_ip member holds when a Type 1 ST route
// carries no source address. It is ExaBGP's repr of an empty Python byte
// string, and a consumer of that field already reads it, so ze writes the same
// text rather than a second spelling of "absent".
const sourceIPAbsent = "b''"

// AppendJSON satisfies nlri.JSONAppender.
func (m *MUP) AppendJSON(buf []byte) []byte {
	if !m.parsed {
		// Keys: arch, code, parsed, raw.
		buf = append(buf, `{"arch":`...)
		buf = strconv.AppendUint(buf, uint64(m.archType), 10)
		buf = append(buf, `,"code":`...)
		buf = strconv.AppendUint(buf, uint64(m.routeType), 10)
		buf = append(buf, `,"parsed":false,"raw":"`...)
		buf = m.appendRaw(buf)
		return append(buf, `"}`...)
	}

	switch m.routeType {
	case MUPISD:
		return m.appendJSONISD(buf)
	case MUPDSD:
		return m.appendJSONDSD(buf)
	case MUPT1ST:
		return m.appendJSONT1ST(buf)
	case MUPT2ST:
		return m.appendJSONT2ST(buf)
	}
	// Unreachable: parsed is set only for the four route types above.
	return append(buf, `{}`...)
}

// appendJSONISD writes an Interwork Segment Discovery route.
// Keys: arch, code, name, prefix_ip, prefix_ip_len, raw, rd.
func (m *MUP) appendJSONISD(buf []byte) []byte {
	buf = m.appendArchCode(buf)
	buf = append(buf, `,"name":"`...)
	buf = append(buf, nameISD...)
	buf = append(buf, `","prefix_ip":"`...)
	buf = append(buf, m.prefix.String()...)
	buf = append(buf, `","prefix_ip_len":`...)
	buf = strconv.AppendUint(buf, uint64(m.prefixBits), 10)
	return m.appendRawRD(buf)
}

// appendJSONDSD writes a Direct Segment Discovery route.
// Keys: arch, code, ip, name, raw, rd.
func (m *MUP) appendJSONDSD(buf []byte) []byte {
	buf = m.appendArchCode(buf)
	buf = append(buf, `,"ip":"`...)
	buf = append(buf, m.address.String()...)
	buf = append(buf, `","name":"`...)
	buf = append(buf, nameDSD...)
	buf = append(buf, '"')
	return m.appendRawRD(buf)
}

// appendJSONT1ST writes a Type 1 Session Transformed route.
// Keys: arch, code, endpoint_ip, endpoint_ip_len, name, prefix_ip,
// prefix_ip_len, qfi, raw, rd, source_ip, source_ip_len, teid.
func (m *MUP) appendJSONT1ST(buf []byte) []byte {
	buf = m.appendArchCode(buf)
	buf = append(buf, `,"endpoint_ip":"`...)
	buf = append(buf, m.endpoint.String()...)
	buf = append(buf, `","endpoint_ip_len":`...)
	buf = strconv.AppendUint(buf, uint64(m.endpointBits), 10)
	buf = append(buf, `,"name":"`...)
	buf = append(buf, nameT1ST...)
	buf = append(buf, `","prefix_ip":"`...)
	buf = append(buf, m.prefix.String()...)
	buf = append(buf, `","prefix_ip_len":`...)
	buf = strconv.AppendUint(buf, uint64(m.prefixBits), 10)

	// teid and qfi are strings, which is how ExaBGP writes them (t1st.py json).
	buf = append(buf, `,"qfi":"`...)
	buf = strconv.AppendUint(buf, uint64(m.qfi), 10)
	buf = append(buf, `","raw":"`...)
	buf = m.appendRaw(buf)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, m.rd.String()...)
	buf = append(buf, `","source_ip":"`...)
	buf = append(buf, m.sourceIP()...)
	buf = append(buf, `","source_ip_len":`...)
	buf = strconv.AppendUint(buf, uint64(m.sourceBits), 10)
	buf = append(buf, `,"teid":"`...)
	buf = strconv.AppendUint(buf, uint64(m.teid), 10)
	return append(buf, `"}`...)
}

// appendJSONT2ST writes a Type 2 Session Transformed route.
// Keys: arch, code, endpoint_ip, endpoint_len, name, raw, rd, teid.
func (m *MUP) appendJSONT2ST(buf []byte) []byte {
	buf = m.appendArchCode(buf)
	buf = append(buf, `,"endpoint_ip":"`...)
	buf = append(buf, m.endpoint.String()...)
	buf = append(buf, `","endpoint_len":`...)
	buf = strconv.AppendUint(buf, uint64(m.endpointBits), 10)
	buf = append(buf, `,"name":"`...)
	buf = append(buf, nameT2ST...)
	buf = append(buf, `","raw":"`...)
	buf = m.appendRaw(buf)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, m.rd.String()...)
	buf = append(buf, `","teid":"`...)
	buf = strconv.AppendUint(buf, uint64(m.teid), 10)
	return append(buf, `"}`...)
}

// appendArchCode opens the object with the two members every route type shares
// ahead of its own: the architecture type and the route type.
func (m *MUP) appendArchCode(buf []byte) []byte {
	buf = append(buf, `{"arch":`...)
	buf = strconv.AppendUint(buf, uint64(m.archType), 10)
	buf = append(buf, `,"code":`...)
	return strconv.AppendUint(buf, uint64(m.routeType), 10)
}

// appendRawRD closes an object whose last two members are raw and rd. The two
// discovery route types end there; T1ST and T2ST carry members after rd and
// write their own.
func (m *MUP) appendRawRD(buf []byte) []byte {
	buf = append(buf, `,"raw":"`...)
	buf = m.appendRaw(buf)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, m.rd.String()...)
	return append(buf, `"}`...)
}

// appendRaw writes the whole NLRI as uppercase hex, the architecture type,
// route type and length octets included. That is what ExaBGP's `raw` member
// holds, and it is the text an operator compares against a packet capture.
func (m *MUP) appendRaw(buf []byte) []byte {
	var header [mupHeaderLen]byte
	header[0] = byte(m.archType)
	header[1] = byte(m.routeType >> 8)
	header[2] = byte(m.routeType)
	header[3] = byte(len(m.body)) //nolint:gosec // body came from a 1-octet Length
	buf = textbuf.HexUpper(buf, header[:])
	return textbuf.HexUpper(buf, m.body)
}

// sourceIP answers the source_ip member of a Type 1 ST route.
func (m *MUP) sourceIP() string {
	if m.sourceBits == 0 {
		return sourceIPAbsent
	}
	return m.source.String()
}

// mupToJSON converts a parsed MUP NLRI to a JSON-friendly map, for the RPC
// decoder path. The members are the ones AppendJSON writes, with the same
// values.
func mupToJSON(m *MUP) map[string]any {
	if !m.parsed {
		return map[string]any{
			"arch":   int(m.archType),
			"code":   int(m.routeType),
			"parsed": false,
			"raw":    m.rawHex(),
		}
	}

	route := map[string]any{
		"arch": int(m.archType),
		"code": int(m.routeType),
		"raw":  m.rawHex(),
		"rd":   m.rd.String(),
	}

	switch m.routeType {
	case MUPISD:
		route["name"] = nameISD
		route["prefix_ip"] = m.prefix.String()
		route["prefix_ip_len"] = int(m.prefixBits)
	case MUPDSD:
		route["name"] = nameDSD
		route["ip"] = m.address.String()
	case MUPT1ST:
		route["name"] = nameT1ST
		route["prefix_ip"] = m.prefix.String()
		route["prefix_ip_len"] = int(m.prefixBits)
		route["teid"] = textbuf.StringUint32(m.teid)
		route["qfi"] = textbuf.StringUint8(m.qfi)
		route["endpoint_ip"] = m.endpoint.String()
		route["endpoint_ip_len"] = int(m.endpointBits)
		route["source_ip"] = m.sourceIP()
		route["source_ip_len"] = int(m.sourceBits)
	case MUPT2ST:
		route["name"] = nameT2ST
		route["endpoint_ip"] = m.endpoint.String()
		route["endpoint_len"] = int(m.endpointBits)
		route["teid"] = textbuf.StringUint32(m.teid)
	}

	return route
}

// rawHex answers the raw member for the map path.
func (m *MUP) rawHex() string {
	// An NLRI is at most the 4-octet header plus what a 1-octet Length can
	// declare, so the scratch never grows.
	var scratch [mupHeaderLen + 255]byte
	n := m.WriteTo(scratch[:], 0)
	return textbuf.StringHexUpper(scratch[:n])
}
