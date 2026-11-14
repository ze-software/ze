// Design: docs/architecture/testing/interop.md -- a peer that stops reading, judged against a frozen BIRD.
// Related: check_peer_stops_reading.go -- the checker and its predicates.
package bgp

func init() {
	specialCheckers[peerStopsReadingScenario] = checkPeerStopsReading
}
