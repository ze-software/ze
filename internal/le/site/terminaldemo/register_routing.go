// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: scenarios_routing.go -- runRPKI, runIRR

package siteterminaldemo

func init() {
	scenarios.add(demoRPKI, scenario{run: actionOnly(runRPKI), validate: validateRPKI})
	scenarios.add("irr-filter", scenario{run: actionOnly(runIRR), validate: validateIRR})
}
