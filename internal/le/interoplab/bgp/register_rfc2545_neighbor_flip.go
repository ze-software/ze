// Design: docs/architecture/testing/interop.md -- real neighbor-watch condition changes.
// Related: check_rfc2545_neighbor_flip.go -- the checker registered here.
package bgp

func init() {
	specialCheckers[rfc2545NeighborFlipScenario] = checkRFC2545NeighborConditionFlip
}
