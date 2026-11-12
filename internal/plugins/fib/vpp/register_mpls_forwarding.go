// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` on the VPP data plane
// Related: mplsforwarding.go -- the reader this registers
// Related: register.go -- declares "vpp" as this plugin's Registration.DataPlane

package fibvpp

import "github.com/ze-software/ze/internal/core/mplsfib"

// vppDataPlane is the data plane this package writes and reads, the name
// `interface { backend }` gives VPP and register.go declares as DataPlane.
const vppDataPlane = "vpp"

// init registers the reader `show mpls forwarding` answers from when the
// config selects VPP. It runs in every process that links this package, and
// the reader reads VPP itself, so it serves the daemon's command handler
// whether the fib-vpp writer runs in the daemon or forked.
func init() {
	if err := mplsfib.RegisterForwardingReader(vppDataPlane, readMPLSForwarding); err != nil {
		panic("BUG: fib-vpp: MPLS forwarding reader registration refused: " + err.Error())
	}
}
