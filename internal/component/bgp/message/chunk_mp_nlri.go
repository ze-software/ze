// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI / MP_UNREACH_NLRI chunking
// Related: update_split.go — UPDATE splitting and chunking

package message

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/iter"
)

// NewNLRIElements creates a generic element iterator for NLRI wire bytes.
// The iterator yields one NLRI at a time as subslices of nlriData.
// Use GetNLRISizeFunc to obtain the appropriate sizeFunc for the address family.
func NewNLRIElements(nlriData []byte, sizeFunc NLRISizeFunc) iter.Elements {
	return iter.NewElements(nlriData, iter.SizeFunc(sizeFunc))
}

// ChunkMPNLRI splits MP family NLRIs respecting maxSize.
//
// Handles all NLRI formats: Add-Path, Labeled, VPN, EVPN, FlowSpec, BGP-LS.
// Returns subslices of nlriData (zero-copy).
//
// Returns error if:
//   - Single NLRI exceeds maxSize (ErrNLRITooLarge)
//   - NLRI is truncated/malformed (ErrNLRIMalformed)
//
// RFC 4271 Section 4.3 - UPDATE message format, max 4096 bytes.
// RFC 8654 - Extended Message raises max to 65535 bytes.
// RFC 4760 - MP_REACH_NLRI / MP_UNREACH_NLRI wire format.
// RFC 7911 - ADD-PATH adds 4-byte path-id before each NLRI.
// RFC 8277 - Labeled unicast: length includes label bits.
// RFC 4364 - VPN: labels + 8-byte RD + prefix.
// RFC 7432 - EVPN: [route-type:1][length:1][payload].
// RFC 5575 - FlowSpec: max 4095 bytes per NLRI (CAN split).
// RFC 7752 - BGP-LS: 2-byte length, single NLRI can exceed 4096.
func ChunkMPNLRI(nlriData []byte, afi nlri.AFI, safi nlri.SAFI, addPath bool, maxSize int, dst [][]byte) ([][]byte, error) {
	if len(nlriData) == 0 {
		return dst, nil
	}

	sizeFunc := GetNLRISizeFunc(afi, safi, addPath)
	e := NewNLRIElements(nlriData, sizeFunc)

	// Fast path: validate structure and return single chunk if all fits
	if len(nlriData) <= maxSize {
		for e.Next() != nil {
		}
		if err := e.Err(); err != nil {
			return dst, err
		}
		return append(dst, nlriData), nil
	}

	// Slow path: split into chunks respecting element boundaries
	chunkStart := 0
	prevOffset := 0

	for elem := e.Next(); elem != nil; elem = e.Next() {
		nlriSize := len(elem)

		// RFC 7752 Section 3.2: BGP-LS NLRI uses 2-byte length field.
		// Single NLRI can exceed standard 4096-byte message size.
		// MUST return error if single NLRI > maxSize (cannot split).
		if nlriSize > maxSize {
			return dst, fmt.Errorf("%w: %d bytes, max %d", ErrNLRITooLarge, nlriSize, maxSize)
		}

		// Would overflow? Emit current chunk as subslice
		if e.Offset()-chunkStart > maxSize && prevOffset > chunkStart {
			dst = append(dst, nlriData[chunkStart:prevOffset])
			chunkStart = prevOffset
		}

		prevOffset = e.Offset()
	}

	if err := e.Err(); err != nil {
		return dst, err
	}

	// Emit remainder as subslice
	if prevOffset > chunkStart {
		dst = append(dst, nlriData[chunkStart:prevOffset])
	}

	return dst, nil
}

// ErrNLRIMalformed is returned when NLRI structure is invalid.
var ErrNLRIMalformed = fmt.Errorf("malformed NLRI")

// SplitMPNLRI splits MP family NLRIs, returning fitting slice and remaining.
// Returns subslices for zero-copy efficiency.
// This enables O(n) splitting across multiple calls instead of O(n²).
//
// Used when forwarding wire UPDATEs to peers with smaller buffers:
// - Extended Message peer (RFC 8654: 65535) → standard peer (RFC 4271: 4096)
//
// Returns:
//   - (data, nil, nil) if all data fits within maxSize
//   - (fitting, remaining, nil) if split was needed
//   - (nil, nil, error) if NLRI is malformed or single NLRI exceeds maxSize
//
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes.
// RFC 8654 - Extended Message raises to 65535 bytes.
// RFC 4760 - MP_REACH_NLRI / MP_UNREACH_NLRI wire format.
// RFC 7911 - ADD-PATH: 4-byte path-id before each NLRI.
func SplitMPNLRI(nlriData []byte, afi nlri.AFI, safi nlri.SAFI, addPath bool, maxSize int) (fitting, remaining []byte, err error) {
	if maxSize <= 0 {
		return nil, nil, fmt.Errorf("invalid maxSize: %d", maxSize)
	}
	if len(nlriData) == 0 {
		return nil, nil, nil
	}
	if len(nlriData) <= maxSize {
		return nlriData, nil, nil
	}

	sizeFunc := GetNLRISizeFunc(afi, safi, addPath)
	e := NewNLRIElements(nlriData, sizeFunc)

	prevOffset := 0
	for elem := e.Next(); elem != nil; elem = e.Next() {
		// RFC 7752 Section 3.2: BGP-LS can have single NLRI > 4096 bytes.
		// MUST error if single NLRI exceeds maxSize (cannot split one NLRI).
		if len(elem) > maxSize {
			return nil, nil, fmt.Errorf("%w: %d bytes, max %d", ErrNLRITooLarge, len(elem), maxSize)
		}
		if e.Offset() > maxSize {
			// This NLRI would exceed limit, split here
			return nlriData[:prevOffset], nlriData[prevOffset:], nil
		}
		prevOffset = e.Offset()
	}

	if err := e.Err(); err != nil {
		return nil, nil, err
	}

	// Everything fit (shouldn't reach here due to len check above, but safe)
	return nlriData, nil, nil
}

// NLRISizeFunc returns the size of the first NLRI in the buffer.
type NLRISizeFunc func(data []byte) (int, error)

// GetNLRISizeFunc returns the appropriate size function for the family.
// Exported for wire mode API input to split concatenated NLRIs.
func GetNLRISizeFunc(afi nlri.AFI, safi nlri.SAFI, addPath bool) NLRISizeFunc {
	switch {
	case safi == nlri.SAFIEVPN: // EVPN
		if addPath {
			return addPathEVPNNLRISize
		}
		return evpnNLRISize

	case safi == nlri.SAFIFlowSpec || safi == 134: // FlowSpec (133=IPv4, 134=IPv6)
		if addPath {
			return addPathFlowSpecNLRISize
		}
		return flowSpecNLRISize

	case afi == nlri.AFIBGPLS && safi == 71: // BGP-LS
		if addPath {
			return addPathBGPLSNLRISize
		}
		return bgpLSNLRISize

	case safi == nlri.SAFIVPN: // VPN (MPLS VPN)
		if addPath {
			return addPathVPNNLRISize
		}
		return vpnNLRISize

	case safi == nlri.SAFIMPLSLabel: // Labeled unicast
		if addPath {
			return addPathLabeledNLRISize
		}
		return labeledNLRISize

	default: // Unicast (SAFI 1, 2)
		if addPath {
			return addPathNLRISize
		}
		return basicNLRISize
	}
}

// =============================================================================
// NLRI Size Functions
// =============================================================================

// basicNLRISize calculates size of basic IPv4/IPv6 unicast NLRI.
// Format: [prefix-len-bits:1][prefix-bytes:ceil(len/8)].
// RFC 4271 Section 4.3 - NLRI encoding.
func basicNLRISize(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrNLRIMalformed
	}
	prefixLen := int(data[0])
	prefixBytes := (prefixLen + 7) / 8
	return 1 + prefixBytes, nil
}

// addPathNLRISize calculates size of Add-Path NLRI.
// Format: [path-id:4][prefix-len-bits:1][prefix-bytes].
// RFC 7911 Section 3 - ADD-PATH NLRI encoding.
func addPathNLRISize(data []byte) (int, error) {
	if len(data) < 5 {
		return 0, ErrNLRIMalformed
	}
	prefixLen := int(data[4])
	prefixBytes := (prefixLen + 7) / 8
	return 4 + 1 + prefixBytes, nil
}

// labeledNLRISize calculates size of labeled unicast NLRI (SAFI 4).
// Format: [total-bits:1][labels:3*N][prefix-bytes]
// The total-bits includes label bits + prefix bits.
// RFC 8277 Section 2 - Labeled unicast NLRI encoding.
func labeledNLRISize(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrNLRIMalformed
	}
	totalBits := int(data[0])
	totalBytes := (totalBits + 7) / 8
	return 1 + totalBytes, nil
}

// addPathLabeledNLRISize calculates size of Add-Path labeled unicast NLRI.
// Format: [path-id:4][total-bits:1][labels:3*N][prefix-bytes].
// RFC 7911 Section 3 - ADD-PATH encoding; RFC 8277 Section 2 - labeled unicast.
func addPathLabeledNLRISize(data []byte) (int, error) {
	if len(data) < 5 {
		return 0, ErrNLRIMalformed
	}
	totalBits := int(data[4])
	totalBytes := (totalBits + 7) / 8
	return 4 + 1 + totalBytes, nil
}

// vpnNLRISize calculates size of VPN NLRI (SAFI 128).
// Format: [total-bits:1][labels:3*N][RD:8][prefix-bytes]
// The total-bits includes labels + RD (64 bits) + prefix bits.
// RFC 4364 Section 4.3.4 - VPN-IPv4 NLRI encoding.
func vpnNLRISize(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrNLRIMalformed
	}
	totalBits := int(data[0])
	totalBytes := (totalBits + 7) / 8
	return 1 + totalBytes, nil
}

// addPathVPNNLRISize calculates size of Add-Path VPN NLRI.
// Format: [path-id:4][total-bits:1][labels:3*N][RD:8][prefix-bytes].
// RFC 7911 Section 3 - ADD-PATH encoding; RFC 4364 Section 4.3.4 - VPN NLRI.
func addPathVPNNLRISize(data []byte) (int, error) {
	if len(data) < 5 {
		return 0, ErrNLRIMalformed
	}
	totalBits := int(data[4])
	totalBytes := (totalBits + 7) / 8
	return 4 + 1 + totalBytes, nil
}

// evpnNLRISize calculates size of EVPN NLRI.
// Format: [route-type:1][length:1][payload:length].
// RFC 7432 Section 7 - EVPN NLRI encoding.
func evpnNLRISize(data []byte) (int, error) {
	if len(data) < 2 {
		return 0, ErrNLRIMalformed
	}
	// route-type is data[0], length is data[1]
	length := int(data[1])
	return 2 + length, nil
}

// addPathEVPNNLRISize calculates size of Add-Path EVPN NLRI.
// Format: [path-id:4][route-type:1][length:1][payload:length].
// RFC 7911 Section 3 - ADD-PATH encoding; RFC 7432 Section 7 - EVPN NLRI.
func addPathEVPNNLRISize(data []byte) (int, error) {
	if len(data) < 6 {
		return 0, ErrNLRIMalformed
	}
	// path-id is data[0:4], route-type is data[4], length is data[5]
	length := int(data[5])
	return 4 + 2 + length, nil
}

// flowSpecNLRISize calculates size of FlowSpec NLRI.
// Format: [length:1-2][components:length]
// Length < 240: 1 byte
// Length >= 240: 2 bytes (0xF0|high, low).
// RFC 5575 Section 4 - FlowSpec NLRI encoding (max 4095 bytes).
func flowSpecNLRISize(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, ErrNLRIMalformed
	}

	if data[0] < 0xF0 {
		// 1-byte length
		length := int(data[0])
		return 1 + length, nil
	}

	// 2-byte length
	if len(data) < 2 {
		return 0, ErrNLRIMalformed
	}
	length := (int(data[0]&0x0F) << 8) | int(data[1])
	return 2 + length, nil
}

// addPathFlowSpecNLRISize calculates size of Add-Path FlowSpec NLRI.
// Format: [path-id:4][length:1-2][components:length].
// RFC 7911 Section 3 - ADD-PATH encoding; RFC 5575 Section 4 - FlowSpec NLRI.
func addPathFlowSpecNLRISize(data []byte) (int, error) {
	if len(data) < 5 {
		return 0, ErrNLRIMalformed
	}

	// Skip path-id (4 bytes), then check length encoding
	if data[4] < 0xF0 {
		// 1-byte length
		length := int(data[4])
		return 4 + 1 + length, nil
	}

	// 2-byte length
	if len(data) < 6 {
		return 0, ErrNLRIMalformed
	}
	length := (int(data[4]&0x0F) << 8) | int(data[5])
	return 4 + 2 + length, nil
}

// bgpLSNLRISize calculates size of BGP-LS NLRI.
// Format: [nlri-type:2][total-length:2][payload:total-length].
// RFC 7752 Section 3.2 - BGP-LS NLRI encoding (2-byte length, can exceed 4096).
func bgpLSNLRISize(data []byte) (int, error) {
	if len(data) < 4 {
		return 0, ErrNLRIMalformed
	}
	// nlri-type is data[0:2], length is data[2:4]
	length := int(binary.BigEndian.Uint16(data[2:4]))
	return 4 + length, nil
}

// addPathBGPLSNLRISize calculates size of Add-Path BGP-LS NLRI.
// Format: [path-id:4][nlri-type:2][total-length:2][payload:total-length].
// RFC 7911 Section 3 - ADD-PATH encoding; RFC 7752 Section 3.2 - BGP-LS NLRI.
func addPathBGPLSNLRISize(data []byte) (int, error) {
	if len(data) < 8 {
		return 0, ErrNLRIMalformed
	}
	// path-id is data[0:4], nlri-type is data[4:6], length is data[6:8]
	length := int(binary.BigEndian.Uint16(data[6:8]))
	return 4 + 4 + length, nil
}
