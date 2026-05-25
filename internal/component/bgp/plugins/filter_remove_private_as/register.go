package filter_remove_private_as

import (
	frpaschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_remove_private_as/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-remove-private-as",
		Description:  "Named AS-path action filter that removes RFC 6996 Private Use ASNs",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         frpaschema.ZeFilterRemovePrivateASYANG,
		FilterTypes:  []string{"remove-private-as"},
		RunEngine:    RunFilterRemovePrivateAS,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
