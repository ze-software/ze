// Design: docs/architecture/wire/attributes.md -- AS-PATH wire encoding
// RFC: rfc/short/rfc6793.md -- ASN4-to-ASN2 transcoding for RS-client forwarding
// RFC: rfc/short/rfc7947.md -- Route Server: MUST NOT modify AS_PATH semantics
// Related: aspath_slot.go -- ASPathEdit, the edit-set rail that prepends on EBGP egress
// Related: aspath_collapse.go -- CollapseAS4Family, the ingest step that makes every
// payload here four-octet truth, which is why this file only narrows
// Related: aspath_as4.go -- shared AS4_PATH construction rule (RFC 6793 Section 4.2.2)

package wireu

import (
	"encoding/binary"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TranscodeASPath narrows the AS-path family of a four-octet UPDATE payload for
// an OLD-speaker destination, without prepending any AS numbers.
//
// It narrows only. Every payload on the forward path is four-octet truth,
// because the ingest collapse reconciles the AS-path family once per received
// UPDATE (aspath_collapse.go, CollapseAS4Family) and relabels the encoding
// context, so widening has no caller left and the direction is a defect rather
// than a request. A two-octet source is therefore REFUSED: answering 0 would
// hand the destination a two-octet AS_PATH labeled four-octet, which is a
// silently wrong answer rather than a reported one (ai/rules/principles.md).
//
// RFC 6793 Section 4.2.2: when sending to an OLD speaker, an AS number above
// 65535 is encoded as AS_TRANS (23456) in AS_PATH, and the real four-octet
// values are carried in a new AS4_PATH attribute. The AGGREGATOR follows the
// same rule through AS4_AGGREGATOR.
//
// RFC 7947 Section 2.2.2: a route server MUST NOT modify AS_PATH for an RS
// client. Narrowing preserves the AS numbers and changes only the wire
// encoding, so it does not violate that rule.
//
// Returns 0 when srcASN4 == dstASN4, which is the destination that needs no
// work at all. Returns the number of bytes written to dst on success.
func TranscodeASPath(dst, payload []byte, srcASN4, dstASN4 bool) (int, error) {
	if srcASN4 == dstASN4 {
		return 0, nil
	}
	if !srcASN4 {
		return 0, fmt.Errorf("transcode AS_PATH: %w", ErrASPathSourceNotASN4)
	}

	if len(payload) < 4 {
		return 0, fmt.Errorf("transcode AS_PATH: %w", ErrUpdateTruncated)
	}

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if len(payload) < 2+wdLen+2 {
		return 0, fmt.Errorf("transcode AS_PATH: %w", ErrUpdateTruncated)
	}

	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2

	if len(payload) < attrsStart+attrLen {
		return 0, fmt.Errorf("transcode AS_PATH: %w", ErrUpdateTruncated)
	}

	nlriStart := attrsStart + attrLen

	aspAttrOff := -1
	aspHdrLen := 0
	aspValueLen := 0
	as4AttrOff := -1
	as4HdrLen := 0
	as4ValueLen := 0
	aggAttrOff := -1
	aggHdrLen := 0
	aggValueLen := 0
	as4AggAttrOff := -1
	as4AggHdrLen := 0
	as4AggValueLen := 0

	off := attrsStart
	for off < attrsStart+attrLen {
		if off+3 > len(payload) {
			return 0, fmt.Errorf("transcode AS_PATH: truncated attribute at offset %d: %w", off, ErrUpdateMalformed)
		}

		flags := attribute.AttributeFlags(payload[off])
		code := attribute.AttributeCode(payload[off+1])

		var length int
		var hdrLen int
		if flags.IsExtLength() {
			if off+4 > len(payload) {
				return 0, fmt.Errorf("transcode AS_PATH: truncated ext-length attribute: %w", ErrUpdateMalformed)
			}
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		} else {
			length = int(payload[off+2])
			hdrLen = 3
		}

		if off+hdrLen+length > len(payload) {
			return 0, fmt.Errorf("transcode AS_PATH: attribute value overflows payload: %w", ErrUpdateMalformed)
		}

		if code == attribute.AttrASPath {
			aspAttrOff = off
			aspHdrLen = hdrLen
			aspValueLen = length
		}
		if code == attribute.AttrAS4Path {
			as4AttrOff = off
			as4HdrLen = hdrLen
			as4ValueLen = length
		}
		if code == attribute.AttrAggregator {
			aggAttrOff = off
			aggHdrLen = hdrLen
			aggValueLen = length
		}
		if code == attribute.AttrAS4Aggregator {
			as4AggAttrOff = off
			as4AggHdrLen = hdrLen
			as4AggValueLen = length
		}

		off += hdrLen + length
	}

	if aspAttrOff == -1 && aggAttrOff == -1 {
		return copy(dst, payload), nil
	}

	// --- AS_PATH transcoding ---

	var existingPath *attribute.ASPath
	var newASPValueLen, newASPHdrLen int
	if aspAttrOff != -1 {
		aspValueStart := aspAttrOff + aspHdrLen
		aspValue := payload[aspValueStart : aspValueStart+aspValueLen]

		var err error
		existingPath, err = attribute.ParseASPath(aspValue, srcASN4)
		if err != nil {
			return 0, fmt.Errorf("transcode AS_PATH: parse existing: %w", err)
		}

		newASPValueLen = existingPath.LenWithASN4(dstASN4)
		newASPHdrLen = 3
		if newASPValueLen > 255 {
			newASPHdrLen = 4
		}
	}

	// RFC 6793 Section 4.2.2: emit AS4_PATH when the path holds a non-mappable
	// ASN and the destination is an OLD speaker. as4PathForPath owns the rule.
	as4Path := as4PathForPath(existingPath, dstASN4)
	newAS4WireSize := as4PathWireSize(as4Path)

	// --- AGGREGATOR transcoding (RFC 6793 Section 4.2.2) ---

	var aggASN uint32
	var aggIP []byte
	var needAS4Agg bool
	var newAggValueLen int
	if aggAttrOff != -1 && aggValueLen == 8 {
		aggValueStart := aggAttrOff + aggHdrLen
		aggASN = binary.BigEndian.Uint32(payload[aggValueStart : aggValueStart+4])
		aggIP = payload[aggValueStart+4 : aggValueStart+8]
		needAS4Agg = aggASN > 65535
		newAggValueLen = 6
	}

	// --- Compute new attrLen ---

	newAttrLen := attrLen

	if aspAttrOff != -1 {
		newAttrLen += (newASPHdrLen + newASPValueLen) - (aspHdrLen + aspValueLen)
	}
	if as4AttrOff != -1 {
		newAttrLen -= as4HdrLen + as4ValueLen
	}
	newAttrLen += newAS4WireSize

	if newAggValueLen != 0 {
		newAggHdrLen := 3
		newAttrLen += (newAggHdrLen + newAggValueLen) - (aggHdrLen + aggValueLen)
	}
	if as4AggAttrOff != -1 && newAggValueLen != 0 {
		newAttrLen -= as4AggHdrLen + as4AggValueLen
	}
	if needAS4Agg {
		newAttrLen += 3 + 8 // header(3) + AS4_AGGREGATOR value(8)
	}

	// --- Build output: iterate attributes, replace/skip special ones ---

	n := copy(dst, payload[:attrsStart])

	off = attrsStart
	for off < nlriStart {
		flags := attribute.AttributeFlags(payload[off])
		code := attribute.AttributeCode(payload[off+1])
		var length, hdrLen int
		if flags.IsExtLength() {
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		} else {
			length = int(payload[off+2])
			hdrLen = 3
		}

		switch code {
		case attribute.AttrASPath:
			if existingPath != nil {
				n += attribute.WriteHeaderTo(dst, n, attribute.FlagTransitive,
					attribute.AttrASPath, uint16(newASPValueLen)) //nolint:gosec // bounded by BGP max
				n += existingPath.WriteToWithASN4(dst, n, dstASN4)
			} else {
				n += copy(dst[n:], payload[off:off+hdrLen+length])
			}

		case attribute.AttrAS4Path:
			// Skip: replaced by new AS4_PATH appended at end.

		case attribute.AttrAggregator:
			switch {
			case newAggValueLen != 0:
				n += attribute.WriteHeaderTo(dst, n,
					attribute.FlagOptional|attribute.FlagTransitive,
					attribute.AttrAggregator, uint16(newAggValueLen)) //nolint:gosec // bounded by BGP max
				// RFC 6793 Section 4.2.2: "set the AS number field in the existing
				// AGGREGATOR attribute to the reserved AS number, AS_TRANS".
				asn := aggASN
				if asn > 65535 {
					asn = asTrans
				}
				binary.BigEndian.PutUint16(dst[n:], uint16(asn)) //nolint:gosec // AS_TRANS handles overflow
				n += 2
				n += copy(dst[n:], aggIP)
			case length != 6 && length != 8:
				// Genuinely malformed: no other AGGREGATOR length is readable
				// (RFC 4271 Section 5.1.7, RFC 6793 Section 4.2.2).
				if tn := WriteTombstone(dst, n, payload[off], attribute.AttrAggregator, hdrLen, length, TombstoneInvalidLength); tn > 0 {
					n += tn
				} else {
					n += copy(dst[n:], payload[off:off+hdrLen+length])
				}
			default:
				// Well formed but not re-encodable at this width. Optional
				// transitive, so it travels on unchanged (RFC 4271 Section 5.1.7).
				n += copy(dst[n:], payload[off:off+hdrLen+length])
			}

		case attribute.AttrAS4Aggregator:
			if newAggValueLen == 0 {
				n += copy(dst[n:], payload[off:off+hdrLen+length])
			}
			// Otherwise skip: replaced by new AS4_AGGREGATOR appended at end.

		default:
			n += copy(dst[n:], payload[off:off+hdrLen+length])
		}

		off += hdrLen + length
	}

	// Append new AS4_PATH.
	n += writeAS4PathAttr(dst, n, as4Path)

	// Append new AS4_AGGREGATOR.
	if needAS4Agg {
		n += attribute.WriteHeaderTo(dst, n,
			attribute.FlagOptional|attribute.FlagTransitive,
			attribute.AttrAS4Aggregator, 8)
		binary.BigEndian.PutUint32(dst[n:], aggASN)
		n += 4
		n += copy(dst[n:], aggIP)
	}

	// Copy NLRI.
	n += copy(dst[n:], payload[nlriStart:])

	// Update attrLen.
	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(newAttrLen)) //nolint:gosec // bounded by BGP max

	return n, nil
}
