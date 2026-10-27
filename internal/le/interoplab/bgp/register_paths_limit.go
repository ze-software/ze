// Design: docs/architecture/testing/interop.md -- the PATHS-LIMIT scenarios that need a second daemon.
package bgp

func init() {
	specialCheckers[pathsLimitReceiveScenario] = checkPathsLimitReceive
	specialCheckers[pathsLimitBestPathScenario] = checkPathsLimitBestPath
}
