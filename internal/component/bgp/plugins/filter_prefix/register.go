package filter_prefix

import (
	fpyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-prefix",
		Description:  "Named prefix-list filter (CIDR + ge/le + accept/reject)",
		ConfigRoots:  []string{configRootBGP},
		Dependencies: []string{configRootBGP},
		YANG:         fpyang.ZeFilterPrefixYANG,
		FilterTypes:  []string{"prefix-list"},
		RunEngine:    RunFilterPrefix,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
