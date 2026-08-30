package filter_remove_private_as

import (
	frpayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_remove_private_as/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// The BGP daemon appears twice under one spelling: as the config subtree this
// plugin reads, and as the plugin it depends on. Each meaning gets its name.
const (
	configRootBGP = "bgp"
	pluginNameBGP = "bgp"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-remove-private-as",
		Description:  "Named AS-path action filter that removes RFC 6996 Private Use ASNs",
		ConfigRoots:  []string{configRootBGP},
		Dependencies: []string{pluginNameBGP},
		YANG:         frpayang.ZeFilterRemovePrivateASYANG,
		FilterTypes:  []string{"remove-private-as"},
		RunEngine:    runFilterRemovePrivateAS,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
