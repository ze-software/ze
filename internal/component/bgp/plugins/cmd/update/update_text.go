// Design: docs/architecture/api/commands.md — text UPDATE parsing handlers
// Overview: doc.go — bgp-cmd-update plugin registration
// Detail: update_text_nlri.go — NLRI section parsing
// Detail: update_text_evpn.go — EVPN NLRI parsing
// Detail: update_text_flowspec.go — FlowSpec NLRI parsing
// Detail: update_text_vpls.go — VPLS NLRI parsing
// RFC: rfc/short/rfc1997.md -- COMMUNITIES attribute and well-known values (parseCommunityText)
// RFC: rfc/short/rfc3765.md -- NOPEER well-known community (parseCommunityText)
// RFC: rfc/short/rfc2545.md -- the next-hop value must be a global IPv6 address (parseNhopFlat)
// RFC: rfc/short/rfc4271.md -- ATOMIC_AGGREGATE and AGGREGATOR (parseAggregatorText)
// RFC: rfc/short/rfc4456.md -- ORIGINATOR_ID and CLUSTER_LIST (parseClusterIDText)
// RFC: rfc/short/rfc6793.md -- the AGGREGATOR ASN is written four-octet (parseAggregatorText)
// RFC: rfc/short/rfc7607.md -- AS 0 is refused in the AGGREGATOR (parseAggregatorText)
// RFC: rfc/short/rfc7311.md -- the AIGP metric (parseCommonAttributeText)
// RFC: rfc/short/rfc9252.md -- the SRv6 Service TLV of BGP Prefix-SID (parseCommonAttributeText)
//
// update_text.go provides the update text parser for the "update text" command format.
//
// Grammar (flat — no set/add/del on attributes):
//
//	<update-text>  := <attribute>* <nlri-section>+
//	<attribute>    := <attr-name> <value> | atomic-aggregate
//	<attr-name>    := origin | med | local-preference | as-path | community |
//	                  large-community | extended-community | aggregator |
//	                  originator-id | cluster-list | aigp | bgp-prefix-sid-srv6 |
//	                  nhop | path-information | rd | label
//	<nlri-section> := nlri <family> <nlri-op>+
//	<nlri-op>      := add <prefix>+ [watchdog <name>] | del <prefix>+
//
// atomic-aggregate is the one attribute that takes no value: RFC 4271 Section
// 4.3 f) makes it "a well-known discretionary attribute of length 0", so its
// presence in the command is the whole value.
//
// Attributes are flat declarations (keyword + value). No set/add/del on attributes.
// add/del are NLRI-only keywords (MP_REACH vs MP_UNREACH).
// Attributes must precede all nlri sections (no interleaving).
//
// Note: rd and label are ignored for families that don't support them.
package update

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingOriginValue               = errors.New("missing origin value")
	errMissingLocalPreferenceValue      = errors.New("missing local-preference value")
	errMissingMedValue                  = errors.New("missing med value")
	errMissingAsPathValue               = errors.New("missing as-path value")
	errMissingOriginASValue             = errors.New("missing origin-as value")
	errMissingCommunityValue            = errors.New("missing community value")
	errMissingLargeCommunityValue       = errors.New("missing large-community value")
	errMissingExtendedCommunityValue    = errors.New("missing extended-community value")
	errMissingAggregatorValue           = errors.New("missing aggregator value")
	errMissingOriginatorIDValue         = errors.New("missing originator-id value")
	errMissingClusterListValue          = errors.New("missing cluster-list value")
	errMissingAIGPValue                 = errors.New("missing aigp value")
	errMissingPrefixSIDValue            = errors.New("missing bgp-prefix-sid-srv6 value")
	errEmptyClusterList                 = errors.New("cluster-list requires at least one cluster id")
	errSetDelKeywordsRemovedUseNext     = errors.New("set/del keywords removed; use: next-hop <address|self>")
	errSetDelKeywordsRemovedUseRd       = errors.New("set/del keywords removed; use: rd <value>")
	errSetDelKeywordsRemovedUseLabel    = errors.New("set/del keywords removed; use: label <value> or label [ <value>... ]")
	errSetDelKeywordsRemovedUsePathInfo = errors.New("set/del keywords removed; use: path-information <id>")
	errLabelRequiresValue               = errors.New("label requires a value (0-1048575) or a list [ <value>... ]")
	errPathInfoRequiresValue            = errors.New("path-information requires a value (0-4294967295)")
	errUsageSendUpdateEncoding          = errors.New("usage: send bgp <selector> update <text|hex|b64|cursor>")
)

// labelMax is the largest MPLS label. RFC 3032 Section 2.1 gives the Label field
// 20 bits, so a wider value is refused rather than truncated onto the wire.
const labelMax = 0xFFFFF

// YANG schema paths for attribute validation.
const (
	yangPathOrigin    = "bgp/peer/update/attribute/origin"
	yangPathMED       = "bgp/peer/update/attribute/med"
	yangPathLocalPref = "bgp/peer/update/attribute/local-preference"
)

// UpdateText command keywords.
const (
	kwAttr     = "attr"
	kwNLRI     = "nlri"
	kwWatchdog = "watchdog"
	kwNhop     = "nhop"             // Top-level next-hop keyword.
	kwPathInfo = "path-information" // ADD-PATH path-id keyword.
)

// UpdateText action keywords (NLRI-only: add=MP_REACH, del=MP_UNREACH).
const (
	kwAdd = "add"
	kwDel = "del"
	kwSet = "set" // Rejected with migration hint — kept for detection only.
	kwEOR = "eor" // End-of-RIB marker (RFC 4724).
)

// Attribute keywords for per-attribute syntax.
const (
	kwOrigin            = "origin"
	kwMED               = "med"
	kwLocalPref         = "local-preference"
	kwASPath            = "as-path"
	kwOriginAS          = "origin-as"
	kwCommunity         = "community"
	kwLargeCommunity    = "large-community"
	kwExtendedCommunity = "extended-community"
)

// Structure keywords for NLRI modifiers.
const (
	kwRD    = "rd"    // Route Distinguisher for VPN families.
	kwLabel = "label" // MPLS label for VPN/labeled families.
)

// isAttributeKeyword returns true if token is a per-attribute keyword.
//
// atomic-aggregate is one of them even though ParseUpdateText handles it in its
// own case: the NLRI sub-parsers ask isBoundaryKeyword where a section ends, and
// a value-less attribute ends one exactly as a valued attribute does.
func isAttributeKeyword(token string) bool {
	switch token {
	case kwOrigin, kwMED, kwLocalPref, kwASPath, kwOriginAS,
		kwCommunity, kwLargeCommunity, kwExtendedCommunity,
		textparse.KWAtomicAggregate, textparse.KWAggregator,
		textparse.KWOriginatorID, textparse.KWClusterList, textparse.KWAIGP,
		textparse.KWPrefixSIDSRv6:
		return true
	}
	return false
}

// isBoundaryKeyword returns true if token starts a new top-level section.
// Used by NLRI sub-parsers (EVPN, FlowSpec, VPLS) to detect section boundaries.
// Tokens are already alias-resolved (nhop→next-hop, path-id→path-information).
func isBoundaryKeyword(token string) bool {
	switch token {
	case kwAttr, kwNLRI, kwWatchdog, textparse.KWNextHop, kwPathInfo, kwRD, kwLabel:
		return true
	}
	return isAttributeKeyword(token)
}

// parsedAttrs collects attribute declarations during flat parsing.
// Includes next-hop which is NOT part of path attributes.
// Path-id moved to per-NLRI-section modifier (in nlriAccum).
type parsedAttrs struct {
	NextHop     netip.Addr
	NextHopSelf bool

	// Path attributes (wire-first: build directly to wire format).
	Origin              *uint8
	LocalPreference     *uint32
	MED                 *uint32
	ASPath              []uint32
	OriginAS            uint32
	Communities         []uint32
	LargeCommunities    []bgptypes.LargeCommunity
	ExtendedCommunities []attribute.ExtendedCommunity
	AtomicAggregate     bool
	Aggregator          *attribute.Aggregator
	OriginatorID        netip.Addr
	ClusterList         []uint32
	AIGP                *uint64
	PrefixSID           []byte // BGP Prefix-SID TLVs (attribute 40), already encoded

	// VPN/labeled NLRI modifiers.
	RD     nlri.RouteDistinguisher // Route Distinguisher for VPN families.
	Labels []uint32                // MPLS labels for VPN/labeled families.
	PathID uint32                  // ADD-PATH path identifier (RFC 7911 Section 3).
}

// nlriAccum holds VPN/labeled NLRI accumulator values for snapshot.
type nlriAccum struct {
	PathID uint32
	RD     nlri.RouteDistinguisher
	Labels []uint32
}

// nlriParseResult holds the return values from NLRI section parsing.
type nlriParseResult struct {
	Family   family.Family
	Announce []nlri.NLRI
	Withdraw []nlri.NLRI
	Watchdog string
	Consumed int
}

// snapshot returns a wire-format snapshot of the current attribute state.
// Builds attributes using Builder for wire-first encoding.
// Also returns the current NLRI accumulators (pathID, RD, labels).
func (a *parsedAttrs) snapshot() (*attribute.AttributesWire, bgptypes.RouteNextHop, nlriAccum) {
	// Build wire-format attributes.
	// Note: ORIGIN and AS_PATH are not forced here; reactor adds mandatory
	// attributes if missing (with correct iBGP/eBGP AS_PATH handling).
	b := attribute.NewBuilder()

	if a.Origin != nil {
		b.SetOrigin(*a.Origin)
	}
	if len(a.ASPath) > 0 {
		b.SetASPath(a.ASPath)
	}
	if a.LocalPreference != nil {
		b.SetLocalPref(*a.LocalPreference)
	}
	if a.MED != nil {
		b.SetMED(*a.MED)
	}
	for _, c := range a.Communities {
		b.AddCommunityValue(c)
	}
	for _, lc := range a.LargeCommunities {
		b.AddLargeCommunity(lc.GlobalAdmin, lc.LocalData1, lc.LocalData2)
	}
	for _, ec := range a.ExtendedCommunities {
		b.AddExtendedCommunity(ec)
	}
	if a.AtomicAggregate {
		b.SetAtomicAggregate()
	}
	if a.Aggregator != nil {
		b.SetAggregator(a.Aggregator.ASN, a.Aggregator.Address)
	}
	if a.OriginatorID.IsValid() {
		b.SetOriginatorID(a.OriginatorID)
	}
	if len(a.ClusterList) > 0 {
		b.SetClusterList(a.ClusterList)
	}
	if a.AIGP != nil {
		b.SetAIGP(*a.AIGP)
	}
	if len(a.PrefixSID) > 0 {
		b.SetPrefixSID(a.PrefixSID)
	}

	// Build wire bytes and wrap
	wireBytes := b.Build()
	var wire *attribute.AttributesWire
	if len(wireBytes) > 0 {
		wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	// Convert to bgptypes.RouteNextHop: Self takes precedence if set
	var nh bgptypes.RouteNextHop
	if a.NextHopSelf {
		nh = bgptypes.NewNextHopSelf()
	} else if a.NextHop.IsValid() {
		nh = bgptypes.NewNextHopExplicit(a.NextHop)
	}

	// Deep copy labels slice
	var labels []uint32
	if a.Labels != nil {
		labels = make([]uint32, len(a.Labels))
		copy(labels, a.Labels)
	}
	return wire, nh, nlriAccum{PathID: a.PathID, RD: a.RD, Labels: labels}
}

// parseCommonAttributeText parses a common BGP attribute by keyword into parsedAttrs.
// Returns the number of args consumed (0 if keyword not handled), or error.
func parseCommonAttributeText(key string, args []string, idx int, attrs *parsedAttrs) (int, error) {
	switch key {
	case kwOrigin:
		if idx+1 >= len(args) {
			return 0, errMissingOriginValue
		}
		// YANG validation for origin enum (single source of truth)
		if plugin.YANGValidator() != nil {
			if err := plugin.YANGValidator().Validate(yangPathOrigin, args[idx+1]); err != nil {
				return 0, fmt.Errorf("invalid origin: %w", err)
			}
		}
		origin, err := parseOriginText(args[idx+1])
		if err != nil {
			return 0, err
		}
		attrs.Origin = &origin
		return 1, nil

	case "local-preference":
		if idx+1 >= len(args) {
			return 0, errMissingLocalPreferenceValue
		}
		lp, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid local-preference: %w", err)
		}
		lpVal := uint32(lp)
		// YANG validation for local-preference uint32 (single source of truth)
		if plugin.YANGValidator() != nil {
			if err := plugin.YANGValidator().Validate(yangPathLocalPref, lpVal); err != nil {
				return 0, fmt.Errorf("invalid local-preference: %w", err)
			}
		}
		attrs.LocalPreference = &lpVal
		return 1, nil

	case "med":
		if idx+1 >= len(args) {
			return 0, errMissingMedValue
		}
		med, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid med: %w", err)
		}
		medVal := uint32(med)
		// YANG validation for MED uint32 (single source of truth)
		if plugin.YANGValidator() != nil {
			if err := plugin.YANGValidator().Validate(yangPathMED, medVal); err != nil {
				return 0, fmt.Errorf("invalid med: %w", err)
			}
		}
		attrs.MED = &medVal
		return 1, nil

	case "as-path":
		if idx+1 >= len(args) {
			return 0, errMissingAsPathValue
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		asPath := make([]uint32, 0, len(tokens))
		for _, tok := range tokens {
			asn, err := strconv.ParseUint(tok, 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid ASN in as-path: %s", tok)
			}
			asPath = append(asPath, uint32(asn))
		}
		attrs.ASPath = asPath
		return consumed, nil

	case kwOriginAS:
		// origin-as <asn>: originate as a virtual router with this AS. Unlike the
		// verbatim as-path, the reactor applies the normal export rule ([asn] to
		// iBGP, [localAS, asn] to eBGP). Carried as a batch directive, not a wire
		// attribute, so the reactor synthesizes the AS_PATH per peer.
		if idx+1 >= len(args) {
			return 0, errMissingOriginASValue
		}
		n, err := strconv.ParseUint(args[idx+1], 10, 32)
		if err != nil || n == 0 {
			return 0, fmt.Errorf("invalid origin-as %q: expected 1..4294967295", args[idx+1])
		}
		attrs.OriginAS = uint32(n) //nolint:gosec // bounded by ParseUint bitSize=32
		return 1, nil

	case kwCommunity:
		if idx+1 >= len(args) {
			return 0, errMissingCommunityValue
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		communities := make([]uint32, 0, len(tokens))
		for _, tok := range tokens {
			c, err := parseCommunityText(tok)
			if err != nil {
				return 0, err
			}
			communities = append(communities, c)
		}
		attrs.Communities = communities
		return consumed, nil

	case kwLargeCommunity:
		if idx+1 >= len(args) {
			return 0, errMissingLargeCommunityValue
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		lcs := make([]bgptypes.LargeCommunity, 0, len(tokens))
		for _, tok := range tokens {
			lc, err := attribute.ParseLargeCommunity(tok)
			if err != nil {
				return 0, err
			}
			lcs = append(lcs, lc)
		}
		attrs.LargeCommunities = lcs
		return consumed, nil

	case kwExtendedCommunity:
		if idx+1 >= len(args) {
			return 0, errMissingExtendedCommunityValue
		}
		// Use route.ParseExtendedCommunities which handles both function syntax
		// (traffic-rate, discard, redirect, traffic-marking) and list syntax.
		ecs, consumed, err := route.ParseExtendedCommunities(args[idx+1:])
		if err != nil {
			return 0, err
		}
		attrs.ExtendedCommunities = ecs
		return consumed, nil

	case textparse.KWAggregator:
		if idx+1 >= len(args) {
			return 0, errMissingAggregatorValue
		}
		asn, addr, err := parseAggregatorText(args[idx+1])
		if err != nil {
			return 0, err
		}
		attrs.Aggregator = &attribute.Aggregator{ASN: asn, Address: addr}
		return 1, nil

	case textparse.KWOriginatorID:
		if idx+1 >= len(args) {
			return 0, errMissingOriginatorIDValue
		}
		addr, err := parseRouterIDText(args[idx+1])
		if err != nil {
			return 0, fmt.Errorf("invalid originator-id: %w", err)
		}
		attrs.OriginatorID = addr
		return 1, nil

	case textparse.KWClusterList:
		if idx+1 >= len(args) {
			return 0, errMissingClusterListValue
		}
		tokens, consumed := parseBracketedListText(args[idx+1:])
		// RFC 4456 Section 8: CLUSTER_LIST "is a sequence of CLUSTER_ID values
		// representing the reflection path that the route has passed". An empty
		// sequence names no reflector, so it is refused rather than encoded as a
		// zero-length attribute the operator did not ask for.
		if len(tokens) == 0 {
			return 0, errEmptyClusterList
		}
		ids := make([]uint32, 0, len(tokens))
		for _, tok := range tokens {
			id, err := parseClusterIDText(tok)
			if err != nil {
				return 0, err
			}
			ids = append(ids, id)
		}
		attrs.ClusterList = ids
		return consumed, nil

	case textparse.KWAIGP:
		if idx+1 >= len(args) {
			return 0, errMissingAIGPValue
		}
		// RFC 7311 Section 3: the AIGP TLV carries "Length: 11" with an 8-octet
		// "Value: Accumulated IGP Metric", so the metric is an unsigned 64-bit
		// integer and any wider or non-numeric token is refused here.
		metric, err := strconv.ParseUint(args[idx+1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid aigp %q: expected 0..18446744073709551615", args[idx+1])
		}
		attrs.AIGP = &metric
		return 1, nil

	case textparse.KWPrefixSIDSRv6:
		if idx+1 >= len(args) {
			return 0, errMissingPrefixSIDValue
		}
		// ExaBGP writes the value in parentheses:
		// `bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0] )`.
		// The tokens are rejoined and handed to the SAME encoder the config file
		// reaches, so both spellings produce one attribute (prefixsid.go).
		value, consumed, err := route.ParseParenthesizedValue(args[idx+1:])
		if err != nil {
			return 0, fmt.Errorf("invalid bgp-prefix-sid-srv6: %w", err)
		}
		tlvs, err := attribute.ParsePrefixSIDSRv6(value)
		if err != nil {
			return 0, err
		}
		attrs.PrefixSID = tlvs
		return consumed, nil
	}

	return 0, nil
}

// parseAggregatorText parses the "<asn>:<ip>" AGGREGATOR value form.
//
// RFC 4271 Section 5.1.7: "A BGP speaker that performs route aggregation MAY add
// the AGGREGATOR attribute, which SHALL contain its own AS number and IP
// address." RFC 4271 Section 4.3 g) fixes the layout: "the last AS number that
// formed the aggregate route (encoded as 2 octets), followed by the IP address of
// the BGP speaker that formed the aggregate route (encoded as 4 octets)". Ze
// writes the RFC 6793 four-octet AS form, and narrows it to AS_TRANS for a peer
// that did not negotiate four-octet AS numbers (attribute.Aggregator).
func parseAggregatorText(s string) (uint32, netip.Addr, error) {
	asnText, addrText, found := strings.Cut(s, ":")
	if !found {
		return 0, netip.Addr{}, fmt.Errorf("invalid aggregator %q: expected <asn>:<ip>", s)
	}

	asn, err := strconv.ParseUint(asnText, 10, 32)
	if err != nil {
		return 0, netip.Addr{}, fmt.Errorf("invalid aggregator ASN %q: expected 1..4294967295", asnText)
	}
	// RFC 7607 Section 2: "A BGP speaker MUST NOT originate or propagate a route
	// with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or
	// AS4_AGGREGATOR attributes."
	if asn == 0 {
		return 0, netip.Addr{}, errors.New("invalid aggregator ASN 0: RFC 7607 forbids originating AS 0")
	}

	addr, err := parseRouterIDText(addrText)
	if err != nil {
		return 0, netip.Addr{}, fmt.Errorf("invalid aggregator address: %w", err)
	}
	return uint32(asn), addr, nil //nolint:gosec // G115: bounded by ParseUint bitSize=32
}

// parseClusterIDText parses one CLUSTER_ID as dotted-decimal "a.b.c.d".
//
// RFC 4456 Section 7: "all RRs in the same cluster can be configured with a
// 4-byte CLUSTER_ID so that an RR can discard routes from other RRs in the same
// cluster." Four octets is what the attribute carries, and the dotted form is the
// one Ze renders (attribute.ClusterList.AppendText), so it is the one it reads.
func parseClusterIDText(s string) (uint32, error) {
	addr, err := parseRouterIDText(s)
	if err != nil {
		return 0, fmt.Errorf("invalid cluster-list id: %w", err)
	}
	v4 := addr.As4()
	return binary.BigEndian.Uint32(v4[:]), nil
}

// parseRouterIDText parses a four-octet router identifier written as an IPv4
// address, and returns it in its IPv4 form.
//
// RFC 4456 Section 8: ORIGINATOR_ID "is 4 bytes long and it will be created by an
// RR in reflecting a route." The CLUSTER_ID (Section 7) and the AGGREGATOR
// address (RFC 4271 Section 4.3 g)) are the same four octets. An IPv6 address has
// nowhere to go in four octets, so it is refused here rather than truncated or
// zero-filled at the encoder.
func parseRouterIDText(s string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%q is not an address", s)
	}
	if !addr.Is4() && !addr.Is4In6() {
		return netip.Addr{}, fmt.Errorf("%q is not an IPv4 address (the field is 4 octets)", s)
	}
	return addr.Unmap(), nil
}

// parseOriginText parses origin string to value.
func parseOriginText(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "igp":
		return 0, nil
	case "egp":
		return 1, nil
	case "incomplete":
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid origin: %s (valid: igp, egp, incomplete)", s)
	}
}

// parseBracketedListText parses [ v1 v2 ] or v1,v2 or [ v1, v2 ] style lists.
// Returns tokens and consumed arg count.
func parseBracketedListText(args []string) ([]string, int) {
	if len(args) == 0 {
		return nil, 0
	}

	first := args[0]

	// Case 1: "[" as separate token
	if first == "[" {
		var tokens []string
		consumed := 1
		for i := 1; i < len(args); i++ {
			if args[i] == "]" {
				return tokens, i + 1
			}
			// Split by comma if present
			for tok := range strings.SplitSeq(args[i], ",") {
				tok = strings.TrimSpace(tok)
				if tok != "" {
					tokens = append(tokens, tok)
				}
			}
			consumed = i + 1
		}
		return tokens, consumed
	}

	// Case 2: "[value]" as single token (entire list in one arg)
	if strings.HasPrefix(first, "[") && strings.HasSuffix(first, "]") {
		inner := first[1 : len(first)-1]
		var tokens []string
		for tok := range strings.SplitSeq(inner, " ") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				tokens = append(tokens, tok)
			}
		}
		return tokens, 1
	}

	// Case 3: "[value" followed by more tokens then "value]" (brackets attached)
	if strings.HasPrefix(first, "[") {
		var tokens []string
		// First token without leading bracket
		firstVal := strings.TrimPrefix(first, "[")
		for tok := range strings.SplitSeq(firstVal, ",") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				tokens = append(tokens, tok)
			}
		}
		consumed := 1

		// Continue until we find closing bracket
		for i := 1; i < len(args); i++ {
			consumed = i + 1
			arg := args[i]
			if before, ok := strings.CutSuffix(arg, "]"); ok {
				// Last token - strip trailing bracket
				lastVal := before
				for tok := range strings.SplitSeq(lastVal, ",") {
					tok = strings.TrimSpace(tok)
					if tok != "" {
						tokens = append(tokens, tok)
					}
				}
				return tokens, consumed
			}
			// Middle tokens
			for tok := range strings.SplitSeq(arg, ",") {
				tok = strings.TrimSpace(tok)
				if tok != "" {
					tokens = append(tokens, tok)
				}
			}
		}
		return tokens, consumed
	}

	// Case 4: Single value or comma-separated list without brackets
	var tokens []string
	for tok := range strings.SplitSeq(first, ",") {
		tok = strings.TrimSpace(tok)
		if tok != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens, 1
}

// parseCommunityText parses a community token via the canonical parser:
// ASN:value, hex (0xNNNNNNNN), bare integer, or any IANA well-known name
// (no-export, no-advertise, no-export-subconfed, nopeer, blackhole, ...).
// Delegates to attribute.ParseCommunity so this "update text" replay path
// (used by watchdog/GR route replay, among others) stays in sync with the
// config-time YANG community leaf-list parser instead of duplicating a
// partial well-known-name table. Bug found by spec-as112-3: this function
// previously hardcoded only 3 of the ~15 registered well-known names, so a
// route configured with e.g. "community [ nopeer ]" (RFC 3765, ze-bgp-conf.yang:237)
// failed to re-parse when replayed through watchdog announce, silently
// dropping the route instead of announcing it with the configured community.
func parseCommunityText(s string) (uint32, error) {
	v, err := attribute.ParseCommunity(s)
	if err != nil {
		return 0, fmt.Errorf("invalid community format: %s (expected ASN:value or well-known name)", s)
	}
	return v, nil
}

// errAttrsAfterNLRI is returned when attributes appear after the first nlri section.
var errAttrsAfterNLRI = errors.New("attributes must precede all nlri sections")

// resolveAliases returns a copy of args with all keyword aliases resolved to canonical forms.
// Uses textparse.ResolveAlias so "next"→"next-hop", "pref"→"local-preference", etc.
func resolveAliases(args []string) []string {
	resolved := make([]string, len(args))
	for i, token := range args {
		resolved[i] = textparse.ResolveAlias(token)
	}
	return resolved
}

// ParseUpdateText parses the "update text" command format.
// Flat grammar: attributes are keyword-value pairs (no set/add/del).
// Attributes must all precede the nlri sections.
// All keyword aliases are resolved before parsing (next→next-hop, pref→local-preference, etc.).
func ParseUpdateText(args []string) (*bgptypes.UpdateTextResult, error) {
	args = resolveAliases(args)

	var attrs parsedAttrs
	var groups []bgptypes.NLRIGroup
	var eorFamilies []family.Family
	var watchdog string
	seenNLRI := false
	i := 0

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		switch token {
		case kwNLRI:
			seenNLRI = true
			wire, nh, nlriAcc := attrs.snapshot()
			result, err := parseNLRISection(args[i:], nlriAcc)
			if err != nil {
				return nil, err
			}

			// RFC 4724: EOR is signaled by valid family with empty announce/withdraw lists.
			if len(result.Announce) == 0 && len(result.Withdraw) == 0 && result.Family.AFI != 0 {
				eorFamilies = append(eorFamilies, result.Family)
			} else {
				groups = append(groups, bgptypes.NLRIGroup{
					Family:       result.Family,
					Announce:     result.Announce,
					Withdraw:     result.Withdraw,
					Wire:         wire,
					NextHop:      nh,
					WatchdogName: result.Watchdog,
					OriginAS:     attrs.OriginAS,
				})
				if result.Watchdog != "" {
					watchdog = result.Watchdog
				}
			}
			i += result.Consumed

		case kwWatchdog:
			if i+1 >= len(args) {
				return nil, errors.New("missing watchdog name")
			}
			watchdog = args[i+1]
			i += 2

		case textparse.KWNextHop:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parseNhopFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		case textparse.KWAtomicAggregate:
			// RFC 4271 Section 4.3 f): "ATOMIC_AGGREGATE is a well-known
			// discretionary attribute of length 0." It takes no value, so it
			// cannot go through parseCommonAttributeText, whose contract reads a
			// zero consumed-count as "keyword not handled".
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			attrs.AtomicAggregate = true
			i++

		case kwPathInfo:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parsePathInfoFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwRD:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parseRDFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwLabel:
			if seenNLRI {
				return nil, errAttrsAfterNLRI
			}
			consumed, err := parseLabelFlat(args[i:], &attrs)
			if err != nil {
				return nil, err
			}
			i += consumed

		default: // attribute keywords and unknown tokens
			if isAttributeKeyword(token) {
				if seenNLRI {
					return nil, errAttrsAfterNLRI
				}
				// Reject old set/add/del syntax with migration hint.
				if i+1 < len(args) && (args[i+1] == kwSet || args[i+1] == kwAdd || args[i+1] == kwDel) {
					return nil, fmt.Errorf("%s keyword removed for attributes; use: %s <value>", args[i+1], token)
				}
				// Flat attribute parsing: keyword + value(s).
				extra, err := parseCommonAttributeText(token, args, i, &attrs)
				if err != nil {
					return nil, err
				}
				if extra == 0 {
					return nil, fmt.Errorf("missing value for %s", token)
				}
				i += 1 + extra
				continue
			}
			return nil, fmt.Errorf("unexpected token '%s'; valid: origin, med, local-preference (pref), as-path (path), community (s-com), large-community (l-com), extended-community (x-com), atomic-aggregate, aggregator, originator-id, cluster-list, aigp, bgp-prefix-sid-srv6, next-hop (next), path-information (info), rd, label, nlri, watchdog", token)
		}
	}

	return &bgptypes.UpdateTextResult{Groups: groups, WatchdogName: watchdog, EORFamilies: eorFamilies}, nil
}

// parseNhopFlat parses next-hop <address|self> (flat, no set/del).
func parseNhopFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "next-hop" (alias-resolved)
	if len(args) < 2 {
		return 0, errors.New("next-hop requires a value (address or self)")
	}
	// Reject old set/del syntax.
	if args[1] == kwSet || args[1] == kwDel {
		return 0, errSetDelKeywordsRemovedUseNext
	}
	value := args[1]
	if value == kwSelf {
		accum.NextHopSelf = true
		accum.NextHop = netip.Addr{}
		return 2, nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return 0, fmt.Errorf("invalid next-hop: %w", err)
	}
	// RFC 2545 Section 3: the Network Address of Next Hop field carries the
	// GLOBAL IPv6 address of the next hop. Ze appends the link-local half itself,
	// from the session's link-local leaf, when the section's condition holds, so
	// a link-local offered here has no global address to follow.
	if err := attribute.ValidateGlobalNextHop(addr); err != nil {
		return 0, err
	}
	accum.NextHop = addr
	accum.NextHopSelf = false
	return 2, nil
}

// parseRDFlat parses rd <value> (flat, no set/del).
// RD format: ASN:NN or IP:NN (e.g., "65000:100" or "192.0.2.1:100").
func parseRDFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "rd"
	if len(args) < 2 {
		return 0, errors.New("rd requires a value (ASN:NN or IP:NN)")
	}
	if args[1] == kwSet || args[1] == kwDel {
		return 0, errSetDelKeywordsRemovedUseRd
	}
	rd, err := nlri.ParseRDString(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid rd: %w", err)
	}
	accum.RD = rd
	return 2, nil
}

// parsePathInfoFlat parses path-information <id> at the top level.
//
// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the Path
// Identifier field, which is of four octets", so the value is read as a 32-bit
// unsigned number and any wider or non-numeric token is refused by name.
//
// The keyword is accepted here AND inside an nlri section, as `rd` and `label`
// are. It used to be accepted only inside one, so a command that wrote all three
// in the same position had two of them read and the third refused as an unknown
// token (parseNLRISection).
func parsePathInfoFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "path-information" (alias-resolved from "info").
	if len(args) < 2 {
		return 0, errPathInfoRequiresValue
	}
	if args[1] == kwSet || args[1] == kwDel {
		return 0, errSetDelKeywordsRemovedUsePathInfo
	}
	id, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid path-information %q: expected 0..4294967295", args[1])
	}
	accum.PathID = uint32(id) //nolint:gosec // G115: bounded by ParseUint bitSize 32
	return 2, nil
}

// parseLabelFlat parses label <value> or label [ <value>... ] (flat, no set/del).
func parseLabelFlat(args []string, accum *parsedAttrs) (int, error) {
	// args[0] = "label"
	if len(args) < 2 {
		return 0, errLabelRequiresValue
	}
	if args[1] == kwSet || args[1] == kwDel {
		return 0, errSetDelKeywordsRemovedUseLabel
	}
	labels, consumed, err := parseLabelStack(args[1:])
	if err != nil {
		return 0, err
	}
	accum.Labels = labels
	return 1 + consumed, nil
}

// parseLabelStack reads the value of a `label` keyword: one bare number, or a
// bracketed list of them.
//
// An MPLS label stack is a LIST, so the command has to be able to say one. RFC
// 8277 Section 2 encodes "one or more Label fields" of three octets each and
// puts the Bottom of Stack bit on the last one; the encoder registered for the
// family writes those octets, and this parser only reads the values.
//
// One parser for both positions the keyword appears in, top level and inside an
// nlri section, so the two cannot accept different things.
func parseLabelStack(args []string) ([]uint32, int, error) {
	tokens, consumed := parseBracketedListText(args)
	if len(tokens) == 0 {
		return nil, 0, errLabelRequiresValue
	}

	labels := make([]uint32, 0, len(tokens))
	for _, token := range tokens {
		label, err := strconv.ParseUint(token, 10, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid label %q: expected 0..%d", token, labelMax)
		}
		if label > labelMax {
			return nil, 0, fmt.Errorf("label out of range (max %d): %d", labelMax, label)
		}
		labels = append(labels, uint32(label)) //nolint:gosec // G115: bounded by the check above
	}
	return labels, consumed, nil
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-update", Handler: handleUpdate, RequiresSelector: true},
	)
	pluginserver.RegisterProcessCleanup(ClearProcessCursors)
}

// handleUpdate dispatches update subcommands by encoding.
// Syntax: send bgp <selector> update <encoding> ...
func handleUpdate(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	if len(args) < 1 {
		return nil, errUsageSendUpdateEncoding
	}

	encoding := strings.ToLower(args[0])
	switch encoding {
	case "text":
		return handleUpdateText(ctx, args[1:])
	case "hex":
		return handleUpdateHex(ctx, args[1:])
	case "b64":
		return handleUpdateB64(ctx, args[1:])
	case "cursor":
		return handleUpdateCursor(ctx, args[1:])
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
}

// handleUpdateText handles: send bgp <selector> update text ...
// Parses the update text format and dispatches to reactor batch methods.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4724 Section 2: End-of-RIB marker.
func handleUpdateText(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	result, err := ParseUpdateText(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	if result.WatchdogName != "" {
		errMsg := "watchdog not yet implemented for update text"
		return &plugin.Response{Status: plugin.StatusError, Error: errMsg}, errors.New(errMsg)
	}

	// BGP-specific operations: EOR, announce, withdraw
	bgpReactor, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}

	// Handle EOR markers (RFC 4724)
	peerSelector := ctx.PeerSelector()
	sel := selector.ParseDefault(peerSelector)
	var eorSent int
	for _, fam := range result.EORFamilies {
		if err := bgpReactor.AnnounceEOR(sel, uint16(fam.AFI), uint8(fam.SAFI), ctx.Sender); err != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
		}
		eorSent++
	}

	// If only EOR (no NLRI groups), return early
	if len(result.Groups) == 0 {
		if eorSent > 0 {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: plugin.Map{
					"eor": eorSent,
				},
			}, nil
		}
		return &plugin.Response{
			Status: "warning",
			Data:   plugin.Map{"message": "no routes or EOR markers to send"},
		}, nil
	}

	resp, err := DispatchNLRIGroups(ctx, result.Groups)
	if err != nil {
		return resp, err
	}

	// Add EOR count to response if both were sent
	if eorSent > 0 {
		if respData, ok := resp.Data.(plugin.Map); ok {
			respData["eor"] = eorSent
		}
	}

	return resp, nil
}

// DispatchNLRIGroups sends NLRI groups to the reactor for announce/withdraw.
// Returns response with counts and any warnings, or error response.
func DispatchNLRIGroups(ctx *pluginserver.CommandContext, groups []bgptypes.NLRIGroup) (*plugin.Response, error) {
	bgpReactor, errResp, bgpErr := requireBGPReactor(ctx)
	if bgpErr != nil {
		return errResp, bgpErr
	}

	sel := selector.ParseDefault(ctx.PeerSelector())
	var announced, withdrawn int
	var warnings []string

	// RFC 9494: the LLGR readvertise producer stamps meta["stale"] on the
	// UpdateRoute RPC; carry it onto the announce batch so AnnounceNLRIBatch runs
	// the per-peer readvertise egress filters. Zero for every non-stale command.
	staleLevel := staleLevelFromMeta(ctx.Meta)

	// RFC 4271 Section 9.2 has this speaker withhold a route a peer already
	// holds, and each peer's Adj-RIB-Out is what answers that (adj_rib_out.go).
	// A RESEND exists to put those same routes back on the wire, so it says so
	// and the reactor sends them.
	replay := replayFromMeta(ctx.Meta)

	for _, group := range groups {
		if len(group.Announce) > 0 {
			batch := bgptypes.NLRIBatch{
				Family:   group.Family,
				NLRIs:    group.Announce,
				NextHop:  group.NextHop,
				Wire:     group.Wire,
				OriginAS: group.OriginAS,
				Stale:    staleLevel,
				Replay:   replay,
			}
			if err := bgpReactor.AnnounceNLRIBatch(sel, batch, ctx.Sender); err != nil {
				if errors.Is(err, route.ErrNoPeersAcceptedFamily) {
					warnings = append(warnings, fmt.Sprintf("announce %v: %s", group.Family, err))
					continue
				}
				return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
			}
			announced += len(group.Announce)
		}
		if len(group.Withdraw) > 0 {
			// The withdrawal carries what the operator wrote in front of `nlri`, the
			// same block the announce above carries. Dropping Wire and NextHop here
			// is how `send bgp <selector> update text <attributes> nlri <family> del
			// <nlri>` acknowledged attributes that never reached the wire: the
			// reactor received a batch that named none, so it could not tell the
			// command apart from one that asked for none (ai/rules/principles.md).
			// RFC 4760 Section 4 permits both shapes -- "An UPDATE message that
			// contains the MP_UNREACH_NLRI is not required to carry any other path
			// attributes" -- so the command is what decides which one is sent.
			batch := bgptypes.NLRIBatch{
				Family:  group.Family,
				NLRIs:   group.Withdraw,
				NextHop: group.NextHop,
				Wire:    group.Wire,
			}
			switch err := bgpReactor.WithdrawNLRIBatch(sel, batch, ctx.Sender); {
			case err == nil:
			case errors.Is(err, route.ErrWithdrawWithheld):
				// The named peers had been advertised nothing on this session,
				// so the withdrawal named no route to them (RFC 4271
				// Section 4.3). The command DID what it asked for, so it is
				// counted and answered `done`, and the peers it wrote nothing
				// to are named: a zero UPDATE count with a bare `done` and no
				// reason is the silent no-op this rail must never produce
				// (ai/rules/principles.md).
				warnings = append(warnings, fmt.Sprintf("withdraw %v: %s", group.Family, err))
			case errors.Is(err, route.ErrNoPeersAcceptedFamily):
				warnings = append(warnings, fmt.Sprintf("withdraw %v: %s", group.Family, err))
				continue
			default:
				return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
			}
			withdrawn += len(group.Withdraw)
		}
	}

	if announced == 0 && withdrawn == 0 {
		msg := "no routes to announce or withdraw"
		if len(warnings) > 0 {
			msg = textbuf.Join(warnings, "; ")
		}
		return &plugin.Response{
			Status: "warning",
			Data:   plugin.Map{"message": msg},
		}, nil
	}

	result := &plugin.RouteResult{
		Announced: uint32(announced), //nolint:gosec // bounded by NLRI count
		Withdrawn: uint32(withdrawn), //nolint:gosec // bounded by NLRI count
		Warnings:  warnings,
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   result,
	}, nil
}

// staleLevelFromMeta reads the LLGR stale level (RFC 9494) from route metadata.
// Returns 0 when meta is nil or carries no "stale" key. In-process (DirectBridge)
// the value arrives as the uint8 the RIB set; a forked plugin's JSON round-trip
// decodes numbers as float64, so both are accepted.
func staleLevelFromMeta(meta map[string]any) uint8 {
	if meta == nil {
		return 0
	}
	switch v := meta["stale"].(type) {
	case uint8:
		return v
	case int:
		if v < 0 || v > 255 {
			return 0
		}
		return uint8(v)
	case float64:
		if v < 0 || v > 255 {
			return 0
		}
		return uint8(v)
	default:
		return 0
	}
}

// replayFromMeta reports whether this command re-advertises what the peer was
// already sent, so the destination peer's Adj-RIB-Out must not suppress it.
//
// False when meta is nil or carries no "replay" key, which is every ordinary
// announce. In-process (DirectBridge) the value arrives as the bool the RIB set;
// a forked plugin's JSON round-trip decodes it as a bool too, so one case
// covers both.
func replayFromMeta(meta map[string]any) bool {
	if meta == nil {
		return false
	}
	replay, ok := meta["replay"].(bool)
	return ok && replay
}
