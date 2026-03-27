// Design: docs/architecture/core-design.md — policy filter chain
// Related: filter_chain.go — policy filter chain execution

package reactor

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// attrNameToCode maps filter text attribute names to wire codes.
var attrNameToCode = map[string]attribute.AttributeCode{
	"origin":             attribute.AttrOrigin,
	"as-path":            attribute.AttrASPath,
	"next-hop":           attribute.AttrNextHop,
	"med":                attribute.AttrMED,
	"local-preference":   attribute.AttrLocalPref,
	"atomic-aggregate":   attribute.AttrAtomicAggregate,
	"aggregator":         attribute.AttrAggregator,
	"community":          attribute.AttrCommunity,
	"originator-id":      attribute.AttrOriginatorID,
	"cluster-list":       attribute.AttrClusterList,
	"extended-community": attribute.AttrExtCommunity,
	"large-community":    attribute.AttrLargeCommunity,
}

// FormatAttrsForFilter formats selected attributes from wire into text for filter input.
// Only attributes named in `declared` are included. Returns space-separated "name value" pairs.
// If declared is empty, all parseable attributes are included.
func FormatAttrsForFilter(attrs *attribute.AttributesWire, declared []string) string {
	if attrs == nil {
		return ""
	}

	var parts []string

	if len(declared) == 0 {
		// No declared list: format all attributes.
		parts = formatAllAttrs(attrs)
	} else {
		// Only format declared attributes.
		for _, name := range declared {
			code, ok := attrNameToCode[name]
			if !ok {
				continue
			}
			parsed, err := attrs.Get(code)
			if err != nil || parsed == nil {
				continue
			}
			if text := formatSingleAttr(parsed); text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, " ")
}

// formatAllAttrs formats all known attributes from wire.
func formatAllAttrs(attrs *attribute.AttributesWire) []string {
	order := []string{
		"origin", "as-path", "next-hop", "med", "local-preference",
		policyAttrAtomicAggregate, "aggregator", "community", "originator-id",
		"cluster-list", "extended-community", "large-community",
	}
	var parts []string
	for _, name := range order {
		code := attrNameToCode[name]
		parsed, err := attrs.Get(code)
		if err != nil || parsed == nil {
			continue
		}
		if text := formatSingleAttr(parsed); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

// formatSingleAttr formats one attribute as "name value" text.
func formatSingleAttr(attr attribute.Attribute) string {
	switch a := attr.(type) {
	case *attribute.Origin:
		return fmt.Sprintf("origin %s", attribute.FormatOrigin(uint8(*a)))

	case *attribute.ASPath:
		var asns []uint32
		for _, seg := range a.Segments {
			asns = append(asns, seg.ASNs...)
		}
		if len(asns) == 0 {
			return ""
		}
		return fmt.Sprintf("as-path %s", attribute.FormatASPath(asns))

	case *attribute.NextHop:
		if !a.Addr.IsValid() {
			return ""
		}
		return fmt.Sprintf("next-hop %s", a.Addr.String())

	case *attribute.MED:
		return fmt.Sprintf("med %d", uint32(*a))

	case *attribute.LocalPref:
		return fmt.Sprintf("local-preference %d", uint32(*a))

	case *attribute.AtomicAggregate:
		return policyAttrAtomicAggregate

	case *attribute.Aggregator:
		return fmt.Sprintf("aggregator %d:%s", a.ASN, a.Address.String())

	case attribute.Communities:
		comms := make([]uint32, len(a))
		for i, c := range a {
			comms[i] = uint32(c)
		}
		return fmt.Sprintf("community %s", attribute.FormatCommunities(comms))

	case *attribute.ClusterList:
		if len(*a) == 0 {
			return ""
		}
		ids := make([]string, len(*a))
		for i, id := range *a {
			ids[i] = fmt.Sprintf("%d.%d.%d.%d", byte(id>>24), byte(id>>16), byte(id>>8), byte(id))
		}
		return fmt.Sprintf("cluster-list %s", strings.Join(ids, " "))

	case attribute.LargeCommunities:
		lcs := make([]attribute.LargeCommunity, len(a))
		copy(lcs, a)
		return fmt.Sprintf("large-community %s", attribute.FormatLargeCommunities(lcs))

	case attribute.ExtendedCommunities:
		ecs := make([]attribute.ExtendedCommunity, len(a))
		copy(ecs, a)
		return fmt.Sprintf("extended-community %s", attribute.FormatExtendedCommunities(ecs))
	}

	return ""
}
