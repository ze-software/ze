package filter_modify

import (
	fmschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_modify/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-modify",
		Description:  "Named route attribute modifier (set local-preference, med, origin, next-hop)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         fmschema.ZeFilterModifyYANG,
		FilterTypes:  []string{"modify"},
		RunEngine:    RunFilterModify,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
