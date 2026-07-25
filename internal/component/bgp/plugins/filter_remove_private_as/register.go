package filter_remove_private_as

import (
	frpayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_remove_private_as/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-remove-private-as",
		Description:  "Named AS-path action filter that removes RFC 6996 Private Use ASNs",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         frpayang.ZeFilterRemovePrivateASYANG,
		FilterTypes:  []string{"remove-private-as"},
		RunEngine:    RunFilterRemovePrivateAS,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
