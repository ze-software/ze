package mrt_test

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

// rawBGPMessage builds a message whose declared length may deliberately differ
// from the buffer length, so length-field boundaries can be exercised.
func rawBGPMessage(declaredLen uint16, msgType byte, bufLen int) []byte {
	msg := make([]byte, bufLen)
	for i := range min(16, bufLen) {
		msg[i] = 0xff
	}
	if bufLen >= 18 {
		binary.BigEndian.PutUint16(msg[16:18], declaredLen)
	}
	if bufLen >= 19 {
		msg[18] = msgType
	}
	return msg
}

// parseNoPanic parses and reports whether the call returned a usable result,
// asserting the parser's core invariant: a nil error implies a non-nil message.
func parseNoPanic(t *testing.T, data []byte) {
	t.Helper()
	parsed, err := mrt.ParseBGPMessage(data)
	if err == nil && parsed == nil {
		t.Fatalf("ParseBGPMessage returned nil message and nil error for %x", data)
	}
}

func TestParseBGPMessage_LengthBoundaries(t *testing.T) {
	// VALIDATES: the message Length field boundary. RFC 4271 Section 4.1 sets the
	// floor at 19; RFC 8654 raises the ceiling to 65535, and MRT captures of
	// extended-message sessions legitimately carry lengths above 4096.
	// PREVENTS: (a) accepting a sub-header length, which would slice a negative
	// body; (b) rejecting valid RFC 8654 extended messages recorded in a dump.
	tests := []struct {
		name      string
		declared  uint16
		bufLen    int
		wantError bool
	}{
		{"length 18 is below the 19-octet header floor", 18, 19, true},
		{"length 19 is the exact header-only minimum", 19, 19, false},
		{"length 4096 is the RFC 4271 ceiling", 4096, 4096, false},
		{"length 4097 is valid under RFC 8654", 4097, 4097, false},
		{"length 65535 is the RFC 8654 ceiling", 65535, 65535, false},
		{"declared length beyond the buffer is rejected", 100, 19, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := rawBGPMessage(tt.declared, 4, tt.bufLen) // KEEPALIVE
			parsed, err := mrt.ParseBGPMessage(msg)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, parsed)
			assert.Equal(t, uint8(4), parsed.Type)
		})
	}
}

func TestParseBGPMessage_TruncatedAtEveryLengthField(t *testing.T) {
	// VALIDATES: every prefix of a well-formed UPDATE either parses or errors,
	// but never panics. Each truncation point lands inside a different length
	// field (withdrawn length, attribute length, attribute value).
	// PREVENTS: index-out-of-range on a dump truncated mid-record, which is the
	// normal shape of an interrupted collector file.
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN IGP
	body := make([]byte, 2+2+len(attrs)+2)
	binary.BigEndian.PutUint16(body[0:], 0)
	binary.BigEndian.PutUint16(body[2:], uint16(len(attrs)))
	copy(body[4:], attrs)
	body[4+len(attrs)] = 8
	body[5+len(attrs)] = 10
	full := buildBGPMessage(2, body)

	for n := range full {
		truncated := full[:n]
		assert.NotPanics(t, func() { parseNoPanic(t, truncated) },
			"truncating to %d bytes must not panic", n)
	}
}

func TestParseBGPMessage_UpdateTruncatedLengthFields(t *testing.T) {
	// VALIDATES: an UPDATE whose withdrawn-length or attribute-length field
	// overruns the body is rejected with an error.
	// PREVENTS: a malformed length silently yielding a short read that looks
	// like an empty UPDATE.
	tests := []struct {
		name string
		body []byte
	}{
		{"withdrawn length overruns body", []byte{0xFF, 0xFF, 0, 0}},
		{"attribute length overruns body", []byte{0, 0, 0xFF, 0xFF}},
		{"body shorter than the 4 fixed octets", []byte{0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mrt.ParseBGPMessage(buildBGPMessage(2, tt.body))
			require.Error(t, err)
		})
	}
}

func TestParseBGPMessage_ZeroLengthAttributeSet(t *testing.T) {
	// VALIDATES: an UPDATE with no withdrawn routes and a zero-length attribute
	// set parses to an empty-but-present ParsedUpdate.
	// PREVENTS: treating a legal end-of-RIB style UPDATE as malformed.
	body := []byte{0, 0, 0, 0}
	parsed, err := mrt.ParseBGPMessage(buildBGPMessage(2, body))
	require.NoError(t, err)
	require.NotNil(t, parsed.Update)
	assert.Empty(t, parsed.Update.WithdrawnPrefixes)
	assert.Empty(t, parsed.Update.Attributes)
	assert.Empty(t, parsed.Update.AnnouncedPrefixes)
}

func TestParseASPath_TruncatedSegment(t *testing.T) {
	// VALIDATES: an AS_PATH segment whose count overruns the value is rejected.
	// PREVENTS: reading ASNs past the attribute boundary.
	_, err := mrt.ParseASPath([]byte{2, 4, 0, 0, 0, 1}, true) // claims 4 ASNs, has 1
	require.Error(t, err)
}

func TestParseASPath_Empty(t *testing.T) {
	// VALIDATES: an empty AS_PATH (a legal IBGP-originated path) yields no
	// segments and no error.
	// PREVENTS: rejecting locally originated routes.
	segs, err := mrt.ParseASPath(nil, true)
	require.NoError(t, err)
	assert.Empty(t, segs)
}

func TestParseASPath_WidthChangesResult(t *testing.T) {
	// VALIDATES: identical bytes decode to a different result at 2-byte and
	// 4-byte width, which is exactly why RFC 6396 fixes the width per record
	// type and why ParseASPath takes it as a parameter.
	// PREVENTS: any future change that guesses the width from the data. The
	// RFC 6396 pitfall note calls an AS-width mismatch "silent corruption".
	data := []byte{2, 2, 0, 0, 0xFD, 0xE8, 0, 0, 0xFD, 0xE9}

	four, err := mrt.ParseASPath(data, true)
	require.NoError(t, err)
	require.Len(t, four, 1)
	assert.Equal(t, []uint32{65000, 65001}, four[0].ASNs)

	// Read at the wrong width the same bytes must not yield the same path.
	// Here the misread runs off the end, so it errors; either way the result
	// differs from the correct decode.
	two, err := mrt.ParseASPath(data, false)
	if err == nil {
		require.Len(t, two, 1)
		assert.NotEqual(t, []uint32{65000, 65001}, two[0].ASNs)
	}

	// A genuine 2-byte path decodes correctly at 2-byte width and yields a
	// different AS set at 4-byte width.
	twoByteWire := []byte{2, 2, 0xFD, 0xE8, 0xFD, 0xE9}
	got, err := mrt.ParseASPath(twoByteWire, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, []uint32{65000, 65001}, got[0].ASNs)

	wrong, err := mrt.ParseASPath(twoByteWire, true)
	if err == nil {
		for _, seg := range wrong {
			assert.NotEqual(t, []uint32{65000, 65001}, seg.ASNs)
		}
	}
}

func TestParseAttributes_ExtendedLength(t *testing.T) {
	// VALIDATES: the extended-length flag (0x10) selects a 2-octet length field.
	// PREVENTS: misreading long attributes such as a large AS_PATH or MP_REACH.
	value := make([]byte, 300)
	data := make([]byte, 4+len(value))
	data[0] = 0x50 // optional + extended length
	data[1] = 2    // AS_PATH
	binary.BigEndian.PutUint16(data[2:], uint16(len(value)))
	copy(data[4:], value)

	attrs := mrt.ParseAttributes(data)
	require.Len(t, attrs, 1)
	assert.Len(t, attrs[0].Value, 300)
}

func TestParseAttributes_TruncatedNeverPanics(t *testing.T) {
	// VALIDATES: every prefix of a valid attribute section is safe to parse.
	// PREVENTS: panics on attribute sections cut short by a truncated record.
	data := []byte{
		0x40, 1, 1, 0x00,
		0x40, 5, 4, 0, 0, 0, 100,
		0x50, 2, 0x01, 0x2C,
	}
	for n := range data {
		assert.NotPanics(t, func() {
			// Every decoded attribute must lie wholly inside the input.
			for _, a := range mrt.ParseAttributes(data[:n]) {
				assert.LessOrEqual(t, len(a.Value), n)
			}
		}, "truncating to %d bytes must not panic", n)
	}
}

// FuzzParseBGPMessage fuzzes the complete-message entry point.
// MRT files are external input (downloaded from public route collectors), so
// the parser must never panic on arbitrary bytes.
func FuzzParseBGPMessage(f *testing.F) {
	f.Add(buildBGPMessage(4, nil))
	f.Add(buildBGPMessage(2, []byte{0, 0, 0, 0}))
	f.Add(buildBGPMessage(3, []byte{6, 2, 0xDE, 0xAD}))
	f.Add(buildBGPMessage(1, []byte{4, 0, 100, 0, 90, 1, 2, 3, 4, 0}))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		parsed, err := mrt.ParseBGPMessage(data)
		if err == nil && parsed == nil {
			t.Fatalf("nil message with nil error for %x", data)
		}
	})
}

// FuzzParseAttributes fuzzes the path-attribute walker and every attribute
// decoder that runs over an attribute value.
func FuzzParseAttributes(f *testing.F) {
	f.Add([]byte{0x40, 1, 1, 0x00})
	f.Add([]byte{0x50, 2, 0x01, 0x2C})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, a := range mrt.ParseAttributes(data) {
			if len(a.Value) > len(data) {
				t.Fatalf("attribute value %d longer than input %d", len(a.Value), len(data))
			}
			if mp, err := mrt.ParseMPReach(a.Value); err == nil && mp == nil {
				t.Fatal("ParseMPReach: nil result with nil error")
			}
			if mp, err := mrt.ParseMPUnreach(a.Value); err == nil && mp == nil {
				t.Fatal("ParseMPUnreach: nil result with nil error")
			}
			if addr, err := mrt.ParseMPReachRIBEntry(a.Value); err == nil && !addr.IsValid() {
				t.Fatal("ParseMPReachRIBEntry: invalid address with nil error")
			}
		}
	})
}

// FuzzParseASPath fuzzes AS_PATH decoding at both RFC 6396 widths.
func FuzzParseASPath(f *testing.F) {
	f.Add([]byte{2, 3, 0, 0, 0xFD, 0xE8, 0, 0, 0xFD, 0xE9, 0, 0, 0xFD, 0xEA}, true)
	f.Add([]byte{2, 2, 0xFD, 0xE8, 0xFD, 0xE9}, false)
	f.Add([]byte{}, true)

	f.Fuzz(func(t *testing.T, data []byte, fourByte bool) {
		segs, err := mrt.ParseASPath(data, fourByte)
		if err != nil {
			return
		}
		width := 2
		if fourByte {
			width = 4
		}
		total := 0
		for _, s := range segs {
			total += 2 + len(s.ASNs)*width
		}
		if total > len(data) {
			t.Fatalf("decoded %d bytes of segments from %d bytes of input", total, len(data))
		}
	})
}

// FuzzParsePrefixes fuzzes NLRI decoding for both address families.
func FuzzParsePrefixes(f *testing.F) {
	f.Add([]byte{8, 10, 16, 192, 168}, uint16(1), false)
	f.Add([]byte{32, 0x20, 0x01, 0x0d, 0xb8}, uint16(2), false)
	f.Add([]byte{0, 0, 0, 7, 32, 0x20, 0x01, 0x0d, 0xb8}, uint16(2), true)

	f.Fuzz(func(t *testing.T, data []byte, afi uint16, addPath bool) {
		// Prefixes decoded before any damage are still returned alongside the
		// error, so both halves of the result must hold the invariant.
		pfxs, _ := mrt.ParsePrefixesAFI(data, afi, addPath) //nolint:errcheck // a damaged NLRI still returns its good prefixes
		for _, p := range pfxs {
			// A decoder must never emit an invalid prefix: downstream code reads
			// netip's zero Prefix as 0.0.0.0/0.
			if !p.IsValid() {
				t.Fatalf("ParsePrefixesAFI emitted an invalid prefix from %x (afi=%d)", data, afi)
			}
		}
	})
}
