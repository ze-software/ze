// Design: docs/architecture/textbuf-string-building.md -- tests, benchmarks, and fuzzing for Buffer wire parsing.
// Related: parse.go -- positional counted wire parser on Buffer.

package textbuf

import (
	"encoding/binary"
	"math"
	"net/netip"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	freezePanicMsg   = "BUG: textbuf write after String"
	readOnlyPanicMsg = "BUG: textbuf mutable bytes from read-only string buffer"
)

// VALIDATES: CountedBytes/CountedStr round-trip through Adopt, AdoptString,
// FromBytes, and FromString with zero-copy aliasing, 3-index cap isolation,
// zero-copy String ownership transfer, and Reset unfreezing.
// PREVENTS: copying parsed payloads, cross-field overwrites on append, or
// writing back into immutable string memory after Reset.
func TestBufferAdoptAndParseCounted(t *testing.T) {
	t.Parallel()

	var writer Buffer
	writer.Reset().
		Str("row ").
		Uint(42).Byte(' ').
		CountedBytes([]byte("10.0.0.1")).Byte(' ').
		CountedBytes(nil).Byte(' ').
		CountedStr("as-path 65001 65002").Byte('\n')

	raw := append([]byte(nil), writer.Bytes()...)
	rawStr := string(raw)

	// Bare append helpers produce identical framing.
	bare := CountedBytes(nil, []byte("10.0.0.1"))
	bare = append(bare, ' ')
	bare = CountedBytes(bare, nil)
	bare = append(bare, ' ')
	bare = CountedStr(bare, "as-path 65001 65002")
	assert.Equal(t, "8:10.0.0.1 0: 19:as-path 65001 65002", string(bare))

	// Stack Adopt([]byte) round-trip and backing-array identity.
	var rb Buffer
	rb.Adopt(raw)
	assert.True(t, rb.TrimTrailingLF())
	assert.False(t, rb.TrimTrailingLF())

	verb, ok := rb.ReadWord4()
	require.True(t, ok)
	assert.Equal(t, MakeWord4("row"), verb)
	assert.Equal(t, Word4Width, rb.Pos())

	seq, err := rb.ReadUintSpace()
	require.NoError(t, err)
	assert.Equal(t, uint64(42), seq)

	peerIdx := rb.Pos() + len("8:")
	peerBytes, err := rb.ReadCountedBytesSpace()
	require.NoError(t, err)
	assert.Equal(t, []byte("10.0.0.1"), peerBytes)
	assert.Same(t, &raw[peerIdx], &peerBytes[0], "parsed bytes must share adopted backing array")
	assert.Equal(t, len(peerBytes), cap(peerBytes), "3-index slice must cap at field length")

	// Appending to a parsed slice must reallocate instead of overwriting raw.
	peerBytes = append(peerBytes, '!', '!', '!')
	assert.Equal(t, "10.0.0.1!!!", string(peerBytes))
	assert.Equal(t, byte(' '), raw[peerIdx+len("10.0.0.1")], "append must not overwrite following delimiter")

	emptyBytes, err := rb.ReadCountedBytesSpace()
	require.NoError(t, err)
	assert.Empty(t, emptyBytes)
	assert.Equal(t, 0, cap(emptyBytes))

	attrsBytes, err := rb.ReadCountedBytesEnd()
	require.NoError(t, err)
	assert.Equal(t, []byte("as-path 65001 65002"), attrsBytes)
	assert.True(t, rb.EOF())
	assert.Equal(t, 0, rb.Remaining())

	// Stack AdoptString(string) round-trip.
	var rs Buffer
	rs.AdoptString(rawStr)
	assert.True(t, rs.TrimTrailingLF())

	verb, ok = rs.ReadWord4()
	require.True(t, ok)
	assert.Equal(t, MakeWord4("row"), verb)

	seq, err = rs.ReadUintSpace()
	require.NoError(t, err)
	assert.Equal(t, uint64(42), seq)

	peerStr, err := rs.ReadCountedStringSpace()
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", peerStr)

	emptyStr, err := rs.ReadCountedStringSpace()
	require.NoError(t, err)
	assert.Equal(t, "", emptyStr)

	attrsStr, err := rs.ReadCountedStringEnd()
	require.NoError(t, err)
	assert.Equal(t, "as-path 65001 65002", attrsStr)
	assert.True(t, rs.EOF())

	// FromBytes and FromString preserve backing data without copying.
	owned := []byte("3:foo 3:bar")
	fb := FromBytes(owned)
	assert.Same(t, &owned[0], &fb.Bytes()[0])
	f1, err := fb.ReadCountedBytesSpace()
	require.NoError(t, err)
	assert.Equal(t, []byte("foo"), f1)
	assert.Equal(t, []byte("3:bar"), fb.RestBytes())
	assert.True(t, fb.Skip(2))
	assert.Equal(t, "bar", fb.RestString())
	assert.False(t, fb.Skip(-1))
	assert.False(t, fb.Skip(100))

	orig := "3:foo 3:bar"
	fs := FromString(orig)
	assert.Same(t, unsafe.StringData(orig), unsafe.StringData(fs.RestString()))
	s1, err := fs.ReadCountedStringSpace()
	require.NoError(t, err)
	assert.Equal(t, "foo", s1)
	assert.Equal(t, "3:bar", fs.RestString())

	// Adopt + String transfers ownership zero-copy and resets the buffer.
	transfer := []byte("owned-payload")
	var tb Buffer
	tb.Adopt(transfer)
	got := tb.String()
	assert.Equal(t, "owned-payload", got)
	assert.Same(t, &transfer[0], unsafe.StringData(got))
	assert.Equal(t, 0, tb.Len())
	assert.Equal(t, 0, tb.Pos())
	assert.Equal(t, "reused", tb.Str("reused").String())

	// AdoptString + String returns zero-copy and clears readOnly/pos.
	var sb Buffer
	sb.AdoptString(orig)
	assert.Same(t, unsafe.StringData(orig), unsafe.StringData(sb.String()))
	assert.Equal(t, 0, sb.Pos())
	assert.Equal(t, "", sb.Reset().Str("").String())

	// Adopt + Reset and large AdoptString + Reset(200) clear pos and readOnly safely.
	rb.Reset().Str("fresh")
	assert.Equal(t, 0, rb.Pos())
	assert.Equal(t, "fresh", rb.String())

	longOrig := strings.Repeat("Z", 300)
	var lb Buffer
	lb.AdoptString(longOrig)
	lb.Reset(200).Str("mutated")
	assert.Equal(t, "mutated", lb.String())
	assert.Equal(t, strings.Repeat("Z", 300), longOrig, "Reset after AdoptString must not reuse string backing memory")
}

// VALIDATES: Every Read*/Peek* parser method freezes the buffer against
// subsequent writes, and read-only string buffers reject both writes and
// mutable []byte accessors.
// PREVENTS: corrupting parsed views via post-read writes or handing out
// writable []byte slices aliasing immutable Go string memory.
func TestBufferReadFreezesAgainstWrites(t *testing.T) {
	t.Parallel()

	readers := []struct {
		name string
		data string
		read func(b *Buffer)
	}{
		{name: "PeekWord4", data: "upd ", read: func(b *Buffer) { _, _ = b.PeekWord4() }},
		{name: "ReadWord4", data: "upd ", read: func(b *Buffer) { _, _ = b.ReadWord4() }},
		{name: "ReadUint", data: "123", read: func(b *Buffer) { _, _ = b.ReadUint() }},
		{name: "ReadUintSpace", data: "123 ", read: func(b *Buffer) { _, _ = b.ReadUintSpace() }},
		{name: "ReadIPv4", data: "10.0.0.1", read: func(b *Buffer) { _, _ = b.ReadIPv4() }},
		{name: "ReadIPv4Space", data: "10.0.0.1 ", read: func(b *Buffer) { _, _ = b.ReadIPv4Space() }},
		{name: "ReadTokenBytes", data: "10.0.0.1", read: func(b *Buffer) { _, _ = b.ReadTokenBytes() }},
		{name: "ReadTokenBytesSpace", data: "10.0.0.1 ", read: func(b *Buffer) { _, _ = b.ReadTokenBytesSpace() }},
		{name: "ReadTokenBytesEnd", data: "10.0.0.1", read: func(b *Buffer) { _, _ = b.ReadTokenBytesEnd() }},
		{name: "ReadTokenString", data: "10.0.0.1", read: func(b *Buffer) { _, _ = b.ReadTokenString() }},
		{name: "ReadTokenStringSpace", data: "10.0.0.1 ", read: func(b *Buffer) { _, _ = b.ReadTokenStringSpace() }},
		{name: "ReadTokenStringEnd", data: "10.0.0.1", read: func(b *Buffer) { _, _ = b.ReadTokenStringEnd() }},
		{name: "ReadSpace", data: " ", read: func(b *Buffer) { _ = b.ReadSpace() }},
		{name: "PeekCountedHeader", data: "3:abc", read: func(b *Buffer) { _, _, _ = b.PeekCountedHeader() }},
		{name: "ReadCountedBytes", data: "3:abc", read: func(b *Buffer) { _, _ = b.ReadCountedBytes() }},
		{name: "ReadCountedBytesSpace", data: "3:abc ", read: func(b *Buffer) { _, _ = b.ReadCountedBytesSpace() }},
		{name: "ReadCountedBytesEnd", data: "3:abc", read: func(b *Buffer) { _, _ = b.ReadCountedBytesEnd() }},
		{name: "ReadCountedString", data: "3:abc", read: func(b *Buffer) { _, _ = b.ReadCountedString() }},
		{name: "ReadCountedStringSpace", data: "3:abc ", read: func(b *Buffer) { _, _ = b.ReadCountedStringSpace() }},
		{name: "ReadCountedStringEnd", data: "3:abc", read: func(b *Buffer) { _, _ = b.ReadCountedStringEnd() }},
		{name: "RestBytes", data: "abc", read: func(b *Buffer) { _ = b.RestBytes() }},
		{name: "RestString", data: "abc", read: func(b *Buffer) { _ = b.RestString() }},
	}

	for _, tc := range readers {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.Adopt([]byte(tc.data))
			tc.read(&b)
			assert.PanicsWithValue(t, freezePanicMsg, func() { b.Byte('x') })
			assert.PanicsWithValue(t, freezePanicMsg, func() { b.Str("x") })
			assert.PanicsWithValue(t, freezePanicMsg, func() { b.CountedBytes([]byte("x")) })
			assert.PanicsWithValue(t, freezePanicMsg, func() { b.CountedStr("x") })
		})
	}

	t.Run("AdoptStringRejectsWritesAndMutableBytes", func(t *testing.T) {
		t.Parallel()
		var b Buffer
		b.AdoptString("3:abc ")
		assert.PanicsWithValue(t, freezePanicMsg, func() { b.Byte('x') })
		assert.PanicsWithValue(t, freezePanicMsg, func() { b.CountedStr("x") })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _ = b.Bytes() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _ = b.RestBytes() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadCountedBytes() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadCountedBytesSpace() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadCountedBytesEnd() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadTokenBytes() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadTokenBytesSpace() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadTokenBytesEnd() })
	})

	t.Run("FromStringRejectsMutableBytes", func(t *testing.T) {
		t.Parallel()
		b := FromString("3:abc")
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _ = b.Bytes() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _ = b.RestBytes() })
		assert.PanicsWithValue(t, readOnlyPanicMsg, func() { _, _ = b.ReadCountedBytesEnd() })
	})
}

// VALIDATES: Word4 packing, PeekWord4 non-advancing behavior, ReadWord4 cursor
// advancement, short-input rejection, trailing-byte sensitivity, and MakeWord4
// length validation.
// PREVENTS: verb mis-dispatch on non-space delimiters (e.g. tab) or partial
// reads advancing the cursor.
func TestBufferWord4(t *testing.T) {
	t.Parallel()

	upd := MakeWord4("upd")
	eor := MakeWord4("eor")
	assert.NotEqual(t, upd, eor)

	for _, bad := range []string{"", "a", "ab", "abcd"} {
		assert.PanicsWithValue(t, "BUG: textbuf.MakeWord4 requires a 3-byte token", func() {
			_ = MakeWord4(bad)
		})
	}

	for _, short := range []string{"", "u", "up", "upd"} {
		var b Buffer
		b.AdoptString(short)
		w, ok := b.PeekWord4()
		assert.False(t, ok)
		assert.Equal(t, Word4(0), w)
		assert.Equal(t, 0, b.Pos())

		w, ok = b.ReadWord4()
		assert.False(t, ok)
		assert.Equal(t, Word4(0), w)
		assert.Equal(t, 0, b.Pos())
	}

	var b Buffer
	b.AdoptString("upd 1")
	w, ok := b.PeekWord4()
	require.True(t, ok)
	assert.Equal(t, upd, w)
	assert.Equal(t, 0, b.Pos())

	w, ok = b.ReadWord4()
	require.True(t, ok)
	assert.Equal(t, upd, w)
	assert.Equal(t, Word4Width, b.Pos())

	// Tab in 4th byte must not match space-terminated MakeWord4.
	var tab Buffer
	tab.AdoptString("upd\t1")
	w, ok = tab.ReadWord4()
	require.True(t, ok)
	assert.NotEqual(t, upd, w)
}

// VALIDATES: Fused decimal uint64 parsing across zero, MaxUint64, overflow,
// width limit, invalid digits, colon termination, and space handling.
// PREVENTS: silent uint64 wraparound, accepting counted-field colons as
// numbers, or advancing the cursor on malformed input.
func TestBufferReadUintBoundaries(t *testing.T) {
	t.Parallel()

	validEOF := []struct {
		input string
		want  uint64
	}{
		{input: "0", want: 0},
		{input: "1", want: 1},
		{input: "42", want: 42},
		{input: "9999999999999999999", want: 9999999999999999999},
		{input: "18446744073709551615", want: math.MaxUint64},
	}

	for _, tc := range validEOF {
		t.Run("ReadUint_EOF_"+tc.input, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(tc.input)
			got, err := b.ReadUint()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.True(t, b.EOF())
		})

		t.Run("ReadUint_Space_"+tc.input, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(tc.input + " rest")
			got, err := b.ReadUint()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, len(tc.input), b.Pos())
			require.NoError(t, b.ReadSpace())
			assert.Equal(t, "rest", b.RestString())
		})

		t.Run("ReadUintSpace_"+tc.input, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(tc.input + " rest")
			got, err := b.ReadUintSpace()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, "rest", b.RestString())
		})
	}

	invalidCases := []struct {
		name      string
		input     string
		wantErr   error
		spaceOnly bool
	}{
		{name: "empty", input: "", wantErr: ErrEmptyNumber},
		{name: "leading_space", input: " 1", wantErr: ErrEmptyNumber},
		{name: "leading_colon", input: ":1", wantErr: ErrEmptyNumber},
		{name: "colon_after_digits", input: "5:hello", wantErr: ErrNumberHasColon},
		{name: "max_then_colon", input: "18446744073709551615:", wantErr: ErrNumberHasColon},
		{name: "non_digit_alpha", input: "abc", wantErr: ErrInvalidDigit},
		{name: "non_digit_mid", input: "12a ", wantErr: ErrInvalidDigit},
		{name: "negative_sign", input: "-1 ", wantErr: ErrInvalidDigit},
		{name: "plus_sign", input: "+1 ", wantErr: ErrInvalidDigit},
		{name: "tab_terminator", input: "42\t", wantErr: ErrInvalidDigit},
		{name: "overflow_max_plus_1", input: "18446744073709551616", wantErr: ErrNumberOverflow},
		{name: "overflow_all_nines", input: "99999999999999999999 ", wantErr: ErrNumberOverflow},
		{name: "too_wide_21_leading_zeros", input: "000000000000000000001 ", wantErr: ErrNumberTooWide},
		{name: "too_wide_21_digits", input: "184467440737095516150 ", wantErr: ErrNumberTooWide},
		{name: "missing_space_at_eof", input: "42", wantErr: ErrMissingSpace, spaceOnly: true},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !tc.spaceOnly {
				var b Buffer
				b.AdoptString(tc.input)
				_, err := b.ReadUint()
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, 0, b.Pos(), "ReadUint must not move cursor on error")
			}
			var bs Buffer
			bs.AdoptString(tc.input)
			_, err := bs.ReadUintSpace()
			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, 0, bs.Pos(), "ReadUintSpace must not move cursor on error")
		})
	}

	t.Run("ReadSpaceErrors", func(t *testing.T) {
		t.Parallel()
		var empty Buffer
		empty.AdoptString("")
		assert.ErrorIs(t, empty.ReadSpace(), ErrMissingSpace)
		assert.Equal(t, 0, empty.Pos())

		var tab Buffer
		tab.AdoptString("\t")
		assert.ErrorIs(t, tab.ReadSpace(), ErrMissingSpace)
		assert.Equal(t, 0, tab.Pos())
	})
}

// VALIDATES: Counted field parsing for empty values, UTF-8 byte counts,
// embedded delimiters, partial header peeking, and every error boundary.
// PREVENTS: rune-vs-byte miscounts, delimiter confusion inside payloads,
// out-of-bounds slice panics, or partial cursor advances on error.
func TestBufferReadCountedBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("EmptyCounted", func(t *testing.T) {
		t.Parallel()
		var bb Buffer
		bb.Adopt([]byte("0: 0:"))
		v1, err := bb.ReadCountedBytesSpace()
		require.NoError(t, err)
		assert.Empty(t, v1)
		v2, err := bb.ReadCountedBytesEnd()
		require.NoError(t, err)
		assert.Empty(t, v2)
		assert.True(t, bb.EOF())

		var bs Buffer
		bs.AdoptString("0: 0:")
		s1, err := bs.ReadCountedStringSpace()
		require.NoError(t, err)
		assert.Equal(t, "", s1)
		s2, err := bs.ReadCountedStringEnd()
		require.NoError(t, err)
		assert.Equal(t, "", s2)
		assert.True(t, bs.EOF())
	})

	t.Run("UTF8ByteCountAndEmbeddedDelimiters", func(t *testing.T) {
		t.Parallel()
		utf8Val := "héllo" // 6 bytes, 5 runes
		delimVal := "a b\r\n:c\x00"

		var w Buffer
		w.Reset().CountedStr(utf8Val).Byte(' ').CountedStr(delimVal)
		encoded := append([]byte(nil), w.Bytes()...)

		var rb Buffer
		rb.Adopt(encoded)
		got1, err := rb.ReadCountedBytesSpace()
		require.NoError(t, err)
		assert.Equal(t, []byte(utf8Val), got1)
		got2, err := rb.ReadCountedBytesEnd()
		require.NoError(t, err)
		assert.Equal(t, []byte(delimVal), got2)

		var rs Buffer
		rs.AdoptString(string(encoded))
		s1, err := rs.ReadCountedStringSpace()
		require.NoError(t, err)
		assert.Equal(t, utf8Val, s1)
		s2, err := rs.ReadCountedStringEnd()
		require.NoError(t, err)
		assert.Equal(t, delimVal, s2)
	})

	t.Run("PeekCountedHeaderPartialPayload", func(t *testing.T) {
		t.Parallel()
		var b Buffer
		b.AdoptString("100:abc")
		size, headerLen, err := b.PeekCountedHeader()
		require.NoError(t, err)
		assert.Equal(t, uint64(100), size)
		assert.Equal(t, 4, headerLen)
		assert.Equal(t, 0, b.Pos(), "PeekCountedHeader must not advance cursor")

		_, err = b.ReadCountedString()
		assert.ErrorIs(t, err, ErrCountOutOfBounds)
		assert.Equal(t, 0, b.Pos())
	})

	errorCases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "empty_input", input: "", wantErr: ErrEmptyCount},
		{name: "leading_colon", input: ":abc", wantErr: ErrEmptyCount},
		{name: "leading_space", input: " 1:a", wantErr: ErrEmptyCount},
		{name: "missing_colon_word", input: "5hello", wantErr: ErrMissingColon},
		{name: "missing_colon_space", input: "0 ", wantErr: ErrMissingColon},
		{name: "missing_colon_eof", input: "0", wantErr: ErrMissingColon},
		{name: "invalid_digit_alpha", input: "abc", wantErr: ErrInvalidDigit},
		{name: "invalid_digit_minus", input: "-1:a", wantErr: ErrInvalidDigit},
		{name: "too_wide", input: "000000000000000000001:a", wantErr: ErrNumberTooWide},
		{name: "overflow", input: "18446744073709551616:a", wantErr: ErrNumberOverflow},
		{name: "out_of_bounds_small", input: "99:abc", wantErr: ErrCountOutOfBounds},
		{name: "out_of_bounds_max_uint64", input: "18446744073709551615:a", wantErr: ErrCountOutOfBounds},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var bb Buffer
			bb.Adopt([]byte(tc.input))
			_, err := bb.ReadCountedBytes()
			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, 0, bb.Pos(), "ReadCountedBytes must leave cursor unchanged on error")

			var bs Buffer
			bs.AdoptString(tc.input)
			_, err = bs.ReadCountedString()
			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, 0, bs.Pos(), "ReadCountedString must leave cursor unchanged on error")
		})
	}

	t.Run("MissingSpaceAndTrailingBytesRestoreCursor", func(t *testing.T) {
		t.Parallel()
		var bb Buffer
		bb.Adopt([]byte("3:abc"))
		_, err := bb.ReadCountedBytesSpace()
		assert.ErrorIs(t, err, ErrMissingSpace)
		assert.Equal(t, 0, bb.Pos())

		var bs Buffer
		bs.AdoptString("3:abc")
		_, err = bs.ReadCountedStringSpace()
		assert.ErrorIs(t, err, ErrMissingSpace)
		assert.Equal(t, 0, bs.Pos())

		var tb Buffer
		tb.Adopt([]byte("3:abc extra"))
		_, err = tb.ReadCountedBytesEnd()
		assert.ErrorIs(t, err, ErrTrailingBytes)
		assert.Equal(t, 0, tb.Pos())

		var ts Buffer
		ts.AdoptString("3:abc extra")
		_, err = ts.ReadCountedStringEnd()
		assert.ErrorIs(t, err, ErrTrailingBytes)
		assert.Equal(t, 0, ts.Pos())
	})
}

// VALIDATES: Full positional wire line parsing executes with zero heap
// allocations for both Adopt([]byte) and AdoptString(string).
// PREVENTS: escape-analysis regressions that silently allocate Buffer or
// field views on the hot parse path.
func TestBufferParseZeroAlloc(t *testing.T) {
	verbUpd := MakeWord4("upd")
	lineStr := "upd 14 65000 65001 42 10.0.0.1 10.0.0.2 1.1.1.1 - 40:AAAbQAEBAAJAAgAEAAH96EADAQoAAAEAAhgKAAA="
	lineBytes := []byte(lineStr)

	var sink uint64

	allocsBytes := testing.AllocsPerRun(100, func() {
		var b Buffer
		b.Adopt(lineBytes)
		w, ok := b.ReadWord4()
		if !ok {
			t.FailNow()
		}
		if w != verbUpd {
			t.FailNow()
		}
		u1, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		u2, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		u3, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		u4, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		c1, err := b.ReadTokenBytesSpace()
		if err != nil {
			t.FailNow()
		}
		c2, err := b.ReadTokenBytesSpace()
		if err != nil {
			t.FailNow()
		}
		c3, err := b.ReadTokenBytesSpace()
		if err != nil {
			t.FailNow()
		}
		c4, err := b.ReadTokenBytesSpace()
		if err != nil {
			t.FailNow()
		}
		c5, err := b.ReadCountedBytesEnd()
		if err != nil {
			t.FailNow()
		}
		sink = u1 + u2 + u3 + u4 + uint64(len(c1)+len(c2)+len(c3)+len(c4)+len(c5))
	})
	assert.Equal(t, 0.0, allocsBytes, "Adopt([]byte) full parse must allocate 0")
	assert.Positive(t, sink)

	allocsStr := testing.AllocsPerRun(100, func() {
		var b Buffer
		b.AdoptString(lineStr)
		w, ok := b.ReadWord4()
		if !ok {
			t.FailNow()
		}
		if w != verbUpd {
			t.FailNow()
		}
		u1, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		u2, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		u3, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		u4, err := b.ReadUintSpace()
		if err != nil {
			t.FailNow()
		}
		c1, err := b.ReadTokenStringSpace()
		if err != nil {
			t.FailNow()
		}
		c2, err := b.ReadTokenStringSpace()
		if err != nil {
			t.FailNow()
		}
		c3, err := b.ReadTokenStringSpace()
		if err != nil {
			t.FailNow()
		}
		c4, err := b.ReadTokenStringSpace()
		if err != nil {
			t.FailNow()
		}
		c5, err := b.ReadCountedStringEnd()
		if err != nil {
			t.FailNow()
		}
		sink = u1 + u2 + u3 + u4 + uint64(len(c1)+len(c2)+len(c3)+len(c4)+len(c5))
	})
	assert.Equal(t, 0.0, allocsStr, "AdoptString(string) full parse must allocate 0")
	assert.Positive(t, sink)
}

// VALIDATES: Hostile byte sequences never panic any Buffer wire parser method
// under either Adopt([]byte) or AdoptString(string).
// PREVENTS: out-of-bounds slicing, integer overflow, or unsafe slice panics
// on untrusted wire input.
func FuzzBufferParseCounted(f *testing.F) {
	f.Add([]byte("upd 14 65000 65001 42 10.0.0.1 10.0.0.2 1.1.1.1 - 8:AAAAAA==\n"))
	f.Add([]byte("row 42 1 1700000000 7 8:10.0.0.1 11:10.0.0.0/24 0: 8:deadbeef 5:attrs\n"))
	f.Add([]byte(""))
	f.Add([]byte("\n"))
	f.Add([]byte("upd "))
	f.Add([]byte("row 18446744073709551615 0: 1:a"))
	f.Add([]byte("row 18446744073709551616 18446744073709551615:a"))
	f.Add([]byte("000000000000000000001:abc"))
	f.Add([]byte("5:ab"))
	f.Add([]byte(":"))
	f.Add([]byte("5hello"))
	f.Add([]byte("row 5:notnum 0:"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var bb Buffer
		bb.Adopt(data)
		bb.TrimTrailingLF()
		_, _ = bb.PeekWord4()
		_, _ = bb.ReadWord4()
		_, _ = bb.ReadUintSpace()
		_, _ = bb.ReadUint()
		_, _ = bb.ReadIPv4Space()
		_, _ = bb.ReadIPv4()
		_, _ = bb.ReadTokenBytesSpace()
		_, _ = bb.ReadTokenBytes()
		_, _ = bb.ReadTokenBytesEnd()
		_ = bb.ReadSpace()
		_, _, _ = bb.PeekCountedHeader()
		_, _ = bb.ReadCountedBytesSpace()
		_, _ = bb.ReadCountedBytes()
		_, _ = bb.ReadCountedBytesEnd()
		_ = bb.RestBytes()
		if bb.Pos() < 0 {
			t.Fatalf("negative cursor: %d", bb.Pos())
		}
		if bb.Pos() > len(data) {
			t.Fatalf("cursor %d exceeds input length %d", bb.Pos(), len(data))
		}

		var bs Buffer
		bs.AdoptString(string(data))
		bs.TrimTrailingLF()
		_, _ = bs.PeekWord4()
		_, _ = bs.ReadWord4()
		_, _ = bs.ReadUintSpace()
		_, _ = bs.ReadUint()
		_, _ = bs.ReadIPv4Space()
		_, _ = bs.ReadIPv4()
		_, _ = bs.ReadTokenStringSpace()
		_, _ = bs.ReadTokenString()
		_, _ = bs.ReadTokenStringEnd()
		_ = bs.ReadSpace()
		_, _, _ = bs.PeekCountedHeader()
		_, _ = bs.ReadCountedStringSpace()
		_, _ = bs.ReadCountedString()
		_, _ = bs.ReadCountedStringEnd()
		_ = bs.RestString()
		if bs.Pos() < 0 {
			t.Fatalf("negative cursor: %d", bs.Pos())
		}
		if bs.Pos() > len(data) {
			t.Fatalf("cursor %d exceeds input length %d", bs.Pos(), len(data))
		}
	})
}

// BenchmarkBufferParseUpdLineBytes measures positional upd line parsing via Adopt([]byte).
func BenchmarkBufferParseUpdLineBytes(b *testing.B) {
	verbUpd := MakeWord4("upd")
	line := []byte("upd 14 65000 65001 42 10.0.0.1 10.0.0.2 1.1.1.1 - 40:AAAbQAEBAAJAAgAEAAH96EADAQoAAAEAAhgKAAA=")
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))

	var sink uint64
	for b.Loop() {
		var buf Buffer
		buf.Adopt(line)
		w, ok := buf.ReadWord4()
		if !ok {
			b.Fatal("short verb")
		}
		if w != verbUpd {
			b.Fatal("wrong verb")
		}
		u1, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		u2, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		u3, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		u4, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		c1, err := buf.ReadTokenBytesSpace()
		if err != nil {
			b.Fatal(err)
		}
		c2, err := buf.ReadTokenBytesSpace()
		if err != nil {
			b.Fatal(err)
		}
		c3, err := buf.ReadTokenBytesSpace()
		if err != nil {
			b.Fatal(err)
		}
		c4, err := buf.ReadTokenBytesSpace()
		if err != nil {
			b.Fatal(err)
		}
		c5, err := buf.ReadCountedBytesEnd()
		if err != nil {
			b.Fatal(err)
		}
		sink = u1 + u2 + u3 + u4 + uint64(len(c1)+len(c2)+len(c3)+len(c4)+len(c5))
	}
	if sink == 0 {
		b.Fatal("unexpected zero sink")
	}
}

// BenchmarkBufferParseUpdLineString measures positional upd line parsing via AdoptString(string).
func BenchmarkBufferParseUpdLineString(b *testing.B) {
	verbUpd := MakeWord4("upd")
	line := "upd 14 65000 65001 42 10.0.0.1 10.0.0.2 1.1.1.1 - 40:AAAbQAEBAAJAAgAEAAH96EADAQoAAAEAAhgKAAA="
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))

	var sink uint64
	for b.Loop() {
		var buf Buffer
		buf.AdoptString(line)
		w, ok := buf.ReadWord4()
		if !ok {
			b.Fatal("short verb")
		}
		if w != verbUpd {
			b.Fatal("wrong verb")
		}
		u1, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		u2, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		u3, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		u4, err := buf.ReadUintSpace()
		if err != nil {
			b.Fatal(err)
		}
		c1, err := buf.ReadTokenStringSpace()
		if err != nil {
			b.Fatal(err)
		}
		c2, err := buf.ReadTokenStringSpace()
		if err != nil {
			b.Fatal(err)
		}
		c3, err := buf.ReadTokenStringSpace()
		if err != nil {
			b.Fatal(err)
		}
		c4, err := buf.ReadTokenStringSpace()
		if err != nil {
			b.Fatal(err)
		}
		c5, err := buf.ReadCountedStringEnd()
		if err != nil {
			b.Fatal(err)
		}
		sink = u1 + u2 + u3 + u4 + uint64(len(c1)+len(c2)+len(c3)+len(c4)+len(c5))
	}
	if sink == 0 {
		b.Fatal("unexpected zero sink")
	}
}

// VALIDATES: String() detaches a read-only backing slice even when TrimTrailingLF
// has shortened len(b.b) to zero while cap(b.b) remains non-zero.
// PREVENTS: Bytes(), Reset(n), or pool Get()/Release() aliasing and mutating
// immutable Go string memory after a read-only buffer is emptied by String().
func TestBufferReadOnlyStringAfterTrimToEmpty(t *testing.T) {
	t.Parallel()

	// Single-byte string trimmed to len=0, cap=1.
	var b Buffer
	b.AdoptString("\n")
	require.True(t, b.TrimTrailingLF())
	assert.Equal(t, "", b.String())
	bs := b.Bytes()
	assert.Equal(t, 0, len(bs))
	assert.Equal(t, 0, cap(bs), "String() must detach read-only backing slice even when len == 0")

	// Large heap string (>128B inline) trimmed to len=0, cap=200.
	orig := strings.Repeat("\n", 200)
	b.AdoptString(orig)
	for b.TrimTrailingLF() {
	}
	assert.Equal(t, "", b.String())
	b.Reset(200).Str("corrupt")
	assert.Equal(t, strings.Repeat("\n", 200), orig, "Reset(200) must not reuse immutable string backing array")

	// Pooled buffer path: AdoptString -> trim to empty -> String -> Release -> Get.
	pb := Get()
	origPooled := strings.Repeat("\n", 200)
	pb.AdoptString(origPooled)
	for pb.TrimTrailingLF() {
	}
	assert.Equal(t, "", pb.String())
	assert.Equal(t, 0, cap(pb.b), "pooled buffer must not retain read-only string capacity across Release")
	pb.Release()
}

// VALIDATES: ReadIPv4 and ReadIPv4Space parse dotted-quad IPv4 addresses into
// big-endian uint32 in one pass with zero allocations, reject leading zeros,
// out-of-range octets, wrong dot counts, and non-digit bytes without moving
// the cursor, and match netip.ParseAddr across all valid boundary values.
// PREVENTS: octal ambiguity, silent acceptance of truncated or overflowing
// octets, or heap allocations when parsing IPv4 fields on hot wire paths.
func TestBufferReadIPv4(t *testing.T) {
	t.Parallel()

	validCases := []struct {
		input string
		want  uint32
	}{
		{"0.0.0.0", 0x00000000},
		{"1.1.1.1", 0x01010101},
		{"10.0.0.1", 0x0a000001},
		{"127.0.0.1", 0x7f000001},
		{"192.168.0.255", 0xc0a800ff},
		{"255.255.255.255", 0xffffffff},
	}
	for _, tc := range validCases {
		t.Run("EOF_"+tc.input, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(tc.input)
			got, err := b.ReadIPv4()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.True(t, b.EOF())
		})
		t.Run("Space_"+tc.input, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(tc.input + " rest")
			got, err := b.ReadIPv4Space()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, "rest", b.RestString())
		})
	}

	// Chained reads across multiple IPv4 fields.
	var chain Buffer
	chain.AdoptString("10.0.0.1 10.0.0.2 1.1.1.1")
	p1, err := chain.ReadIPv4Space()
	require.NoError(t, err)
	p2, err := chain.ReadIPv4Space()
	require.NoError(t, err)
	p3, err := chain.ReadIPv4()
	require.NoError(t, err)
	assert.Equal(t, uint32(0x0a000001), p1)
	assert.Equal(t, uint32(0x0a000002), p2)
	assert.Equal(t, uint32(0x01010101), p3)
	assert.True(t, chain.EOF())

	// Missing trailing space leaves cursor unchanged.
	var noSpace Buffer
	noSpace.AdoptString("10.0.0.1")
	_, err = noSpace.ReadIPv4Space()
	require.ErrorIs(t, err, ErrMissingSpace)
	assert.Equal(t, 0, noSpace.Pos())

	invalidInputs := []string{
		"",
		" ",
		"1",
		"1.2.3",
		"1.2.3 ",
		"1.2.3.",
		"1.2.3.4.",
		"1.2.3.4.5",
		".1.2.3",
		"1..2.3",
		"256.0.0.1",
		"1.256.0.1",
		"1.2.256.1",
		"1.2.3.256",
		"999.0.0.1",
		"01.0.0.1",
		"1.00.0.1",
		"1.2.3.04",
		"10.0.0.1a",
		"-1.0.0.1",
		"10.0.0.1\t",
		"2001:db8::1",
	}
	for _, bad := range invalidInputs {
		t.Run("Invalid_"+bad, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(bad)
			_, err := b.ReadIPv4()
			require.ErrorIs(t, err, ErrInvalidIPv4)
			assert.Equal(t, 0, b.Pos(), "ReadIPv4 must not move cursor on error")

			var bs Buffer
			bs.AdoptString(bad)
			_, err = bs.ReadIPv4Space()
			require.ErrorIs(t, err, ErrInvalidIPv4)
			assert.Equal(t, 0, bs.Pos(), "ReadIPv4Space must not move cursor on error")
		})
	}
}

// VALIDATES: ReadIPv4 allocates zero bytes on the heap per call.
// PREVENTS: accidental escape of Buffer or temporary slices during IPv4 parsing.
func TestBufferReadIPv4ZeroAlloc(t *testing.T) {
	const input = "192.168.100.200 10.0.0.1"
	var sink uint32
	allocs := testing.AllocsPerRun(100, func() {
		var b Buffer
		b.AdoptString(input)
		a, err := b.ReadIPv4Space()
		if err != nil {
			t.FailNow()
		}
		c, err := b.ReadIPv4()
		if err != nil {
			t.FailNow()
		}
		sink = a ^ c
	})
	assert.Equal(t, 0.0, allocs)
	assert.NotZero(t, sink)
}

// VALIDATES: ReadTokenBytes* and ReadTokenString* parse space-delimited tokens
// zero-copy with 3-index slice cap isolation, reject empty tokens, control
// bytes, missing spaces, and trailing bytes without advancing the cursor.
// PREVENTS: cross-field slice overwrites, control-byte injection, or silent
// acceptance of double spaces on positional token wire lines.
func TestBufferReadToken(t *testing.T) {
	t.Parallel()

	raw := []byte("10.0.0.1 2001:db8::1 - AAAbQAE=")
	var rb Buffer
	rb.Adopt(raw)

	tok1, err := rb.ReadTokenBytesSpace()
	require.NoError(t, err)
	assert.Equal(t, []byte("10.0.0.1"), tok1)
	assert.Same(t, &raw[0], &tok1[0])
	assert.Equal(t, len(tok1), cap(tok1))
	tok1 = append(tok1, '!')
	assert.Equal(t, byte(' '), raw[len("10.0.0.1")], "append must not overwrite following space")

	tok2, err := rb.ReadTokenBytesSpace()
	require.NoError(t, err)
	assert.Equal(t, []byte("2001:db8::1"), tok2)

	tok3, err := rb.ReadTokenBytesSpace()
	require.NoError(t, err)
	assert.Equal(t, []byte("-"), tok3)

	tok4, err := rb.ReadTokenBytesEnd()
	require.NoError(t, err)
	assert.Equal(t, []byte("AAAbQAE="), tok4)
	assert.True(t, rb.EOF())

	// ReadTokenString* on immutable string.
	line := "10.0.0.1 2001:db8::1 - AAAbQAE="
	var rs Buffer
	rs.AdoptString(line)
	s1, err := rs.ReadTokenStringSpace()
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", s1)
	assert.Same(t, unsafe.StringData(line), unsafe.StringData(s1))
	s2, err := rs.ReadTokenStringSpace()
	require.NoError(t, err)
	assert.Equal(t, "2001:db8::1", s2)
	s3, err := rs.ReadTokenStringSpace()
	require.NoError(t, err)
	assert.Equal(t, "-", s3)
	s4, err := rs.ReadTokenStringEnd()
	require.NoError(t, err)
	assert.Equal(t, "AAAbQAE=", s4)
	assert.True(t, rs.EOF())

	// ReadTokenString stops before space without consuming it.
	var peek Buffer
	peek.AdoptString("abc def")
	head, err := peek.ReadTokenString()
	require.NoError(t, err)
	assert.Equal(t, "abc", head)
	assert.Equal(t, " def", peek.RestString())

	// Error cases leave cursor unchanged.
	errCases := []struct {
		name  string
		input string
		call  func(b *Buffer) error
		want  error
	}{
		{"empty_eof", "", func(b *Buffer) error { _, err := b.ReadTokenString(); return err }, ErrEmptyToken},
		{"leading_space", " abc", func(b *Buffer) error { _, err := b.ReadTokenStringSpace(); return err }, ErrEmptyToken},
		{"missing_space", "abc", func(b *Buffer) error { _, err := b.ReadTokenStringSpace(); return err }, ErrMissingSpace},
		{"trailing_bytes", "abc def", func(b *Buffer) error { _, err := b.ReadTokenStringEnd(); return err }, ErrTrailingBytes},
		{"trailing_space_on_end", "abc ", func(b *Buffer) error { _, err := b.ReadTokenStringEnd(); return err }, ErrTrailingBytes},
		{"embedded_tab", "ab\tc", func(b *Buffer) error { _, err := b.ReadTokenString(); return err }, ErrInvalidTokenByte},
		{"embedded_cr", "ab\rc", func(b *Buffer) error { _, err := b.ReadTokenString(); return err }, ErrInvalidTokenByte},
		{"embedded_lf", "ab\nc", func(b *Buffer) error { _, err := b.ReadTokenString(); return err }, ErrInvalidTokenByte},
		{"embedded_del", "ab\x7fc", func(b *Buffer) error { _, err := b.ReadTokenString(); return err }, ErrInvalidTokenByte},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b Buffer
			b.AdoptString(tc.input)
			err := tc.call(&b)
			require.ErrorIs(t, err, tc.want)
			assert.Equal(t, 0, b.Pos())
		})
	}
}

// FuzzBufferReadIPv4 verifies ReadIPv4 agrees with netip.ParseAddr on every
// single-token input without spaces and never panics on arbitrary bytes.
func FuzzBufferReadIPv4(f *testing.F) {
	f.Add("0.0.0.0")
	f.Add("10.0.0.1")
	f.Add("255.255.255.255")
	f.Add("01.0.0.1")
	f.Add("256.0.0.1")
	f.Add("1.2.3")
	f.Add("1.2.3.4.5")
	f.Add("2001:db8::1")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		var b Buffer
		b.AdoptString(input)
		got, err := b.ReadIPv4()
		if strings.ContainsRune(input, ' ') {
			return
		}
		addr, stdErr := netip.ParseAddr(input)
		stdOK := stdErr == nil && addr.Is4()
		if !stdOK {
			if err == nil {
				t.Fatalf("ReadIPv4(%q) accepted non-IPv4 input as %#08x", input, got)
			}
			return
		}
		if err != nil {
			t.Fatalf("ReadIPv4(%q) rejected valid IPv4: %v", input, err)
		}
		if !b.EOF() {
			t.Fatalf("ReadIPv4(%q) did not reach EOF: pos=%d len=%d", input, b.Pos(), len(input))
		}
		b4 := addr.As4()
		want := binary.BigEndian.Uint32(b4[:])
		if got != want {
			t.Fatalf("ReadIPv4(%q) = %#08x, want %#08x", input, got, want)
		}
	})
}
