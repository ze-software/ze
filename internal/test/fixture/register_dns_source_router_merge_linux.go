//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: dns_source_router_merge_linux.go -- the scenario registered below
// Related: dns_source_received_interface_linux.go -- the scenario registered below

package fixture

func init() {
	// The access concentrator is scripted on the far end of the .ci's
	// netns-link veth pair, inside the test's own namespace.
	Register("pppoe/dns-source-router-merge",
		observer02("dns-source-router-merge-test", dnsSourceRouterMerge), StopsDaemon)
	// The daemon creates both veth pairs; the fixture advertises from their
	// far ends, in the guest's own namespace.
	Register("plugin/dns-source-received-interface",
		observer02("dns-source-received-interface-test", dnsSourceReceivedInterface), StopsDaemon)
}
