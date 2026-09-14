// Design: docs/features/ai-first.md -- doctor check registration for the system component
// Related: doctor.go -- the four checks registered here
//
// The system component is not a plugin, so it registers through the component
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field. Keeping the checks and their registration under
// internal/component/config/system means removing the component removes its
// diagnostics with it.

package system

func init() {
	registerSystemDoctorChecks()
}
