package filter_community

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/filterapi"
	fcyang "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_community/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	// Register AttrModHandlers for community attribute codes (progressive build path).
	filterapi.RegisterAttrModHandler(byte(attribute.AttrCommunity), communityAttrModHandler)
	filterapi.RegisterAttrModHandler(byte(attribute.AttrLargeCommunity), largeCommunityAttrModHandler)
	filterapi.RegisterAttrModHandler(byte(attribute.AttrExtCommunity), extCommunityAttrModHandler)

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
