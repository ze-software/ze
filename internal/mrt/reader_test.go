// VALIDATES: openReader resolves "-" to stdin and sniffs compression by magic
// bytes (gzip 1f 8b, bzip2 "BZh") for the extension-less stdin stream, while real
// files keep extension-based sniffing unchanged (AC-5, AC-6, A-3).
// PREVENTS: a gzipped MRT piped on stdin being silently misread as raw (R-4).
package mrt

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/cliio"
)

func mrtRecord() []byte {
	r := make([]byte, CommonHeaderLen)
	WriteCommonHeader(r, 0, 1234, TypeBGP4MP, BGP4MPMessageAS4, 0)
	return r
}

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// readAllFromOpen opens filename via openReader (injecting stdin for "-") and
// returns the fully-read, decompressed bytes.
func readAllFromOpen(t *testing.T, filename string) []byte {
	t.Helper()
	rc, err := openReader(filename)
	if err != nil {
		t.Fatalf("openReader(%q): %v", filename, err)
	}
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %q: %v", filename, err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("close %q: %v", filename, err)
	}
	return data
}

func TestOpenReaderDash(t *testing.T) {
	rec := mrtRecord()

	// "-" reads raw stdin, unchanged.
	restore := cliio.SwapStreams(bytes.NewReader(rec), &bytes.Buffer{})
	got := readAllFromOpen(t, "-")
	restore()
	if !bytes.Equal(got, rec) {
		t.Fatalf("stdin raw MRT: got %d bytes, want %d", len(got), len(rec))
	}

	// A real file reads unchanged via the extension-sniff path.
	dir := t.TempDir()
	p := filepath.Join(dir, "rib.mrt")
	if err := os.WriteFile(p, rec, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readAllFromOpen(t, p); !bytes.Equal(got, rec) {
		t.Fatalf("file raw MRT mismatch")
	}
}

// bzip2Fixture is `printf 'hello-mrt-bzip2-magic-payload' | bzip2 -c` (Go's
// compress/bzip2 has no writer, so a real compressed blob is embedded).
var bzip2Fixture = []byte{
	0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0x3b, 0x8a, 0x0d, 0x10, 0x00, 0x00,
	0x06, 0x99, 0x80, 0x00, 0x02, 0x10, 0x00, 0x3e, 0xe6, 0xd4, 0x30, 0x20, 0x00, 0x31, 0x4d, 0x32,
	0x31, 0x31, 0x31, 0x08, 0x8f, 0x50, 0xd1, 0xa6, 0x99, 0x1e, 0xa7, 0x44, 0x2c, 0x89, 0x0b, 0x1a,
	0x8a, 0xbc, 0x19, 0xe9, 0x64, 0x08, 0xc1, 0xce, 0xcf, 0xc5, 0xdc, 0x91, 0x4e, 0x14, 0x24, 0x0e,
	0xe2, 0x83, 0x44, 0x00,
}

func TestOpenReaderMagicSniff(t *testing.T) {
	rec := mrtRecord()

	// gzip magic on stdin -> decompressed (R-4, AC-6).
	gz := gzipBytes(t, rec)
	restore := cliio.SwapStreams(bytes.NewReader(gz), &bytes.Buffer{})
	got := readAllFromOpen(t, "-")
	restore()
	if !bytes.Equal(got, rec) {
		t.Fatalf("gzip stdin not decompressed: got %d bytes, want %d", len(got), len(rec))
	}

	// bzip2 magic on stdin -> decompressed.
	restore = cliio.SwapStreams(bytes.NewReader(bzip2Fixture), &bytes.Buffer{})
	got = readAllFromOpen(t, "-")
	restore()
	if string(got) != "hello-mrt-bzip2-magic-payload" {
		t.Fatalf("bzip2 stdin not decompressed: got %q", got)
	}

	// raw (no magic) stdin passes through unchanged.
	restore = cliio.SwapStreams(bytes.NewReader(rec), &bytes.Buffer{})
	got = readAllFromOpen(t, "-")
	restore()
	if !bytes.Equal(got, rec) {
		t.Fatalf("raw stdin altered")
	}

	// Real-path extension sniffing unchanged: a .gz file still decompresses (A-3).
	dir := t.TempDir()
	gzp := filepath.Join(dir, "rib.mrt.gz")
	if err := os.WriteFile(gzp, gz, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readAllFromOpen(t, gzp); !bytes.Equal(got, rec) {
		t.Fatalf(".gz file not decompressed via extension sniff")
	}
}
