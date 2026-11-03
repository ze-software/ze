//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- startup cardinality proof.
// Related: pppoe_config_integration_linux_test.go -- external scenario fixture.

package iface

// PPPoEStartupCountsForTest reads counts from the actual registered delivery.
// The caller MUST use a fresh userns child and wait for server startup before
// asserting counts. A pre-start read checks that no stale observation exists.
// This accessor neither parses nor reconciles nor resets the observation.
// Safe for concurrent use; observed=false means delivery has not been observed.
func PPPoEStartupCountsForTest() (parsed, active int, observed bool) {
	snapshot := pppoeStartupObservation.Load()
	if snapshot == nil {
		return 0, 0, false
	}
	return snapshot.parsed, snapshot.active, true
}
