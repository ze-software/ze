// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: scenarios.go -- the runners these registrations name
// Related: validate_runtime.go -- the validators these registrations name

package siteterminaldemo

import "io"

// actionOnly adapts a runner that reads only its action.
func actionOnly(run func(action string) error) scenarioRunner {
	return func(action string, _ []string, _ io.Writer) error { return run(action) }
}

func init() {
	scenarios.add("cli-dashboard", scenario{run: actionOnly(runCLIDashboard), validate: validateCLIDashboard})
	scenarios.add(demoZefsConfig, scenario{run: actionOnly(runZeFSConfig), validate: validateZeFSConfig})
	scenarios.add("rbac", scenario{run: actionOnly(runRBAC), validate: validateRBAC})
	scenarios.add("traceroute", scenario{run: actionOnly(runTraceroute), validate: validateTraceroute})
	scenarios.add("web-config", scenario{run: actionOnly(runWebConfig), validate: validateWebConfig})
	scenarios.add("rib-fib", scenario{run: runRIBFIB, validate: validateRIBFIB})
	scenarios.add("health-reports", scenario{run: actionOnly(runHealthReports), validate: validateHealthReports})
	scenarios.add(demoConfigViews, scenario{run: actionOnly(runConfigViews), validate: validateConfigViews})
	scenarios.add(demoCommitConfirmed, scenario{run: actionOnly(runCommitConfirmed), validate: validateCommitConfirmed})
	// These three tapes start no lab: their validators drive ze directly.
	scenarios.add("launcher", scenario{validate: validateLauncher})
	scenarios.add("host-inventory", scenario{validate: validateHostInventory})
	scenarios.add("config-graph", scenario{validate: validateConfigGraph})
}
