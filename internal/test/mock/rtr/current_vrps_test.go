// Design: docs/guide/rpki.md -- complete current mock RTR snapshots.
package rtr

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRTRVRPFileReset proves that a Serial Query cannot merge a full snapshot
// with obsolete VRPs: it gets Cache Reset, then Reset Query gets only the new
// full set with its actual serial, exact prefix/ASN and End-of-Data boundary.
func TestRTRVRPFileReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current-vrps")
	require.NoError(t, os.WriteFile(path, []byte("7\n10.254.0.0/24,24,65999\n"), 0o600))
	cache := &rtrMockCache{vrpFile: path, serial: 1, refresh: 1, retry: 1}
	var out bytes.Buffer
	require.NoError(t, rtrMockFileQuery(&out, cache, 2, rtrMockPDUSerialQuery, 1))
	require.Equal(t, []byte{2, 8, 0, 0, 0, 0, 0, 8}, out.Bytes())
	out.Reset()
	require.NoError(t, rtrMockFileQuery(&out, cache, 2, rtrMockPDUResetQuery, 0))
	wire := out.Bytes()
	require.Len(t, wire, 8+20+24)
	require.Equal(t, byte(rtrMockPDUCacheResp), wire[1])
	require.Equal(t, byte(rtrMockPDUIPv4Prefix), wire[9])
	require.Equal(t, []byte{1, 24, 24, 0, 10, 254, 0, 0}, wire[16:24])
	require.Equal(t, uint32(65999), binary.BigEndian.Uint32(wire[24:28]))
	require.Equal(t, byte(rtrMockPDUEndOfData), wire[29])
	require.Equal(t, uint32(7), binary.BigEndian.Uint32(wire[36:40]))
	// A later Reset Query reads the replacement, rather than a remembered file
	// body or a merged history of earlier complete sets.
	require.NoError(t, os.WriteFile(path, []byte("8\n10.252.0.0/24,24,65001\n"), 0o600))
	out.Reset()
	require.NoError(t, rtrMockFileQuery(&out, cache, 2, rtrMockPDUResetQuery, 0))
	require.Equal(t, []byte{10, 252, 0, 0}, out.Bytes()[20:24])
	require.Equal(t, uint32(8), binary.BigEndian.Uint32(out.Bytes()[36:40]))
	out.Reset()
	require.NoError(t, rtrMockFileQuery(&out, cache, 2, rtrMockPDUSerialQuery, 8))
	require.Len(t, out.Bytes(), 8+24)
	require.Equal(t, byte(rtrMockPDUEndOfData), out.Bytes()[9])
}

// TestRTRVRPFileRejectsMalformedSnapshot proves invalid external file input
// never becomes a partial usable set, including range, duplication and size.
func TestRTRVRPFileRejectsMalformedSnapshot(t *testing.T) {
	for name, text := range map[string]string{
		"empty": "", "zero": "0\n", "serial-width": "4294967296\n",
		"short-max": "1\n10.0.0.0/24,23,65001\n", "long-max": "1\n10.0.0.0/24,33,65001\n",
		"bad-prefix": "1\nx/24,24,65001\n", "bad-ASN": "1\n10.0.0.0/24,24,-1\n",
		"duplicate": "1\n10.0.0.0/24,24,65001\n10.0.0.0/24,24,65001\n",
		"oversize":  "1\n" + strings.Repeat("x", rtrMockSnapshotBytes),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "current")
			require.NoError(t, os.WriteFile(path, []byte(text), 0o600))
			_, rows, err := rtrMockReadVRPs(path)
			require.Error(t, err)
			require.Nil(t, rows)
		})
	}
}
