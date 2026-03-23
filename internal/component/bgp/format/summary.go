// Design: docs/architecture/api/json-format.md — message formatting

package format

import (
	"encoding/binary"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// formatSummary formats an UPDATE as lightweight NLRI metadata.
// Extracts only: which legacy sections are present and which MP families appear.
// Cost: parse section offsets + scan attribute headers for MP_REACH/MP_UNREACH + 3 bytes per MP attr.
//
// Output JSON structure (under bgp.nlri):
//
//	"announce":   bool   - legacy NLRI section has bytes
//	"withdrawn":  bool   - legacy withdrawn section has bytes
//	"mp-reach":   string - MP_REACH_NLRI family name, or "" if absent
//	"mp-unreach": string - MP_UNREACH_NLRI family name, or "" if absent
func formatSummary(peer plugin.PeerInfo, rawBytes []byte, msgID uint64, direction string) string {
	sections, err := wire.ParseUpdateSections(rawBytes)
	if err != nil {
		return formatSummaryEmpty(peer, msgID, direction)
	}

	announce := sections.NLRILen(rawBytes) > 0
	withdrawn := sections.WithdrawnLen() > 0

	var mpReach, mpUnreach string
	if attrsBytes := sections.Attrs(rawBytes); len(attrsBytes) > 0 {
		mpReach, mpUnreach = scanMPFamilies(attrsBytes)
	}

	return buildSummaryJSON(peer, msgID, direction, announce, withdrawn, mpReach, mpUnreach)
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
			afi := nlri.AFI(binary.BigEndian.Uint16(attrs[valueOff : valueOff+2]))
			safi := nlri.SAFI(attrs[valueOff+2])
			mpReach = nlri.Family{AFI: afi, SAFI: safi}.String()
		} else if attrCode == attribute.AttrMPUnreachNLRI && valueLen >= 3 && mpUnreach == "" {
			afi := nlri.AFI(binary.BigEndian.Uint16(attrs[valueOff : valueOff+2]))
			safi := nlri.SAFI(attrs[valueOff+2])
			mpUnreach = nlri.Family{AFI: afi, SAFI: safi}.String()
		}

		// Advance to next attribute
		off = valueOff + valueLen
	}
	return mpReach, mpUnreach
}

// buildSummaryJSON builds the summary format JSON string.
// message.id is always included (even when 0) — intentional divergence from parsed/raw/full
// formats which omit id when 0. Summary consumers need stable field presence for lightweight parsing.
// NOTE: Output uses "message":{"type":"update" prefix, same as other formats.
// FormatSentMessage relies on this for its string replacement to "type":"sent".
func buildSummaryJSON(peer plugin.PeerInfo, msgID uint64, direction string, announce, withdrawn bool, mpReach, mpUnreach string) string {
	var sb strings.Builder
	sb.Grow(256)

	sb.WriteString(`{"type":"bgp","bgp":{"message":{"type":"update","id":`)
	sb.WriteString(strconv.FormatUint(msgID, 10))
	if direction != "" {
		sb.WriteString(`,"direction":"`)
		sb.WriteString(direction)
		sb.WriteByte('"')
	}
	sb.WriteString(`}`)
	writePeerJSON(&sb, peer)
	sb.WriteString(`,"nlri":{"announce":`)
	sb.WriteString(strconv.FormatBool(announce))
	sb.WriteString(`,"withdrawn":`)
	sb.WriteString(strconv.FormatBool(withdrawn))
	sb.WriteString(`,"mp-reach":"`)
	sb.WriteString(mpReach)
	sb.WriteString(`","mp-unreach":"`)
	sb.WriteString(mpUnreach)
	sb.WriteString(`"}}}`)
	sb.WriteByte('\n')

	return sb.String()
}

// formatSummaryEmpty returns summary JSON for a malformed or empty UPDATE.
func formatSummaryEmpty(peer plugin.PeerInfo, msgID uint64, direction string) string {
	return buildSummaryJSON(peer, msgID, direction, false, false, "", "")
}
