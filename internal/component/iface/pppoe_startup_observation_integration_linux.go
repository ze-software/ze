//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- startup cardinality proof.
// Related: register.go -- the actual OnConfigure producer.
// Related: pppoe_startup_observation_integration_linux_test.go -- read-only export.

package iface

import "sync/atomic"

// pppoeStartupCounts is an immutable observation of one startup delivery.
// Zero counts are valid observations; a nil pointer means no delivery was observed.
type pppoeStartupCounts struct {
	parsed int
	active int
}

// Each scenario runs in a fresh userns child. Retain the first delivery so a
// later startup cannot hide a bad count. Only observePPPoEStartup writes this.
var pppoeStartupObservation atomic.Pointer[pppoeStartupCounts]

// observePPPoEStartup MUST be called from OnConfigure while pppoeMu is held,
// after reconciliation, with the parsed slice length saved before reconciliation.
// It receives no mutable config or client references and returns no decision.
// Publication and reads are safe for concurrent use.
func observePPPoEStartup(parsed, active int) {
	pppoeStartupObservation.CompareAndSwap(nil, &pppoeStartupCounts{
		parsed: parsed,
		active: active,
	})
}
