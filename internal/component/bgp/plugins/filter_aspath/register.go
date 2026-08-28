package filter_aspath

import (
	fayang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_aspath/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-aspath",
		Description:  "Named AS-path regex filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{configRootBGP},
		Dependencies: []string{configRootBGP},
		YANG:         fayang.ZeFilterAsPathYANG,
		FilterTypes:  []string{"as-path-list"},
		RunEngine:    runFilterAsPath,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
