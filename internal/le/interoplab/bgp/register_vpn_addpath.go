// Design: docs/architecture/testing/interop.md -- discovered native interop scenario.
// Related: check_vpn_addpath.go -- the checker this file registers.
package bgp

func init() {
	specialCheckers[vpnAddPathScenario] = checkVPNAddPathFRR
}
