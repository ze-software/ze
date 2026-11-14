// Design: docs/contributing/running-commands.md -- waiting for another session's job
// Overview: ui_fixture_le_job_admits_heavy_action.go -- the scenario this file names

package fixture

// test/ui/le-job-admits-heavy-action.ci runs the scenario as `le test fixture
// ui/le-job-admits-heavy-action`.
func init() {
	Register(leHeavyFixture, uiDriver(leJobAdmitsHeavyAction))
}
