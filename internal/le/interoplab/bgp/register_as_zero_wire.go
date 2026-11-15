// Design: docs/architecture/testing/interop.md -- actual raw-write AS-zero test.
package bgp

func init() {
	specialCheckers[asZeroRawScenario] = checkASZeroRawFRR
}
