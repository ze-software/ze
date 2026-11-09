// Design: docs/features/dns-resolver.md -- show system dns drops a consumer the engine removed
// Related: dns_source_withdraw_consumers.go -- the observers registered below

package fixture

func init() {
	// spec-home-cpe-23b review round 3 ISSUE 3: a DHCP server and an l2tp
	// pool the reload removes withdraw their selections from show system
	// dns. Each draft pairs its observer with plugin/show-system-dns-withdraw-trigger.
	Register("plugin/show-system-dns-withdraw-dhcp", observer02("show-system-dns-withdraw-dhcp-test", showSystemDNSWithdrawDHCP))
	Register("plugin/show-system-dns-withdraw-pool", observer02("show-system-dns-withdraw-pool-test", showSystemDNSWithdrawPool))
}
