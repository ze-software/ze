// Design: docs/architecture/testing/interop.md -- the IGP export-policy scenarios.
// Related: check_export_policy.go -- the checkers this file registers.
package bgp

func init() {
	specialCheckers[isisExportPolicyScenario] = checkISISExportPolicy
	specialCheckers[ospfExportPolicyScenario] = checkOSPFExportPolicy
}
