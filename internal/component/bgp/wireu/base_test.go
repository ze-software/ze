// RFC: rfc/short/rfc4271.md — UPDATE message wire format (Section 4.3)
// Overview: wire_update.go — WireUpdate, the base the span index rides in
//
// The base is the wire bytes plus the attribute span index built over them. Once it is
// published it is shared by every destination goroutine in the forward loop, so nothing
// may write it and no read may take a lock (plan/spec-wire-edit-1-base-index.md).

package wireu

import (
	"encoding/binary"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// baseTestCtxID registers an encoding context, which value parsing needs.
func baseTestCtxID(t *testing.T) bgpctx.ContextID {
	t.Helper()
	id, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	return id
}

// baseTestAttrs is the well-known-mandatory trio plus MED and a community.
func baseTestAttrs() []byte {
	return []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x2a, // MED = 42
		0xc0, 0x08, 0x04, 0xfd, 0xe9, 0x00, 0x07, // COMMUNITY 65001:7
	}
}

// baseTestBody assembles an UPDATE body around a path-attribute section.
func baseTestBody(pathAttrs, nlri []byte) []byte {
	body := make([]byte, 2+2+len(pathAttrs)+len(nlri))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(pathAttrs))) //nolint:gosec // fixture sizes
	copy(body[4:], pathAttrs)
	copy(body[4+len(pathAttrs):], nlri)
	return body
}

// TestBaseImmutableAfterPublication drives many concurrent readers at one base.
//
// VALIDATES: AC-4 — a base read concurrently by many destination goroutines takes no lock
// on any read and reports nothing under -race.
// PREVENTS: the retired RWMutex being needed again by a write that crept back in after
// publication, which the race detector is the only thing that would catch.
func TestBaseImmutableAfterPublication(t *testing.T) {
	wu := NewWireUpdate(baseTestBody(baseTestAttrs(), []byte{24, 10, 0, 0}), baseTestCtxID(t))

	// Publish the base exactly as the receive path does: build it once, up front.
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	require.NotNil(t, attrs)

	const readers = 16
	var wg sync.WaitGroup
	for range readers {
		wg.Go(func() {
			for range 200 {
				raw, err := attrs.GetRaw(attribute.AttrNextHop)
				assert.NoError(t, err)
				assert.Equal(t, []byte{192, 0, 2, 1}, raw)

				has, err := attrs.Has(attribute.AttrPrefixSID)
				assert.NoError(t, err)
				assert.False(t, has)

				med, err := attrs.Get(attribute.AttrMED)
				assert.NoError(t, err)
				assert.NotNil(t, med)

				assert.Equal(t, 5, attrs.Count())
			}
		})
	}
	wg.Wait()
}

// TestSnapshotCarriesSpanIndex proves the contract-A copy keeps its index.
//
// VALIDATES: AC-7 — every attribute reads back identically from the snapshot, and the
// snapshot needs no index rebuild.
// PREVENTS: the copy silently falling back to a rebuild, which would put an attribute walk
// back on the fire-and-forget delivery path this snapshot exists to protect.
func TestSnapshotCarriesSpanIndex(t *testing.T) {
	wu := NewWireUpdate(baseTestBody(baseTestAttrs(), []byte{24, 10, 0, 0}), 0)
	orig, err := wu.Attrs()
	require.NoError(t, err)
	require.NotNil(t, orig)

	snap := wu.Snapshot()

	allocs := testing.AllocsPerRun(50, func() {
		if _, err := snap.Attrs(); err != nil {
			t.Fatal(err)
		}
	})
	assert.Zero(t, allocs, "the snapshot's index must already exist, not be rebuilt on first use")

	got, err := snap.Attrs()
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, orig.Count(), got.Count())

	// The snapshot must read its OWN bytes, not the original's.
	assert.NotSame(t, &orig.Packed()[0], &got.Packed()[0], "the snapshot must own its payload")

	for _, code := range []attribute.AttributeCode{
		attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop,
		attribute.AttrMED, attribute.AttrCommunity,
	} {
		want, err := orig.GetRaw(code)
		require.NoError(t, err)
		got, err := got.GetRaw(code)
		require.NoError(t, err)
		assert.Equal(t, want, got, "attribute %s must read back identically", code)
	}
}

// TestSnapshotOfEmptyAndMalformedUpdates covers the two shapes with no index to carry.
//
// VALIDATES: AC-7 at its edges — an UPDATE with no attribute section, and one whose
// sections do not parse, snapshot without inventing an index or losing an error.
// PREVENTS: the carry-over path publishing a base over bytes it never indexed.
func TestSnapshotOfEmptyAndMalformedUpdates(t *testing.T) {
	t.Run("no attribute section", func(t *testing.T) {
		snap := NewWireUpdate(baseTestBody(nil, nil), 0).Snapshot()
		attrs, err := snap.Attrs()
		require.NoError(t, err)
		assert.Nil(t, attrs, "an UPDATE with no attributes has no AttributesWire")
	})

	t.Run("truncated sections", func(t *testing.T) {
		orig := NewWireUpdate([]byte{0x00}, 0)
		_, wantErr := orig.Attrs()
		require.Error(t, wantErr)

		snap := orig.Snapshot()
		attrs, err := snap.Attrs()
		require.Error(t, err, "the snapshot must carry the same verdict as the original")
		assert.Nil(t, attrs)
	})
}

// TestAPIBuiltBaseAnswersHas resolves A-5.
//
// VALIDATES: A-5 — an announce-rail base, which never passes through the receive walk,
// answers Has correctly with no accessor called first.
// PREVENTS: an eager index that exists only on the receive path, leaving API-originated
// UPDATEs on a code path nothing else exercises.
func TestAPIBuiltBaseAnswersHas(t *testing.T) {
	aw := attribute.NewAttributesWire(baseTestAttrs(), 0)

	has, err := aw.Has(attribute.AttrMED)
	require.NoError(t, err)
	assert.True(t, has)

	has, err = aw.Has(attribute.AttrPrefixSID)
	require.NoError(t, err)
	assert.False(t, has)

	assert.Equal(t, 5, aw.Count())
	assert.False(t, aw.Spilled())
}
