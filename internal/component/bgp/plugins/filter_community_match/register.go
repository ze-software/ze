package filter_community_match

import (
	cmyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community_match/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-community-match",
		Description:  "Named community match filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         cmyang.ZeFilterCommunityMatchYANG,
		FilterTypes:  []string{"community-match"},
		RunEngine:    RunFilterCommunityMatch,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
