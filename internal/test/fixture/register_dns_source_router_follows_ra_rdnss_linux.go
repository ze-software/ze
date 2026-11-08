//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: dns_source_router_follows_ra_rdnss_linux.go -- the scenario registered below

package fixture

func init() {
	// The daemon creates the veth pair from the .ci config; the fixture
	// advertises on its far end and creates the stub's dummy link.
	Register("plugin/dns-source-router-follows-ra-rdnss",
		observer02("dns-source-router-follows-ra-rdnss-test", dnsSourceRouterFollowsRARDNSS))
}
