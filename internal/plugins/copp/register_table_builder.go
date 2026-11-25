// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- config-time firewall table builder for CoPP
// Related: translate.go -- translatePolicy builds the ze_copp table
// Related: register.go -- applyCoppPolicy installs the same translated table at apply time

package copp

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
)

// init registers CoPP's config table builder with the firewall component so
// firewall-producer-tables verifies the ze_copp table against the candidate
// firewall backend at commit, reload, boot and `ze config validate`.
func init() {
	if err := firewall.RegisterConfigTableBuilder("copp", buildCoppTablesFromTree); err != nil {
		panic("BUG: copp: firewall config table builder registration failed: " + err.Error())
	}
}

// buildCoppTablesFromTree extracts control-plane-protection and bgp from tree
// without mutating tree and returns the exact ze_copp table translatePolicy
// produces for applyCoppPolicy. Returns (nil, nil) when control-plane-protection
// or its bgp container is absent or inactive.
//
// Safe for concurrent use.
func buildCoppTablesFromTree(tree *config.Tree) ([]firewall.Table, error) {
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- a nil tree holds no CoPP configuration
	if tree == nil {
		return nil, nil
	}
	cppTree := tree.GetContainer(configRoot)
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- absent control-plane-protection configures no CoPP table
	if cppTree == nil {
		return nil, nil
	}
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- inactive control-plane-protection configures no CoPP table
	if cppTree.IsInactive() {
		return nil, nil
	}
	bgpTree := cppTree.GetContainer("bgp")
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- absent bgp container under control-plane-protection configures no CoPP table
	if bgpTree == nil {
		return nil, nil
	}
	// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- inactive bgp container configures no CoPP table
	if bgpTree.IsInactive() {
		return nil, nil
	}

	sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRoot, bgpConfigRoot})
	if err != nil {
		return nil, err
	}
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		policy, found, err := parseCoppSections(sections, section.Data)
		if err != nil {
			return nil, err
		}
		// refuse-by-default: guards firewall.RegisterConfigTableBuilder -- a control-plane-protection section without an active bgp body produces no CoPP table
		if !found {
			return nil, nil
		}
		return []firewall.Table{translatePolicy(policy)}, nil
	}
	return nil, nil
}
