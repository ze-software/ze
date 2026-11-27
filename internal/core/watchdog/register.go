// Design: docs/architecture/doctor-and-health-checks.md -- watchdog doctor code registration
// Related: doctor_codes.go -- CodeDoctorDaemonStallDetected metadata and event code

package watchdog

func init() {
	registerDoctorCodes()
}
