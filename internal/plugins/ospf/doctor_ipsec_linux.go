//go:build linux

// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- kernel XFRM readiness probe (Linux).
// RFC: rfc/short/rfc4552.md -- OSPFv3 IPsec needs CAP_NET_ADMIN + kernel XFRM.

package ospf

import "github.com/vishvananda/netlink"

// xfrmAvailable reports whether the kernel XFRM dataplane can be queried. Listing the
// SPD requires CAP_NET_ADMIN and a kernel with IPsec support, so a failure is exactly the
// "cannot install SA/policy" condition RFC 4552 IPsec would hit at interface-up.
func xfrmAvailable() bool {
	_, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	return err == nil
}
