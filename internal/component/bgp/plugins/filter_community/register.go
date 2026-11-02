package filter_community

import (
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	fcyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func init() {
	// Register AttrModHandlers for community attribute codes (progressive
	// build path).
	filterapi.RegisterAttrModHandler(byte(attribute.AttrCommunity), communityAttrModHandler)
	filterapi.RegisterAttrModHandler(byte(attribute.AttrLargeCommunity), largeCommunityAttrModHandler)
	filterapi.RegisterAttrModHandler(byte(attribute.AttrExtCommunity), extCommunityAttrModHandler)

	// JSON formatters for community attributes.
	attribute.RegisterJSONFormatter(attribute.AttrCommunity, "communities", appendCommunitiesJSON)
	attribute.RegisterJSONFormatter(attribute.AttrLargeCommunity, "large-communities", appendLargeCommunitiesJSON)
	attribute.RegisterJSONFormatter(attribute.AttrExtCommunity, "extended-communities", appendExtCommunitiesJSON)
	attribute.RegisterJSONFormatter(attribute.AttrIPv6ExtCommunity, "ipv6-extended-communities", appendIPv6ExtCommunitiesJSON)

	// The members' view readers (community_filter.go), resolved once the
	// session facts are registered.
	resolveCommunityReaders()

	reg := registry.Registration{
		Name:         "bgp-filter-community",
		Description:  "Community tag/strip filter (standard, large, extended)",
		ConfigRoots:  []string{configRootBGP},
		Dependencies: []string{configRootBGP},
		YANG:         fcyang.ZeFilterCommunityYANG,
		RunEngine:    runFilterCommunity,
		// The members an operator names in a peer's chain and the RFC 7999
		// Section 3.1 agreement check inserted on bilateral eBGP export,
		// served by evaluateCommunityFilter (community_filter.go).
		Filters:          communityFilterDecls(),
		FilterInsertions: communityInsertions(),
		// The counters of the members (metrics.go).
		ConfigureMetrics: setMetricsRegistry,
	}
	// The forked form (`ze plugin bgp-filter-community`) runs the engine over
	// TLS connect-back, as every external plugin does. A handler that returns
	// at once leaves the daemon waiting on a connect-back that never comes.
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return fcyang.ZeFilterCommunityYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error(reg.Name+": registration failed", "error", err)
		os.Exit(1)
	}
}
