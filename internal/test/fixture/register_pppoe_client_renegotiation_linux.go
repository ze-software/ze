//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- LCP renegotiation in the network phase
// Related: pppoe_client_renegotiation_linux.go -- the scenario registered below

package fixture

func init() {
	// The access concentrator is scripted on the far end of the .ci's
	// netns-link veth pair, inside the test's own namespace.
	Register("pppoe/pppoe-client-lcp-renegotiation",
		observer02("pppoe-client-renegotiation-test", pppoeClientRenegotiation))
}
