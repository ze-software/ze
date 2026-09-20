// Design: docs/architecture/wire/nlri.md — MVPN in-process JSON writer
// RFC: rfc/short/rfc6514.md -- MCAST-VPN NLRI (SAFI 5), Sections 4.5 and 4.6
// Related: mvpn.go -- mvpnToJSON, the registry decoder's answer for the same NLRI
//
// AppendJSON writes the MVPN NLRI's JSON representation directly into a
// caller-provided []byte, bypassing the wire-encode / hex / re-parse /
// map-marshal round-trip used by the RPC decoder path (DecodeNLRIHex).

package mvpn

import (
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// AppendJSON satisfies nlri.JSONAppender.
// Keys alphabetical to match json.Marshal(map[string]any) output.
//
// The members are ExaBGP's, from the json() methods of sharedjoin.py,
// sourcejoin.py, sourcead.py and GenericMVPN. A reader cannot tell which of the
// two decode paths served it, so mvpnToJSON (mvpn.go) MUST answer the same
// members for the same octets, and TestBothJSONPathsAgree holds them together.
//
// "raw" is the whole NLRI in upper-case hex, the RFC 6514 Section 4 Route Type
// and Length octets included, because that is what a reader needs to decode a
// route type ze does not parse.
func (m *MVPN) AppendJSON(buf []byte) []byte {
	routeType := m.RouteType()

	buf = append(buf, `{"code":`...)
	buf = strconv.AppendUint(buf, uint64(routeType), 10)

	if !routeType.bodyParsed() {
		buf = append(buf, `,"parsed":false,"raw":"`...)
		buf = textbuf.HexUpper(buf, m.packed)
		return append(buf, `"}`...)
	}

	buf = append(buf, `,"group":"`...)
	buf = m.group.AppendTo(buf)
	buf = append(buf, `","name":"`...)
	buf = append(buf, routeType.name()...)
	buf = append(buf, `","parsed":true,"raw":"`...)
	buf = textbuf.HexUpper(buf, m.packed)
	buf = append(buf, `","rd":"`...)
	buf = append(buf, m.rd.String()...)
	buf = append(buf, `","source":"`...)
	buf = m.source.AppendTo(buf)
	buf = append(buf, '"')

	// RFC 6514 Section 4.6 gives only the two C-multicast routes a Source AS.
	// ExaBGP writes it as a decimal STRING, so the member keeps its type across
	// both feeds.
	if routeType.hasSourceAS() {
		buf = append(buf, `,"source-as":"`...)
		buf = strconv.AppendUint(buf, uint64(m.sourceAS), 10)
		buf = append(buf, '"')
	}

	return append(buf, '}')
}
