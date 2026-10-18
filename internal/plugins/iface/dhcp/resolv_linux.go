// Design: docs/features/interfaces.md -- DNS resolver config from DHCP
// Overview: dhcp_linux.go -- DHCP client types and lifecycle

//go:build linux

package ifacedhcp

import (
	"github.com/ze-software/ze/internal/component/config/system"
)

// resolvers answers the resolv.conf writer this client's leases go to.
func (c *DHCPClient) resolvers() *system.Resolvers {
	if c.config.Resolvers == nil {
		return system.ProcessResolvers()
	}
	return c.config.Resolvers
}

// setV4Resolvers records the DNS servers the DHCPv4 lease now carries; an
// empty list withdraws them, when the lease ends or an ACK carries none.
//
// RFC 2132 Section 3.8: "The domain name server option specifies a list of
// Domain Name System (STD 13, RFC 1035 [8]) name servers available to the
// client." Each ACK's list replaces the previous one.
func (c *DHCPClient) setV4Resolvers(servers []string) {
	if err := c.resolvers().SetLease(system.DHCPv4ResolverSource(c.ifaceName, c.unit), servers); err != nil {
		loggerPtr.Load().Warn("iface dhcp v4: resolv.conf write failed", "iface", c.ifaceName, "err", err)
	}
}

// setV6Resolvers records the DNS servers the DHCPv6 lease now carries; an
// empty list withdraws them.
//
// RFC 3646 Section 3: "The DNS Recursive Name Server option provides a list
// of one or more IPv6 addresses of DNS recursive name servers to which a
// client's DNS resolver MAY send DNS queries [1]."
//
// RFC 9915 Section 18.2.10.5: "When a client receives a Reply for a Renew,
// Rebind, or Information-Request that does not include a requested
// configuration option that it previously received in a Reply, the client
// SHOULD stop using the previously received configuration information." The
// lease's list is empty after such a Reply, so the servers leave the file.
func (c *DHCPClient) setV6Resolvers(servers []string) {
	if err := c.resolvers().SetLease(system.DHCPv6ResolverSource(c.ifaceName, c.unit), servers); err != nil {
		loggerPtr.Load().Warn("iface dhcp v6: resolv.conf write failed", "iface", c.ifaceName, "err", err)
	}
}
