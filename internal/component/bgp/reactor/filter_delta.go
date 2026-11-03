// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking
// Related: filter_chain.go -- PolicyFilterChain returns text delta; parseFilterAttrs parses it
// Related: filter_format.go -- attrNameToCode, FormatAttrsForFilter
// Related: forward_build.go -- buildModifiedPayload consumes ModAccumulator ops
// Related: aspath_edit.go -- prependASPathOps, removePrivateASOps: the wire half both paths reach
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
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// remove-private values. The attribute names themselves are in filter_chain.go.
const (
	removePrivateASStrip  = "strip"
	removePrivateASPeerAS = "peer-as"
)

// textDeltaToModOps compares the parsed original and modified filter
// attributes, encoding changed attributes to wire VALUE bytes as
// AttrModSet operations on the ModAccumulator. The bytes are carved from the
// caller's scratch (filter_scratch.go).
//
// Both structs come from parseFilterAttrs; the call site parses each filter
// text exactly once and shares them read-only across the three extractors
// (textDeltaToModOps, ExtractRemovePrivateASOps, ExtractASPathPrependOps).
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
func textDeltaToModOps(scratch *valueScratch, origAttrs, modAttrs *filterAttrs, mods *filterapi.ModAccumulator) {
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
			wireVal, err := encodeAttrValue(scratch, directive.encoderName, modVal)
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
			wireVal, err := encodeAttrValue(scratch, name, modVal)
			if err != nil {
				fwdLogger().Warn("policy filter delta: encode failed",
					"attr", name, "value", modVal, "error", err)
				continue
			}
			mods.Op(byte(code), filterapi.AttrModSet, wireVal)
			continue
		}

		// Removed attributes: present in original, absent in modified.
		//
		// AttrModSuppress, never a Set with no bytes. An empty Set is a legal
		// thing to ask for here: ATOMIC_AGGREGATE is "a well-known discretionary
		// attribute of length 0" (RFC 4271 Section 4.3f), and it is one of the
		// codes genericAttrSetHandler serves. So the handler cannot read an empty
		// value as a removal, and it does not: it calls p.Op then p.Emit, and
		// AttrPlan.emit decides the header class from the final length with no
		// zero case, which puts a three-byte header of length 0 on the wire.
		// For MULTI_EXIT_DISC that is an attribute RFC 4271 Section 5.1.4 gives
		// four octets, and a conformant receiver answers with RFC 7606.
		if origPresent {
			code, ok := attrNameToCode[name]
			if !ok {
				continue
			}
			mods.Op(byte(code), filterapi.AttrModSuppress, nil)
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
// off the route. There are two ways one can be there, and the SUBJECT answers
// for both.
//
// RFC 4271 Section 5.1.4: "A BGP speaker MUST implement a mechanism (based on
// local configuration) that allows the MULTI_EXIT_DISC attribute to be removed
// from a route."
//
// THE ROUTE ARRIVED WITH IT. appendSingleAttr (filter_format.go) renders `med`
// into the subject whenever the wire carries MULTI_EXIT_DISC, so the metric the
// peer sent is in the text this predicate reads.
//
// A FILTER EARLIER IN THE SAME CHAIN SET IT. `filter { import { filters [
// modify:SET-MED modify:DROP-MED ]; final accept; } }` is legal -- validateNoConflict
// (filter_modify/config.go) refuses the pair inside ONE definition, not across a
// chain -- and applyFilterDelta (filter_chain.go) merges each filter's delta
// INTO the current subject, so a Set by an earlier filter is in the same text.
//
// One reading serves both because the subject is a SUPERSET of what the route
// arrived with: policyFilterFunc seeds `current` with the full rendered subject
// and only ever rewrites it through applyFilterDelta. Reading the wire as a
// second source would be two declarations of one fact with nothing to arbitrate
// them (ai/rules/principles.md).
//
// This rests on every caller of AppendUpdateForFilter passing a nil declared
// list, so the subject is never narrowed to a subset that could drop `med`.
// TestFilterSubjectReadsUnmodifiableAttributes (filter_format_attrs_test.go)
// proves that MED decides live and dry-run policy even when only LOCAL_PREF
// is declared modifiable.
func medRemoveHasWork(modAttrs *filterAttrs) bool {
	return modAttrs.has(faMED)
}

type communityDirective struct {
	code        attribute.AttributeCode
	action      uint8
	encoderName string // key into encodeAttrValue
	valueSize   int    // per-value wire size (4=standard, 12=large, 8=extended)
}

var communityDirectives = map[string]communityDirective{
	policyAttrCommunityAdd:            {attribute.AttrCommunity, filterapi.AttrModAdd, policyAttrCommunity, 4},
	policyAttrCommunityRemove:         {attribute.AttrCommunity, filterapi.AttrModRemove, policyAttrCommunity, 4},
	policyAttrLargeCommunityAdd:       {attribute.AttrLargeCommunity, filterapi.AttrModAdd, policyAttrLargeCommunity, 12},
	policyAttrLargeCommunityRemove:    {attribute.AttrLargeCommunity, filterapi.AttrModRemove, policyAttrLargeCommunity, 12},
	policyAttrExtendedCommunityAdd:    {attribute.AttrExtCommunity, filterapi.AttrModAdd, policyAttrExtendedCommunity, 8},
	policyAttrExtendedCommunityRemove: {attribute.AttrExtCommunity, filterapi.AttrModRemove, policyAttrExtendedCommunity, 8},
}

// encodeAttrValue converts a text attribute value to wire VALUE bytes.
// The returned bytes contain only the attribute value (no header) and are
// carved from the caller's scratch, which owns them until the modify block
// releases it (filter_scratch.go).
func encodeAttrValue(scratch *valueScratch, name, value string) ([]byte, error) {
	switch name {
	case policyAttrOrigin:
		return encodeOriginValue(scratch, value)
	case policyAttrASPath:
		return encodeASPathValue(scratch, value)
	case policyAttrNextHop:
		return encodeNextHopValue(scratch, value)
	case policyAttrMED:
		return encodeUint32Value(scratch, value)
	case policyAttrLocalPreference:
		return encodeUint32Value(scratch, value)
	case policyAttrAtomicAggregate:
		return []byte{}, nil // Zero-length value.
	case policyAttrAggregator:
		return encodeAggregatorValue(scratch, value)
	case policyAttrCommunity:
		return encodeCommunityValue(scratch, value)
	case policyAttrOriginatorID:
		return encodeIPv4Value(scratch, value)
	case policyAttrClusterList:
		return encodeClusterListValue(scratch, value)
	case policyAttrExtendedCommunity:
		return encodeExtCommunityValue(scratch, value)
	case policyAttrLargeCommunity:
		return encodeLargeCommunityValue(scratch, value)
	case policyAttrAIGP:
		return encodeAIGPValue(scratch, value)
	}
	return nil, fmt.Errorf("unsupported attribute: %s", name)
}

// encodeOriginValue encodes "igp"/"egp"/"incomplete" to a 1-byte wire value.
// The names and their wire values come from the attribute package, which holds
// the one spelling of them. "?" is ExaBGP's word for incomplete and is not one
// of the RFC 4271 names, so it is answered before the table is asked.
func encodeOriginValue(scratch *valueScratch, s string) ([]byte, error) {
	lower := strings.ToLower(s)
	origin := attribute.OriginIncomplete
	if lower != "?" {
		named, ok := attribute.OriginFromText(lower)
		if !ok {
			return nil, fmt.Errorf("invalid origin: %s", s)
		}
		origin = named
	}

	buf := scratch.carveBytes(1)
	buf[0] = byte(origin)
	return buf, nil
}

// encodeASPathValue encodes space-separated ASNs to wire AS_PATH value bytes.
// Wire format: one or more segments of type(1) + count(1) + ASNs(4 each).
func encodeASPathValue(scratch *valueScratch, s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []byte{}, nil
	}

	tokens := strings.Fields(s)
	asns := scratch.carveASNs(len(tokens))
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

	buf := scratch.carveBytes(totalSize)
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
func encodeNextHopValue(scratch *valueScratch, s string) ([]byte, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil, fmt.Errorf("invalid next-hop: %s", s)
	}
	if !addr.Is4() {
		return nil, fmt.Errorf("next-hop must be IPv4: %s", s)
	}

	ip4 := addr.As4()
	buf := scratch.carveBytes(len(ip4))
	copy(buf, ip4[:])
	return buf, nil
}

// encodeUint32Value encodes a decimal integer to 4 wire bytes (big-endian).
func encodeUint32Value(scratch *valueScratch, s string) ([]byte, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid uint32: %s", s)
	}
	buf := scratch.carveBytes(4)
	binary.BigEndian.PutUint32(buf, uint32(v)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	return buf, nil
}

// encodeAggregatorValue encodes "ASN:IP" to wire bytes (ASN(4) + IP(4) = 8 bytes).
func encodeAggregatorValue(scratch *valueScratch, s string) ([]byte, error) {
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
	buf := scratch.carveBytes(8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(asn)) //nolint:gosec // G115: bounded by ParseUint 32-bit
	ip4 := addr.As4()
	copy(buf[4:8], ip4[:])
	return buf, nil
}

// encodeCommunityValue encodes space-separated community strings to wire value bytes.
// Each community is 4 bytes (big-endian uint32).
func encodeCommunityValue(scratch *valueScratch, s string) ([]byte, error) {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return []byte{}, nil
	}
	buf := scratch.carveBytes(len(tokens) * 4)
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
func encodeLargeCommunityValue(scratch *valueScratch, s string) ([]byte, error) {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return []byte{}, nil
	}
	buf := scratch.carveBytes(len(tokens) * 12)
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
// Each extended community is 8 bytes.
//
// THE ONE ENCODER THAT ALLOCATES OUTSIDE THE SCRATCH, and it is the attribute
// package that does it: Builder is the only public parser for extended
// communities, and it builds a whole attribute before this can take the value
// out of it. The value is copied into the scratch anyway, so every operation
// buffer on this path has one lifetime rather than two.
func encodeExtCommunityValue(scratch *valueScratch, s string) ([]byte, error) {
	b := attribute.NewBuilder()
	if err := b.ParseExtCommunity(s); err != nil {
		return nil, err
	}
	wire := b.Build()
	if len(wire) == 0 {
		return []byte{}, nil
	}

	value := stripAttrHeader(wire)
	buf := scratch.carveBytes(len(value))
	copy(buf, value)
	return buf, nil
}

// encodeIPv4Value encodes a dotted-decimal IPv4 string to 4 wire bytes.
// Used for ORIGINATOR_ID.
func encodeIPv4Value(scratch *valueScratch, s string) ([]byte, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil, fmt.Errorf("invalid IPv4: %s", s)
	}
	if !addr.Is4() {
		return nil, fmt.Errorf("expected IPv4: %s", s)
	}

	ip4 := addr.As4()
	buf := scratch.carveBytes(len(ip4))
	copy(buf, ip4[:])
	return buf, nil
}

// encodeAIGPValue encodes a decimal metric string to an 11-byte AIGP TLV value.
// RFC 7311: type(1) + length(2) + metric(8).
func encodeAIGPValue(scratch *valueScratch, s string) ([]byte, error) {
	metric, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid aigp metric: %s", s)
	}
	buf := scratch.carveBytes(attribute.AIGPWireLen)
	attribute.WriteAIGPMetric(buf, 0, metric)
	return buf, nil
}

// encodeClusterListValue encodes space-separated dotted-decimal IDs to wire bytes.
// Each cluster ID is 4 bytes.
func encodeClusterListValue(scratch *valueScratch, s string) ([]byte, error) {
	tokens := strings.Fields(s)
	buf := scratch.carveBytes(len(tokens) * 4)
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
// because the local AS is only known at the call site (filter_ordered.go, on
// both the import and the export chain). The attributes come from the call
// site's single parse of the modified text and are read, never mutated.
//
// asn4 is the width of the payload the segment is spliced into, and every call
// site derives it from wherever that payload came from: the import chain and
// the forwarded export chain read the SOURCE encoding context, because those
// bytes are still in the sending peer's encoding, while exportFilterForBody
// passes the destination's sendASN4, because the session write path has already
// encoded that body in the destination's send context. It is the same fact
// ExtractRemovePrivateASOps takes under the same name, for the same reason:
// aspathHandler splices these AS numbers into the AS_PATH value where it
// already sits (into its leading AS_SEQUENCE when that has room), so a segment
// encoded at any other width leaves an attribute that does not decode as one
// AS_PATH.
//
// Does nothing if the modified attributes do not contain as-path-prepend.
// The text names the count; prependASPathOps, which the library chain's
// prepend intent reaches too, writes the operations.
func ExtractASPathPrependOps(scratch *valueScratch, modAttrs *filterAttrs, attrs *attribute.AttributesWire, asn4 bool, localAS uint32, mods *filterapi.ModAccumulator) {
	countStr, ok := modAttrs.get(faASPathPrepend)
	if !ok || countStr == "" {
		return
	}
	count, err := strconv.ParseUint(countStr, 10, 8)
	if err != nil || count == 0 || count > policychain.ASPathPrependMax {
		fwdLogger().Warn("as-path-prepend: invalid count", "value", countStr)
		return
	}
	prependASPathOps(scratch, attrs, asn4, localAS, uint8(count), mods)
}

// ExtractRemovePrivateASOps checks the parsed modified filter attributes
// for a "remove-private" directive and emits AS_PATH / AS4_PATH Set or
// Suppress ops after rewriting raw path segments. The plugin supplies the
// policy intent; the reactor owns wire-safe segment preservation. The
// attributes come from the call site's single parse of the modified text and
// are read, never mutated.
//
// RFC 6996 Section 4 requires private-use ASNs to be removed from AS path
// attributes (including AS4_PATH if utilizing a four-octet AS number space)
// before being advertised to the global Internet.
func ExtractRemovePrivateASOps(scratch *valueScratch, modAttrs *filterAttrs, attrs *attribute.AttributesWire, asn4 bool, peerAS uint32, mods *filterapi.ModAccumulator) {
	mode, ok := extractRemovePrivateASMode(modAttrs)
	if !ok {
		return
	}
	removePrivateASOps(scratch, attrs, asn4, mode, peerAS, mods)
}

// extractRemovePrivateASMode answers the text's remove-private mode, and
// false when the text names none or an unknown one.
func extractRemovePrivateASMode(modAttrs *filterAttrs) (policychain.RemovePrivateMode, bool) {
	mode, ok := modAttrs.get(faRemovePrivate)
	if !ok {
		return policychain.RemovePrivateUnspecified, false
	}
	switch mode {
	case removePrivateASStrip:
		return policychain.RemovePrivateStrip, true
	case removePrivateASPeerAS:
		return policychain.RemovePrivatePeerAS, true
	default:
		fwdLogger().Warn("remove-private-as: invalid mode", "mode", mode)
		return policychain.RemovePrivateUnspecified, false
	}
}
