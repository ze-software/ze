// VALIDATES: processMRTFile (the ze-analyze duplicate reader used by aspath,
// communities, count-attrs, mrt-dump, density, attributes) reads "-" from stdin
// and magic-sniffs gzip on the stdin stream (AC-7, AC-6). Fixing the openReader
// choke point alone does NOT cover these subcommands.
// PREVENTS: a gzipped MRT piped into `ze analyze aspath -` being misread as raw.
package analyze

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/core/cliio"
)

func bgp4mpRecord() []byte {
	rec := make([]byte, 12)
	binary.BigEndian.PutUint32(rec[0:4], 1000) // timestamp
	binary.BigEndian.PutUint16(rec[4:6], mrtBGP4MP)
	binary.BigEndian.PutUint16(rec[6:8], subtypeBGP4MPMessageAS4)
	binary.BigEndian.PutUint32(rec[8:12], 0) // zero-length body
	return rec
}

func countBGP4MP(t *testing.T, stdin []byte) int {
	t.Helper()
	restore := cliio.SwapStreams(bytes.NewReader(stdin), &bytes.Buffer{})
	defer restore()
	count := 0
	err := processMRTFile("-", mrtHandler{
		OnBGP4MP: func(_ []byte, _ uint16, _ uint32) { count++ },
	})
	if err != nil {
		t.Fatalf("processMRTFile(-): %v", err)
	}
	return count
}

func TestProcessMRTFileDash(t *testing.T) {
	rec := bgp4mpRecord()

	// Raw MRT on stdin.
	if n := countBGP4MP(t, rec); n != 1 {
		t.Fatalf("raw stdin: dispatched %d records, want 1", n)
	}

	// Gzipped MRT on stdin: magic-sniffed and decoded, never misread as raw (AC-6).
	var gzbuf bytes.Buffer
	gw := gzip.NewWriter(&gzbuf)
	if _, err := gw.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if n := countBGP4MP(t, gzbuf.Bytes()); n != 1 {
		t.Fatalf("gzip stdin: dispatched %d records, want 1", n)
	}
}
