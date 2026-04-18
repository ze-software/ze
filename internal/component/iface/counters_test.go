package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBaseline_SubtractsWhenSet verifies applyBaseline returns the
// delta between raw and baseline, not the raw value.
//
// VALIDATES: baseline subtraction produces the "since last clear"
// view of counters.
// PREVENTS: regression where applyBaseline becomes a no-op and
// `clear interface counters` silently has no effect on reads.
func TestBaseline_SubtractsWhenSet(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{
		"eth0": {RxBytes: 100, RxPackets: 10, TxBytes: 200, TxPackets: 20},
	}}
	raw := &InterfaceStats{RxBytes: 150, RxPackets: 15, TxBytes: 300, TxPackets: 30}
	store.applyBaseline("eth0", raw)
	assert.Equal(t, uint64(50), raw.RxBytes)
	assert.Equal(t, uint64(5), raw.RxPackets)
	assert.Equal(t, uint64(100), raw.TxBytes)
	assert.Equal(t, uint64(10), raw.TxPackets)
}

// TestBaseline_NoopWhenMissing verifies applyBaseline leaves the
// stats untouched when no baseline is stored for the interface.
//
// VALIDATES: counters display raw kernel values when the operator
// has not issued `clear interface counters` since boot.
func TestBaseline_NoopWhenMissing(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{}}
	raw := &InterfaceStats{RxBytes: 100, TxBytes: 200}
	store.applyBaseline("eth0", raw)
	assert.Equal(t, uint64(100), raw.RxBytes)
	assert.Equal(t, uint64(200), raw.TxBytes)
}

// TestBaseline_WrapRebasesToZero verifies that when the raw counter
// drops below the baseline (kernel-level reset: interface bounce,
// driver reload, delete+recreate), applyBaseline detects the
// monotonicity violation, drops the baseline, and returns the raw
// value so subsequent reads see sane deltas from the kernel's new
// zero.
//
// VALIDATES: the rebase-on-wrap behavior requested as part of the
// clear-verb design -- operators never see negative or "rewound"
// deltas after a kernel reset.
// PREVENTS: regression where the baseline outlives a kernel counter
// reset and every subsequent read underflows or returns garbage.
func TestBaseline_WrapRebasesToZero(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{
		"eth0": {RxBytes: 1000, RxPackets: 100, TxBytes: 2000, TxPackets: 200},
	}}
	// Kernel has been reset: current raw is way below baseline.
	raw := &InterfaceStats{RxBytes: 5, RxPackets: 1}
	store.applyBaseline("eth0", raw)
	// Raw returned unchanged -- the operator now sees "since kernel reset".
	assert.Equal(t, uint64(5), raw.RxBytes)
	assert.Equal(t, uint64(1), raw.RxPackets)
	// Baseline was dropped.
	store.mu.RLock()
	_, present := store.data["eth0"]
	store.mu.RUnlock()
	assert.False(t, present, "wrap should have dropped the baseline")

	// Subsequent reads continue from the kernel's new zero without
	// any further adjustment -- a re-read at raw=10 returns 10, not
	// 5 (from before) or some delta.
	raw2 := &InterfaceStats{RxBytes: 10}
	store.applyBaseline("eth0", raw2)
	assert.Equal(t, uint64(10), raw2.RxBytes)
}

// TestBaseline_SetAndClear verifies setBaseline stores a snapshot and
// nil drops it.
func TestBaseline_SetAndClear(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{}}
	store.setBaseline("eth0", &InterfaceStats{RxBytes: 42})
	store.mu.RLock()
	v, ok := store.data["eth0"]
	store.mu.RUnlock()
	assert.True(t, ok)
	assert.Equal(t, uint64(42), v.RxBytes)

	store.setBaseline("eth0", nil)
	store.mu.RLock()
	_, present := store.data["eth0"]
	store.mu.RUnlock()
	assert.False(t, present, "nil stats should drop the baseline")
}

// TestBaseline_ClearAll verifies clearAllBaselines empties the map.
func TestBaseline_ClearAll(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{
		"eth0": {RxBytes: 1},
		"eth1": {RxBytes: 2},
	}}
	store.clearAllBaselines()
	store.mu.RLock()
	n := len(store.data)
	store.mu.RUnlock()
	assert.Zero(t, n)
}
