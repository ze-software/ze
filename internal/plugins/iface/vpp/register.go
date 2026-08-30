// Design: docs/research/vpp-deployment-reference.md -- VPP interface backend registration
// Overview: ifacevpp.go -- VPP Backend implementation

package ifacevpp

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/iface"
)

// backendVPP is the name this plugin answers to: the value of the `interface
// backend` config leaf, the health check name, and the Component every doctor
// check reports under.
const backendVPP = "vpp"

func init() {
	if err := iface.RegisterBackend(backendVPP, newVPPBackend); err != nil {
		fmt.Fprintf(os.Stderr, "iface-vpp: backend registration failed: %v\n", err)
		os.Exit(1)
	}
	RegisterHealthCheck()
	if err := registerDoctorChecks(); err != nil {
		fmt.Fprintf(os.Stderr, "iface-vpp: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}
