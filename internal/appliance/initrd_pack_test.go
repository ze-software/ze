// Design: docs/architecture/appliance/installer-initrd.md -- AC-11 pure-Go initrd packer round-trip

package appliance

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type newcEntry struct {
	name string
	mode uint64
	data []byte
}

func align4(n int) int { return (n + 3) &^ 3 }

func hexField(t *testing.T, hdr []byte, off int) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(string(hdr[off:off+8]), 16, 32)
	if err != nil {
		t.Fatalf("bad hex header field at %d (%q): %v", off, hdr[off:off+8], err)
	}
	return v
}

// parseNewc walks a newc ("070701") cpio stream, returning every record
// including the TRAILER!!! sentinel.
func parseNewc(t *testing.T, b []byte) []newcEntry {
	t.Helper()
	var out []newcEntry
	off := 0
	for {
		if off+110 > len(b) {
			t.Fatalf("truncated newc header at offset %d", off)
		}
		hdr := b[off : off+110]
		if string(hdr[:6]) != "070701" {
			t.Fatalf("bad newc magic %q at offset %d", hdr[:6], off)
		}
		mode := hexField(t, hdr, 14)
		filesize := int(hexField(t, hdr, 54))
		namesize := int(hexField(t, hdr, 94))
		off += 110
		if off+namesize > len(b) {
			t.Fatal("truncated newc name")
		}
		name := string(b[off : off+namesize-1]) // strip trailing NUL
		off += namesize
		off = align4(off)
		if off+filesize > len(b) {
			t.Fatal("truncated newc data")
		}
		data := append([]byte(nil), b[off:off+filesize]...)
		off += filesize
		off = align4(off)
		out = append(out, newcEntry{name: name, mode: mode, data: data})
		if name == "TRAILER!!!" {
			return out
		}
	}
}

// VALIDATES: the pure-Go initrd packer (writeInitrdPack) emits a gzip stream
// that gunzips to a newc cpio with exactly one `init` entry (mode 0100755)
// carrying the original bytes, terminated by TRAILER!!!, with a reproducible
// (zeroed) gzip header (AC-11).
// PREVENTS: a malformed initrd the kernel cannot unpack -- wrong file mode, a
// broken newc header/padding, a missing trailer, or non-reproducible output.
//
// TestWriteInitrdPack proves AC-11: the packer produces a gzip stream that
// gunzips to a newc cpio containing exactly one entry `init` (mode 0100755) with
// the original bytes, terminated by TRAILER!!!.
func TestWriteInitrdPack(t *testing.T) {
	payload := []byte("\x7fELF\x02\x01\x01 fake ze-installer init binary \x00\xff\x10")
	dest := filepath.Join(t.TempDir(), "initrd.img.gz")

	if err := writeInitrdPack(dest, payload); err != nil {
		t.Fatalf("writeInitrdPack: %v", err)
	}

	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read packed initrd: %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip.NewReader (not a valid gzip stream): %v", err)
	}
	// Reproducibility: the packer zeroes the gzip Name/ModTime header fields.
	if gz.Name != "" {
		t.Errorf("gzip Name = %q, want empty (reproducible header)", gz.Name)
	}
	if !gz.ModTime.IsZero() {
		t.Errorf("gzip ModTime = %v, want zero (reproducible header)", gz.ModTime)
	}
	cpioBytes, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}

	entries := parseNewc(t, cpioBytes)
	if len(entries) != 2 {
		t.Fatalf("got %d newc records, want 2 (init + TRAILER!!!): %+v", len(entries), entries)
	}

	init := entries[0]
	if init.name != "init" {
		t.Errorf("first entry name = %q, want %q", init.name, "init")
	}
	if init.mode != 0o100755 {
		t.Errorf("init mode = 0o%o, want 0o100755", init.mode)
	}
	if !bytes.Equal(init.data, payload) {
		t.Errorf("init data mismatch: got %d bytes, want %d", len(init.data), len(payload))
	}

	if entries[1].name != "TRAILER!!!" {
		t.Errorf("last record = %q, want TRAILER!!!", entries[1].name)
	}
	if len(entries[1].data) != 0 {
		t.Errorf("TRAILER!!! has %d bytes of data, want 0", len(entries[1].data))
	}
}
