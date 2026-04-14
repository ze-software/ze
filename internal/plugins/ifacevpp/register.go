// Design: docs/research/vpp-deployment-reference.md -- VPP interface backend registration
// Overview: ifacevpp.go -- VPP Backend implementation

package ifacevpp

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

func init() {
	if err := iface.RegisterBackend("vpp", newVPPBackend); err != nil {
		fmt.Fprintf(os.Stderr, "iface-vpp: backend registration failed: %v\n", err)
		os.Exit(1)
	}
}
