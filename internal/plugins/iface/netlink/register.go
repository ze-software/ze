package ifacenetlink

import (
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// backendName is the name this backend registers under, which the iface
// backend leaf selects.
const backendName = "kernel"

// init registers compile-time values only, so a refusal (a duplicate backend
// or check name, a malformed check) is a Ze defect no config can reach.
func init() {
	if err := iface.RegisterBackend(backendName, newNetlinkBackend); err != nil {
		panic("BUG: ifacenetlink: backend registration failed: " + err.Error())
	}
	// The macvlan capability doctor check (doctor.go) travels with the kernel
	// backend, so removing this package removes the check.
	if err := diagnostic.RegisterDoctorCheck(macvlanDoctorCheck); err != nil {
		panic("BUG: ifacenetlink: doctor check registration failed: " + err.Error())
	}
}
