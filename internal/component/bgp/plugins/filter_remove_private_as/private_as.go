// Design: docs/architecture/core-design.md -- remove-private-as policy action
// RFC: rfc/short/rfc6996.md -- Private Use ASN ranges
// Related: filter_remove_private_as.go -- SDK entry point and filter evaluator

package filter_remove_private_as

import (
	"encoding/binary"
	"encoding/json"
	"math"
)

const (
	removeModeStripText  = "strip"
	removeModePeerASText = "peer-as"
)

type removeMode uint8

const (
	removeModeStrip removeMode = iota
	removeModePeerAS
)

func (m removeMode) String() string {
	if m == removeModePeerAS {
		return removeModePeerASText
	}
	return removeModeStripText
}

// intentValue is the value of the as-path remove intent for the mode, a JSON
// string: "private" strips each Private Use ASN, "private-peer-as" replaces
// it with the peer's AS (docs/plugin-development/protocol.md, "The Sets of a
// filter-update answer").
func (m removeMode) intentValue() json.RawMessage {
	if m == removeModePeerAS {
		return json.RawMessage(`"private-peer-as"`)
	}
	return json.RawMessage(`"private"`)
}

// RFC 6996 Section 5: Private Use ASNs are 64512-65534 and
// 4200000000-4294967294, inclusive.
func isPrivateASN(asn uint32) bool {
	return (asn >= 64512 && asn <= 65534) || (asn >= 4200000000 && asn <= 4294967294)
}

// asPathFieldHasPrivateASN reports whether the fields object the engine sent
// carries a Private Use ASN in its as-path. The filter declares as-path as its
// only route field, so the object is {"as-path":[...]} (an AS_SET nested as
// an array) and every number in it is an ASN: the scan reads each decimal run
// in place and allocates nothing. An absent as-path carries none.
func asPathFieldHasPrivateASN(fields []byte) bool {
	var asn uint64
	digits := false
	for _, c := range fields {
		if c >= '0' && c <= '9' {
			asn = asn*10 + uint64(c-'0')
			digits = true
			if asn > math.MaxUint32 {
				return false // no ASN: the engine writes 32-bit ASNs only
			}
			continue
		}
		if digits && isPrivateASN(uint32(asn)) {
			return true
		}
		asn, digits = 0, false
	}
	return digits && isPrivateASN(uint32(asn))
}

func hasPrivateAS4PathPayload(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + withdrawnLen
	if len(payload) < attrLenOff+2 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrStart := attrLenOff + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return false
	}
	for off := attrStart; off < attrEnd; {
		if off+3 > attrEnd {
			return false
		}
		flags := payload[off]
		code := payload[off+1]
		hdrLen := 3
		valueLen := int(payload[off+2])
		if flags&0x10 != 0 {
			if off+4 > attrEnd {
				return false
			}
			hdrLen = 4
			valueLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		}
		valueStart := off + hdrLen
		valueEnd := valueStart + valueLen
		if valueEnd > attrEnd {
			return false
		}
		if code == 17 && as4PathValueHasPrivateASN(payload[valueStart:valueEnd]) {
			return true
		}
		off = valueEnd
	}
	return false
}

func as4PathValueHasPrivateASN(value []byte) bool {
	for off := 0; off < len(value); {
		if off+2 > len(value) {
			return false
		}
		count := int(value[off+1])
		off += 2
		for range count {
			if off+4 > len(value) {
				return false
			}
			if isPrivateASN(binary.BigEndian.Uint32(value[off:])) {
				return true
			}
			off += 4
		}
	}
	return false
}
