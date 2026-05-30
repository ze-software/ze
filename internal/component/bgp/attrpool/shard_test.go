package attrpool

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// defaultShardOf returns the shard a default (numShards-wide) pool, such as the
// ones New() builds, selects for data. Tests use it to reason about which shard
// a payload lands in without threading a pool's shardMask through every call.
func defaultShardOf(data []byte) uint32 {
	return shardOf(data, numShards-1)
}

// differentSameShard returns a distinct byte slice, derived from prefix, that
// hashes to the same shard as ref. Used by tests that exercise per-shard
// behavior (free-list reuse, intra-shard dedup) which only manifests when two
// distinct payloads land in the same shard.
func differentSameShard(ref []byte, prefix string) []byte {
	target := defaultShardOf(ref)
	for i := 0; ; i++ {
		cand := fmt.Appendf(nil, "%s-%d", prefix, i)
		if !bytes.Equal(cand, ref) && defaultShardOf(cand) == target {
			return cand
		}
	}
}

// TestHandleShardDecodeRoute verifies the shard id packed into the high bits of
// the Slot field round-trips: packShardSlot then shardID/shardSlot recover the
// original components, and PoolIdx/BufferBit/Slot stay consistent.
//
// VALIDATES: shard-id-in-slot encoding is lossless and does not disturb the
// existing Handle fields (AC-5).
//
// PREVENTS: a handle being routed to the wrong shard, which would read another
// shard's slot table (AC-4).
func TestHandleShardDecodeRoute(t *testing.T) {
	for i := range numShards {
		shard := uint32(i)
		for _, slot := range []uint32{0, 1, 42, shardSlotMask - 1, shardSlotMask} {
			packed := packShardSlot(shard, slot)
			require.LessOrEqual(t, packed, uint32(0x3FFFFFF), "packed slot must fit 26 bits")

			for _, buf := range []uint32{0, 1} {
				for _, idx := range []uint8{0, 5, 30} {
					h := NewHandleWithBuffer(buf, idx, packed)
					require.Equal(t, shard, h.shardID(), "shardID must decode")
					require.Equal(t, slot, h.shardSlot(), "shardSlot must decode")
					require.Equal(t, buf, h.BufferBit(), "bufferBit preserved")
					require.Equal(t, idx, h.PoolIdx(), "poolIdx preserved")
					require.Equal(t, packed, h.Slot(), "full Slot() preserved")
					require.True(t, h.IsValid(), "valid poolIdx stays valid")
				}
			}
		}
	}
}

// TestShardRoundTripAllShards verifies intern/get/release works for data that
// lands in every shard, and that the returned handle routes back to the shard
// that stored it.
//
// VALIDATES: AC-1, AC-2, AC-3, AC-4 across all shards.
//
// PREVENTS: data interned into shard X being unreachable or read from shard Y.
func TestShardRoundTripAllShards(t *testing.T) {
	p := New(1024)

	// Generate one distinct payload per shard so every shard is exercised.
	payloads := make(map[uint32][]byte)
	for i := 0; len(payloads) < numShards; i++ {
		data := fmt.Appendf(nil, "round-trip-%d", i)
		s := defaultShardOf(data)
		if _, ok := payloads[s]; !ok {
			payloads[s] = data
		}
	}
	require.Len(t, payloads, numShards, "every shard must receive a payload")

	for shard, data := range payloads {
		h := mustIntern(t, p, data)
		require.Equal(t, shard, h.shardID(), "handle must encode its shard")

		got, err := p.Get(h)
		require.NoError(t, err)
		require.Equal(t, data, got, "Get must return the exact bytes (zero-copy)")

		require.NoError(t, p.Release(h))
		_, err = p.Get(h)
		require.ErrorIs(t, err, ErrSlotDead, "released handle reads dead slot")
	}
}

// TestDedupStaysGlobal verifies identical bytes always land in one shard with a
// single stored copy, regardless of concurrency.
//
// VALIDATES: AC-1 dedup is preserved by content-hash shard selection.
//
// PREVENTS: the same bytes being interned twice into different shards with
// different handles, breaking the global deduplication guarantee.
func TestDedupStaysGlobal(t *testing.T) {
	p := New(1024)

	data := []byte("globally-deduplicated")
	h1 := mustIntern(t, p, data)
	h2 := mustIntern(t, p, data)
	require.Equal(t, h1, h2, "identical bytes return the same handle")
	require.Equal(t, h1.shardID(), h2.shardID(), "identical bytes hash to one shard")

	// Exactly one stored copy: 2 interns, 1 hit.
	m := p.Metrics()
	require.Equal(t, int64(2), m.InternTotal)
	require.Equal(t, int64(1), m.InternHits)
	require.Equal(t, int32(1), m.LiveSlots, "only one slot stores the data")
}

// TestNewWithShardsValidation verifies the shard-count argument is constrained to
// powers of two in 1..numShards, and that the idx validation still applies.
//
// VALIDATES: NewWithShards rejects shard counts that would not fit the shard-id
// sub-field or break the mask-based selection.
func TestNewWithShardsValidation(t *testing.T) {
	// Valid powers of two, 1..16.
	for _, n := range []int{1, 2, 4, 8, 16} {
		p, err := NewWithShards(0, 1024, n)
		require.NoError(t, err, "shards=%d must be valid", n)
		require.Len(t, p.shards, n, "pool must hold exactly n shards")
		require.Equal(t, uint32(n-1), p.shardMask, "mask must be n-1")
	}

	// Invalid shard counts.
	for _, n := range []int{0, -1, 3, 5, 6, 7, 15, 32, 64} {
		_, err := NewWithShards(0, 1024, n)
		require.ErrorIs(t, err, ErrInvalidShardCount, "shards=%d must be rejected", n)
	}

	// idx validation still fires (and takes precedence over shard-count checks).
	_, err := NewWithShards(31, 1024, 16)
	require.ErrorIs(t, err, ErrInvalidIdx, "idx=31 is reserved")
}

// TestUnshardedPoolDegeneratesToSingleShard verifies a 1-shard pool puts every
// payload in shard 0 and round-trips correctly: the pre-sharding single-lock
// behavior is preserved for low-cardinality pools like ORIGIN.
//
// VALIDATES: N=1 is a correct, contention-free degenerate of the sharded pool.
func TestUnshardedPoolDegeneratesToSingleShard(t *testing.T) {
	p, err := NewWithShards(2, 1024, 1) // idx=2, like the ORIGIN pool
	require.NoError(t, err)
	require.Len(t, p.shards, 1)

	handles := make([]Handle, 0, 64)
	for i := range 64 {
		data := fmt.Appendf(nil, "origin-like-%d", i)
		h := mustIntern(t, p, data)
		require.Zero(t, h.shardID(), "every handle in a 1-shard pool uses shard 0")

		got, gerr := p.Get(h)
		require.NoError(t, gerr)
		require.Equal(t, data, got)
		handles = append(handles, h)
	}

	// All 64 distinct payloads live in the single shard.
	m := p.Metrics()
	require.Equal(t, int32(64), m.LiveSlots)

	for _, h := range handles {
		require.NoError(t, p.Release(h))
	}
}

// TestForeignShardHandleRoutingIsBoundsSafe verifies that a handle whose shard id
// is outside a pool's shard range is rejected with an error instead of panicking
// on the shards slice. This is the safety net that a fixed [16]shard array gave
// for free but a variable-length slice does not.
//
// VALIDATES: variable shard count never lets a stray/foreign handle index out of
// bounds (AC-4 routing safety under per-pool shard counts).
func TestForeignShardHandleRoutingIsBoundsSafe(t *testing.T) {
	p, err := NewWithShards(0, 1024, 1) // single shard: valid shard id is only 0
	require.NoError(t, err)

	// Craft a handle that claims shard id 5 — in range for a 16-shard pool but
	// out of range here. Slot index 0 within that phantom shard.
	foreign := NewHandleWithBuffer(0, 0, packShardSlot(5, 0))
	require.Equal(t, uint32(5), foreign.shardID())

	require.NotPanics(t, func() {
		_, gerr := p.Get(foreign)
		require.ErrorIs(t, gerr, ErrWrongPool)

		_, lerr := p.Length(foreign)
		require.ErrorIs(t, lerr, ErrWrongPool)

		require.ErrorIs(t, p.Release(foreign), ErrWrongPool)
		require.ErrorIs(t, p.AddRef(foreign), ErrWrongPool)
	})

	// GetBySlot/ReleaseBySlot route by the same shard high-bits and must also be
	// bounds-safe rather than panic.
	require.NotPanics(t, func() {
		_, gerr := p.GetBySlot(packShardSlot(5, 0))
		require.ErrorIs(t, gerr, ErrSlotOutOfBounds)
		require.ErrorIs(t, p.ReleaseBySlot(packShardSlot(5, 0)), ErrSlotOutOfBounds)
	})
}
