// Design: docs/architecture/core-design.md -- connected prefixes in the shared Loc-RIB
// Related: connected_distance_fixture.go -- the scenario the two drivers below share

package fixture

func init() {
	// The two drivers differ in the winner they expect AND in whether Ze is
	// supposed to have programmed the prefix, because those are the two halves
	// of what a connected win means: it takes the prefix, and it takes it by Ze
	// installing nothing.
	Register("plugin/connected-distance-arbitration",
		observer02("connected-distance-test", connectedDistanceWinner02("connected", false)))
	Register("plugin/connected-distance-raised-loses",
		observer02("connected-distance-test", connectedDistanceWinner02("bgp", true)))
}
