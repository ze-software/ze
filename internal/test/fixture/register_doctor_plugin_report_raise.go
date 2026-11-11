// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon
// Related: doctor_plugin_report_raise.go -- doctorPluginReportRaise, the observer registered below

package fixture

func init() {
	// test/plugin/doctor-plugin-report-raise-external.ci: a forked plugin
	// raises a warning through report-issue, show doctor reports it under its
	// code with its fix, an unregistered code is refused, and a clear removes it.
	Register("plugin/doctor-plugin-report-raise", observeDriver(reportRaisePlugin, doctorPluginReportRaise), StopsDaemon)
}
