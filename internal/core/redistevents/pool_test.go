package redistevents

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/memguard"
)

// VALIDATES: AC-7 -- ReleaseBatch zeroes a filled entry's per-entry value
// fields (OriginAS, Metric) so a recycled batch never leaks the prior route's
// origin AS or metric to the next producer. Relies on clear(b.Entries) in
// ReleaseBatch covering value-type additions with no per-field reset.
// PREVENTS: a pooled batch handing a stale OriginAS to a producer that leaves
// the field zero, injecting a wrong origin AS into a redistributed route.
func TestRouteChangeBatchPoolResetsOriginAS(t *testing.T) {
	b := AcquireBatch()
	b.Entries = append(b.Entries, RouteChangeEntry{
		Action:   ActionAdd,
		Prefix:   netip.MustParsePrefix("192.0.2.0/24"),
		Metric:   100,
		OriginAS: 64512,
	})
	// Keep a view over the backing array so we can inspect the slot the entry
	// lived in after ReleaseBatch truncates len back to 0.
	backing := b.Entries[:cap(b.Entries)]

	ReleaseBatch(b)

	// The no-leak guarantee holds in both builds: the prior route's OriginAS
	// (64512) and Metric (100) are gone. The reset value is build-variant —
	// release zeroes, debug poisons with a recognizable sentinel.
	assert.NotEqual(t, uint32(64512), backing[0].OriginAS, "ReleaseBatch must drop the prior OriginAS")
	assert.NotEqual(t, uint32(100), backing[0].Metric, "ReleaseBatch must drop the prior Metric")
	if memguard.Enabled {
		assert.Equal(t, uint32(0xDEADBEEF), backing[0].OriginAS, "debug: ReleaseBatch poisons the entry's OriginAS")
		assert.Equal(t, uint32(0xDEADBEEF), backing[0].Metric, "debug: ReleaseBatch poisons the entry's Metric")
	} else {
		assert.Equal(t, uint32(0), backing[0].OriginAS, "release: ReleaseBatch zeroes the entry's OriginAS")
		assert.Equal(t, uint32(0), backing[0].Metric, "release: ReleaseBatch zeroes the entry's Metric")
	}

	// the previous exact-value assertion on the re-acquired slot
	// relied on sync.Pool returning the SAME recycled backing array, which is
	// never guaranteed — under -race/GC Get() returns a fresh zeroed batch, and
	// the debug poison makes the recycled value (0xDEADBEEF) differ from the fresh
	// value (0), so no single exact value is deterministic here. Replaced with the
	// invariant that holds in every case (no leak of the prior OriginAS). The exact
	// deterministic reset value is still asserted on backing[0] above.
	b2 := AcquireBatch()
	require.Empty(t, b2.Entries, "re-acquired batch must start with no entries")
	grown := b2.Entries[:1]
	assert.NotEqual(t, uint32(64512), grown[0].OriginAS, "re-acquired batch must not leak the prior OriginAS")
	ReleaseBatch(b2)
}

// TestReleaseBatchPoisonsEntries proves contract-D enforcement: in debug builds
// ReleaseBatch overwrites entries with a recognizable sentinel, so a batch
// retained past the synchronous-dispatch boundary reads poison — not a zero that
// could masquerade as a valid 0.0.0.0/0 route. In release the same batch is
// zeroed. The netip fields stay zero (nil pointer) so poisoning is GC-safe.
//
// VALIDATES: Debug builds make a use-after-ReleaseBatch loud (AC-6).
//
// PREVENTS: A retained redistribution batch silently handing a zeroed-but-
// plausible route to a consumer after dispatch.
func TestReleaseBatchPoisonsEntries(t *testing.T) {
	b := AcquireBatch()
	b.Entries = append(b.Entries, RouteChangeEntry{
		Action:  ActionAdd,
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Metric:  42,
	})
	// A consumer wrongly retains a view of the backing entry past dispatch.
	retained := b.Entries[:cap(b.Entries)]

	ReleaseBatch(b)

	if memguard.Enabled {
		assert.Equal(t, uint32(0xDEADBEEF), retained[0].Metric, "debug: released entry is poisoned, not zeroed")
		assert.Equal(t, actionPoison, retained[0].Action, "debug: Action carries the poison sentinel")
		assert.False(t, retained[0].Prefix.IsValid(), "debug: netip fields stay zero (nil pointer, GC-safe)")
		assert.False(t, retained[0].NextHop.IsValid(), "debug: NextHop stays zero (nil pointer, GC-safe)")
	} else {
		assert.Zero(t, retained[0].Metric, "release: released entry is cleared to zero")
		assert.Equal(t, ActionUnspecified, retained[0].Action, "release: Action cleared to zero")
	}
}
