// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- unit tests for pure-stdlib pprof summary parser
// Related: profile_summary.go -- summarizePprof implementation

package show

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"runtime/pprof"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProfileSummaryAllTypes feeds real Go runtime/pprof output for all six
// supported profile types into summarizePprof and verifies the structured
// summary contract.
//
// VALIDATES: AC-1 -- summarizePprof decodes real heap, goroutine, allocs,
// mutex, block, and cpu profiles into bounded top-functions JSON maps with
// format "summary", sample-type, sample-unit, total, top-functions, and
// truncated fields.
// PREVENTS: protobuf wire-decoding regressions on any Go profile type or
// broken summary response structure.
func TestProfileSummaryAllTypes(t *testing.T) {
	for _, profileType := range []string{profileTypeHeap, profileTypeGoroutine, profileTypeAllocs, profileTypeMutex, profileTypeBlock} {
		t.Run(profileType, func(t *testing.T) {
			p := pprof.Lookup(profileType)
			require.NotNil(t, p, "pprof.Lookup(%q) returned nil", profileType)

			var buf bytes.Buffer
			require.NoError(t, p.WriteTo(&buf, 0))

			summary, err := summarizePprof(profileType, buf.Bytes())
			require.NoError(t, err)
			assertProfileSummaryShape(t, summary, profileType)
		})
	}

	t.Run(profileTypeCPU, func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, pprof.StartCPUProfile(&buf))
		deadline := time.Now().Add(time.Second)
		var acc uint64
		for time.Now().Before(deadline) {
			for i := range uint64(10000) {
				acc += i * 3
			}
		}
		_ = acc
		pprof.StopCPUProfile()

		summary, err := summarizePprof(profileTypeCPU, buf.Bytes())
		require.NoError(t, err)
		assertProfileSummaryShape(t, summary, profileTypeCPU)
	})
}

// TestProfileSummaryMalformedInput verifies summarizePprof rejects empty,
// non-gzip, truncated, corrupt protobuf, invalid string-table index, and
// oversized (>32 MiB decompressed) payloads with explicit errors and no panics.
//
// VALIDATES: AC-2 / R-2 -- malformed or oversized profile inputs return errors
// cleanly without panicking or exhausting memory.
// PREVENTS: daemon panics or unbounded decompression on corrupt pprof payloads.
func TestProfileSummaryMalformedInput(t *testing.T) {
	gzipBytes := func(payload []byte) []byte {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write(payload)
		require.NoError(t, err)
		require.NoError(t, gz.Close())
		return buf.Bytes()
	}

	// Build a valid-looking ValueType with string-table index 99 (out of range).
	badStringIdxProto := func() []byte {
		var vt []byte
		vt = binary.AppendUvarint(vt, (pprofValueTypeFieldType<<3)|protoWireVarint)
		vt = binary.AppendUvarint(vt, 99)
		vt = binary.AppendUvarint(vt, (pprofValueTypeFieldUnit<<3)|protoWireVarint)
		vt = binary.AppendUvarint(vt, 0)

		var out []byte
		out = binary.AppendUvarint(out, (pprofFieldSampleType<<3)|protoWireBytes)
		out = binary.AppendUvarint(out, uint64(len(vt)))
		out = append(out, vt...)
		// String table with a single empty string at index 0.
		out = binary.AppendUvarint(out, (pprofFieldStringTable<<3)|protoWireBytes)
		out = binary.AppendUvarint(out, 0)
		return out
	}()

	// Build a gzip payload that decompresses to maxProfileDecompressBytes + 1 bytes.
	oversizedGzip := func() []byte {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		chunk := make([]byte, 64<<10)
		remaining := maxProfileDecompressBytes + 1
		for remaining > 0 {
			n := min(remaining, len(chunk))
			_, err := gz.Write(chunk[:n])
			require.NoError(t, err)
			remaining -= n
		}
		require.NoError(t, gz.Close())
		return buf.Bytes()
	}()

	validGzip := gzipBytes([]byte{0x0a, 0x00})
	truncatedGzip := validGzip[:len(validGzip)/2]

	// Protobuf with invalid wire type 3 (deprecated start-group).
	invalidWireType := gzipBytes([]byte{(1 << 3) | 3})
	// Protobuf claiming a 100-byte submessage when only 2 bytes remain.
	truncatedSubmsg := gzipBytes([]byte{(pprofFieldSampleType << 3) | protoWireBytes, 100, 0x01, 0x02})
	// Protobuf with a truncated varint (continuation bit set on final byte).
	truncatedVarint := gzipBytes([]byte{0x80})

	cases := []struct {
		name  string
		input []byte
	}{
		{name: "empty", input: nil},
		{name: "non-gzip garbage", input: []byte("not a gzip stream")},
		{name: "truncated gzip", input: truncatedGzip},
		{name: "corrupt wire type", input: invalidWireType},
		{name: "truncated submessage length", input: truncatedSubmsg},
		{name: "truncated varint", input: truncatedVarint},
		{name: "out-of-range string table index", input: gzipBytes(badStringIdxProto)},
		{name: "decompressed exceeds 32 MiB cap", input: oversizedGzip},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, err := summarizePprof(profileTypeHeap, tc.input)
			assert.Error(t, err)
			assert.Nil(t, summary)
		})
	}
}

func assertProfileSummaryShape(t *testing.T, summary map[string]any, wantType string) {
	t.Helper()
	assert.Equal(t, pipeOpSummary, summary[keyFormat])
	assert.Equal(t, wantType, summary[keyType])
	sampleType, ok := summary["sample-type"].(string)
	require.True(t, ok, "sample-type is %T, want string", summary["sample-type"])
	assert.NotEmpty(t, sampleType)
	sampleUnit, ok := summary["sample-unit"].(string)
	require.True(t, ok, "sample-unit is %T, want string", summary["sample-unit"])
	assert.NotEmpty(t, sampleUnit)
	_, hasTotal := summary[triageKeyTotal]
	assert.True(t, hasTotal, "missing total field")
	top, ok := summary["top-functions"].([]map[string]any)
	require.True(t, ok, "top-functions is %T, want []map[string]any", summary["top-functions"])
	assert.NotNil(t, top)
	assert.LessOrEqual(t, len(top), maxProfileSummaryEntries)
	_, ok = summary[triageKeyTruncated].(bool)
	assert.True(t, ok, "truncated is %T, want bool", summary[triageKeyTruncated])

	var prevCum, prevFlat int64
	for i, row := range top {
		cum, ok := row["cum"].(int64)
		require.True(t, ok, "row %d cum is %T, want int64", i, row["cum"])
		flat, ok := row["flat"].(int64)
		require.True(t, ok, "row %d flat is %T, want int64", i, row["flat"])
		cumPct, ok := row["cum-pct"].(float64)
		require.True(t, ok, "row %d cum-pct is %T, want float64", i, row["cum-pct"])
		flatPct, ok := row["flat-pct"].(float64)
		require.True(t, ok, "row %d flat-pct is %T, want float64", i, row["flat-pct"])

		assert.GreaterOrEqual(t, cumPct, 0.0)
		assert.LessOrEqual(t, cumPct, 100.0)
		assert.GreaterOrEqual(t, flatPct, 0.0)
		assert.LessOrEqual(t, flatPct, 100.0)

		if i > 0 {
			assert.LessOrEqual(t, cum, prevCum, "row %d cum must be sorted descending", i)
			if cum == prevCum {
				assert.LessOrEqual(t, flat, prevFlat, "row %d flat must break cum ties descending", i)
			}
		}
		prevCum = cum
		prevFlat = flat
	}
}

// TestProfileSummaryBoundaryAndUnpackedVarints builds deterministic synthetic
// gzipped pprof protobuf profiles to verify the top-32 cap boundaries (0, 32,
// and 33 functions) and unpacked protoWireVarint encoding for Sample.location_id
// and Sample.value alongside packed encoding.
//
// VALIDATES: AC-1 -- top-functions returns [] with truncated=false at 0, 32
// rows with truncated=false at 32, and 32 rows with truncated=true and
// total-count=33 at 33, while decoding both packed and unpacked repeated varints.
// PREVENTS: off-by-one truncation bugs or failures on older unpacked pprof wire streams.
func TestProfileSummaryBoundaryAndUnpackedVarints(t *testing.T) {
	buildSyntheticProfile := func(funcCount int, unpacked bool) []byte {
		appendDelim := func(dst []byte, field uint64, sub []byte) []byte {
			dst = binary.AppendUvarint(dst, (field<<3)|protoWireBytes)
			dst = binary.AppendUvarint(dst, uint64(len(sub)))
			return append(dst, sub...)
		}
		appendVar := func(dst []byte, field, val uint64) []byte {
			dst = binary.AppendUvarint(dst, (field<<3)|protoWireVarint)
			return binary.AppendUvarint(dst, val)
		}

		// String table: 0="", 1="inuse_space", 2="bytes", 3="pkg/file.go", 4..="fnN".
		stringsTable := []string{"", "inuse_space", "bytes", "pkg/file.go"}
		for i := range funcCount {
			stringsTable = append(stringsTable, "pkg.fn"+strconv.Itoa(i+1))
		}

		var proto []byte
		// SampleType: type=1 ("inuse_space"), unit=2 ("bytes").
		var vt []byte
		vt = appendVar(vt, pprofValueTypeFieldType, 1)
		vt = appendVar(vt, pprofValueTypeFieldUnit, 2)
		proto = appendDelim(proto, pprofFieldSampleType, vt)

		// Functions, Locations, and Samples (one unique function per sample).
		for i := range funcCount {
			id := uint64(i + 1)
			nameIdx := uint64(4 + i)

			var fn []byte
			fn = appendVar(fn, pprofFunctionFieldID, id)
			fn = appendVar(fn, pprofFunctionFieldName, nameIdx)
			fn = appendVar(fn, pprofFunctionFieldFilename, 3)
			proto = appendDelim(proto, pprofFieldFunction, fn)

			var ln []byte
			ln = appendVar(ln, pprofLineFieldFunctionID, id)
			ln = appendVar(ln, pprofLineFieldLine, uint64(10+i))

			var loc []byte
			loc = appendVar(loc, pprofLocationFieldID, id)
			loc = appendDelim(loc, pprofLocationFieldLine, ln)
			proto = appendDelim(proto, pprofFieldLocation, loc)

			val := uint64((i + 1) * 100)
			var sample []byte
			if unpacked {
				sample = appendVar(sample, pprofSampleFieldLocation, id)
				sample = appendVar(sample, pprofSampleFieldValue, val)
			} else {
				var packedLocs, packedVals []byte
				packedLocs = binary.AppendUvarint(packedLocs, id)
				packedVals = binary.AppendUvarint(packedVals, val)
				sample = appendDelim(sample, pprofSampleFieldLocation, packedLocs)
				sample = appendDelim(sample, pprofSampleFieldValue, packedVals)
			}
			proto = appendDelim(proto, pprofFieldSample, sample)
		}

		for _, s := range stringsTable {
			proto = appendDelim(proto, pprofFieldStringTable, []byte(s))
		}

		var gzBuf bytes.Buffer
		gz := gzip.NewWriter(&gzBuf)
		_, err := gz.Write(proto)
		require.NoError(t, err)
		require.NoError(t, gz.Close())
		return gzBuf.Bytes()
	}

	cases := []struct {
		name           string
		funcCount      int
		unpacked       bool
		wantCount      int
		wantTotalCount int
		wantTruncated  bool
	}{
		{name: "0 functions packed", funcCount: 0, unpacked: false, wantCount: 0, wantTotalCount: 0, wantTruncated: false},
		{name: "32 functions packed", funcCount: 32, unpacked: false, wantCount: 32, wantTotalCount: 32, wantTruncated: false},
		{name: "33 functions packed", funcCount: 33, unpacked: false, wantCount: 32, wantTotalCount: 33, wantTruncated: true},
		{name: "33 functions unpacked varints", funcCount: 33, unpacked: true, wantCount: 32, wantTotalCount: 33, wantTruncated: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, err := summarizePprof(profileTypeHeap, buildSyntheticProfile(tc.funcCount, tc.unpacked))
			require.NoError(t, err)
			assertProfileSummaryShape(t, summary, profileTypeHeap)

			top, ok := summary["top-functions"].([]map[string]any)
			require.True(t, ok)
			assert.Len(t, top, tc.wantCount)
			assert.Equal(t, tc.wantCount, summary[keyCount])
			assert.Equal(t, tc.wantTotalCount, summary[triageKeyTotalCount])
			assert.Equal(t, tc.wantTruncated, summary[triageKeyTruncated])
			if tc.funcCount > 0 {
				wantTopName := "pkg.fn" + strconv.Itoa(tc.funcCount)
				assert.Equal(t, wantTopName, top[0]["function"])
			}
		})
	}
}
