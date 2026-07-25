package filter_community

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	fcyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func init() {
	// Register AttrModHandlers for community attribute codes (progressive build path).
	filterapi.RegisterAttrModHandler(byte(attribute.AttrCommunity), communityAttrModHandler)
	filterapi.RegisterAttrModHandler(byte(attribute.AttrLargeCommunity), largeCommunityAttrModHandler)
	filterapi.RegisterAttrModHandler(byte(attribute.AttrExtCommunity), extCommunityAttrModHandler)

	// JSON formatters for community attributes.
	attribute.RegisterJSONFormatter(attribute.AttrCommunity, "communities", appendCommunitiesJSON)
	attribute.RegisterJSONFormatter(attribute.AttrLargeCommunity, "large-communities", appendLargeCommunitiesJSON)
	attribute.RegisterJSONFormatter(attribute.AttrExtCommunity, "extended-communities", appendExtCommunitiesJSON)
	attribute.RegisterJSONFormatter(attribute.AttrIPv6ExtCommunity, "ipv6-extended-communities", appendIPv6ExtCommunitiesJSON)

	// Route filter pipeline contribution (BGP-owned seam, not the generic registry).
	if err := filterapi.Register(filterapi.Filter{
		Name:     "bgp-filter-community",
		Stage:    filterapi.FilterStagePolicy,
		Priority: 0,
		Ingress:  ingressFilter,
		Egress:   egressFilter,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "bgp-filter-community: filter registration failed: %v\n", err)
		os.Exit(1)
	}

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-community",
		Description:  "Community tag/strip filter (standard, large, extended)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         fcyang.ZeFilterCommunityYANG,
		RunEngine:    RunFilterCommunity,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
