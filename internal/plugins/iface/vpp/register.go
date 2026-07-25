// Design: docs/research/vpp-deployment-reference.md -- VPP interface backend registration
// Overview: ifacevpp.go -- VPP Backend implementation

package ifacevpp

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/iface"
)

func init() {
	if err := iface.RegisterBackend("vpp", newVPPBackend); err != nil {
		fmt.Fprintf(os.Stderr, "iface-vpp: backend registration failed: %v\n", err)
		os.Exit(1)
	}
	RegisterHealthCheck()
	if err := registerDoctorChecks(); err != nil {
		fmt.Fprintf(os.Stderr, "iface-vpp: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}
