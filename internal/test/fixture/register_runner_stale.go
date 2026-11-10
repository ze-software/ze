// Design: docs/architecture/testing/runner-architecture.md -- which binaries a fixture reaches.

package fixture

func init() {
	Register("runner/stale-runner-notice", staleRunnerNotice)
}
