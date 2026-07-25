package filter_aspath_length

import (
	falyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_aspath_length/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-aspath-length",
		Description:  "Named AS-path length filter (accept/reject based on hop count)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         falyang.ZeFilterAsPathLengthYANG,
		FilterTypes:  []string{"as-path-length"},
		RunEngine:    RunFilterAsPathLength,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
