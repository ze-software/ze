// Design: docs/architecture/route-selection.md -- real BGP next-hop admission
// Related: rfc4271_next_hop_daemon_fixture.go -- paired selecting-RIB placements
package fixture

func init() {
	Register("plugin/rfc4271-next-hop-kernel", rfc4271NextHopKernelDriver, StopsDaemon)
	Register("plugin/rfc4271-next-hop-kernel-empty", rfc4271NextHopKernelEmpty)
	Register("plugin/rfc4271-next-hop-link-cleanup", rfc4271NextHopLinkCleanup)
}
