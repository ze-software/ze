// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking
// Related: filter_chain.go -- PolicyFilterChain returns text delta; parseFilterAttrs parses it
// Related: filter_format.go -- attrNameToCode, FormatAttrsForFilter
// Related: forward_build.go -- buildModifiedPayload consumes ModAccumulator ops
// RFC: rfc/short/rfc6996.md -- Private Use ASN ranges (remove-private rewriting)
// RFC: rfc/short/rfc6793.md -- AS4_PATH handling (four-octet AS number space)

package reactor

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

const (
	policyAttrASPath        = "as-path"
	policyAttrASPathPrepend = "as-path-prepend"
	policyAttrRemovePrivate = "remove-private"
	policyAttrMEDRemove     = "med-remove"
	removePrivateASStrip    = "strip"
	removePrivateASPeerAS   = "peer-as"
)

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

// textDeltaToModOps compares the parsed original and modified filter
// attribute maps, encoding changed attributes to wire VALUE bytes as
// AttrModSet operations on the ModAccumulator.
//
// Both maps come from parseFilterAttrs; the call site parses each filter
// text exactly once and shares the maps read-only across the three
// extractors (textDeltaToModOps, ExtractRemovePrivateASOps,
// ExtractASPathPrependOps).
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
func textDeltaToModOps(origAttrs, modAttrs *filterAttrs, mods *filterapi.ModAccumulator) {
	for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // prealloc linter crashes on range-over-int
		name := filterAttrNames[id]

		// Directives with their own extractor, and the two attributes the wire
		// layer owns. med-remove is here rather than in the loop because it is
		// honored on the INGRESS chain alone: see ExtractMEDRemoveOps.
		if id == faNLRI || id == faASPath || id == faASPathPrepend ||
			id == faRemovePrivate || id == faMEDRemove {
			continue
		}

		modVal, modPresent := modAttrs.get(id)
		origVal, origPresent := origAttrs.get(id)

		// Community add/remove directives.
		if directive, ok := communityDirectives[name]; ok && modPresent {
			wireVal, err := encodeAttrValue(directive.encoderName, modVal)
			if err != nil {
				fwdLogger().Warn("policy filter delta: community directive encode failed",
					"directive", name, "value", modVal, "error", err)
				continue
			}
			if directive.action == filterapi.AttrModRemove && directive.valueSize > 0 {
				for off := 0; off+directive.valueSize <= len(wireVal); off += directive.valueSize {
					mods.Op(byte(directive.code), directive.action, wireVal[off:off+directive.valueSize])
				}
			} else {
				mods.Op(byte(directive.code), directive.action, wireVal)
			}
			continue
		}

		// Changed or added attributes.
		if modPresent {
			if origPresent && origVal == modVal {
				continue
			}
			code, ok := attrNameToCode[name]
			if !ok {
				continue
			}
			wireVal, err := encodeAttrValue(name, modVal)
			if err != nil {
				fwdLogger().Warn("policy filter delta: encode failed",
					"attr", name, "value", modVal, "error", err)
				continue
			}
			mods.Op(byte(code), filterapi.AttrModSet, wireVal)
			continue
		}

		// Removed attributes: present in original, absent in modified.
		if origPresent {
			code, ok := attrNameToCode[name]
			if !ok {
				continue
			}
			mods.Op(byte(code), filterapi.AttrModSet, nil)
		}
	}
}

// ExtractMEDRemoveOps converts the `med-remove` directive an import filter
// wrote into its delta text, and it is the mechanism RFC 4271 Section 5.1.4
// requires: "A BGP speaker MUST implement a mechanism (based on local
// configuration) that allows the MULTI_EXIT_DISC attribute to be removed from a
// route."
//
// ONE SUPPRESSION, TWO REASONS TO TRIGGER IT. The op is the same
// filterapi.AttrModSuppress on attribute 4 that applyFactsMED
// (forward_med.go) records for the propagation rule, applied by the same
// handler (genericAttrSetHandler, filter_delta_handlers.go), so the configured
// mechanism and the propagation rule cannot disagree about what a suppression
// of MULTI_EXIT_DISC does to the wire.
//
// INGRESS ONLY, AND THAT IS A CONFORMANCE BOUNDARY RATHER THAN A LIMITATION.
// Section 5.1.4 continues: "If a BGP speaker is configured to remove the
// MULTI_EXIT_DISC attribute from a route, then this removal MUST be done prior
// to determining the degree of preference of the route and prior to performing
// route selection (Decision Process phases 1 and 2)." The import chain's
// rewritten payload replaces the WireUpdate before the UPDATE is dispatched to
// the RIB plugin that runs those phases (filter_ordered.go), so removing here
// satisfies that ordering for every route.
//
// An EXPORT-side removal would break a different requirement. RFC 4271 Section
// 9.1.2.2: "If an implementation chooses to remove MULTI_EXIT_DISC, then the
// optional comparison on MULTI_EXIT_DISC, if performed, MUST be performed only
// among EBGP-learned routes ... Including the MULTI_EXIT_DISC of an EBGP-learned
// route in the comparison with an IBGP-learned route, then removing the
// MULTI_EXIT_DISC attribute, and advertising the route has been proven to cause
// route loops." A removal on the way out to an internal peer is exactly that
// shape, because the decision process has already compared the value. Removing
// on the way in cannot be: there is no value left to compare.
//
// So this extractor is called from the import site alone (filter_ordered.go),
// and the export site never converts the directive. bgp-filter-modify refuses
// to emit it on an export chain and says so (filter_modify/filter_modify.go),
// which is the operator-facing half; this omission is the half that binds.
//
// A ROUTE WITH NOTHING TO REMOVE IS LEFT ALONE, and both callers ask
// medRemoveHasWork before calling this. Recording the suppression anyway would
// send every such route through buildModifiedPayload for a byte that is not
// there, which is the cost applyFactsMED (forward_med.go) refuses in the same
// words.
func ExtractMEDRemoveOps(modAttrs *filterAttrs, mods *filterapi.ModAccumulator) {
	if !modAttrs.has(faMEDRemove) {
		return
	}
	mods.Op(byte(attribute.AttrMED), filterapi.AttrModSuppress, nil)
}

// medRemoveHasWork reports whether a med-remove directive has a metric to take
// off the route. There are two ways one can be there, and both count.
//
// THE ROUTE ARRIVED WITH IT, which only the WIRE can answer. appendSingleAttr
// (filter_format.go) switches on *attribute.MED while knownAttrParsers builds
// the value form attribute.MED (core/bgp/attribute/wire.go, simple.go), so the
// case never matches and `med` never reaches the text a filter is given.
// Reading modAttrs alone makes the removal silently do nothing, measured
// against GoBGP on 2026-08-15
// (test/interop/scenarios/bgp-med-remove-configured-gobgp).
//
// A FILTER EARLIER IN THE SAME CHAIN SET IT, which only the TEXT can answer.
// `filter import [ modify:SET-MED modify:DROP-MED ]` is legal config --
// validateNoConflict (filter_modify/config.go) refuses the pair inside ONE
// definition, not across a chain -- and textDeltaToModOps records that Set, so
// without this arm the operator's second filter is ignored on a route that
// arrived with no metric.
//
// A nil or unreadable attribute section answers TRUE: the operator asked for a
// removal, and a rebuild that changes no byte is a cheaper mistake than a
// configured removal that silently does not happen.
func medRemoveHasWork(modAttrs *filterAttrs, attrs *attribute.AttributesWire) bool {
	if modAttrs.has(faMED) {
		return true
	}
	if attrs == nil {
		return true
	}
	present, err := attrs.Has(attribute.AttrMED)
	return err != nil || present
}

type communityDirective struct {
	code        attribute.AttributeCode
	action      uint8
	encoderName string // key into encodeAttrValue
	valueSize   int    // per-value wire size (4=standard, 12=large, 8=extended)
}

var communityDirectives = map[string]communityDirective{
	"community-add":             {attribute.AttrCommunity, filterapi.AttrModAdd, "community", 4},
	"community-remove":          {attribute.AttrCommunity, filterapi.AttrModRemove, "community", 4},
	"large-community-add":       {attribute.AttrLargeCommunity, filterapi.AttrModAdd, "large-community", 12},
	"large-community-remove":    {attribute.AttrLargeCommunity, filterapi.AttrModRemove, "large-community", 12},
	"extended-community-add":    {attribute.AttrExtCommunity, filterapi.AttrModAdd, "extended-community", 8},
	"extended-community-remove": {attribute.AttrExtCommunity, filterapi.AttrModRemove, "extended-community", 8},
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
	case "aigp":
		return encodeAIGPValue(value)
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

// encodeAIGPValue encodes a decimal metric string to an 11-byte AIGP TLV value.
// RFC 7311: type(1) + length(2) + metric(8).
func encodeAIGPValue(s string) ([]byte, error) {
	metric, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid aigp metric: %s", s)
	}
	buf := make([]byte, attribute.AIGPWireLen)
	attribute.WriteAIGPMetric(buf, 0, metric)
	return buf, nil
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

// ExtractASPathPrependOps checks the parsed modified filter attributes for
// an "as-path-prepend N" directive and emits an AttrModPrepend op with N
// copies of localAS as wire bytes. Called separately from textDeltaToModOps
// because the local AS is only known at the call site (reactor_notify.go
// for import, reactor_api_forward.go for export). The map comes from the
// call site's single parseFilterAttrs(modified) and is read, never mutated.
//
// Does nothing if the modified attributes do not contain as-path-prepend.
func ExtractASPathPrependOps(modAttrs *filterAttrs, localAS uint32, mods *filterapi.ModAccumulator) {
	countStr, ok := modAttrs.get(faASPathPrepend)
	if !ok || countStr == "" {
		return
	}
	count, err := strconv.ParseUint(countStr, 10, 8)
	if err != nil || count == 0 || count > 32 {
		fwdLogger().Warn("as-path-prepend: invalid count", "value", countStr)
		return
	}

	// Build wire value: AS_SEQUENCE segment with N copies of localAS.
	// Format: type(1) + count(1) + ASNs(4 each).
	n := int(count)
	wireLen := 2 + n*4
	buf := make([]byte, wireLen)
	buf[0] = byte(attribute.ASSequence)
	buf[1] = byte(n)
	for i := range n {
		binary.BigEndian.PutUint32(buf[2+i*4:], localAS)
	}
	mods.Op(byte(attribute.AttrASPath), filterapi.AttrModPrepend, buf)
}

// ExtractRemovePrivateASOps checks the parsed modified filter attributes
// for a "remove-private" directive and emits AS_PATH / AS4_PATH Set or
// Suppress ops after rewriting raw path segments. The plugin supplies the
// policy intent; the reactor owns wire-safe segment preservation. The map
// comes from the call site's single parseFilterAttrs(modified) and is
// read, never mutated.
//
// RFC 6996 Section 4 requires private-use ASNs to be removed from AS path
// attributes (including AS4_PATH if utilizing a four-octet AS number space)
// before being advertised to the global Internet.
func ExtractRemovePrivateASOps(modAttrs *filterAttrs, attrs *attribute.AttributesWire, asn4 bool, peerAS uint32, mods *filterapi.ModAccumulator) {
	mode, ok := extractRemovePrivateASMode(modAttrs)
	if !ok || attrs == nil {
		return
	}

	rawASPath, err := attrs.GetRaw(attribute.AttrASPath)
	if err == nil && len(rawASPath) > 0 {
		if rewritten, changed := rewriteASPathRemovePrivate(rawASPath, asn4, mode, peerAS); changed {
			mods.Op(byte(attribute.AttrASPath), filterapi.AttrModSet, rewritten)
		}
	} else if err != nil {
		fwdLogger().Warn("remove-private-as: AS_PATH raw lookup failed", "error", err)
	}

	rawAS4Path, err := attrs.GetRaw(attribute.AttrAS4Path)
	if err == nil && len(rawAS4Path) > 0 {
		if rewritten, changed := rewriteAS4PathRemovePrivate(rawAS4Path, mode, peerAS); changed {
			if len(rewritten) == 0 {
				mods.Op(byte(attribute.AttrAS4Path), filterapi.AttrModSuppress, nil)
			} else {
				mods.Op(byte(attribute.AttrAS4Path), filterapi.AttrModSet, rewritten)
			}
		}
	} else if err != nil {
		fwdLogger().Warn("remove-private-as: AS4_PATH raw lookup failed", "error", err)
	}
}

func extractRemovePrivateASMode(modAttrs *filterAttrs) (string, bool) {
	mode, ok := modAttrs.get(faRemovePrivate)
	if !ok {
		return "", false
	}
	switch mode {
	case removePrivateASStrip, removePrivateASPeerAS:
		return mode, true
	default:
		fwdLogger().Warn("remove-private-as: invalid mode", "mode", mode)
		return "", false
	}
}

func rewriteASPathRemovePrivate(value []byte, asn4 bool, mode string, peerAS uint32) ([]byte, bool) {
	path, err := attribute.ParseASPath(value, asn4)
	if err != nil {
		fwdLogger().Warn("remove-private-as: parse AS_PATH failed", "error", err)
		return nil, false
	}
	segments, changed := rewritePrivateASSegments(path.Segments, mode, peerAS)
	if !changed {
		return nil, false
	}
	rewritten := &attribute.ASPath{Segments: segments}
	buf := make([]byte, rewritten.LenWithASN4(asn4))
	rewritten.WriteToWithASN4(buf, 0, asn4)
	return buf, true
}

func rewriteAS4PathRemovePrivate(value []byte, mode string, peerAS uint32) ([]byte, bool) {
	path, err := attribute.ParseAS4Path(value)
	if err != nil {
		fwdLogger().Warn("remove-private-as: parse AS4_PATH failed", "error", err)
		return nil, false
	}
	segments, changed := rewritePrivateASSegments(path.Segments, mode, peerAS)
	if !changed {
		return nil, false
	}
	rewritten := &attribute.AS4Path{Segments: segments}
	buf := make([]byte, rewritten.Len())
	rewritten.WriteTo(buf, 0)
	return buf, true
}

func rewritePrivateASSegments(segments []attribute.ASPathSegment, mode string, peerAS uint32) ([]attribute.ASPathSegment, bool) {
	out := make([]attribute.ASPathSegment, 0, len(segments))
	changed := false
	for _, seg := range segments {
		asns := make([]uint32, 0, len(seg.ASNs))
		for _, asn := range seg.ASNs {
			if !isRFC6996PrivateASN(asn) {
				asns = append(asns, asn)
				continue
			}
			changed = true
			if mode == removePrivateASPeerAS {
				asns = append(asns, peerAS)
			}
		}
		if len(asns) == 0 {
			continue
		}
		out = append(out, attribute.ASPathSegment{Type: seg.Type, ASNs: asns})
	}
	if !changed {
		return nil, false
	}
	return out, true
}

// RFC 6996 Section 5: Private Use ASNs are 64512-65534 and
// 4200000000-4294967294, inclusive.
func isRFC6996PrivateASN(asn uint32) bool {
	return (asn >= 64512 && asn <= 65534) || (asn >= 4200000000 && asn <= 4294967294)
}
