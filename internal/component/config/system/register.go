// Design: docs/features/ai-first.md -- doctor check registration for the system component
// Related: doctor.go -- the four checks registered here
//
// The system component is not a plugin, so it registers through the component
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field. Keeping the checks and their registration under
// internal/component/config/system means removing the component removes its
// diagnostics with it.

package system

import "github.com/ze-software/ze/internal/component/config"

func init() {
	registerSystemDoctorChecks()
	// The host name check runs wherever a config is validated (ze config
	// validate, a commit, a reload), before an applier sets the kernel name.
	if err := config.RegisterTreeValidator("system-host-name", validateSystemHostTree); err != nil {
		panic("BUG: system host name validator registration refused: " + err.Error())
	}
}
