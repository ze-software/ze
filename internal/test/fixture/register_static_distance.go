// Design: docs/architecture/static-routes.md -- static routes reach the FIB through the Loc-RIB
// Related: static_distance_fixture.go -- the scenario the two distance drivers below share
// Related: static_named_table_fixture.go -- the named-table scenario registered below
// Related: static_kernel_fixture.go -- the kernel-reading scenarios registered below

package fixture

func init() {
	// The two drivers share one scenario and differ only in the winner they
	// expect, because the two configurations differ only in the distance they
	// declare. That is what makes them evidence: one number changes and the
	// prefix changes hands.
	Register("static/static-distance-beats-ebgp",
		observer02("static-distance-test", staticDistanceWinner02(protocolStatic)))
	Register("static/static-distance-loses-to-ebgp",
		observer02("static-distance-test", staticDistanceWinner02("bgp")))

	// The named-table driver is the boundary's other side: a route in a named
	// table never enters the Loc-RIB, so no distance arbitrates it.
	Register("static/static-named-table-unchanged-setup", staticNamedTableSetup)
	Register("static/static-named-table-unchanged",
		observer02("static-named-table-test", staticNamedTableUnchanged))

	// The kernel drivers read what the FIB plugin programmed, not only what the
	// system RIB chose: the distance pair repeats the arbitration and asserts the
	// one kernel entry it produces, and the next-hop pair asserts that weights
	// and an interface-only next-hop survive the route's trip through the
	// Loc-RIB.
	Register("static/static-kernel-setup", staticKernelSetup)
	Register("static/static-kernel-distance-static-wins",
		observer02("static-kernel-test", staticKernelDistanceWinner(protocolStatic, kernelStaticGateway)))
	Register("static/static-kernel-distance-bgp-wins",
		observer02("static-kernel-test", staticKernelDistanceWinner("bgp", kernelBGPGateway)))
	Register("static/static-kernel-distance-reload",
		observer02("static-kernel-test", staticKernelDistanceReload))
	Register("static/static-kernel-distance-route-override",
		observer02("static-kernel-test", staticKernelDistanceRouteOverride))
	Register("static/static-kernel-weighted-multipath",
		observer02("static-kernel-test", staticKernelWeightedMultipath))
	Register("static/static-kernel-interface-nexthop",
		observer02("static-kernel-test", staticKernelInterfaceNextHop))
}
