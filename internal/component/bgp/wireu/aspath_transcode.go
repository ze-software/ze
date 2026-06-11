// Design: docs/architecture/wire/attributes.md -- AS-PATH wire encoding
// RFC: rfc/short/rfc6793.md -- ASN4-to-ASN2 transcoding for RS-client forwarding
// RFC: rfc/short/rfc7947.md -- Route Server: MUST NOT modify AS_PATH semantics
// Related: aspath_rewrite.go -- RewriteASPath (prepend + transcode, used by EBGP non-RS)

package wireu

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// TranscodeASPath re-encodes the AS_PATH attribute in an UPDATE payload from
// srcASN4 encoding to dstASN4 encoding without prepending any AS numbers.
//
// Only the 4→2 direction (srcASN4=true, dstASN4=false) is currently used.
// The 2→4 direction re-encodes AS_PATH to 4-byte but does NOT merge an
// existing AS4_PATH (RFC 6793 Section 4.2.3); that merge is done at ingress
// by the receiving session. Callers needing 2→4 with merge must do so
// separately.
//
// RFC 6793 Section 4.2.2: When sending to an OLD speaker (dstASN4=false),
// ASNs > 65535 are encoded as AS_TRANS (23456) in AS_PATH, and the original
// 4-byte values are carried in a new AS4_PATH attribute (type 17).
//
// RFC 7947 Section 2.2.2: Route servers MUST NOT modify AS_PATH for RS-client
// peers. Transcoding preserves semantic content (same AS numbers) while
// changing wire encoding, so it does not violate RFC 7947.
//
// Returns 0 when srcASN4 == dstASN4 (no transcoding needed).
// Returns the number of bytes written to dst on success.
func TranscodeASPath(dst, payload []byte, srcASN4, dstASN4 bool) (int, error) {
	if srcASN4 == dstASN4 {
		return 0, nil
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

		off += hdrLen + length
	}

	if aspAttrOff == -1 {
		return copy(dst, payload), nil
	}

	aspValueStart := aspAttrOff + aspHdrLen
	aspValue := payload[aspValueStart : aspValueStart+aspValueLen]

	existingPath, err := attribute.ParseASPath(aspValue, srcASN4)
	if err != nil {
		return 0, fmt.Errorf("transcode AS_PATH: parse existing: %w", err)
	}

	oldASPWireSize := aspHdrLen + aspValueLen
	newASPValueLen := existingPath.LenWithASN4(dstASN4)
	newASPHdrLen := 3
	if newASPValueLen > 255 {
		newASPHdrLen = 4
	}

	// RFC 6793 Section 4.2.2: compute AS4_PATH size for attrLen calculation.
	// Written directly into dst later (no intermediate buffer needed).
	var as4Path *attribute.AS4Path
	var newAS4ValLen, newAS4WireSize int
	if !dstASN4 && hasNonMappableASN(existingPath) {
		as4Path = &attribute.AS4Path{Segments: existingPath.Segments}
		newAS4ValLen = as4Path.Len()
		as4HdrSize := 3
		if newAS4ValLen > 255 {
			as4HdrSize = 4
		}
		newAS4WireSize = as4HdrSize + newAS4ValLen
	}

	oldAS4WireSize := 0
	if as4AttrOff != -1 {
		oldAS4WireSize = as4HdrLen + as4ValueLen
	}

	newAttrLen := attrLen + (newASPHdrLen + newASPValueLen) - oldASPWireSize - oldAS4WireSize + newAS4WireSize

	// Build output. Handle AS4_PATH appearing before or after AS_PATH.
	n := 0
	aspAttrEnd := aspAttrOff + oldASPWireSize

	if as4AttrOff != -1 && as4AttrOff < aspAttrOff {
		// AS4_PATH before AS_PATH: skip AS4_PATH in the prefix, then handle AS_PATH.
		as4AttrEnd := as4AttrOff + oldAS4WireSize
		n += copy(dst[n:], payload[:as4AttrOff])
		n += copy(dst[n:], payload[as4AttrEnd:aspAttrOff])
	} else {
		n += copy(dst[n:], payload[:aspAttrOff])
	}

	// Write transcoded AS_PATH.
	n += attribute.WriteHeaderTo(dst, n, attribute.FlagTransitive,
		attribute.AttrASPath, uint16(newASPValueLen)) //nolint:gosec // bounded by BGP max
	n += existingPath.WriteToWithASN4(dst, n, dstASN4)

	// Copy remaining attrs after AS_PATH, skipping old AS4_PATH if it follows.
	remaining := payload[aspAttrEnd:nlriStart]
	if as4AttrOff != -1 && as4AttrOff > aspAttrOff {
		relOff := as4AttrOff - aspAttrEnd
		n += copy(dst[n:], remaining[:relOff])
		n += copy(dst[n:], remaining[relOff+oldAS4WireSize:])
	} else {
		n += copy(dst[n:], remaining)
	}

	// Write new AS4_PATH directly into dst.
	if as4Path != nil {
		n += attribute.WriteHeaderTo(dst, n,
			attribute.FlagOptional|attribute.FlagTransitive,
			attribute.AttrAS4Path, uint16(newAS4ValLen)) //nolint:gosec // bounded by BGP max
		n += as4Path.WriteTo(dst, n)
	}

	// Copy NLRI.
	n += copy(dst[n:], payload[nlriStart:])

	// Update attrLen.
	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(newAttrLen)) //nolint:gosec // bounded by BGP max

	return n, nil
}

// hasNonMappableASN returns true if any ASN in the path exceeds 65535.
// RFC 6793: non-mappable ASNs require AS4_PATH when sending to OLD speakers.
func hasNonMappableASN(p *attribute.ASPath) bool {
	for _, seg := range p.Segments {
		for _, asn := range seg.ASNs {
			if asn > 65535 {
				return true
			}
		}
	}
	return false
}
