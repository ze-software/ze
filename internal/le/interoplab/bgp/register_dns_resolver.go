// Design: docs/architecture/testing/interop.md -- ze's LAN DNS resolver against Unbound, Knot DNS and dhcpcd.
package bgp

func init() {
	specialCheckers[scenarioDNSForwardUnbound] = checkDNSForwardUnbound
	specialCheckers[scenarioDNSDHCPLAN] = checkDNSDHCPLAN
}
