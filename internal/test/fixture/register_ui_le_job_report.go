// Design: docs/contributing/running-commands.md -- waiting for another session's job
// Overview: ui_fixture_le_job_report.go -- the scenario this file names

package fixture

// test/ui/le-job-report.ci runs the scenario as `le test fixture
// ui/le-job-report`, and the scenario runs itself again under that name as
// the command of the two jobs it starts.
func init() {
	Register(leJobReportFixture, uiDriver(leJobReport))
}
