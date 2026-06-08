package filter_aspath

import (
	fayang "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_aspath/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-aspath",
		Description:  "Named AS-path regex filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         fayang.ZeFilterAsPathYANG,
		FilterTypes:  []string{"as-path-list"},
		RunEngine:    RunFilterAsPath,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
