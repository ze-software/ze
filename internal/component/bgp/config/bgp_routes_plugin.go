// Design: docs/architecture/update-building.md -- generic attributes of a plugin family's config route
// Overview: bgp_routes.go -- update block extraction that calls the family's config route parser

package bgpconfig

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// errPluginRouteASSet refuses a generic AS_PATH that holds a set. The send path
// of a plugin family builds AS_PATH itself and drops any other, so without the
// refusal the route would go out with a path the operator did not write.
var errPluginRouteASSet = errors.New("the generic AS_PATH attribute holds an AS_SET or AS_CONFED_SET, and RFC 9774 Section 3 says BGP speakers \"MUST NOT advertise BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs\": write the path as an as-path sequence instead")

// configRouteGeneric is the generic half of a plugin family's update block: the
// path attributes the central dispatcher encodes for every family, so no family
// plugin can leave one out. Session-independent attributes are pre-built here;
// AS_PATH, LOCAL_PREF and AGGREGATOR depend on the session and stay typed until
// BuildPlugin runs.
type configRouteGeneric struct {
	attrs              []PluginRouteAttrConfig
	pathID             PathID
	asPath             []uint32
	localPreference    uint32
	hasLocalPreference bool
	aggregator         Aggregator
}

// buildConfigRouteRequest pre-parses an update block's attribute{} tree with the
// shared static-route attribute parsers. It answers the request the family
// plugin reads (NLRI tokens, next hop, extended communities) and the generic
// attributes the dispatcher encodes itself. RD is NOT included: it is carried
// inline with the NLRI content tokens. fam is the family of the nlri entry the
// attributes serve; the Prefix-SID is judged against it (RFC 9252 Section 7).
// The configured path-information rides in the generic half: the reactor, not
// the plugin parser, writes it, and only on a session with ADD-PATH send for
// the family (RFC 7911 Section 3).
func buildConfigRouteRequest(attr *config.Tree, content []string, isIPv6 bool, fam family.Family) (registry.ConfigRouteRequest, configRouteGeneric, error) {
	var sr StaticRouteConfig
	if err := applyAttributesFromTree(attr, &sr); err != nil {
		return registry.ConfigRouteRequest{}, configRouteGeneric{}, err
	}
	parsed, err := ParseRouteAttributes(&sr)
	if err != nil {
		return registry.ConfigRouteRequest{}, configRouteGeneric{}, err
	}
	// RFC 9252 Section 7: no transposition where the route family has no label field.
	if err := attribute.CheckSRv6TranspositionForFamily(parsed.PrefixSID, fam); err != nil {
		return registry.ConfigRouteRequest{}, configRouteGeneric{}, err
	}
	attrs, err := genericPluginRouteAttrs(parsed)
	if err != nil {
		return registry.ConfigRouteRequest{}, configRouteGeneric{}, err
	}

	nextHop, _ := attr.Get("next-hop")
	request := registry.ConfigRouteRequest{
		Content:      content,
		NextHop:      nextHop,
		IsIPv6:       isIPv6,
		ExtCommunity: parsed.ExtendedCommunity.Bytes,
	}
	generic := configRouteGeneric{
		attrs:              attrs,
		pathID:             parsed.PathID,
		asPath:             parsed.ASPath.Values,
		localPreference:    parsed.LocalPreference,
		hasLocalPreference: parsed.HasLocalPreference,
		aggregator:         parsed.Aggregator,
	}
	return request, generic, nil
}

// genericPluginRouteAttrs encodes the session-independent generic attributes of
// an update block with the attribute types BuildUnicast appends for a static
// route, so a plugin family's route carries them byte for byte as a unicast one
// would. Communities are sorted, as BuildUnicast sorts them. The AIGP attribute
// is built as the static path builds it (reactor appendAIGPRaw); the session
// writer removes it where AIGP_SESSION is disabled (RFC 7311 Section 3.3), on
// both paths.
func genericPluginRouteAttrs(parsed *ParsedRouteAttributes) ([]PluginRouteAttrConfig, error) {
	// RFC 4271 Section 5.1.1: ORIGIN is always present, IGP unless configured.
	attrs := []PluginRouteAttrConfig{typedPluginAttr(attribute.Origin(parsed.Origin))}
	// RFC 4271 Section 5.1.4: a configured MED of 0 is a metric too.
	if parsed.HasMED {
		attrs = append(attrs, typedPluginAttr(attribute.MED(parsed.MED)))
	}
	// RFC 4271 Section 5.1.6.
	if parsed.AtomicAggregate {
		attrs = append(attrs, typedPluginAttr(attribute.AtomicAggregate{}))
	}
	// RFC 1997.
	if len(parsed.Community.Values) > 0 {
		communities := make(attribute.Communities, len(parsed.Community.Values))
		for i, value := range parsed.Community.Values {
			communities[i] = attribute.Community(value)
		}
		slices.Sort(communities)
		attrs = append(attrs, typedPluginAttr(communities))
	}
	// RFC 4456 Section 8.
	if parsed.OriginatorID != 0 {
		id := parsed.OriginatorID
		address := netip.AddrFrom4([4]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)})
		attrs = append(attrs, typedPluginAttr(attribute.OriginatorID(address)))
	}
	// RFC 4456 Section 8.
	if len(parsed.ClusterList) > 0 {
		attrs = append(attrs, typedPluginAttr(attribute.ClusterList(slices.Clone(parsed.ClusterList))))
	}
	// RFC 5701 Section 2.
	if len(parsed.ExtendedCommunity.IPv6Bytes) > 0 {
		attrs = append(attrs, PluginRouteAttrConfig{
			Code:  uint8(attribute.AttrIPv6ExtCommunity),
			Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive),
			Value: parsed.ExtendedCommunity.IPv6Bytes,
		})
	}
	// RFC 8669 Section 3; the per-peer send rule (Section 8) is toPluginParams'.
	if len(parsed.PrefixSID) > 0 {
		attrs = append(attrs, PluginRouteAttrConfig{
			Code:  uint8(attribute.AttrPrefixSID),
			Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive),
			Value: parsed.PrefixSID,
		})
	}
	// RFC 8092 Section 3.
	if len(parsed.LargeCommunity.Values) > 0 {
		large := make(attribute.LargeCommunities, len(parsed.LargeCommunity.Values))
		for i, value := range parsed.LargeCommunity.Values {
			large[i] = attribute.LargeCommunity{GlobalAdmin: value[0], LocalData1: value[1], LocalData2: value[2]}
		}
		attrs = append(attrs, typedPluginAttr(large))
	}
	// RFC 7311 Section 3.
	if parsed.AIGPMetric != nil {
		value := make([]byte, attribute.AIGPWireLen)
		attribute.WriteAIGPMetric(value, 0, *parsed.AIGPMetric)
		attrs = append(attrs, PluginRouteAttrConfig{Code: uint8(attribute.AttrAIGP), Flags: uint8(attribute.FlagOptional), Value: value})
	}
	for _, raw := range parsed.RawAttributes {
		// RFC 9774 Section 3. ParseRouteAttributes keeps a generic AS_PATH raw
		// only when it holds a set.
		if attribute.AttributeCode(raw.Code) == attribute.AttrASPath {
			return nil, errPluginRouteASSet
		}
		attrs = append(attrs, PluginRouteAttrConfig{Code: raw.Code, Flags: raw.Flags, Value: raw.Value})
	}
	return attrs, nil
}

// typedPluginAttr encodes one attribute value with its own encoder.
func typedPluginAttr(attr attribute.Attribute) PluginRouteAttrConfig {
	value := make([]byte, attr.Len()) // config load: once per route
	attr.WriteTo(value, 0)
	return PluginRouteAttrConfig{Code: uint8(attr.Code()), Flags: uint8(attr.Flags()), Value: value}
}

// pluginRouteFromRegistry joins the family plugin's route with the generic
// attributes the dispatcher encoded. It refuses the two ways a configured
// attribute could still miss the wire or reach it twice: extended communities
// the family did not emit, and an attribute both halves wrote.
func pluginRouteFromRegistry(famName string, req registry.ConfigRouteRequest, pr registry.PluginRoute, generic configRouteGeneric) (PluginRouteConfig, error) {
	if len(req.ExtCommunity) > 0 && !pluginRouteCarries(pr.Attrs, attribute.AttrExtCommunity) {
		return PluginRouteConfig{}, fmt.Errorf("extended-community is configured, but a %s route does not carry extended communities, so the route would be sent without them: remove extended-community from this update's attribute block", famName)
	}
	attrs := make([]PluginRouteAttrConfig, 0, len(generic.attrs)+len(pr.Attrs))
	attrs = append(attrs, generic.attrs...)
	for i := range pr.Attrs {
		code := attribute.AttributeCode(pr.Attrs[i].Code)
		// RFC 4271 Section 6.3: an attribute given twice is a Malformed Attribute List.
		if slices.ContainsFunc(generic.attrs, func(a PluginRouteAttrConfig) bool { return a.Code == pr.Attrs[i].Code }) {
			return PluginRouteConfig{}, fmt.Errorf("attribute 0x%02x (%s) is written by the attribute block and by the %s route itself, and RFC 4271 Section 6.3 makes an attribute given twice a Malformed Attribute List: remove it from the attribute block", pr.Attrs[i].Code, code, famName)
		}
		attrs = append(attrs, PluginRouteAttrConfig{Code: pr.Attrs[i].Code, Flags: pr.Attrs[i].Flags, Value: pr.Attrs[i].Value})
	}
	return PluginRouteConfig{
		Family:             famName,
		IsIPv6:             pr.IsIPv6,
		NLRI:               pr.NLRI,
		NextHop:            pr.NextHop,
		Attrs:              attrs,
		PathID:             uint32(generic.pathID),
		ASPath:             generic.asPath,
		LocalPreference:    generic.localPreference,
		HasLocalPreference: generic.hasLocalPreference,
		HasAggregator:      generic.aggregator.Valid,
		AggregatorASN:      generic.aggregator.ASN,
		AggregatorIP:       generic.aggregator.IP,
		Group:              pr.Group,
		MapV4NextHop:       pr.MapV4NextHop,
	}, nil
}

// pluginRouteCarries reports whether a plugin's attributes hold code.
func pluginRouteCarries(attrs []registry.PluginRouteAttr, code attribute.AttributeCode) bool {
	return slices.ContainsFunc(attrs, func(a registry.PluginRouteAttr) bool { return attribute.AttributeCode(a.Code) == code })
}
