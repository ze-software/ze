package filter_prefix

import (
	fpyang "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_prefix/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-prefix",
		Description:  "Named prefix-list filter (CIDR + ge/le + accept/reject)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         fpyang.ZeFilterPrefixYANG,
		FilterTypes:  []string{"prefix-list"},
		RunEngine:    RunFilterPrefix,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
