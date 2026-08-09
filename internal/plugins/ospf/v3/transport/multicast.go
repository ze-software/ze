// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- OSPFv3 IPv6 multicast groups
// RFC: rfc/short/rfc5340.md (§2.9 AllSPFRouters ff02::5 / AllDRouters ff02::6)

package transport

import "net/netip"

// Protocol is the IP protocol number for OSPF. RFC 5340 §2.9 runs OSPFv3 directly
// over IPv6 with Next Header 89 (same protocol number as OSPFv2 over IPv4).
const Protocol = 89

// RFC 5340 §2.9: ff02::5 is AllSPFRouters and ff02::6 is AllDRouters -- the IPv6
// link-local-scope multicast equivalents of OSPFv2's 224.0.0.5 / 224.0.0.6. All
// OSPFv3 routers join AllSPFRouters on every enabled interface; only the DR and
// BDR join AllDRouters.
var (
	AllSPFRouters = netip.MustParseAddr("ff02::5")
	AllDRouters   = netip.MustParseAddr("ff02::6")
)
