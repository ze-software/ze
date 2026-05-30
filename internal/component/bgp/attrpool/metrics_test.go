package attrpool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMetricsAggregateAcrossShards verifies Metrics() sums per-shard counters so
// the aggregate equals the prior single-pool semantics (AC-8), even when the
// interned data is spread across many shards.
//
// VALIDATES: AC-8 - InternTotal/InternHits/LiveSlots/TotalSlots aggregate.
//
// PREVENTS: Metrics reporting only one shard's counters after sharding.
func TestMetricsAggregateAcrossShards(t *testing.T) {
	p := New(1024)

	// Intern enough distinct payloads to touch many shards, each twice so every
	// shard records exactly one dedup hit.
	const distinct = 64
	for i := range distinct {
		data := fmt.Appendf(nil, "metric-payload-%d", i)
		mustIntern(t, p, data)
		mustIntern(t, p, data) // dedup hit
	}

	m := p.Metrics()
	require.Equal(t, int64(2*distinct), m.InternTotal, "total interns summed across shards")
	require.Equal(t, int64(distinct), m.InternHits, "one dedup hit per payload, summed")
	require.Equal(t, int32(distinct), m.LiveSlots, "one live slot per payload, summed")
	require.Equal(t, int32(distinct), m.TotalSlots, "total slots summed across shards")
	require.InDelta(t, 0.5, m.DeduplicationRate(), 0.001, "half of interns were hits")
}

// TestMetricsAccuracy verifies metrics reflect actual pool state.
//
// VALIDATES: Observability correctness.
//
// PREVENTS: Misleading metrics causing incorrect capacity planning
// or missed memory issues in production.
func TestMetricsAccuracy(t *testing.T) {
	p := New(1024)

	h1 := mustIntern(t, p, []byte("AAAA"))
	h2 := mustIntern(t, p, []byte("BBBB"))
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
	mustIntern(t, p, []byte("data"))
	mustIntern(t, p, []byte("data"))
	mustIntern(t, p, []byte("data"))

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

	h := mustIntern(t, p, []byte("to-be-released"))
	mustIntern(t, p, []byte("keep-alive"))
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

	mustIntern(t, p, []byte("some data here"))

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

	mustIntern(t, p, []byte("unique1"))
	mustIntern(t, p, []byte("unique2"))
	mustIntern(t, p, []byte("unique3"))

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
