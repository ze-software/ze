package bgp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSpanSlice verifies Span extracts correct subslice.
//
// VALIDATES: Span.Slice returns buf[Start:Start+Len]
// PREVENTS: Off-by-one errors in slice extraction.
func TestSpanSlice(t *testing.T) {
	buf := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name string
		span Span
		want []byte
	}{
		{"full buffer", Span{0, 10}, buf},
		{"first half", Span{0, 5}, []byte{0, 1, 2, 3, 4}},
		{"second half", Span{5, 5}, []byte{5, 6, 7, 8, 9}},
		{"middle", Span{3, 4}, []byte{3, 4, 5, 6}},
		{"single byte", Span{5, 1}, []byte{5}},
		{"empty at start", Span{0, 0}, []byte{}},
		{"empty at end", Span{10, 0}, []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.span.Slice(buf)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSpanEnd verifies End calculation.
//
// VALIDATES: End returns Start + Len
// PREVENTS: Incorrect offset calculation.
func TestSpanEnd(t *testing.T) {
	tests := []struct {
		span Span
		want int
	}{
		{Span{0, 10}, 10},
		{Span{5, 3}, 8},
		{Span{0, 0}, 0},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.span.End())
	}
}

// TestSpanIsEmpty verifies empty detection.
//
// VALIDATES: IsEmpty returns true only for zero-length spans
// PREVENTS: False positives/negatives for empty check.
func TestSpanIsEmpty(t *testing.T) {
	assert.True(t, Span{0, 0}.IsEmpty())
	assert.True(t, Span{5, 0}.IsEmpty())
	assert.False(t, Span{0, 1}.IsEmpty())
	assert.False(t, Span{5, 10}.IsEmpty())
}

// TestSpanValid verifies bounds checking.
//
// VALIDATES: Valid returns true only for in-bounds spans
// PREVENTS: Buffer overrun from invalid spans.
func TestSpanValid(t *testing.T) {
	tests := []struct {
		name   string
		span   Span
		bufLen int
		want   bool
	}{
		{"full buffer", Span{0, 10}, 10, true},
		{"within buffer", Span{2, 5}, 10, true},
		{"empty at end", Span{10, 0}, 10, true},
		{"exceeds buffer", Span{5, 10}, 10, false},
		{"negative start", Span{-1, 5}, 10, false},
		{"negative len", Span{0, -1}, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.span.Valid(tt.bufLen))
		})
	}
}
