// Design: docs/architecture/testing/interop.md -- the IGP import-policy scenarios.
// Related: check_import_policy.go -- the checkers this file registers.
// Related: prepare.go -- linkDeclarations, which hands these links to the core.
package bgp

import "github.com/ze-software/ze/internal/le/interoplab"

func init() {
	specialCheckers[isisImportPolicyScenario] = checkISISImportPolicy
	specialCheckers[ospfImportPolicyScenario] = checkOSPFImportPolicy
	// FRR-A -- Ze -- FRR-B: Ze meets FRR-A on eth1 (VLAN 101) and FRR-B on
	// eth2 (VLAN 102); each FRR reaches its link as its own eth1. The FRRs
	// share no link, so FRR-B can learn FRR-A's prefixes only from LSPs Ze
	// floods on.
	linkDeclarations[isisImportPolicyScenario] = []interoplab.Link{
		{Name: "frr-a", VLAN: 101, IPv4: isisImportLinkA, Members: []string{"ze", peerFRR}},
		{Name: "frr-b", VLAN: 102, IPv4: isisImportLinkB, Members: []string{"ze", peerFRRSink}},
	}
}
