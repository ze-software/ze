package filter_community_match

import (
	cmschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_community_match/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:         "bgp-filter-community-match",
		Description:  "Named community match filter (ordered entries, first match wins, accept/reject)",
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		YANG:         cmschema.ZeFilterCommunityMatchYANG,
		FilterTypes:  []string{"community-match"},
		RunEngine:    RunFilterCommunityMatch,
		CLIHandler:   func(_ []string) int { return 0 },
	})
}
