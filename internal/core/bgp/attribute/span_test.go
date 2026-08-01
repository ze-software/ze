// RFC: rfc/short/rfc4271.md — path attribute wire format (Section 4.3)
// Overview: span.go — Span, SpanIndex, BuildSpanIndex

package attribute

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spanAttr encodes one attribute TLV, choosing the header size class from the length.
func spanAttr(flags byte, code AttributeCode, value []byte) []byte {
	if len(value) > 255 {
		hdr := []byte{flags | 0x10, byte(code), byte(len(value) >> 8), byte(len(value))}
		return append(hdr, value...)
	}
	hdr := []byte{flags, byte(code), byte(len(value))}
	return append(hdr, value...)
}

// spanWellKnown is the well-known-mandatory trio: ORIGIN, AS_PATH, NEXT_HOP.
func spanWellKnown() []byte {
	out := spanAttr(0x40, AttrOrigin, []byte{0x00})
	out = append(out, spanAttr(0x40, AttrASPath, nil)...)
	return append(out, spanAttr(0x40, AttrNextHop, []byte{192, 0, 2, 1})...)
}

// walkAttrs is the independent oracle: an AttrIterator walk, which shares no code
// with the span builder. It returns one row per attribute, or ok=false when the
// section does not walk cleanly to its end.
type oracleRow struct {
	code      AttributeCode
	valueOff  int
	valueLen  int
	headerLen int
}

func walkAttrs(packed []byte) ([]oracleRow, bool) {
	var rows []oracleRow
	iter := NewAttrIterator(packed)
	for {
		before := iter.Offset()
		code, _, value, ok := iter.Next()
		if !ok {
			break
		}
		after := iter.Offset()
		rows = append(rows, oracleRow{
			code:      code,
			valueOff:  after - len(value),
			valueLen:  len(value),
			headerLen: (after - before) - len(value),
		})
	}
	return rows, iter.Offset() == len(packed)
}

// assertIndexMatchesOracle compares a built index against the iterator walk.
func assertIndexMatchesOracle(t *testing.T, idx *SpanIndex, packed []byte) {
	t.Helper()
	rows, complete := walkAttrs(packed)
	require.True(t, complete, "oracle walk must consume the whole section")
	require.Equal(t, len(rows), idx.Len(), "span count")
	for i, row := range rows {
		span := idx.At(i)
		assert.Equal(t, row.code, span.Code, "span %d code", i)
		assert.Equal(t, row.valueOff, int(span.Offset), "span %d value offset", i)
		assert.Equal(t, row.valueLen, int(span.Length), "span %d value length", i)
		assert.Equal(t, row.headerLen, int(span.HdrLen), "span %d header length", i)
		assert.True(t, idx.Has(row.code), "presence bit for %s", row.code)
	}
}

// TestSpanFitsEightBytes pins the span budget.
//
// VALIDATES: the 8-byte-per-span design decision — one cache line covers a typical
// UPDATE's whole index instead of three.
// PREVENTS: a parsed value, a flags byte, or any other field creeping back into the
// span and re-inflating it toward the 24-byte attrIndex it replaced.
func TestSpanFitsEightBytes(t *testing.T) {
	assert.LessOrEqual(t, int(unsafe.Sizeof(Span{})), 8, "one span must fit in 8 bytes")
	assert.LessOrEqual(t, int(unsafe.Sizeof([SpanInline]Span{})), 64,
		"the inline span array must fit in 64 bytes")
	assert.Equal(t, 8, SpanInline, "inline capacity carried over from the lazy builder's pre-size")
}

// TestSpanIndexMatchesAttributesWire proves the eager index describes exactly the
// attributes an independent walk of the same bytes finds.
//
// VALIDATES: AC-1 — one span per attribute, in wire order, with the same code,
// value offset and value length.
// PREVENTS: an off-by-one in the header size class, which would hand every consumer
// a value slice shifted by one byte.
func TestSpanIndexMatchesAttributesWire(t *testing.T) {
	long := make([]byte, 300) // forces the 4-byte extended-length header
	for i := range long {
		long[i] = byte(i)
	}

	tests := []struct {
		name   string
		packed []byte
	}{
		{"empty section", nil},
		{"well-known mandatory", spanWellKnown()},
		{"zero-length value", spanAttr(0x40, AttrASPath, nil)},
		{"extended length header", spanAttr(0xC0, AttrMPReachNLRI, long)},
		{"255-octet value (last non-extended)", spanAttr(0xC0, AttrCommunity, make([]byte, 255))},
		{"256-octet value (first extended)", spanAttr(0xC0, AttrCommunity, make([]byte, 256))},
		{"mixed header classes", append(spanWellKnown(), spanAttr(0xC0, AttrMPReachNLRI, long)...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := BuildSpanIndex(tt.packed)
			require.NoError(t, err)
			assertIndexMatchesOracle(t, &idx, tt.packed)
		})
	}
}

// TestSpanIndexRejectsDuplicateCode pins the RFC 4271 Section 4.3 verdict.
//
// VALIDATES: AC-2 — a duplicate type code fails the build with the same verdict the
// lazy builder's seen[code] check produced, and yields no index at all.
// PREVENTS: a half-built index reading as "this UPDATE has these attributes", which
// would silently bypass every attribute-based policy.
func TestSpanIndexRejectsDuplicateCode(t *testing.T) {
	packed := append(spanWellKnown(), spanAttr(0x40, AttrOrigin, []byte{0x02})...)

	idx, err := BuildSpanIndex(packed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate attribute")
	assert.Equal(t, 0, idx.Len(), "a failed build must publish no spans")
	assert.False(t, idx.Has(AttrOrigin), "a failed build must publish no presence bits")
}

// TestSpanIndexRejectsTruncatedAttribute pins the containment verdict.
//
// VALIDATES: AC-3 — an attribute whose value runs past the end of the section fails
// the build, matching offset+hdrLen+length > len(packed).
// PREVENTS: a span whose slice bounds would panic, or silently read a neighbour's
// bytes, in every consumer that trusts the index.
func TestSpanIndexRejectsTruncatedAttribute(t *testing.T) {
	packed := append(spanWellKnown(), 0x40, byte(AttrLocalPref), 0x04, 0x00) // claims 4, holds 1

	idx, err := BuildSpanIndex(packed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "truncated")
	assert.Equal(t, 0, idx.Len())
}

// TestSpanIndexRejectsOversizeSection covers the fail-closed 16-bit bound.
//
// VALIDATES: the security-review "Integer handling" row — every offset is bounded by
// the RFC 8654 body ceiling, and a larger buffer is refused rather than truncated.
// PREVENTS: an in-process caller handing over a >64KiB section and receiving spans
// whose offsets have silently wrapped.
func TestSpanIndexRejectsOversizeSection(t *testing.T) {
	_, err := BuildSpanIndex(make([]byte, 65536))
	require.ErrorIs(t, err, ErrAttrSectionTooLarge)
}

// TestPresenceBitsetAnswersHasWithoutScan proves absence is settled by the bitset.
//
// VALIDATES: AC-6 — a presence lookup walks neither the spans nor the wire bytes.
// PREVENTS: a return to a linear scan for the RFC 8669 Prefix-SID question, which
// runs on every received UPDATE.
func TestPresenceBitsetAnswersHasWithoutScan(t *testing.T) {
	idx, err := BuildSpanIndex(spanWellKnown())
	require.NoError(t, err)

	assert.True(t, idx.Has(AttrOrigin))
	assert.True(t, idx.Has(AttrASPath))
	assert.True(t, idx.Has(AttrNextHop))
	assert.False(t, idx.Has(AttrPrefixSID))
	assert.False(t, idx.Has(AttrMED))
	assert.False(t, idx.Has(255), "the bitset covers the whole uint8 code domain")

	allocs := testing.AllocsPerRun(100, func() {
		if idx.Has(AttrPrefixSID) {
			t.Fatal("Prefix-SID must be absent")
		}
	})
	assert.Zero(t, allocs, "a presence lookup must allocate nothing")
}

// TestSpanIndexZeroAllocUpToEight pins the inline capacity.
//
// VALIDATES: AC-8 — an UPDATE with 8 or fewer attributes costs the index zero heap
// allocations, and building it into an AttributesWire costs only that struct.
// PREVENTS: the per-UPDATE index allocation the lazy builder made returning.
func TestSpanIndexZeroAllocUpToEight(t *testing.T) {
	packed := spanWellKnown()
	for _, code := range []AttributeCode{AttrMED, AttrLocalPref, AttrCommunity, AttrAIGP, AttrOriginatorID} {
		packed = append(packed, spanAttr(0x80, code, []byte{0, 0, 0, 0})...)
	}
	idx, err := BuildSpanIndex(packed)
	require.NoError(t, err)
	require.Equal(t, SpanInline, idx.Len(), "the fixture must sit exactly at the inline capacity")
	require.False(t, idx.Spilled())

	var target SpanIndex
	allocs := testing.AllocsPerRun(100, func() {
		if err := target.build(packed); err != nil {
			t.Fatal(err)
		}
	})
	assert.Zero(t, allocs, "building an index at or below the inline capacity must not allocate")

	structAllocs := testing.AllocsPerRun(100, func() {
		if aw := NewAttributesWire(packed, 0); aw == nil {
			t.Fatal("constructor returned nil")
		}
	})
	assert.LessOrEqual(t, structAllocs, float64(1),
		"at most the AttributesWire itself allocates; the index rides inside it, "+
			"where the lazy builder used to make a separate []attrIndex")
}

// TestSpanIndexSpillsPastInlineCapacity covers the boundary above the inline array.
//
// VALIDATES: the 0-n span-count boundary row — 8 inline, 9 spills to the heap, and
// every span stays readable in wire order across the seam.
// PREVENTS: an At() that reads the inline array past its length, or a spill index
// off by SpanInline.
func TestSpanIndexSpillsPastInlineCapacity(t *testing.T) {
	codes := []AttributeCode{
		AttrOrigin, AttrASPath, AttrNextHop, AttrMED, AttrLocalPref,
		AttrCommunity, AttrAIGP, AttrOriginatorID, AttrClusterList, AttrExtCommunity,
	}
	var packed []byte
	for _, code := range codes {
		packed = append(packed, spanAttr(0x80, code, []byte{0, 0, 0, 0})...)
	}

	idx, err := BuildSpanIndex(packed)
	require.NoError(t, err)
	require.Equal(t, len(codes), idx.Len())
	assert.True(t, idx.Spilled(), "10 attributes must spill past the 8-slot inline array")
	assertIndexMatchesOracle(t, &idx, packed)

	for i, code := range codes {
		span, ok := idx.Find(code)
		require.True(t, ok, "%s must be findable across the inline/spill seam", code)
		assert.Equal(t, idx.At(i), span, "Find and At must agree for %s", code)
	}
}

// FuzzSpanIndexMatchesIterator drives arbitrary attribute sections through the
// builder and the independent oracle.
//
// VALIDATES: AC-1 over a corpus rather than a fixture list.
// PREVENTS: a span whose slice bounds fall outside the section for some input shape
// nobody enumerated, which is peer-controlled memory access.
func FuzzSpanIndexMatchesIterator(f *testing.F) {
	f.Add(spanWellKnown())
	f.Add(spanAttr(0xC0, AttrMPReachNLRI, make([]byte, 300)))
	f.Add(append(spanWellKnown(), spanAttr(0x40, AttrOrigin, []byte{2})...))
	f.Add([]byte{0x40, 0x01})
	f.Add([]byte(nil))

	f.Fuzz(func(t *testing.T, packed []byte) {
		if len(packed) > 65535 {
			return
		}
		idx, err := BuildSpanIndex(packed)
		if err != nil {
			if idx.Len() != 0 {
				t.Fatalf("failed build published %d spans", idx.Len())
			}
			return
		}
		for i := range idx.Len() {
			span := idx.At(i)
			end := int(span.Offset) + int(span.Length)
			if int(span.Offset) < int(span.HdrLen) || end > len(packed) {
				t.Fatalf("span %d out of range: offset=%d length=%d hdr=%d section=%d",
					i, span.Offset, span.Length, span.HdrLen, len(packed))
			}
		}
		rows, complete := walkAttrs(packed)
		if !complete {
			t.Fatalf("builder accepted a section the oracle could not walk to its end")
		}
		if len(rows) != idx.Len() {
			t.Fatalf("span count %d, oracle count %d", idx.Len(), len(rows))
		}
		for i, row := range rows {
			span := idx.At(i)
			if row.code != span.Code || row.valueOff != int(span.Offset) ||
				row.valueLen != int(span.Length) || row.headerLen != int(span.HdrLen) {
				t.Fatalf("span %d = %+v, oracle = %+v", i, span, row)
			}
		}
	})
}

// BenchmarkAttributeReadNoLock measures the concurrent read throughput the retired
// RWMutex used to bound.
//
// VALIDATES: AC-4, AC-5 — the accessors the forward path uses take no lock, so many
// destination goroutines reading one shared base do not contend.
func BenchmarkAttributeReadNoLock(b *testing.B) {
	aw := NewAttributesWire(spanWellKnown(), 0)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := aw.GetRaw(AttrNextHop); err != nil {
				b.Fatal(err)
			}
			if _, err := aw.Has(AttrPrefixSID); err != nil {
				b.Fatal(err)
			}
		}
	})
}
