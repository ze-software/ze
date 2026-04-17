// Design: docs/architecture/api/json-format.md — message formatting
// Related: text_update.go — UPDATE-path orchestrator that invokes appendSummary
// Related: text.go — appendPeerJSON reused for the peer fragment

package format

import (
	"encoding/binary"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// appendSummary appends an UPDATE summary (lightweight NLRI metadata) to buf.
// Extracts only: which legacy sections are present and which MP families appear.
// Cost: parse section offsets + scan attribute headers for MP_REACH/MP_UNREACH + 3 bytes per MP attr.
//
// Output JSON structure (under bgp.nlri):
//
//	"announce":   bool   - legacy NLRI section has bytes
//	"withdrawn":  bool   - legacy withdrawn section has bytes
//	"mp-reach":   string - MP_REACH_NLRI family name, or "" if absent
//	"mp-unreach": string - MP_UNREACH_NLRI family name, or "" if absent
//
// messageType is "update" or "sent" -- threaded through so callers do not run
// strings.Replace surgery on the resulting JSON.
func appendSummary(buf []byte, peer *plugin.PeerInfo, rawBytes []byte, msgID uint64, direction, messageType string) []byte {
	sections, err := wire.ParseUpdateSections(rawBytes)
	if err != nil {
		return appendSummaryJSON(buf, peer, msgID, direction, messageType, false, false, "", "")
	}

	announce := sections.NLRILen(rawBytes) > 0
	withdrawn := sections.WithdrawnLen() > 0

	var mpReach, mpUnreach string
	if attrsBytes := sections.Attrs(rawBytes); len(attrsBytes) > 0 {
		mpReach, mpUnreach = scanMPFamilies(attrsBytes)
	}

	return appendSummaryJSON(buf, peer, msgID, direction, messageType, announce, withdrawn, mpReach, mpUnreach)
}

// scanMPFamilies walks attribute headers looking for MP_REACH and MP_UNREACH.
// Returns family strings for each, or "" if not found.
// Only reads attribute headers + 3 bytes of value (AFI/SAFI) — no full attribute parsing.
//
// RFC 4271 Section 4.3 attribute header format:
//
//	Flags(1) + Type(1) + Length(1 or 2 if extended)
//
// RFC 4760 Section 3 MP_REACH_NLRI value: AFI(2) + SAFI(1) + ...
// RFC 4760 Section 4 MP_UNREACH_NLRI value: AFI(2) + SAFI(1) + ...
func scanMPFamilies(attrs []byte) (mpReach, mpUnreach string) {
	off := 0
	for off < len(attrs) {
		if off+2 >= len(attrs) {
			break // Need at least flags + type
		}

		flags := attrs[off]
		code := attrs[off+1]
		extended := flags&0x10 != 0

		var valueLen int
		var valueOff int
		if extended {
			if off+4 > len(attrs) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(attrs[off+2 : off+4]))
			valueOff = off + 4
		} else {
			if off+3 > len(attrs) {
				break
			}
			valueLen = int(attrs[off+2])
			valueOff = off + 3
		}

		if valueOff+valueLen > len(attrs) {
			break // Truncated attribute
		}

		// RFC 4760: MP_REACH and MP_UNREACH values start with AFI(2) + SAFI(1).
		attrCode := attribute.AttributeCode(code)
		if attrCode == attribute.AttrMPReachNLRI && valueLen >= 3 && mpReach == "" {
			afi := family.AFI(binary.BigEndian.Uint16(attrs[valueOff : valueOff+2]))
			safi := family.SAFI(attrs[valueOff+2])
			mpReach = family.Family{AFI: afi, SAFI: safi}.String()
		} else if attrCode == attribute.AttrMPUnreachNLRI && valueLen >= 3 && mpUnreach == "" {
			afi := family.AFI(binary.BigEndian.Uint16(attrs[valueOff : valueOff+2]))
			safi := family.SAFI(attrs[valueOff+2])
			mpUnreach = family.Family{AFI: afi, SAFI: safi}.String()
		}

		// Advance to next attribute
		off = valueOff + valueLen
	}
	return mpReach, mpUnreach
}

// appendSummaryJSON appends the summary format JSON to buf, terminated by '\n'.
// message.id is always included (even when 0) — intentional divergence from parsed/raw/full
// formats which omit id when 0. Summary consumers need stable field presence for lightweight parsing.
// messageType is written directly ("update" for received, "sent" for sent).
func appendSummaryJSON(buf []byte, peer *plugin.PeerInfo, msgID uint64, direction, messageType string, announce, withdrawn bool, mpReach, mpUnreach string) []byte {
	buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"`...)
	buf = append(buf, messageType...)
	buf = append(buf, `","id":`...)
	buf = strconv.AppendUint(buf, msgID, 10)
	if direction != "" {
		// Defensive escape: direction is bounded today ("received"/"sent");
		// keep the legacy escape shape without bringing fmt back.
		buf = append(buf, `,"direction":"`...)
		buf = appendJSONString(buf, direction)
		buf = append(buf, '"')
	}
	buf = append(buf, `},`...)
	buf = appendPeerJSON(buf, peer)
	buf = append(buf, `,"nlri":{"announce":`...)
	buf = strconv.AppendBool(buf, announce)
	buf = append(buf, `,"withdrawn":`...)
	buf = strconv.AppendBool(buf, withdrawn)
	buf = append(buf, `,"mp-reach":"`...)
	buf = append(buf, mpReach...)
	buf = append(buf, `","mp-unreach":"`...)
	buf = append(buf, mpUnreach...)
	buf = append(buf, `"}}}`...)
	buf = append(buf, '\n')
	return buf
}
