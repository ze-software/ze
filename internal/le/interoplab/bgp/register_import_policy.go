// Design: docs/architecture/testing/interop.md -- the IGP import-policy scenarios.
// Related: check_import_policy.go -- the checkers this file registers.
// Related: prepare_links.go -- scenarioLinks, which builds the two IS-IS links.
package bgp

func init() {
	specialCheckers[isisImportPolicyScenario] = checkISISImportPolicy
	specialCheckers[ospfImportPolicyScenario] = checkOSPFImportPolicy
	// FRR-A -- Ze -- FRR-B: Ze meets FRR-A on eth1 (VLAN 101) and FRR-B on
	// eth2 (VLAN 102); each FRR reaches its link as its own eth1. The FRRs
	// share no link, so FRR-B can learn FRR-A's prefixes only from LSPs Ze
	// floods on.
	scenarioLinks[isisImportPolicyScenario] = []labLink{
		{vlan: 101, ipv4: isisImportLinkA, members: []int{zeLinkLocalOwner, frrLinkLocalOwner}},
		{vlan: 102, ipv4: isisImportLinkB, members: []int{zeLinkLocalOwner, frrSinkLinkLocalOwner}},
	}
}
