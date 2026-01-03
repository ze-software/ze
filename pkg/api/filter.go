package api

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// Filter keywords.
const (
	filterAll  = "all"
	filterNone = "none"
)

// structuralAttributes cannot be filtered (MP_REACH/UNREACH).
var structuralAttributes = map[attribute.AttributeCode]string{
	attribute.AttrMPReachNLRI:   "MP_REACH_NLRI",
	attribute.AttrMPUnreachNLRI: "MP_UNREACH_NLRI",
}

// attributeNameToCode maps config names to attribute codes.
var attributeNameToCode = map[string]attribute.AttributeCode{
	"origin":               attribute.AttrOrigin,
	"as-path":              attribute.AttrASPath,
	"next-hop":             attribute.AttrNextHop,
	"med":                  attribute.AttrMED,
	"local-pref":           attribute.AttrLocalPref,
	"atomic-aggregate":     attribute.AttrAtomicAggregate,
	"aggregator":           attribute.AttrAggregator,
	"community":            attribute.AttrCommunity, // Singular
	"communities":          attribute.AttrCommunity, // Plural (both accepted)
	"originator-id":        attribute.AttrOriginatorID,
	"cluster-list":         attribute.AttrClusterList,
	"extended-community":   attribute.AttrExtCommunity,   // Singular
	"extended-communities": attribute.AttrExtCommunity,   // Plural (both accepted)
	"large-community":      attribute.AttrLargeCommunity, // Singular
	"large-communities":    attribute.AttrLargeCommunity, // Plural (both accepted)
}

// ParseAttributeFilter parses an attribute filter from config string.
// Accepts: "all", "none", or space-separated attribute names.
// Names: origin, as-path, next-hop, med, local-pref, community/communities, etc.
// Numeric: attr-N for any attribute code (0-255).
// Returns error for unknown names or structural attributes (attr-14, attr-15).
func ParseAttributeFilter(s string) (AttributeFilter, error) {
	s = strings.TrimSpace(s)
	if s == filterAll || s == "" {
		return NewFilterAll(), nil
	}
	if s == filterNone {
		return NewFilterNone(), nil
	}

	names := strings.Fields(s)
	seen := make(map[attribute.AttributeCode]bool, len(names))
	codes := make([]attribute.AttributeCode, 0, len(names))

	for _, name := range names {
		name = strings.ToLower(name)

		// Handle numeric attr-N syntax
		if strings.HasPrefix(name, "attr-") {
			numStr := strings.TrimPrefix(name, "attr-")
			num, err := strconv.Atoi(numStr)
			if err != nil || num < 0 || num > 255 {
				return AttributeFilter{}, fmt.Errorf("invalid attribute code: %s", name)
			}
			code := attribute.AttributeCode(num)

			// Reject structural attributes
			if structName, ok := structuralAttributes[code]; ok {
				return AttributeFilter{}, fmt.Errorf("attr-%d (%s) is structural and cannot be filtered", num, structName)
			}

			if !seen[code] {
				seen[code] = true
				codes = append(codes, code)
			}
			continue
		}

		code, ok := attributeNameToCode[name]
		if !ok {
			return AttributeFilter{}, fmt.Errorf("unknown attribute %q, valid: %s, or attr-N for numeric",
				name, validAttributeNames())
		}
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	return NewFilterSelective(codes), nil
}

// validAttributeNames returns a sorted list of valid attribute names.
func validAttributeNames() string {
	unique := make(map[string]bool)
	for name := range attributeNameToCode {
		//nolint:goconst // These are config keywords, not constants
		if name == "community" || name == "extended-community" || name == "large-community" {
			continue
		}
		unique[name] = true
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// FilterMode defines how attributes are selected.
type FilterMode uint8

const (
	FilterModeAll       FilterMode = iota // Include all attributes (default)
	FilterModeNone                        // Include no attributes
	FilterModeSelective                   // Include only specified codes
)

// AttributeFilter specifies which attributes to include in API output.
// Thread-safe: filter is immutable after construction.
type AttributeFilter struct {
	Mode    FilterMode
	Codes   []attribute.AttributeCode        // For GetMultiple() - slice for wire.GetMultiple
	codeSet map[attribute.AttributeCode]bool // For O(1) Includes() lookup
}

// FilterResult contains filtered attributes and NLRI from UPDATE parsing.
// This is the primary output type for lazy parsing with filtering.
type FilterResult struct {
	// Attributes contains parsed attributes (only those matching the filter).
	// Key is attribute code, value is parsed Attribute interface.
	// nil or empty means no attributes to output.
	Attributes map[attribute.AttributeCode]attribute.Attribute

	// Announced contains announced prefixes (IPv4 from body + IPv6 from MP_REACH).
	Announced []netip.Prefix

	// Withdrawn contains withdrawn prefixes (IPv4 from body + IPv6 from MP_UNREACH).
	Withdrawn []netip.Prefix

	// NextHopIPv4 is the next-hop for IPv4 prefixes (from NEXT_HOP attribute).
	NextHopIPv4 netip.Addr

	// NextHopIPv6 is the next-hop for IPv6 prefixes (from MP_REACH_NLRI).
	NextHopIPv6 netip.Addr
}

// NLRIFilter specifies which address families to include in API output.
// Thread-safe: filter is immutable after construction.
type NLRIFilter struct {
	Mode     FilterMode
	Families map[string]bool // e.g., "ipv4 unicast", "ipv6 unicast"
}

// NewNLRIFilterAll returns a filter that includes all families.
func NewNLRIFilterAll() NLRIFilter {
	return NLRIFilter{Mode: FilterModeAll}
}

// NewNLRIFilterNone returns a filter that excludes all families.
func NewNLRIFilterNone() NLRIFilter {
	return NLRIFilter{Mode: FilterModeNone}
}

// NewNLRIFilterSelective returns a filter for specific families.
func NewNLRIFilterSelective(families map[string]bool) NLRIFilter {
	return NLRIFilter{
		Mode:     FilterModeSelective,
		Families: families,
	}
}

// IncludesFamily returns true if the given family should be included.
func (f NLRIFilter) IncludesFamily(family string) bool {
	switch f.Mode {
	case FilterModeNone:
		return false
	case FilterModeAll:
		return true
	case FilterModeSelective:
		return f.Families[family]
	default:
		return true
	}
}

// IsEmpty returns true if no families would be included.
func (f NLRIFilter) IsEmpty() bool {
	return f.Mode == FilterModeNone || (f.Mode == FilterModeSelective && len(f.Families) == 0)
}

// NextHopFor returns the appropriate next-hop for a prefix based on its address family.
// Returns IPv4 next-hop for IPv4 prefixes, IPv6 next-hop for IPv6 prefixes.
// Falls back to the other if the preferred one is not set.
func (r FilterResult) NextHopFor(p netip.Prefix) netip.Addr {
	if p.Addr().Is6() {
		if r.NextHopIPv6.IsValid() {
			return r.NextHopIPv6
		}
		return r.NextHopIPv4 // fallback
	}
	if r.NextHopIPv4.IsValid() {
		return r.NextHopIPv4
	}
	return r.NextHopIPv6 // fallback
}

// NewFilterAll returns a filter that includes all attributes.
func NewFilterAll() AttributeFilter {
	return AttributeFilter{Mode: FilterModeAll}
}

// NewFilterNone returns a filter that excludes all attributes.
func NewFilterNone() AttributeFilter {
	return AttributeFilter{Mode: FilterModeNone}
}

// NewFilterSelective returns a filter for specific attribute codes.
// Creates both slice (for GetMultiple) and map (for O(1) Includes).
func NewFilterSelective(codes []attribute.AttributeCode) AttributeFilter {
	if len(codes) == 0 {
		return AttributeFilter{Mode: FilterModeSelective}
	}
	codesCopy := make([]attribute.AttributeCode, len(codes))
	copy(codesCopy, codes)

	codeSet := make(map[attribute.AttributeCode]bool, len(codes))
	for _, c := range codes {
		codeSet[c] = true
	}

	return AttributeFilter{
		Mode:    FilterModeSelective,
		Codes:   codesCopy,
		codeSet: codeSet,
	}
}

// IsEmpty returns true if no attributes would be included.
func (f AttributeFilter) IsEmpty() bool {
	return f.Mode == FilterModeNone || (f.Mode == FilterModeSelective && len(f.Codes) == 0)
}

// Includes returns true if the given attribute code should be included.
// O(1) lookup for selective mode via codeSet map.
func (f AttributeFilter) Includes(code attribute.AttributeCode) bool {
	switch f.Mode {
	case FilterModeNone:
		return false
	case FilterModeAll:
		return true
	case FilterModeSelective:
		return f.codeSet[code]
	default:
		return true
	}
}

// Apply returns filtered attributes from AttrsWire (lazy parsing).
// Only parses attributes that match the filter.
// Thread-safe: AttributesWire has internal mutex.
func (f AttributeFilter) Apply(wire *attribute.AttributesWire) (FilterResult, error) {
	result := FilterResult{}

	if wire == nil {
		return result, nil
	}

	switch f.Mode {
	case FilterModeNone:
		return result, nil

	case FilterModeAll:
		attrs, err := wire.All()
		if err != nil {
			return result, err
		}
		if len(attrs) > 0 {
			result.Attributes = make(map[attribute.AttributeCode]attribute.Attribute, len(attrs))
			for _, attr := range attrs {
				result.Attributes[attr.Code()] = attr
			}
		}
		return result, nil

	case FilterModeSelective:
		// Only parse requested attributes (lazy parsing benefit)
		attrs, err := wire.GetMultiple(f.Codes)
		if err != nil {
			return result, err
		}
		if len(attrs) > 0 {
			result.Attributes = attrs
		}
		return result, nil

	default:
		return result, fmt.Errorf("unknown filter mode: %d", f.Mode)
	}
}

// ApplyToUpdate returns filtered attributes AND NLRI from UPDATE.
// This is the main entry point for processing UPDATEs with lazy parsing.
//
// Parameters:
//   - wire: AttributesWire for lazy attribute parsing (may be nil)
//   - body: raw UPDATE body for NLRI extraction
//   - nlriFilter: which address families to include in output
//
// The function:
//  1. Extracts NLRI from body structure (IPv4) if nlriFilter includes ipv4 unicast
//  2. Gets MP_REACH/MP_UNREACH from wire for other families if included
//  3. Applies filter to get requested attributes from wire
func (f AttributeFilter) ApplyToUpdate(wire *attribute.AttributesWire, body []byte, nlriFilter NLRIFilter) (FilterResult, error) {
	result := FilterResult{}

	// Extract IPv4 unicast NLRI if included
	if nlriFilter.IncludesFamily("ipv4 unicast") {
		ipv4Announced, ipv4Withdrawn, ipv4NextHop := extractNLRIFromBody(body)
		result.Announced = ipv4Announced
		result.Withdrawn = ipv4Withdrawn
		result.NextHopIPv4 = ipv4NextHop
	}

	// Get MP NLRI from wire (lazy - only parses MP attrs if present)
	if wire != nil && !nlriFilter.IsEmpty() {
		mpAnnounced, mpNextHop, family := extractMPReachFromWireWithFamily(wire)
		if nlriFilter.IncludesFamily(family) {
			result.Announced = append(result.Announced, mpAnnounced...)
			if mpNextHop.Is6() {
				result.NextHopIPv6 = mpNextHop
			} else if mpNextHop.Is4() {
				result.NextHopIPv4 = mpNextHop
			}
		}

		mpWithdrawn, wFamily := extractMPUnreachFromWireWithFamily(wire)
		if nlriFilter.IncludesFamily(wFamily) {
			result.Withdrawn = append(result.Withdrawn, mpWithdrawn...)
		}
	}

	// Apply filter to get attributes (skipped for FilterModeNone)
	if f.Mode != FilterModeNone && wire != nil {
		filtered, err := f.Apply(wire)
		if err != nil {
			return result, err
		}
		result.Attributes = filtered.Attributes
	}

	return result, nil
}

// extractNLRIFromBody extracts IPv4 NLRI and withdrawn from UPDATE body.
// Also extracts NEXT_HOP if present in path attributes.
// Does NOT parse other attributes - this is the fast path.
func extractNLRIFromBody(body []byte) (announced, withdrawn []netip.Prefix, nextHop netip.Addr) {
	if len(body) < 4 {
		return nil, nil, netip.Addr{}
	}

	// Parse UPDATE structure: withdrawn_len (2) + withdrawn + attr_len (2) + attrs + nlri
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2

	// Parse withdrawn routes (IPv4)
	if withdrawnLen > 0 && offset+withdrawnLen <= len(body) {
		withdrawn = parseIPv4Prefixes(body[offset : offset+withdrawnLen])
	}
	offset += withdrawnLen

	if offset+2 > len(body) {
		return announced, withdrawn, nextHop
	}

	attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(body) {
		return announced, withdrawn, nextHop
	}

	// Quick scan for NEXT_HOP only (don't parse other attrs)
	nextHop = extractNextHopQuick(body[offset : offset+attrLen])

	nlriOffset := offset + attrLen
	nlriLen := len(body) - nlriOffset

	// Parse IPv4 NLRI
	if nlriLen > 0 {
		announced = parseIPv4Prefixes(body[nlriOffset:])
	}

	return announced, withdrawn, nextHop
}

// extractNextHopQuick scans attributes for NEXT_HOP without full parsing.
// Returns zero Addr if not found.
func extractNextHopQuick(pathAttrs []byte) netip.Addr {
	for i := 0; i < len(pathAttrs); {
		if i+2 > len(pathAttrs) {
			break
		}
		flags := pathAttrs[i]
		typeCode := pathAttrs[i+1]
		attrLenBytes := 1
		if flags&0x10 != 0 { // Extended length
			attrLenBytes = 2
		}
		if i+2+attrLenBytes > len(pathAttrs) {
			break
		}
		var attrValueLen int
		if attrLenBytes == 1 {
			attrValueLen = int(pathAttrs[i+2])
			i += 3
		} else {
			attrValueLen = int(binary.BigEndian.Uint16(pathAttrs[i+2 : i+4]))
			i += 4
		}
		if i+attrValueLen > len(pathAttrs) {
			break
		}

		// NEXT_HOP = type code 3
		if typeCode == 3 && attrValueLen == 4 {
			var addrBytes [4]byte
			copy(addrBytes[:], pathAttrs[i:i+4])
			return netip.AddrFrom4(addrBytes)
		}

		i += attrValueLen
	}
	return netip.Addr{}
}

// extractMPReachFromWireWithFamily gets prefixes, next-hop, and family from MP_REACH_NLRI.
// Uses wire's lazy parsing - only parses this specific attribute.
func extractMPReachFromWireWithFamily(wire *attribute.AttributesWire) (prefixes []netip.Prefix, nextHop netip.Addr, family string) {
	attr, err := wire.Get(attribute.AttrMPReachNLRI)
	if err != nil || attr == nil {
		return nil, netip.Addr{}, ""
	}

	mpReach, ok := attr.(*attribute.MPReachNLRI)
	if !ok {
		return nil, netip.Addr{}, ""
	}

	// Get next-hop (first one if multiple)
	if len(mpReach.NextHops) > 0 {
		nextHop = mpReach.NextHops[0]
	}

	// Determine family string
	family = message.AFISAFIToFamily(message.AFI(mpReach.AFI), message.SAFI(mpReach.SAFI))

	// Parse NLRI bytes based on AFI/SAFI
	switch {
	case mpReach.AFI == attribute.AFIIPv6 && mpReach.SAFI == attribute.SAFIUnicast:
		prefixes = parseIPv6Prefixes(mpReach.NLRI)
	case mpReach.AFI == attribute.AFIIPv4 && mpReach.SAFI == attribute.SAFIUnicast:
		prefixes = parseIPv4Prefixes(mpReach.NLRI)
		// TODO: handle other AFI/SAFI combinations
	}

	return prefixes, nextHop, family
}

// extractMPUnreachFromWireWithFamily gets withdrawn prefixes and family from MP_UNREACH_NLRI.
// Uses wire's lazy parsing - only parses this specific attribute.
func extractMPUnreachFromWireWithFamily(wire *attribute.AttributesWire) (prefixes []netip.Prefix, family string) {
	attr, err := wire.Get(attribute.AttrMPUnreachNLRI)
	if err != nil || attr == nil {
		return nil, ""
	}

	mpUnreach, ok := attr.(*attribute.MPUnreachNLRI)
	if !ok {
		return nil, ""
	}

	// Determine family string
	family = message.AFISAFIToFamily(message.AFI(mpUnreach.AFI), message.SAFI(mpUnreach.SAFI))

	// Parse withdrawn NLRI bytes based on AFI/SAFI
	switch {
	case mpUnreach.AFI == attribute.AFIIPv6 && mpUnreach.SAFI == attribute.SAFIUnicast:
		prefixes = parseIPv6Prefixes(mpUnreach.NLRI)
	case mpUnreach.AFI == attribute.AFIIPv4 && mpUnreach.SAFI == attribute.SAFIUnicast:
		prefixes = parseIPv4Prefixes(mpUnreach.NLRI)
		// TODO: handle other AFI/SAFI combinations
	}

	return prefixes, family
}
