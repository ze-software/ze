// Design: docs/architecture/config/syntax.md — BGP route attribute parsing
// RFC: rfc/short/rfc2545.md
// Detail: routeattr_community.go — community attribute types
// Detail: routeattr_prefixsid.go — prefix SID attribute types

package bgpconfig

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/asn"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/parse"
)

// Origin represents the ORIGIN path attribute.
// RFC 4271: 0=IGP, 1=EGP, 2=INCOMPLETE.
type Origin uint8

const (
	OriginIGP        Origin = 0
	OriginEGP        Origin = 1
	OriginIncomplete Origin = 2
)

// ParseOrigin parses an origin string (igp, egp, incomplete).
// Delegates to parse.Origin() for the actual parsing logic.
func ParseOrigin(s string) (Origin, error) {
	v, err := parse.Origin(s)
	if err != nil {
		return 0, err
	}
	return Origin(v), nil
}

func (o Origin) String() string {
	return parse.OriginString(uint8(o))
}

// PathID represents an ADD-PATH path identifier.
// Valid range: 0-4294967295 (0 means not set).
type PathID uint32

// ParsePathID parses a path-information value: a decimal uint32 or the dotted
// a.b.c.d form, through parse.PathID, the one parser config and `update text`
// share. An absent value (empty string) is no path identifier.
func ParsePathID(s string) (PathID, error) {
	if s == "" {
		return 0, nil
	}
	// RFC 7911 Section 3
	id, err := parse.PathID(s)
	if err != nil {
		return 0, err
	}
	return PathID(id), nil
}

func (p PathID) String() string {
	if p == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(p), 10)
}

// MPLSLabel represents an MPLS label stack entry.
// Valid range: 0-1048575 (20 bits).
const (
	MPLSLabelMin = 0
	MPLSLabelMax = 1048575 // 2^20 - 1
)

type MPLSLabel uint32

// ParseMPLSLabel parses an MPLS label value.
func ParseMPLSLabel(s string) (MPLSLabel, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid label %q: %w", s, err)
	}
	if n > MPLSLabelMax {
		return 0, fmt.Errorf("invalid label %d: must be 0-%d", n, MPLSLabelMax)
	}
	return MPLSLabel(n), nil
}

func (l MPLSLabel) String() string {
	if l == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(l), 10)
}

// RouteDistinguisher represents an RD (RFC 4364).
// Formats: Type0 (ASN2:NN4), Type1 (IP:NN2), Type2 (ASN4:NN2).
type RouteDistinguisher struct {
	Raw   string  // Original string
	Bytes [8]byte // Wire-format (2-byte type + 6-byte value)
}

// RD types.
const (
	rdType0 = 0 // 2-byte ASN : 4-byte assigned
	rdType1 = 1 // 4-byte IP : 2-byte assigned
	rdType2 = 2 // 4-byte ASN : 2-byte assigned
)

// ParseRouteDistinguisher parses an RD string to wire format.
func ParseRouteDistinguisher(s string) (RouteDistinguisher, error) {
	if s == "" {
		return RouteDistinguisher{}, nil
	}

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return RouteDistinguisher{}, fmt.Errorf("invalid rd %q: expected format ASN:NN or IP:NN", s)
	}

	var rd RouteDistinguisher
	rd.Raw = s

	// Check if first part is an IP address
	if ip, err := netip.ParseAddr(parts[0]); err == nil && ip.Is4() {
		// Type 1: IP:NN
		num, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return RouteDistinguisher{}, fmt.Errorf("invalid rd number %q", parts[1])
		}
		b := ip.As4()
		rd.Bytes[0], rd.Bytes[1] = 0, rdType1
		copy(rd.Bytes[2:6], b[:])
		rd.Bytes[6], rd.Bytes[7] = byte(num>>8), byte(num)
		return rd, nil
	}

	// Parse as ASN:NN. asn.Parse reads all three RFC 5396 spellings, so
	// `rd 1.10:5` names the same route distinguisher as `rd 65546:5`. The
	// IPv4 probe above runs first, so no address reaches this branch.
	number, err := asn.Parse(parts[0])
	if err != nil {
		return RouteDistinguisher{}, fmt.Errorf("invalid rd ASN %q", parts[0])
	}
	num, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return RouteDistinguisher{}, fmt.Errorf("invalid rd number %q", parts[1])
	}

	if number <= 0xFFFF {
		// Type 0: 2-byte ASN, 4-byte number
		rd.Bytes[0], rd.Bytes[1] = 0, rdType0
		rd.Bytes[2], rd.Bytes[3] = byte(number>>8), byte(number)
		rd.Bytes[4] = byte(num >> 24)
		rd.Bytes[5] = byte(num >> 16)
		rd.Bytes[6] = byte(num >> 8)
		rd.Bytes[7] = byte(num)
	} else {
		// Type 2: 4-byte ASN, 2-byte number
		rd.Bytes[0], rd.Bytes[1] = 0, rdType2
		rd.Bytes[2] = byte(number >> 24)
		rd.Bytes[3] = byte(number >> 16)
		rd.Bytes[4] = byte(number >> 8)
		rd.Bytes[5] = byte(number)
		rd.Bytes[6], rd.Bytes[7] = byte(num>>8), byte(num)
	}

	return rd, nil
}

func (rd RouteDistinguisher) String() string {
	return rd.Raw
}

// IsZero returns true if the RD is empty/unset.
func (rd RouteDistinguisher) IsZero() bool {
	return rd.Raw == ""
}

// ASPath represents the AS_PATH attribute (RFC 4271).
type ASPath struct {
	Raw    string   // Original string (e.g., "[ 57821 6939 ]")
	Values []uint32 // Parsed AS numbers
}

// ParseASPath parses an AS_PATH string: "[ ASN1 ASN2 ... ]".
func ParseASPath(s string) (ASPath, error) {
	asp := ASPath{Raw: s}
	if s == "" {
		return asp, nil
	}

	// Remove brackets and trim whitespace
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	if s == "" {
		return asp, nil
	}

	// Split by whitespace
	parts := strings.Fields(s)
	asp.Values = make([]uint32, 0, len(parts))

	for _, p := range parts {
		// asn.Parse reads the asdot spellings beside the decimal one, so an
		// AS path written in the notation the operator reads is accepted.
		n, err := asn.Parse(p)
		if err != nil {
			return asp, fmt.Errorf("invalid AS number %q: %w", p, err)
		}
		asp.Values = append(asp.Values, n)
	}

	return asp, nil
}

// IsZero returns true if AS_PATH is empty/unset.
func (asp ASPath) IsZero() bool {
	return len(asp.Values) == 0
}

// Aggregator represents the AGGREGATOR attribute (RFC 4271).
// Format: ( ASN:IP ) e.g., ( 18144:219.118.225.189 ).
type Aggregator struct {
	Raw   string  // Original string
	ASN   uint32  // Aggregator AS
	IP    [4]byte // Aggregator IP (IPv4 only per RFC)
	Valid bool    // Whether aggregator is set
}

// ParseAggregator parses an AGGREGATOR string: "( ASN:IP )".
func ParseAggregator(s string) (Aggregator, error) {
	agg := Aggregator{Raw: s}
	if s == "" {
		return agg, nil
	}

	// Remove parentheses and trim whitespace
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	s = strings.TrimSpace(s)

	if s == "" {
		return agg, nil
	}

	// Split by colon - format is ASN:IP
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return agg, fmt.Errorf("invalid aggregator format %q: expected ASN:IP", s)
	}

	// Parse the AS half. It goes through the same reader as every other AS
	// number, so `1.10:192.0.2.1` is accepted. The address half is untouched.
	number, err := asn.Parse(strings.TrimSpace(parts[0]))
	if err != nil {
		return agg, fmt.Errorf("invalid aggregator ASN %q: %w", parts[0], err)
	}
	agg.ASN = number

	// Parse IP
	ip, err := netip.ParseAddr(strings.TrimSpace(parts[1]))
	if err != nil {
		return agg, fmt.Errorf("invalid aggregator IP %q: %w", parts[1], err)
	}
	if !ip.Is4() {
		return agg, fmt.Errorf("aggregator IP must be IPv4: %s", ip)
	}
	agg.IP = ip.As4()
	agg.Valid = true

	return agg, nil
}

// IsZero returns true if aggregator is not set.
func (agg Aggregator) IsZero() bool {
	return !agg.Valid
}

// ParsedRouteAttributes holds all parsed route attributes.
type ParsedRouteAttributes struct {
	Prefix          netip.Prefix
	NextHop         netip.Addr
	Origin          Origin
	LocalPreference uint32
	MED             uint32
	// HasLocalPreference and HasMED say the route carries the attribute; 0 is
	// a legal value for both (RFC 4271 Sections 5.1.4 and 5.1.5).
	HasLocalPreference bool
	HasMED             bool
	Community          Community
	ExtendedCommunity  ExtendedCommunity
	LargeCommunity     LargeCommunity
	PathID             PathID
	Labels             []MPLSLabel // RFC 8277: MPLS label stack
	RD                 RouteDistinguisher
	ASPath             ASPath
	Aggregator         Aggregator
	AtomicAggregate    bool
	OriginatorID       uint32   // RFC 4456
	ClusterList        []uint32 // RFC 4456
	AIGPMetric         *uint64  // RFC 7311
	PrefixSID          []byte   // BGP Prefix-SID TLV bytes (attribute 40), from attribute.EncodePrefixSID*
	RawAttributes      []textparse.RawAttribute
}

// ParseRouteAttributes parses all attributes from a StaticRouteConfig.
func ParseRouteAttributes(src *StaticRouteConfig) (*ParsedRouteAttributes, error) {
	attrs := &ParsedRouteAttributes{
		Prefix:             src.Prefix,
		LocalPreference:    src.LocalPreference,
		MED:                src.MED,
		HasLocalPreference: src.HasLocalPreference,
		HasMED:             src.HasMED,
	}

	// NextHop
	if src.NextHop != "" && src.NextHop != "self" {
		ip, err := netip.ParseAddr(src.NextHop)
		if err != nil {
			return nil, fmt.Errorf("invalid next-hop %q: %w", src.NextHop, err)
		}
		// A LINK-LOCAL ADDRESS IS ACCEPTED HERE, AND THE SESSION DECIDES.
		// draft-ietf-idr-linklocal-capability Section 3 gives it a form RFC 2545
		// has none for: "it MUST set the length of the Next Hop field to 16 and
		// include only the IPv6 Link-Local address in the Next Hop field". That
		// form belongs to a session that negotiated capability 77, and one route
		// is advertised to many peers, so this file cannot answer for it.
		// Peer.resolveNextHop (../reactor/peer.go) asks per peer and leaves the
		// route out of the announcement where the answer is no.
		//
		// RFC 2545 Section 3 still governs the FIRST address of the 32-octet
		// pair, and attribute.ValidateGlobalNextHop still refuses a link-local
		// one there (linkScope.linkLocalNextHop, ../reactor/link_scope.go).
		attrs.NextHop = ip
	}

	// Origin
	origin, err := ParseOrigin(src.Origin)
	if err != nil {
		return nil, err
	}
	attrs.Origin = origin

	// Community (RFC 1997)
	comm, err := ParseCommunity(src.Community)
	if err != nil {
		return nil, err
	}
	attrs.Community = comm

	// Extended Community
	ec, err := ParseExtendedCommunity(src.ExtendedCommunity)
	if err != nil {
		return nil, err
	}
	attrs.ExtendedCommunity = ec

	// Large Community (RFC 8092)
	lc, err := ParseLargeCommunity(src.LargeCommunity)
	if err != nil {
		return nil, err
	}
	attrs.LargeCommunity = lc

	// Path ID
	pid, err := ParsePathID(src.PathInformation)
	if err != nil {
		return nil, err
	}
	attrs.PathID = pid

	// Labels - RFC 8277 multi-label support
	// `labels [...]` takes precedence if both specified (user error, but deterministic)
	if len(src.Labels) > 0 {
		attrs.Labels = make([]MPLSLabel, len(src.Labels))
		for i, ls := range src.Labels {
			label, err := ParseMPLSLabel(ls)
			if err != nil {
				return nil, fmt.Errorf("label[%d]: %w", i, err)
			}
			attrs.Labels[i] = label
		}
	} else if src.Label != "" {
		label, err := ParseMPLSLabel(src.Label)
		if err != nil {
			return nil, err
		}
		attrs.Labels = []MPLSLabel{label}
	}

	// RD
	rd, err := ParseRouteDistinguisher(src.RD)
	if err != nil {
		return nil, err
	}
	attrs.RD = rd

	// AS_PATH
	asp, err := ParseASPath(src.ASPath)
	if err != nil {
		return nil, err
	}
	attrs.ASPath = asp

	// Aggregator
	agg, err := ParseAggregator(src.Aggregator)
	if err != nil {
		return nil, err
	}
	attrs.Aggregator = agg

	// Atomic Aggregate
	attrs.AtomicAggregate = src.AtomicAggregate

	// Originator ID (RFC 4456)
	if src.OriginatorID != "" {
		ip, err := netip.ParseAddr(src.OriginatorID)
		if err != nil {
			return nil, fmt.Errorf("invalid originator-id %q: %w", src.OriginatorID, err)
		}
		if ip.Is4() {
			b := ip.As4()
			attrs.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Cluster List (RFC 4456)
	if src.ClusterList != "" {
		clStr := strings.TrimSpace(src.ClusterList)
		clStr = strings.TrimPrefix(clStr, "[")
		clStr = strings.TrimSuffix(clStr, "]")
		for p := range strings.FieldsSeq(clStr) {
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return nil, fmt.Errorf("invalid cluster-list entry %q: %w", p, err)
			}
			if ip.Is4() {
				b := ip.As4()
				attrs.ClusterList = append(attrs.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	// AIGP (RFC 7311)
	if src.AIGP != "" {
		metric, err := strconv.ParseUint(src.AIGP, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid aigp %q: %w", src.AIGP, err)
		}
		attrs.AIGPMetric = &metric
	}

	// BGP Prefix-SID (RFC 8669 label-index or RFC 9252 SRv6)
	// SRv6 format starts with "l3-service" or "l2-service"
	if src.PrefixSID != "" {
		if strings.HasPrefix(src.PrefixSID, "l3-service") || strings.HasPrefix(src.PrefixSID, "l2-service") {
			sid, err := attribute.EncodePrefixSIDSRv6(src.PrefixSID)
			if err != nil {
				return nil, err
			}
			attrs.PrefixSID = sid
		} else {
			sid, err := attribute.EncodePrefixSID(src.PrefixSID)
			if err != nil {
				return nil, err
			}
			attrs.PrefixSID = sid
		}
	}

	// The generic attribute, `[ 0x<code> 0x<flags> 0x<value> ]`, read by the
	// update text's reader so the two accept and refuse the same statements.
	// Known attribute codes are converted to typed fields.
	if src.Attribute != "" {
		raw, err := textparse.ParseRawAttribute(src.Attribute)
		if err != nil {
			return nil, fmt.Errorf("invalid raw attribute: %w", err)
		}

		if err := parseRawAttributeInto(attrs, src, raw); err != nil {
			return nil, fmt.Errorf("invalid raw attribute: %w", err)
		}
	}

	return attrs, nil
}

// parseRawAttributeInto stores a generic attribute. One of a code a keyword
// writes arrives decoded (textparse.RawAttribute.Typed) and goes into the field
// that keyword fills, replacing the field's default, so the UPDATE carries it
// once: the generic ORIGIN replaces the IGP default, the generic MED sets MED.
// The update text puts the same attribute into its block as the same octets
// (update.parseRawAttributeText). A code the route's own keyword already wrote
// is refused, as the update text refuses it: RFC 4271 Section 6.3 makes an
// attribute given twice a Malformed Attribute List. Any other code is kept as
// written for the wire.
func parseRawAttributeInto(attrs *ParsedRouteAttributes, src *StaticRouteConfig, raw textparse.RawAttribute) error {
	if raw.Typed == nil {
		attrs.RawAttributes = append(attrs.RawAttributes, raw)
		return nil
	}
	code := attribute.AttributeCode(raw.Code)
	keyword, _ := textparse.AttributeKeyword(code)
	twice := fmt.Errorf("attribute 0x%02x (%s) is given twice, by the generic attribute and by %s, and RFC 4271 Section 6.3 makes an attribute given twice a Malformed Attribute List: keep one of the two",
		raw.Code, code, keyword)
	switch typed := raw.Typed.(type) {
	case attribute.Origin:
		if src.Origin != "" {
			return twice
		}
		attrs.Origin = Origin(typed)
	case *attribute.ASPath:
		if src.ASPath != "" {
			return twice
		}
		return rawASPathInto(attrs, raw, typed)
	case attribute.MED:
		if attrs.HasMED {
			return twice
		}
		attrs.MED, attrs.HasMED = uint32(typed), true
	case attribute.LocalPref:
		if attrs.HasLocalPreference {
			return twice
		}
		attrs.LocalPreference, attrs.HasLocalPreference = uint32(typed), true
	case attribute.AtomicAggregate:
		if attrs.AtomicAggregate {
			return twice
		}
		attrs.AtomicAggregate = true
	case *attribute.Aggregator:
		if attrs.Aggregator.Valid {
			return twice
		}
		attrs.Aggregator = Aggregator{ASN: typed.ASN, IP: typed.Address.As4(), Valid: true}
	case attribute.Communities:
		if len(attrs.Community.Values) > 0 {
			return twice
		}
		for _, c := range typed {
			attrs.Community.Values = append(attrs.Community.Values, uint32(c))
		}
	case attribute.OriginatorID:
		if attrs.OriginatorID != 0 {
			return twice
		}
		attrs.OriginatorID = binary.BigEndian.Uint32(raw.Value)
	case attribute.ClusterList:
		if len(attrs.ClusterList) > 0 {
			return twice
		}
		attrs.ClusterList = slices.Clone([]uint32(typed))
	case attribute.ExtendedCommunities:
		if len(attrs.ExtendedCommunity.Bytes) > 0 {
			return twice
		}
		attrs.ExtendedCommunity.Bytes = slices.Clone(raw.Value)
	case attribute.IPv6ExtendedCommunities:
		if len(attrs.ExtendedCommunity.IPv6Bytes) > 0 {
			return twice
		}
		attrs.ExtendedCommunity.IPv6Bytes = slices.Clone(raw.Value)
	case *attribute.AIGP:
		if attrs.AIGPMetric != nil {
			return twice
		}
		return rawAIGPInto(attrs, typed)
	case attribute.LargeCommunities:
		if len(attrs.LargeCommunity.Values) > 0 {
			return twice
		}
		for _, lc := range typed {
			attrs.LargeCommunity.Values = append(attrs.LargeCommunity.Values, [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2})
		}
	case *attribute.PrefixSID:
		if len(attrs.PrefixSID) > 0 {
			return twice
		}
		attrs.PrefixSID = slices.Clone(raw.Value)
	default:
		// A keyword code whose typed form this reader has no field for: refused
		// by name rather than sent raw beside the field's own encoding.
		return fmt.Errorf("attribute 0x%02x (%s) has no generic form in the config file: write the %s keyword instead", raw.Code, code, keyword)
	}
	return nil
}

// rawASPathInto stores a generic AS_PATH. A path of AS_SEQUENCE segments
// becomes the route's as-path, which Ze prepends to for an external peer as it
// does the keyword's. A path holding an AS_SET or AS_CONFED_SET has no typed
// form, because a static route's as-path is one sequence, so it stays raw: the
// send guard then writes the route as a withdrawal, and nothing of it reaches
// the wire. RFC 9774 Section 3: "BGP speakers: MUST NOT advertise BGP UPDATE
// messages containing AS_SETs or AS_CONFED_SETs".
func rawASPathInto(attrs *ParsedRouteAttributes, raw textparse.RawAttribute, path *attribute.ASPath) error {
	var values []uint32
	for _, segment := range path.Segments {
		if segment.Type != attribute.ASSequence {
			attrs.RawAttributes = append(attrs.RawAttributes, raw)
			return nil
		}
		values = append(values, segment.ASNs...)
	}
	attrs.ASPath = ASPath{Raw: hexAttrValue(raw.Value), Values: values}
	return nil
}

// rawAIGPInto stores a generic AIGP, which the aigp field carries as the one
// metric TLV RFC 7311 Section 3.1 defines.
func rawAIGPInto(attrs *ParsedRouteAttributes, aigp *attribute.AIGP) error {
	if len(aigp.TLVs) != 1 {
		return fmt.Errorf("attribute 0x1a (AIGP) holds %d TLVs and the aigp keyword carries one AIGP metric TLV: write aigp <metric>", len(aigp.TLVs))
	}
	metric, found := aigp.Metric()
	if !found {
		return fmt.Errorf("attribute 0x1a (AIGP) holds a TLV of type %d and the aigp keyword carries the AIGP metric TLV, type 1: write aigp <metric>", aigp.TLVs[0].Type)
	}
	attrs.AIGPMetric = &metric
	return nil
}

// hexAttrValue spells an attribute value as the generic attribute writes it.
func hexAttrValue(value []byte) string {
	return "0x" + hex.EncodeToString(value)
}
