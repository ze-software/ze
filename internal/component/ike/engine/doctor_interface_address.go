// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- peers waiting for their interface address
// Related: dynamic_wan.go -- planPeers, which decides which peers wait, and publishes them here
// Related: doctor_virtual_ip.go -- the sibling check this one is shaped after

package engine

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// interfaceWait is the set of peers the running engine holds back because the interface
// they take their local address from has none, as the last plan published it.
type interfaceWait struct {
	iface string
	peers []string
}

// interfaceWaitPtr is the doctor's view of the running engine's waiting peers. The apply
// path and the address worker write it under the engine's apply lock (publishWaiting); the
// doctor reads it from another goroutine, so it is atomic. Nil means no engine has applied
// a configuration, and the check then reports nothing.
var interfaceWaitPtr atomic.Pointer[interfaceWait]

// checkIPsecInterfaceAddress warns once for each peer that waits for its interface
// address, naming the peer, the interface and the two fixes.
//
// A waiting peer is not a fault in the configuration: a CPE boots before its PPPoE WAN is
// up, and the peer starts by itself when the address appears. It is reported because the
// tunnel is down while it lasts, and nothing else says why.
func checkIPsecInterfaceAddress(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	wait := interfaceWaitPtr.Load()
	if wait == nil {
		return nil
	}
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(wait.peers))
	for _, peer := range wait.peers {
		var tb textbuf.Buffer
		tb.Str("ipsec peer ").Str(peer).Str(" is waiting for an IPv4 address on interface ").Str(wait.iface).
			Str(": it has no local-address of its own, so it has no address to send from and is not started. " +
				"It starts when the address appears. Bring the interface up, or give the peer an address with: " +
				"set vpn ipsec site-to-site peer ").Str(peer).Str(" local-address <ipv4>")
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecInterfaceAddress,
			Severity: severityWarning,
			Message:  tb.String(),
		})
	}
	return out
}
