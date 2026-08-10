package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/report"
)

// VALIDATES: AC-19 -- Increasing RX/TX error counters raise iface-errors warning.
// PREVENTS: Interface errors going unnoticed by operators.
func TestIfaceErrorCounters(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()
	resetErrorTracker()

	// First poll: baseline captured, no warning.
	findings := checkErrorsFromStats(map[string]InterfaceStats{
		"eth0": {RxErrors: 0, TxErrors: 0},
	})
	assert.Equal(t, 0, findings)

	// Second poll: errors increased.
	findings = checkErrorsFromStats(map[string]InterfaceStats{
		"eth0": {RxErrors: 5, TxErrors: 3},
	})
	assert.Equal(t, 1, findings)

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeIfaceErrors && w.Subject == "eth0" {
			found = true
			assert.Equal(t, uint64(5), w.Detail["rx_errors_delta"])
			assert.Equal(t, uint64(3), w.Detail["tx_errors_delta"])
		}
	}
	if !found {
		t.Fatal("iface-errors warning not raised when error counters increased")
	}
}

// VALIDATES: AC-19 -- Warning clears when errors stop increasing.
// PREVENTS: Stale warnings after transient error burst.
func TestIfaceErrorCountersCleared(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()
	resetErrorTracker()

	// Baseline.
	checkErrorsFromStats(map[string]InterfaceStats{
		"eth0": {RxErrors: 10, TxErrors: 5},
	})

	// Same counters (no increase).
	findings := checkErrorsFromStats(map[string]InterfaceStats{
		"eth0": {RxErrors: 10, TxErrors: 5},
	})
	assert.Equal(t, 0, findings)

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeIfaceErrors && w.Subject == "eth0" {
			t.Fatal("iface-errors warning should be cleared when errors stop increasing")
		}
	}
}
