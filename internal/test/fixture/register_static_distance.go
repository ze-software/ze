// Design: docs/architecture/static-routes.md -- static routes reach the FIB through the Loc-RIB
// Related: static_distance_fixture.go -- the scenario the two drivers below share

package fixture

func init() {
	// The two drivers share one scenario and differ only in the winner they
	// expect, because the two configurations differ only in the distance they
	// declare. That is what makes them evidence: one number changes and the
	// prefix changes hands.
	Register("static/static-distance-beats-ebgp",
		observer02("static-distance-test", staticDistanceWinner02("static")))
	Register("static/static-distance-loses-to-ebgp",
		observer02("static-distance-test", staticDistanceWinner02("bgp")))
}
