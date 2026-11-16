// Design: docs/architecture/testing/ci-format.md -- registered compiled observers.
package fixture

func init() {
	Register("plugin/classic-mixed-next-hop", classicMixedNextHopDriver, StopsDaemon)
}
