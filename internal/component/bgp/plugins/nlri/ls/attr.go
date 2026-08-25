// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS attribute TLV framework
// RFC: rfc/short/rfc7752.md — BGP-LS attribute type 29 TLV format
// Overview: types.go — core types, TLV constants, and helper functions
// Detail: attr_node.go — node attribute TLV types
// Detail: attr_link.go — link attribute TLV types
// Detail: attr_prefix.go — prefix attribute TLV types
// Detail: attr_srv6.go — SRv6 attribute TLV types
// Detail: register_attr.go — attribute TLV registration
package ls

import (
	"encoding/binary"
	"errors"
)

// lsAttrTLV is the interface for BGP-LS attribute TLV types.
// RFC 7752 Section 3.3 defines the attribute TLV format:
//
//	+------------------+
//	| Type (2 bytes)   |
//	+------------------+
//	| Length (2 bytes)  |
//	+------------------+
//	| Value (variable) |
//	+------------------+
//
// Each TLV type self-registers via init() using RegisterLsAttrTLV.
type lsAttrTLV interface {
	// Code returns the TLV type code (e.g., 1024 for Node Flag Bits).
	Code() uint16
	// Len returns the total wire length including TLV header (4 + value length).
	Len() int
	// WriteTo writes the complete TLV (header + value) to buf at offset.
	// Returns bytes written.
	WriteTo(buf []byte, off int) int
	// ToJSON returns the TLV as a JSON-friendly map.
	ToJSON() map[string]any
}

// lsAttrTLVDecoder decodes a TLV value (without header) into a typed struct.
// The data slice contains only the value bytes; the TLV type and length have
// already been parsed by the iterator.
type lsAttrTLVDecoder func(data []byte) (lsAttrTLV, error)

// lsAttrTLVRegistry maps TLV type codes to their decoders.
// Populated by init() functions in attr_node.go, attr_link.go, attr_prefix.go, attr_srv6.go.
var lsAttrTLVRegistry = map[uint16]lsAttrTLVDecoder{}

// registerLsAttrTLV registers a decoder for a BGP-LS attribute TLV type code.
// Called from init() in each attribute TLV file.
func registerLsAttrTLV(code uint16, decoder lsAttrTLVDecoder) {
	lsAttrTLVRegistry[code] = decoder
}

// lookupLsAttrTLVDecoder returns the registered decoder for a TLV type code,
// or nil if no decoder is registered.
func lookupLsAttrTLVDecoder(code uint16) lsAttrTLVDecoder {
	return lsAttrTLVRegistry[code]
}

// registeredLsAttrTLVCount returns the number of registered attribute TLV decoders.
func registeredLsAttrTLVCount() int {
	return len(lsAttrTLVRegistry)
}

// attrTLVEntry represents a single TLV entry yielded by the iterator.
// It holds the raw type code and value bytes without copying.
type attrTLVEntry struct {
	Type  uint16 // TLV type code
	Value []byte // TLV value (slice into original data, no copy)
}

// iterateAttrTLVs iterates over BGP-LS attribute TLVs in raw wire bytes.
// RFC 7752 Section 3.3: attribute type 29 contains a sequence of TLVs.
// The callback receives each TLV entry. Return false to stop iteration.
// Returns error only on truncated data (TLV header present but value truncated).
func iterateAttrTLVs(data []byte, fn func(attrTLVEntry) bool) error {
	offset := 0
	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			return ErrBGPLSTruncated
		}

		entry := attrTLVEntry{
			Type:  tlvType,
			Value: data[offset+4 : offset+4+tlvLen],
		}

		if !fn(entry) {
			return nil
		}

		offset += 4 + tlvLen
	}

	return nil
}

// ErrUnknownAttrTLV is returned when no decoder is registered for a TLV type code.
var ErrUnknownAttrTLV = errors.New("bgp-ls: unknown attribute TLV type")

// decodeAttrTLV decodes a single TLV entry using the registered decoder.
// Returns ErrUnknownAttrTLV if no decoder is registered for the TLV type code.
func decodeAttrTLV(entry attrTLVEntry) (lsAttrTLV, error) {
	decoder := lsAttrTLVRegistry[entry.Type]
	if decoder == nil {
		return nil, ErrUnknownAttrTLV
	}
	return decoder(entry.Value)
}

// decodeAllAttrTLVs decodes all recognized TLVs from raw attribute bytes.
// Unknown TLV types are silently skipped per RFC 7752 forward compatibility.
//
// It is UNREACHABLE from the session path by design, and its callers are tests.
// Decoding a TLV means judging its fixed length and its values, which RFC 9552
// Section 8.2.2 lists among the semantic validations a BGP-LS Propagator does not
// perform: "the length of a fixed-length TLV is correct or the length of a variable
// length TLV is valid or permissible" and "the values of TLV fields are valid or
// permissible". A received BGP-LS Attribute therefore reaches only the syntactic walk
// (validateBGPLSAttr, internal/component/bgp/message/rfc7606_bgpls.go) and is relayed
// with its TLVs untouched. The absence of a caller is the design; register.go marks the
// point that call would have been wired in.
//
// AttrTLVsToJSON is the sibling that IS called, from the offline decoder alone
// (internal/component/bgp/cli/decode_update.go), where an operator has asked for the
// decoded view of bytes ze is not propagating.
//
// RFC requirement: RFC9552-8.2.2-3 positive -- the semantic decode a Propagator does not
// perform, named at its definition so the absence of a session-path caller reads as the
// design it is rather than as dead code.
func decodeAllAttrTLVs(data []byte) ([]lsAttrTLV, error) {
	var tlvs []lsAttrTLV
	var decErr error

	err := iterateAttrTLVs(data, func(entry attrTLVEntry) bool {
		tlv, e := decodeAttrTLV(entry)
		// RFC 7752 Section 3.3: unrecognized TLVs are forwarded without decoding
		if errors.Is(e, ErrUnknownAttrTLV) {
			return true
		}
		if e != nil {
			decErr = e
			return false
		}
		tlvs = append(tlvs, tlv)
		return true
	})
	if err != nil {
		return tlvs, err
	}

	return tlvs, decErr
}

// AttrTLVsToJSON converts attribute bytes to a JSON-friendly map.
// Each decoded TLV contributes its key-value pairs to the result.
// Unknown TLVs are stored as "generic-lsid-<code>" with hex value.
func AttrTLVsToJSON(data []byte) map[string]any {
	result := make(map[string]any)

	_ = iterateAttrTLVs(data, func(entry attrTLVEntry) bool {
		tlv, err := decodeAttrTLV(entry)
		// RFC 7752 Section 3.3: unrecognized TLVs stored as generic hex
		if errors.Is(err, ErrUnknownAttrTLV) {
			result[genericTLVKey(entry.Type)] = []string{formatHex(entry.Value)}
			return true
		}
		if err != nil {
			return true // malformed TLV, continue with next
		}
		for k, v := range tlv.ToJSON() {
			mergeJSONKey(result, k, v)
		}
		return true
	})

	return result
}

// mergeJSONKey merges a key-value pair into the result map.
// If the key already exists and both values are slices, they are concatenated.
// This handles TLV types that can appear multiple times (e.g., router IDs).
func mergeJSONKey(result map[string]any, key string, value any) {
	existing, ok := result[key]
	if !ok {
		result[key] = value
		return
	}

	// If both are slices, merge them
	if existSlice, ok := existing.([]string); ok {
		if newSlice, ok := value.([]string); ok {
			result[key] = append(existSlice, newSlice...)
			return
		}
	}
	if existSlice, ok := existing.([]map[string]any); ok {
		if newSlice, ok := value.([]map[string]any); ok {
			result[key] = append(existSlice, newSlice...)
			return
		}
	}

	// Otherwise, overwrite
	result[key] = value
}

// genericTLVKey returns the JSON key for an unknown TLV type.
func genericTLVKey(code uint16) string {
	return "generic-lsid-" + uitoa(code)
}

// formatHex returns a hex-encoded string of raw bytes with 0x prefix.
func formatHex(data []byte) string {
	const hextable = "0123456789ABCDEF"
	buf := make([]byte, len(data)*2)
	for i, b := range data {
		buf[i*2] = hextable[b>>4]
		buf[i*2+1] = hextable[b&0x0f]
	}
	return "0x" + string(buf)
}

// uitoa converts a uint16 to its string representation.
func uitoa(v uint16) string {
	if v == 0 {
		return "0"
	}
	var buf [5]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
