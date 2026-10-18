// Design: docs/architecture/testing/interop.md -- ze's DHCPv4 client against dnsmasq.
package bgp

func init() {
	specialCheckers[scenarioDHCPRenewNak] = checkDHCPRenewNak
	specialCheckers[scenarioDHCPRebind] = checkDHCPRebind
}
