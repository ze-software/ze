// VALIDATES: `ze-analyze filter --as-path` derives the AS_PATH width from the
// MRT record type, so a TABLE_DUMP (type 12, 2-byte AS per RFC 6396 Section 4.2)
// file can actually be filtered.
// PREVENTS: the hardcoded as4=true at the OnTableDump call site. ParseASPath
// read every 2-byte path at 4-byte width, overran it, returned an error,
// matchASPath returned false for every record, and the run printed
// "filtered 0/N records" and exited 0 -- silent total data loss, presented as a
// successful filter that matched nothing.
package analyze

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

// tableDumpRecord builds a complete TABLE_DUMP (type 12) MRT record for an IPv4
// prefix carrying the given path attributes.
func tableDumpRecord(t *testing.T, attrs []byte) []byte {
	t.Helper()
	const ipLen = 4
	payload := make([]byte, 0, 2+2+ipLen+1+1+4+ipLen+2+2+len(attrs))
	payload = binary.BigEndian.AppendUint16(payload, 0) // ViewNumber
	payload = binary.BigEndian.AppendUint16(payload, 1) // SeqNumber
	payload = append(payload, 10, 0, 0, 0,              // Prefix 10.0.0.0
		8, // PrefixLen /8
		1) // Status
	payload = binary.BigEndian.AppendUint32(payload, 0)     // OrigTime
	payload = append(payload, 192, 0, 2, 1)                 // PeerIP
	payload = binary.BigEndian.AppendUint16(payload, 64512) // PeerAS
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs)))
	payload = append(payload, attrs...)

	return mrtRecord(mrt.TypeTableDump, mrt.TableDumpAFIIPv4, payload)
}

func TestRunFilter_TableDumpASPathUsesTwoByteWidth(t *testing.T) {
	// A 2-byte AS_SEQUENCE of [65000, 65001], which is how TABLE_DUMP encodes
	// AS_PATH. Read at 4-byte width this overruns and matches nothing.
	asPath := []byte{2, 2, 0xFD, 0xE8, 0xFD, 0xE9}
	record := tableDumpRecord(t, makeAttr(0x40, attrASPath, asPath))

	dir := t.TempDir()
	in := filepath.Join(dir, "in.mrt")
	out := filepath.Join(dir, "out.mrt")
	require.NoError(t, os.WriteFile(in, record, 0o600))

	code, _ := runSubcommand(t, nil, func() int {
		return runFilter([]string{"--as-path", "65000", in, out})
	})
	require.Equal(t, 0, code)

	written, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.NotEmpty(t, written,
		"the record MUST survive the filter: reading its 2-byte AS_PATH at "+
			"4-byte width drops every record and reports success")
	assert.Equal(t, record, written, "the matched record is copied through verbatim")
}

func TestRunFilter_TableDumpASPathStillRejectsNonMatches(t *testing.T) {
	// The width fix must not turn the filter into a pass-through: a pattern
	// that genuinely does not match still drops the record.
	asPath := []byte{2, 2, 0xFD, 0xE8, 0xFD, 0xE9} // 65000 65001
	record := tableDumpRecord(t, makeAttr(0x40, attrASPath, asPath))

	dir := t.TempDir()
	in := filepath.Join(dir, "in.mrt")
	out := filepath.Join(dir, "out.mrt")
	require.NoError(t, os.WriteFile(in, record, 0o600))

	code, _ := runSubcommand(t, nil, func() int {
		return runFilter([]string{"--as-path", "^64500 ", in, out})
	})
	require.Equal(t, 0, code)

	// The writer creates the output lazily, so "nothing matched" surfaces as an
	// absent file rather than an empty one. Both mean the same thing here.
	written, err := os.ReadFile(out)
	if errors.Is(err, os.ErrNotExist) {
		written = nil
	} else {
		require.NoError(t, err)
	}
	assert.Empty(t, written, "a non-matching AS path must still be filtered out")
}
