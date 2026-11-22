// Design: docs/research/vpp-deployment-reference.md -- VPP interface backend registration
// Overview: ifacevpp.go -- VPP Backend implementation

package ifacevpp

import (
	"github.com/ze-software/ze/internal/component/iface"
)

// backendVPP is the name this plugin answers to: the value of the `interface
// backend` config leaf, the health check name, and the Component every doctor
// check reports under.
const backendVPP = "vpp"

// init registers compile-time values only, so a refusal (a duplicate backend
// or check name, a malformed check) is a Ze defect no config can reach.
func init() {
	if err := iface.RegisterBackend(backendVPP, newVPPBackend); err != nil {
		panic("BUG: iface-vpp: backend registration failed: " + err.Error())
	}
	RegisterHealthCheck()
	if err := registerDoctorChecks(); err != nil {
		panic("BUG: iface-vpp: doctor check registration failed: " + err.Error())
	}
}
