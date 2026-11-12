// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` CLI (AC-7)
// Related: show_forwarding.go -- the handler this registers
// Related: forwarding_linux.go -- the kernel reader this registers

package mpls

import (
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/mplsfib"
)

// kernelDataPlane is the name `interface { backend }` gives the Linux kernel,
// the data plane whose AF_MPLS table this package reads. fib-kernel declares
// the same name as its Registration.DataPlane.
const kernelDataPlane = "netlink"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-mpls:show-forwarding",
			Handler:    handleShowMPLSForwarding,
		},
	)
	if err := mplsfib.RegisterForwardingReader(kernelDataPlane, dumpMPLSRoutes); err != nil {
		panic("BUG: mpls: kernel forwarding reader registration refused: " + err.Error())
	}
}
