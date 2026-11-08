//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: dns_source_router_follows_lease_linux.go -- the scenario registered below

package fixture

func init() {
	// The access concentrator is scripted on the far end of the .ci's
	// netns-link veth pair, inside the test's own namespace.
	Register("pppoe/dns-source-router-follows-lease",
		observer02("dns-source-router-follows-lease-test", dnsSourceRouterFollowsLease))
}
