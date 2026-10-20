// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- OSPFv2 multicast groups
// RFC 2328 Appendix B / D.3: AllSPFRouters and AllDRouters.

package transport

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC 2328 Appendix A.1: OSPF packets use IP protocol number 89.
const Protocol = 89

// RFC 2328 Appendix B / D.3: 224.0.0.5 is AllSPFRouters and 224.0.0.6 is AllDRouters.
var (
	AllSPFRouters = netip.MustParseAddr("224.0.0.5")
	AllDRouters   = netip.MustParseAddr("224.0.0.6")
)

// PacketDestination is the IP destination of an OSPFv2 packet meant for one
// neighbor (a Database Description, a Link State Request, a retransmitted or
// requested Link State Update, a direct Link State Acknowledgment) on an
// interface of the given network type. neighbor is that neighbor's address,
// the source of its Hellos.
//
// RFC 2328 Section 8.1: "On physical point-to-point networks, the IP
// destination is always set to the address AllSPFRouters."
//
// The rule matters on an unnumbered link: the neighbor's address is the donor
// address of another router, which no route on this link names, so a unicast
// to it leaves by whatever route the kernel holds, or by none, and never
// reaches the neighbor. Every other network type, virtual links included,
// sends to the neighbor itself. An address that is not IPv4 is returned as
// given: OSPFv3 runs its own transport and its own rules.
func PacketDestination(network string, neighbor netip.Addr) netip.Addr {
	if !neighbor.Is4() {
		return neighbor
	}
	if network != types.NetworkPointToPoint {
		return neighbor
	}
	return AllSPFRouters
}
