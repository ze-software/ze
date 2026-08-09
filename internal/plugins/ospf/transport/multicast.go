// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- OSPFv2 multicast groups
// RFC 2328 Appendix B / D.3: AllSPFRouters and AllDRouters.

package transport

import "net/netip"

// RFC 2328 Appendix A.1: OSPF packets use IP protocol number 89.
const Protocol = 89

// RFC 2328 Appendix B / D.3: 224.0.0.5 is AllSPFRouters and 224.0.0.6 is AllDRouters.
var (
	AllSPFRouters = netip.MustParseAddr("224.0.0.5")
	AllDRouters   = netip.MustParseAddr("224.0.0.6")
)
