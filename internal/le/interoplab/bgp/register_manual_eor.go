// Design: docs/architecture/testing/interop.md -- requested End-of-RIB on a manual-eor peer.
// Related: check_manual_eor.go -- the checker and the requesting process.
package bgp

func init() {
	registerProcessHelper(manualEORScenario, runManualEORProcess)
	specialCheckers[manualEORScenario] = checkManualEORRequested
}
