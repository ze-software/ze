// Design: docs/architecture/appliance/iso-installer.md -- FAT16B appliance image support

package fat

import (
	"bytes"
	"strconv"
	"testing"
	"time"
)

// VALIDATES: an image this package WRITES can be read back by the Reader in the
// same package, with every file's bytes at the offset and length Extents
// reports.
//
// PREVENTS: a directory entry whose fields land in the wrong place. The writer
// builds entries from an embedded `common` struct, and a change that flattens
// those literals -- which a modernize autofix proposes -- can silently move a
// value into a sibling field, because `file` and `directory` both embed
// `common` and both have their own `Attr()`. Nothing in the package compared
// what it writes against what it reads, so the whole 699-line writer had NO
// test at all: a wrong offset, a wrong length or a truncated cluster chain
// produced an image that still parsed and still built.
//
// The round trip is the assertion on purpose. Checking the writer against a
// hand-computed FAT layout would test this test's understanding of FAT16B;
// checking it against the package's own Reader tests the two halves against
// each other, which is what the appliance actually relies on.
func TestWriterReaderRoundTrip(t *testing.T) {
	t.Parallel()
	modTime := time.Date(2026, 8, 25, 14, 30, 0, 0, time.UTC)

	// Three shapes that exercise different paths: a file in the root, a file
	// in a subdirectory, and one large enough to span more than one cluster
	// (4 sectors of 512 bytes = 2048).
	// Mirrors how the appliance uses this package (internal/appliance/cmd_iso.go):
	// Mkdir first, then files inside. A file written at the root is not a shape
	// any caller uses, so this does not assert one.
	want := map[string][]byte{
		"BOOT/CONFIG.TXT":   []byte("dtparam=audio=on\n"),
		"BOOT/BIG.DAT":      bytes.Repeat([]byte("Z"), 5000),
		"BOOT/SUB/DEEP.BIN": []byte("nested"),
	}

	var buf bytes.Buffer
	fw, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	for _, dir := range []string{"BOOT", "BOOT/SUB"} {
		if err := fw.Mkdir(dir, modTime); err != nil {
			t.Fatalf("Mkdir(%q): %v", dir, err)
		}
	}
	// Written in a fixed order so a failure names one path rather than
	// depending on map iteration.
	for _, path := range []string{"BOOT/CONFIG.TXT", "BOOT/BIG.DAT", "BOOT/SUB/DEEP.BIN"} {
		w, err := fw.File(path, modTime)
		if err != nil {
			t.Fatalf("File(%q): %v", path, err)
		}
		if _, err := w.Write(want[path]); err != nil {
			t.Fatalf("Write(%q): %v", path, err)
		}
	}
	if err := fw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	image := buf.Bytes()
	if len(image) == 0 {
		t.Fatal("the writer produced an empty image")
	}

	r, err := NewReader(bytes.NewReader(image))
	if err != nil {
		t.Fatalf("NewReader over the image this package just wrote: %v", err)
	}
	for path, content := range want {
		off, length, err := r.Extents("/" + path)
		if err != nil {
			t.Errorf("Extents(%q): %v", path, err)
			continue
		}
		if length != int64(len(content)) {
			t.Errorf("Extents(%q) length = %d, want %d", path, length, len(content))
			continue
		}
		if off+length > int64(len(image)) {
			t.Errorf("Extents(%q) points past the end of the image: offset %d + length %d > %d",
				path, off, length, len(image))
			continue
		}
		if got := image[off : off+length]; !bytes.Equal(got, content) {
			t.Errorf("%q round-tripped to different bytes\n  at offset %d, length %d\n  got  %q\n  want %q",
				path, off, length, truncate(got), truncate(content))
		}
	}
}

// VALIDATES: a name longer than 8.3 is MANGLED to a short name that the Reader
// then finds, rather than being written verbatim.
//
// PREVENTS: reading the package doc as a refusal. It says filenames "are
// restricted to 8 characters + 3 for the extension", which reads as a
// constraint the caller must meet. The writer does not refuse a longer name --
// `shortFileNameWrite` converts it, as FAT itself does -- so a caller who
// believes the prose gets a file under a name it did not choose. This pins the
// behavior that exists: the long name goes in, a short name comes out, and the
// two halves of this package agree on which.
func TestWriterManglesALongNameConsistently(t *testing.T) {
	t.Parallel()
	modTime := time.Date(2026, 8, 25, 14, 30, 0, 0, time.UTC)
	content := []byte("payload")

	var buf bytes.Buffer
	fw, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := fw.Mkdir("BOOT", modTime); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	w, err := fw.File("BOOT/TOOLONGNAME.TXT", modTime)
	if err != nil {
		t.Fatalf("File: the writer refuses a long name, which this test did not expect: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := fw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	image := buf.Bytes()
	r, err := NewReader(bytes.NewReader(image))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	// The Reader mangles the query the same way, so the ORIGINAL name is what
	// a caller looks up. That agreement is the property worth pinning: it is
	// what lets the appliance ask for the name it wrote.
	off, length, err := r.Extents("/BOOT/TOOLONGNAME.TXT")
	if err != nil {
		t.Fatalf("Extents on the name that was written: %v", err)
	}
	if length != int64(len(content)) {
		t.Fatalf("length = %d, want %d", length, len(content))
	}
	if got := image[off : off+length]; !bytes.Equal(got, content) {
		t.Errorf("mangled name round-tripped to %q, want %q", got, content)
	}
}

// VALIDATES: Extents refuses a path the image does not hold.
//
// PREVENTS: a zero offset reading as a valid answer. A lookup that fails by
// returning (0, 0, nil) hands the caller the boot sector and calls it a file,
// which is the shape ai/rules/evidence.md names: a zero value must never be a
// valid-looking answer.
func TestExtentsRefusesAnAbsentPath(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	fw, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := fw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	r, err := NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	off, length, err := r.Extents("/NOSUCH.BIN")
	if err == nil {
		t.Errorf("Extents on an absent path returned offset %d length %d and no error", off, length)
	}
}

// truncate keeps a failure message readable when a multi-cluster file differs.
func truncate(b []byte) string {
	const max = 48
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "... (" + strconv.Itoa(len(b)) + " bytes)"
}
