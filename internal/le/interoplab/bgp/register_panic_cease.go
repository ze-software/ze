// Design: docs/architecture/testing/interop.md -- a contained read-path panic, judged by FRR and BIRD.
// Related: test/interop/scenarios/bgp-panic-cease -- the ze_le daemon and its two peers.
package bgp

func init() {
	specialCheckers[scenarioPanicCease] = checkPanicCease
}
