// VALIDATES: the preview archive check bounds the bytes it decompresses.
// PREVENTS: a small crafted .tar.gz costing unbounded CPU and time in the
// check that is meant to be safe on an untrusted bundle (gosec G110).
package buildpreview

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// limitArchiveTestBound is the decompressed byte budget the tests hand the
// check. It is far below the production bound so the oversized streams stay
// small on disk and cheap to build, and far above a well-formed archive.
const limitArchiveTestBound int64 = 64 << 10

// limitArchiveEntry is one regular file the fixture archive carries.
type limitArchiveEntry struct {
	name string
	body []byte
	mode int64
}

// writeLimitArchive writes a preview-shaped .tar.gz holding the entries, then
// appends trailing bytes after the tar end marker inside the gzip stream. It
// returns the archive path.
func writeLimitArchive(t *testing.T, entries []limitArchiveEntry, trailing int) string {
	t.Helper()
	return writeLimitArchivePrefixed(t, nil, entries, trailing)
}

// writeLimitArchivePrefixed is writeLimitArchive with raw tar blocks written
// into the gzip stream before the first entry, for headers tar.Writer will not
// encode.
func writeLimitArchivePrefixed(t *testing.T, prefix []byte, entries []limitArchiveEntry, trailing int) string {
	t.Helper()
	var compressed bytes.Buffer
	zip := gzip.NewWriter(&compressed)
	if _, err := zip.Write(prefix); err != nil {
		t.Fatalf("write the raw tar prefix: %v", err)
	}
	archive := tar.NewWriter(zip)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Typeflag: tar.TypeReg, Mode: entry.mode, Size: int64(len(entry.body)), Uid: 0, Gid: 0}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatalf("write the %s tar header: %v", entry.name, err)
		}
		if _, err := archive.Write(entry.body); err != nil {
			t.Fatalf("write the %s tar body: %v", entry.name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close the fixture tar stream: %v", err)
	}
	if trailing > 0 {
		if _, err := zip.Write(make([]byte, trailing)); err != nil {
			t.Fatalf("write the trailing gzip payload: %v", err)
		}
	}
	if err := zip.Close(); err != nil {
		t.Fatalf("close the fixture gzip stream: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ze-preview.tar.gz")
	if err := os.WriteFile(path, compressed.Bytes(), 0o600); err != nil {
		t.Fatalf("write the fixture archive: %v", err)
	}
	return path
}

// writeLimitBinary writes the binary the archive's ze entry must match and
// returns its path.
func writeLimitBinary(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write the fixture binary: %v", err)
	}
	return path
}

// TestCheckArchiveRefusesStreamBeyondSizeLimit proves that the archive check
// refuses a decompressed stream that runs past its bound, at each of the three
// places it reads (the bytes after the tar end marker it drains, an entry body
// it skips, the ze entry it hashes), and refuses a ze entry that declares more
// than the bound. Method: build tiny gzip archives that each run past the
// lowered bound at one place, and require an error that names the size limit.
// The trailing case decompresses to sixteen times the bound after the end
// marker, and the skipped case carries a README body of sixteen times the
// bound: before the bound exists both answer nil. The declared-size case
// carries a ze body of sixteen times the bound, which the declared-size check
// refuses before any read (before the bound exists it answers that the binary
// differs). The hash case carries a ze body of exactly the bound whose bytes
// match the compared binary: its declared size passes, and its header block
// and body run the stream past limit plus one during the hash, so only the
// budget check after the hash refuses it. Without that check the tar reader's
// short read reaches the caller as a raw io.ErrUnexpectedEOF, which the case
// refuses by name.
func TestCheckArchiveRefusesStreamBeyondSizeLimit(t *testing.T) {
	binary := []byte("preview ze binary fixture")
	oversized := make([]byte, 16*limitArchiveTestBound)
	atLimit := bytes.Repeat([]byte{'z'}, int(limitArchiveTestBound))
	cases := []struct {
		name     string
		entries  []limitArchiveEntry
		trailing int
		// compared is the binary the ze entry is checked against; nil means
		// the shared binary fixture.
		compared []byte
	}{
		{
			name: "trailing data after the tar end marker",
			entries: []limitArchiveEntry{
				{name: "ze", body: binary, mode: 0o755},
				{name: "README", body: []byte("readme"), mode: 0o644},
				{name: "LICENSE", body: []byte("license"), mode: 0o644},
			},
			trailing: len(oversized),
		},
		{
			name: "a skipped entry body",
			entries: []limitArchiveEntry{
				{name: "ze", body: binary, mode: 0o755},
				{name: "README", body: oversized, mode: 0o644},
				{name: "LICENSE", body: []byte("license"), mode: 0o644},
			},
		},
		{
			name: "a ze entry whose declared size is above the limit",
			entries: []limitArchiveEntry{
				{name: "ze", body: oversized, mode: 0o755},
				{name: "README", body: []byte("readme"), mode: 0o644},
				{name: "LICENSE", body: []byte("license"), mode: 0o644},
			},
		},
		{
			name: "a ze entry of exactly the limit whose stream runs out during the hash",
			entries: []limitArchiveEntry{
				{name: "ze", body: atLimit, mode: 0o755},
				{name: "README", body: []byte("readme"), mode: 0o644},
				{name: "LICENSE", body: []byte("license"), mode: 0o644},
			},
			compared: atLimit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compared := tc.compared
			if compared == nil {
				compared = binary
			}
			path := writeLimitArchive(t, tc.entries, tc.trailing)
			err := checkArchiveWithin(path, writeLimitBinary(t, compared), limitArchiveTestBound)
			if err == nil {
				t.Fatalf("an archive whose stream runs past a %d byte bound was accepted", limitArchiveTestBound)
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("the stream ran out and the raw short read was answered instead of the size limit: %v", err)
			}
			if !strings.Contains(err.Error(), "size limit") {
				t.Fatalf("oversized archive refused for another reason: %v", err)
			}
		})
	}
}

// TestCheckArchiveAcceptsArchiveWithinSizeLimit is the control for the bound:
// a well-formed preview archive whose whole stream fits the same lowered
// bound is still accepted, so the bound cannot be met by refusing every
// archive. Method: build the three-entry archive with a matching ze entry and
// no trailing data, and require no error.
func TestCheckArchiveAcceptsArchiveWithinSizeLimit(t *testing.T) {
	binary := []byte("preview ze binary fixture")
	path := writeLimitArchive(t, []limitArchiveEntry{
		{name: "ze", body: binary, mode: 0o755},
		{name: "README", body: []byte("readme"), mode: 0o644},
		{name: "LICENSE", body: []byte("license"), mode: 0o644},
	}, 0)
	if err := checkArchiveWithin(path, writeLimitBinary(t, binary), limitArchiveTestBound); err != nil {
		t.Fatalf("a well-formed archive inside the %d byte bound was refused: %v", limitArchiveTestBound, err)
	}
}

// limitArchiveEntries is the well-formed three-entry preview payload.
func limitArchiveEntries(binary []byte) []limitArchiveEntry {
	return []limitArchiveEntry{
		{name: "ze", body: binary, mode: 0o755},
		{name: "README", body: []byte("readme"), mode: 0o644},
		{name: "LICENSE", body: []byte("license"), mode: 0o644},
	}
}

// decompressedLength reads the archive's whole gzip stream and returns how
// many bytes it decompresses to.
func decompressedLength(t *testing.T, path string) int64 {
	t.Helper()
	file, err := os.Open(path) //nolint:gosec // G304: the test reads the fixture it wrote under t.TempDir
	if err != nil {
		t.Fatalf("open the fixture archive: %v", err)
	}
	defer file.Close() //nolint:errcheck // read-only fixture
	zip, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open the fixture gzip stream: %v", err)
	}
	length, err := io.Copy(io.Discard, zip) //nolint:gosec // G110: the test decompresses its own few-kilobyte fixture
	if err != nil {
		t.Fatalf("decompress the fixture archive: %v", err)
	}
	return length
}

// TestCheckArchiveSizeLimitBoundary proves the bound sits exactly at limit:
// a stream of limit decompressed bytes is accepted and one of limit plus one
// is refused. Method: measure the decompressed length of a well-formed
// archive, then pad the gzip stream after the tar end marker with zeros (the
// bytes the final drain reads) to each size.
func TestCheckArchiveSizeLimitBoundary(t *testing.T) {
	binary := []byte("preview ze binary fixture")
	unpadded := decompressedLength(t, writeLimitArchive(t, limitArchiveEntries(binary), 0))
	if unpadded >= limitArchiveTestBound {
		t.Fatalf("the unpadded fixture is %d bytes, not under the %d byte bound", unpadded, limitArchiveTestBound)
	}
	t.Run("exactly the limit is accepted", func(t *testing.T) {
		path := writeLimitArchive(t, limitArchiveEntries(binary), int(limitArchiveTestBound-unpadded))
		if got := decompressedLength(t, path); got != limitArchiveTestBound {
			t.Fatalf("padded fixture decompresses to %d bytes, wanted %d", got, limitArchiveTestBound)
		}
		if err := checkArchiveWithin(path, writeLimitBinary(t, binary), limitArchiveTestBound); err != nil {
			t.Fatalf("an archive of exactly the %d byte bound was refused: %v", limitArchiveTestBound, err)
		}
	})
	t.Run("one byte past the limit is refused", func(t *testing.T) {
		path := writeLimitArchive(t, limitArchiveEntries(binary), int(limitArchiveTestBound-unpadded+1))
		if got := decompressedLength(t, path); got != limitArchiveTestBound+1 {
			t.Fatalf("padded fixture decompresses to %d bytes, wanted %d", got, limitArchiveTestBound+1)
		}
		err := checkArchiveWithin(path, writeLimitBinary(t, binary), limitArchiveTestBound)
		if err == nil {
			t.Fatalf("an archive one byte past the %d byte bound was accepted", limitArchiveTestBound)
		}
		if !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("an archive one byte past the bound was refused for another reason: %v", err)
		}
	})
}

// putTarOctal writes value into a tar header field as zero-padded octal
// followed by a NUL, the form archive/tar parses.
func putTarOctal(t *testing.T, field []byte, value int64) {
	t.Helper()
	digits := strconv.AppendInt(nil, value, 8)
	width := len(field) - 1
	if len(digits) > width {
		t.Fatalf("octal %o does not fit a %d byte tar field", value, len(field))
	}
	for i := range width {
		field[i] = '0'
	}
	copy(field[width-len(digits):width], digits)
	field[width] = 0
}

// rawTarHeader builds one 512-byte USTAR header block by hand, checksum
// included, for a header tar.Writer refuses to write itself.
func rawTarHeader(t *testing.T, name string, typeflag byte, mode, size int64) []byte {
	t.Helper()
	block := make([]byte, 512)
	copy(block[0:100], name)
	putTarOctal(t, block[100:108], mode)
	putTarOctal(t, block[108:116], 0)
	putTarOctal(t, block[116:124], 0)
	putTarOctal(t, block[124:136], size)
	putTarOctal(t, block[136:148], 0)
	block[156] = typeflag
	copy(block[257:263], "ustar\x00")
	copy(block[263:265], "00")
	// The checksum is the byte sum with its own field read as spaces.
	copy(block[148:156], "        ")
	var sum int64
	for _, c := range block {
		sum += int64(c)
	}
	putTarOctal(t, block[148:155], sum)
	block[155] = ' '
	return block
}

// paxRecord encodes one PAX extended header record, "<len> key=value\n",
// where len counts the whole record including its own digits.
func paxRecord(key, value string) string {
	tail := " " + key + "=" + value + "\n"
	length := len(tail) + 1
	for len(strconv.Itoa(length))+len(tail) != length {
		length = len(strconv.Itoa(length)) + len(tail)
	}
	return strconv.Itoa(length) + tail
}

// padTarBlock pads body with zeros to a whole number of 512-byte blocks.
func padTarBlock(body []byte) []byte {
	if rest := len(body) % 512; rest != 0 {
		body = append(body, make([]byte, 512-rest)...)
	}
	return body
}

// writeSparseZeArchive writes a preview archive whose ze entry is a PAX GNU
// sparse 1.0 entry of realSize logical bytes, all one hole, beside valid
// README and LICENSE entries, and returns its path. tar.Writer drops
// GNU.sparse records, so the PAX x header is written by hand, and the ze
// entry's only physical body is an empty 1.0 sparse map. The reader reports
// realSize as the entry size and expands the hole as zeros without reading
// the stream.
func writeSparseZeArchive(t *testing.T, realSize int64) string {
	t.Helper()
	records := paxRecord("GNU.sparse.major", "1") +
		paxRecord("GNU.sparse.minor", "0") +
		paxRecord("GNU.sparse.name", "ze") +
		paxRecord("GNU.sparse.realsize", strconv.Itoa(int(realSize)))
	prefix := rawTarHeader(t, "PaxHeaders.0/ze", tar.TypeXHeader, 0o644, int64(len(records)))
	prefix = append(prefix, padTarBlock([]byte(records))...)
	// An empty 1.0 sparse map: the entry count 0, so the whole logical size
	// is one hole the reader fills with zeros.
	return writeLimitArchivePrefixed(t, prefix, limitArchiveEntries(padTarBlock([]byte("0\n"))), 0)
}

// TestCheckArchiveRefusesSparseEntryBeyondSizeLimit proves the bound also
// covers the bytes hashed from the ze entry when archive/tar synthesizes them.
// A PAX GNU sparse 1.0 header makes the reader report the attacker's
// GNU.sparse.realsize as the entry size and expand the holes as zeros without
// reading the stream, so a bound on compressed or decompressed stream bytes
// alone never sees them. Method: write the PAX x header by hand (tar.Writer
// drops GNU.sparse records), then a regular ze entry whose only physical body
// is an empty 1.0 sparse map, a logical size of sixteen times the bound, and
// valid README and LICENSE entries. Require the size-limit refusal; without
// it the check hashes the synthesized zeros and answers that the binary
// differs.
func TestCheckArchiveRefusesSparseEntryBeyondSizeLimit(t *testing.T) {
	binary := []byte("preview ze binary fixture")
	realSize := 16 * limitArchiveTestBound
	path := writeSparseZeArchive(t, realSize)
	if stream := decompressedLength(t, path); stream >= limitArchiveTestBound {
		t.Fatalf("the sparse fixture stream is %d bytes, not under the %d byte bound", stream, limitArchiveTestBound)
	}
	err := checkArchiveWithin(path, writeLimitBinary(t, binary), limitArchiveTestBound)
	if err == nil {
		t.Fatalf("a sparse ze entry of %d logical bytes past a %d byte bound was accepted", realSize, limitArchiveTestBound)
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("a sparse ze entry past the bound was refused for another reason: %v", err)
	}
}

// TestCheckArchiveSparseEntrySizeLimitBoundary proves the declared-size check
// on a sparse ze entry sits exactly at limit. Method: build the all-hole PAX
// sparse ze entry with a real size of limit, compare it against a binary of
// limit zero bytes (the zeros the reader synthesizes, so the digest matches),
// and require acceptance; then a real size of limit plus one against a binary
// of limit plus one zero bytes, and require the size-limit refusal. Both
// physical streams stay under the bound, so only the declared size decides:
// with the check written as >= the first is refused, and without the check
// the second hashes the synthesized zeros, matches, and is accepted.
func TestCheckArchiveSparseEntrySizeLimitBoundary(t *testing.T) {
	t.Run("a sparse ze entry of exactly the limit is accepted", func(t *testing.T) {
		path := writeSparseZeArchive(t, limitArchiveTestBound)
		if stream := decompressedLength(t, path); stream >= limitArchiveTestBound {
			t.Fatalf("the sparse fixture stream is %d bytes, not under the %d byte bound", stream, limitArchiveTestBound)
		}
		compared := writeLimitBinary(t, make([]byte, limitArchiveTestBound))
		if err := checkArchiveWithin(path, compared, limitArchiveTestBound); err != nil {
			t.Fatalf("a sparse ze entry of exactly the %d byte bound was refused: %v", limitArchiveTestBound, err)
		}
	})
	t.Run("a sparse ze entry one byte past the limit is refused", func(t *testing.T) {
		path := writeSparseZeArchive(t, limitArchiveTestBound+1)
		if stream := decompressedLength(t, path); stream >= limitArchiveTestBound {
			t.Fatalf("the sparse fixture stream is %d bytes, not under the %d byte bound", stream, limitArchiveTestBound)
		}
		compared := writeLimitBinary(t, make([]byte, limitArchiveTestBound+1))
		err := checkArchiveWithin(path, compared, limitArchiveTestBound)
		if err == nil {
			t.Fatalf("a sparse ze entry one byte past the %d byte bound was accepted", limitArchiveTestBound)
		}
		if !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("a sparse ze entry one byte past the bound was refused for another reason: %v", err)
		}
	})
}
