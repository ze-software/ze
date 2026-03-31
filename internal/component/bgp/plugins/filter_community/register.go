package filter_community

import (
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	fcschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_community/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	// Register AttrModHandlers for community attribute codes (progressive build path).
	registry.RegisterAttrModHandler(byte(attribute.AttrCommunity), communityAttrModHandler)
	registry.RegisterAttrModHandler(byte(attribute.AttrLargeCommunity), largeCommunityAttrModHandler)
	registry.RegisterAttrModHandler(byte(attribute.AttrExtCommunity), extCommunityAttrModHandler)

	_ = registry.Register(registry.Registration{
		Name:           "bgp-filter-community",
		Description:    "Community tag/strip filter (standard, large, extended)",
		ConfigRoots:    []string{"bgp"},
		YANG:           fcschema.ZeFilterCommunityYANG,
		RunEngine:      RunFilterCommunity,
		CLIHandler:     func(_ []string) int { return 0 },
		IngressFilter:  ingressFilter,
		EgressFilter:   egressFilter,
		FilterStage:    registry.FilterStagePolicy,
		FilterPriority: 0,
	})
}
