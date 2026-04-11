// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking
// Related: filter_chain.go -- PolicyFilterChain returns text delta
// Related: filter_format.go -- attrNameToCode, FormatAttrsForFilter
// Related: forward_build.go -- buildModifiedPayload consumes ModAccumulator ops

package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

const policyAttrASPath = "as-path"

// extractLegacyNLRIOverride compares the nlri field in the original and
// modified filter text. When the modified text changes the `nlri
// ipv4/unicast add ...` block (the only NLRI block that maps to the legacy
// NLRI section in IPv4 BGP UPDATEs), it returns the wire-encoded prefix
// bytes corresponding to the new (modified) prefix list. Callers pass the
// returned slice to buildModifiedPayload as its nlriOverride argument so
// step 8 of the progressive build writes the filtered prefix list instead
// of copying the original NLRI section verbatim.
//
// Returns nil when:
//   - the nlri field is unchanged,
//   - the nlri block is a non-IPv4-unicast family (MP_REACH rewriting is
//     intentionally not supported in v1 -- filter plugins that need per-
//     NLRI decisions on non-CIDR families must declare raw=true and
//     rewrite the MP_REACH themselves),
//   - the op token is not "add" (withdrawals are handled elsewhere),
//   - a prefix token fails to parse (fail-closed: buildModifiedPayload will
//     fall through to the original copy path and the caller treats the
//     modify result as a no-op).
//
// The returned slice is a fresh allocation; buildModifiedPayload may write
// it into a pool buffer.
func extractLegacyNLRIOverride(original, modified string) []byte {
	if original == modified {
		return nil
	}

	origBlock := extractIPv4UnicastAddBlock(original)
	modBlock := extractIPv4UnicastAddBlock(modified)
	if modBlock == origBlock {
		return nil
	}
	// No ipv4/unicast add block in the modified text: this means the filter
	// dropped every prefix. Return an empty but non-nil slice so the caller
	// knows to replace (not skip) the NLRI section.
	if modBlock == "" {
		return []byte{}
	}

	tokens := strings.Fields(modBlock)
	// Expected shape: "ipv4/unicast add <prefix>...". Anything shorter has no
	// prefixes and is equivalent to the empty case above.
	if len(tokens) < 3 {
		return []byte{}
	}
	// Verify the head matches what we expect.
	if tokens[0] != "ipv4/unicast" || tokens[1] != "add" {
		return nil
	}

	// Upper-bound: every prefix needs 1 length byte + up to 4 address bytes.
	buf := make([]byte, 0, len(tokens[2:])*5)
	for _, tok := range tokens[2:] {
		p, err := netip.ParsePrefix(tok)
		if err != nil {
			return nil // fail-closed per the godoc contract
		}
		if !p.Addr().Is4() {
			return nil
		}
		bits := p.Bits()
		if bits < 0 || bits > 32 {
			return nil
		}
		buf = append(buf, byte(bits))
		if bits == 0 {
			continue
		}
		addr := p.Addr().As4()
		byteLen := (bits + 7) / 8
		buf = append(buf, addr[:byteLen]...)
	}
	return buf
}

// extractIPv4UnicastAddBlock pulls the space-delimited "ipv4/unicast add ..."
// block out of a filter text string. The nlri field can contain multiple
// blocks separated by spaces (e.g., MP_REACH for ipv6/unicast alongside
// legacy ipv4/unicast); this helper walks the tokens and returns only the
// ipv4/unicast add block when present. Returns "" if no such block exists.
func extractIPv4UnicastAddBlock(filterText string) string {
	nlriField := extractNLRIField(filterText)
	if nlriField == "" {
		return ""
	}
	return findNLRIBlock(nlriField, "ipv4/unicast", "add")
}

// extractNLRIField returns the content after "nlri " in filter text, or ""
// if the text has no nlri field. Mirrors the extractNLRIField helper in the
// filter_prefix plugin -- the two packages cannot import each other, so the
// lookup is duplicated locally.
func extractNLRIField(filterText string) string {
	_, after, ok := strings.Cut(filterText, "nlri ")
	if !ok {
		return ""
	}
	return after
}

// findNLRIBlock walks the nlri field text (the content after "nlri ") and
// returns the portion belonging to the given family and op, with the family
// and op tokens restored as the head. The NLRI field may contain multiple
// blocks concatenated like:
//
//	"ipv4/unicast add 10.0.0.0/24 nlri ipv6/unicast add 2001:db8::/32"
//
// where each new block is introduced by another "nlri" keyword. findNLRIBlock
// returns "" if no block with the requested family and op is present.
func findNLRIBlock(nlriField, family, op string) string {
	// Split on the "nlri" keyword boundary. parseFilterAttrs already knows
	// how to capture the nlri field as one glob; here we need to split the
	// glob into per-block substrings.
	// The first token pair is already the family/op; subsequent blocks are
	// introduced by "nlri family op".
	blocks := splitNLRIBlocks(nlriField)
	for _, blk := range blocks {
		tokens := strings.Fields(blk)
		if len(tokens) < 2 {
			continue
		}
		if tokens[0] == family && tokens[1] == op {
			return blk
		}
	}
	return ""
}

// splitNLRIBlocks splits a concatenated nlri field into its per-block
// substrings. The caller must have already stripped the leading "nlri "
// keyword; the input is `<family1> <op1> <prefixes1...> nlri <family2>
// <op2> <prefixes2...>` and the output is one string per block without the
// leading "nlri" keyword.
func splitNLRIBlocks(nlriField string) []string {
	var blocks []string
	remaining := nlriField
	for {
		// The next block, if any, starts after a " nlri " delimiter.
		idx := strings.Index(remaining, " nlri ")
		if idx < 0 {
			remaining = strings.TrimSpace(remaining)
			if remaining != "" {
				blocks = append(blocks, remaining)
			}
			return blocks
		}
		blk := strings.TrimSpace(remaining[:idx])
		if blk != "" {
			blocks = append(blocks, blk)
		}
		remaining = remaining[idx+len(" nlri "):]
	}
}

// textDeltaToModOps compares original and modified filter text, encoding changed
// attributes to wire VALUE bytes as AttrModSet operations on the ModAccumulator.
//
// Both original and modified use the policy filter text format:
//
//	"<attr-name> <value> [<attr-name> <value> ...] [nlri <family> <op> <prefix>...]"
//
// Skipped attributes (not converted to wire ops):
//   - NLRI: not modifiable via the attribute modification pipeline
//   - AS-PATH: modified at the wire layer by EBGP prepend (RFC 4271 Section 9.1.2);
//     a text-level AttrModSet would clobber the prepended local AS on export
//
// Attribute removal: when an attribute is present in original but absent in modified,
// a zero-length AttrModSet op is emitted. The handler writes a zero-length attribute
// (well-known) or omits it entirely (optional/community), effectively removing it.
//
// Parse errors for individual attributes are logged and skipped (fail-open).
func textDeltaToModOps(original, modified string, mods *registry.ModAccumulator) {
	origAttrs := parseFilterAttrs(original)
	modAttrs := parseFilterAttrs(modified)

	// Changed or added attributes.
	for name, modVal := range modAttrs {
		if name == policyAttrNLRI || name == policyAttrASPath {
			continue
		}
		origVal, existed := origAttrs[name]
		if existed && origVal == modVal {
			continue // Unchanged.
		}

		code, ok := attrNameToCode[name]
		if !ok {
			continue // Unknown attribute name; skip.
		}

		wireVal, err := encodeAttrValue(name, modVal)
		if err != nil {
			fwdLogger().Warn("policy filter delta: encode failed",
				"attr", name, "value", modVal, "error", err)
			continue // Skip this attribute; don't fail the entire delta.
		}

		mods.Op(byte(code), registry.AttrModSet, wireVal)
	}

	// Removed attributes: present in original, absent in modified.
	for name := range origAttrs {
		if name == policyAttrNLRI || name == policyAttrASPath {
			continue
		}
		if _, still := modAttrs[name]; still {
			continue // Still present.
		}
		code, ok := attrNameToCode[name]
		if !ok {
			continue
		}
		// Zero-length Set: handler omits optional attributes, writes empty well-known.
		mods.Op(byte(code), registry.AttrModSet, nil)
	}
}

// encodeAttrValue converts a text attribute value to wire VALUE bytes.
// The returned bytes contain only the attribute value (no header).
func encodeAttrValue(name, value string) ([]byte, error) {
	switch name {
	case "origin":
		return encodeOriginValue(value)
	case "as-path":
		return encodeASPathValue(value)
	case "next-hop":
		return encodeNextHopValue(value)
	case "med":
		return encodeUint32Value(value)
	case "local-preference":
		return encodeUint32Value(value)
	case "atomic-aggregate":
		return []byte{}, nil // Zero-length value.
	case "aggregator":
		return encodeAggregatorValue(value)
	case "community":
		return encodeCommunityValue(value)
	case "originator-id":
		return encodeIPv4Value(value)
	case "cluster-list":
		return encodeClusterListValue(value)
	case "extended-community":
		return encodeExtCommunityValue(value)
	case "large-community":
		return encodeLargeCommunityValue(value)
	}
	return nil, fmt.Errorf("unsupported attribute: %s", name)
}

// encodeOriginValue encodes "igp"/"egp"/"incomplete" to a 1-byte wire value.
func encodeOriginValue(s string) ([]byte, error) {
	switch strings.ToLower(s) {
	case "igp":
		return []byte{0}, nil
	case "egp":
		return []byte{1}, nil
	case "incomplete", "?":
		return []byte{2}, nil
	}
	return nil, fmt.Errorf("invalid origin: %s", s)
}

// encodeASPathValue encodes space-separated ASNs to wire AS_PATH value bytes.
// Wire format: one or more segments of type(1) + count(1) + ASNs(4 each).
func encodeASPathValue(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []byte{}, nil
	}

	tokens := strings.Fields(s)
	asns := make([]uint32, 0, len(tokens))
	for _, tok := range tokens {
		asn, err := strconv.ParseUint(tok, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN: %s", tok)
		}
		asns = append(asns, uint32(asn)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	}

	// Calculate total size: segments of up to 255 ASNs each.
	var totalSize int
	remaining := len(asns)
	for remaining > 0 {
		chunk := min(remaining, attribute.MaxASPathSegmentLength)
		totalSize += 2 + chunk*4 // type(1) + count(1) + ASNs
		remaining -= chunk
	}

	buf := make([]byte, totalSize)
	off := 0
	remaining = len(asns)
	idx := 0
	for remaining > 0 {
		chunk := min(remaining, attribute.MaxASPathSegmentLength)
		buf[off] = byte(attribute.ASSequence)
		buf[off+1] = byte(chunk)
		off += 2
		for i := range chunk {
			binary.BigEndian.PutUint32(buf[off:], asns[idx+i])
			off += 4
		}
		idx += chunk
		remaining -= chunk
	}

	return buf, nil
}

// encodeNextHopValue encodes an IPv4 address string to 4 wire bytes.
func encodeNextHopValue(s string) ([]byte, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil, fmt.Errorf("invalid next-hop: %s", s)
	}
	if !addr.Is4() {
		return nil, fmt.Errorf("next-hop must be IPv4: %s", s)
	}
	ip4 := addr.As4()
	return ip4[:], nil
}

// encodeUint32Value encodes a decimal integer to 4 wire bytes (big-endian).
func encodeUint32Value(s string) ([]byte, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid uint32: %s", s)
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(v)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	return buf, nil
}

// encodeAggregatorValue encodes "ASN:IP" to wire bytes (ASN(4) + IP(4) = 8 bytes).
func encodeAggregatorValue(s string) ([]byte, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid aggregator format: %s (expected ASN:IP)", s)
	}
	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid aggregator ASN: %s", parts[0])
	}
	addr, err := netip.ParseAddr(parts[1])
	if err != nil || !addr.Is4() {
		return nil, fmt.Errorf("invalid aggregator IP: %s", parts[1])
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(asn)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	ip4 := addr.As4()
	copy(buf[4:8], ip4[:])
	return buf, nil
}

// encodeCommunityValue encodes space-separated community strings to wire value bytes.
// Each community is 4 bytes (big-endian uint32).
func encodeCommunityValue(s string) ([]byte, error) {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, len(tokens)*4)
	for i, tok := range tokens {
		comm, err := attribute.ParseCommunity(tok)
		if err != nil {
			return nil, err
		}
		binary.BigEndian.PutUint32(buf[i*4:], comm)
	}
	return buf, nil
}

// encodeLargeCommunityValue encodes space-separated large community strings.
// Each large community is 12 bytes (3x uint32).
func encodeLargeCommunityValue(s string) ([]byte, error) {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, len(tokens)*12)
	for i, tok := range tokens {
		lc, err := attribute.ParseLargeCommunity(tok)
		if err != nil {
			return nil, err
		}
		off := i * 12
		binary.BigEndian.PutUint32(buf[off:], lc.GlobalAdmin)
		binary.BigEndian.PutUint32(buf[off+4:], lc.LocalData1)
		binary.BigEndian.PutUint32(buf[off+8:], lc.LocalData2)
	}
	return buf, nil
}

// encodeExtCommunityValue encodes space-separated extended community strings.
// Each extended community is 8 bytes. Uses Builder because there is no public
// single-value parser for extended communities.
func encodeExtCommunityValue(s string) ([]byte, error) {
	b := attribute.NewBuilder()
	if err := b.ParseExtCommunity(s); err != nil {
		return nil, err
	}
	wire := b.Build()
	if len(wire) == 0 {
		return []byte{}, nil
	}
	return stripAttrHeader(wire), nil
}

// encodeIPv4Value encodes a dotted-decimal IPv4 string to 4 wire bytes.
// Used for ORIGINATOR_ID.
func encodeIPv4Value(s string) ([]byte, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil, fmt.Errorf("invalid IPv4: %s", s)
	}
	if !addr.Is4() {
		return nil, fmt.Errorf("expected IPv4: %s", s)
	}
	ip4 := addr.As4()
	return ip4[:], nil
}

// encodeClusterListValue encodes space-separated dotted-decimal IDs to wire bytes.
// Each cluster ID is 4 bytes.
func encodeClusterListValue(s string) ([]byte, error) {
	tokens := strings.Fields(s)
	buf := make([]byte, len(tokens)*4)
	for i, tok := range tokens {
		addr, err := netip.ParseAddr(tok)
		if err != nil || !addr.Is4() {
			return nil, fmt.Errorf("invalid cluster-id: %s", tok)
		}
		ip4 := addr.As4()
		copy(buf[i*4:], ip4[:])
	}
	return buf, nil
}

// stripAttrHeader removes the attribute header (flags + code + length) from wire bytes,
// returning only the value portion. Handles both regular (3-byte) and extended (4-byte) headers.
func stripAttrHeader(wire []byte) []byte {
	if len(wire) < 3 {
		return wire
	}
	flags := wire[0]
	if flags&0x10 != 0 { // Extended length.
		if len(wire) < 4 {
			return wire
		}
		return wire[4:]
	}
	return wire[3:]
}
