package filter_remove_private_as

import (
	frpayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_remove_private_as/yang"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
)

// The BGP daemon appears twice under one spelling: as the config subtree this
// plugin reads, and as the plugin it depends on. Each meaning gets its name.
const (
	configRootBGP = "bgp"
	pluginNameBGP = "bgp"
)

// filterTypeName is the ze:filter list this plugin owns under the policy root.
const filterTypeName = "remove-private-as"

func init() {
	// The route fields the filter type reads and sets, against which a chain
	// at an attachment point is checked (spec "Fields each filter type reads
	// and sets").
	policy.MustRegisterFilterType(filterTypeName,
		[]policy.Field{policy.FieldASPath},
		[]policy.Field{policy.FieldASPath})
	// The plugin answers the remove-private intent over the route view, which
	// only the library chain applies, so a chain naming the type compiles to
	// it (D-12).
	policychain.MustRegisterViewFilter(filterTypeName)

	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-remove-private-as",
		Description:  "Named AS-path action filter that removes RFC 6996 Private Use ASNs",
		ConfigRoots:  []string{configRootBGP},
		ConfigReads:  []string{policy.RootName},
		Dependencies: []string{pluginNameBGP, policy.PluginName},
		YANG:         frpayang.ZeFilterRemovePrivateASYANG,
		FilterTypes:  []string{filterTypeName},
		RunEngine:    runFilterRemovePrivateAS,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
