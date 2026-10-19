// Design: docs/architecture/testing/interop.md -- ze's DHCPv4 client against dnsmasq, its DHCPv6 client against Kea.
package bgp

func init() {
	specialCheckers[scenarioDHCPRenewNak] = checkDHCPRenewNak
	specialCheckers[scenarioDHCPRebind] = checkDHCPRebind
	specialCheckers[scenarioDHCPv6RenewKea] = checkDHCPv6RenewKea
}
