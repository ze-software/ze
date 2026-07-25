package filter_family

import (
	ffyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_family/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-family",
		Description:  "Named address-family policy filter: remove a family's NLRI or tear down the session",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         ffyang.ZeFilterFamilyYANG,
		FilterTypes:  []string{"family-filter"},
		RunEngine:    RunFilterFamily,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
