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

// TestBaseline_SubtractsRxMulticast verifies the rx-multicast counter is
// baseline-adjusted alongside the others, so `clear interface counters` gives a
// consistent since-clear view across every field.
//
// VALIDATES: rx-multicast (added for sFlow if_counters) participates in the
// clear-counters baseline like every other InterfaceStats field.
// PREVENTS: regression where a field is added to InterfaceStats but applyBaseline
// is not updated, leaving that field showing since-boot totals while the rest
// show since-clear deltas.
func TestBaseline_SubtractsRxMulticast(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{
		"eth0": {RxBytes: 100, RxPackets: 10, RxMulticast: 7, TxBytes: 200},
	}}
	raw := &InterfaceStats{RxBytes: 150, RxPackets: 15, RxMulticast: 12, TxBytes: 300}
	store.applyBaseline("eth0", raw)
	assert.Equal(t, uint64(5), raw.RxMulticast, "rx-multicast must be baseline-adjusted")
	assert.Equal(t, uint64(50), raw.RxBytes)
}

// TestBaseline_WrapOnRxMulticast verifies a multicast-only counter regression
// (raw rx-multicast below baseline) is treated as a kernel reset and rebases,
// matching the other fields in wrapped().
func TestBaseline_WrapOnRxMulticast(t *testing.T) {
	store := &baselineStore{data: map[string]InterfaceStats{
		"eth0": {RxBytes: 100, RxMulticast: 50},
	}}
	// Only rx-multicast regressed (driver reset a subset of counters).
	raw := &InterfaceStats{RxBytes: 150, RxMulticast: 3}
	store.applyBaseline("eth0", raw)
	// Wrap detected -> raw returned unchanged, baseline dropped.
	assert.Equal(t, uint64(150), raw.RxBytes)
	assert.Equal(t, uint64(3), raw.RxMulticast)
	store.mu.RLock()
	_, present := store.data["eth0"]
	store.mu.RUnlock()
	assert.False(t, present, "rx-multicast regression should drop the baseline")
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
