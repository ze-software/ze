package pool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMetricsAccuracy verifies metrics reflect actual pool state.
//
// VALIDATES: Observability correctness.
//
// PREVENTS: Misleading metrics causing incorrect capacity planning
// or missed memory issues in production.
func TestMetricsAccuracy(t *testing.T) {
	p := New(1024)

	h1 := p.Intern([]byte("AAAA"))
	h2 := p.Intern([]byte("BBBB"))
	_ = h1
	_ = p.Release(h2)

	m := p.Metrics()

	require.Equal(t, int32(2), m.TotalSlots)
	require.Equal(t, int32(1), m.LiveSlots)
	require.Equal(t, int32(1), m.DeadSlots)
	require.Equal(t, int64(4), m.LiveBytes)
	require.Equal(t, int64(4), m.DeadBytes)
}

// TestMetricsDeduplicationRate verifies dedup rate calculation.
//
// VALIDATES: Deduplication effectiveness metric.
//
// PREVENTS: Incorrect efficiency reporting, missing optimization opportunities.
func TestMetricsDeduplicationRate(t *testing.T) {
	p := New(1024)

	// 3 interns, 2 hits (same data)
	p.Intern([]byte("data"))
	p.Intern([]byte("data"))
	p.Intern([]byte("data"))

	m := p.Metrics()

	require.Equal(t, int64(3), m.InternTotal)
	require.Equal(t, int64(2), m.InternHits)
	require.InDelta(t, 0.666, m.DeduplicationRate(), 0.01)
}

// TestMetricsAfterCompaction verifies metrics update after compaction.
//
// VALIDATES: Metrics consistency after maintenance operations.
//
// PREVENTS: Stale metrics after compaction.
func TestMetricsAfterCompaction(t *testing.T) {
	p := New(1024)

	h := p.Intern([]byte("to-be-released"))
	p.Intern([]byte("keep-alive"))
	_ = p.Release(h)

	before := p.Metrics()
	require.Equal(t, int32(1), before.DeadSlots)

	p.Compact()

	after := p.Metrics()
	require.Equal(t, int32(0), after.DeadSlots)
	require.Equal(t, int32(1), after.LiveSlots)
}

// TestMetricsBufferSize verifies buffer size reporting.
//
// VALIDATES: Memory usage tracking.
//
// PREVENTS: Memory leaks going undetected.
func TestMetricsBufferSize(t *testing.T) {
	p := New(1024)

	before := p.Metrics()
	require.Equal(t, int64(0), before.BufferSize)

	p.Intern([]byte("some data here"))

	after := p.Metrics()
	require.Greater(t, after.BufferSize, int64(0))
}

// TestMetricsZeroDeduplicationRate verifies rate with no duplicates.
//
// VALIDATES: Edge case - all unique entries.
//
// PREVENTS: Division by zero or incorrect calculation.
func TestMetricsZeroDeduplicationRate(t *testing.T) {
	p := New(1024)

	p.Intern([]byte("unique1"))
	p.Intern([]byte("unique2"))
	p.Intern([]byte("unique3"))

	m := p.Metrics()

	require.Equal(t, int64(3), m.InternTotal)
	require.Equal(t, int64(0), m.InternHits)
	require.Equal(t, float64(0), m.DeduplicationRate())
}

// TestMetricsEmptyPool verifies metrics for empty pool.
//
// VALIDATES: Edge case - empty pool.
//
// PREVENTS: Panic or incorrect values for empty pool.
func TestMetricsEmptyPool(t *testing.T) {
	p := New(1024)

	m := p.Metrics()

	require.Equal(t, int32(0), m.TotalSlots)
	require.Equal(t, int32(0), m.LiveSlots)
	require.Equal(t, int32(0), m.DeadSlots)
	require.Equal(t, int64(0), m.InternTotal)
	require.Equal(t, float64(0), m.DeduplicationRate())
}
