package gokrazy

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestTestbootFitsTheRetainedFATExtent exercises the runtime's actual raw
// in-place update. The FAT directory retains its original file length, and the
// loader entry's title and kernel filename must not acquire command-line flags.
func TestTestbootFitsTheRetainedFATExtent(t *testing.T) {
	const arguments = "root=PARTUUID=fixture/PARTNROFF=1 quiet"
	const loaderHeader = "title gokrazy\nlinux /vmlinuz\noptions "
	for _, test := range []struct{ name, prefix, ending string }{
		{"kernel command line", "", "\n"},
		{"legacy command line without newline", "", ""},
		{"loader entry", loaderHeader, "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := []byte(test.prefix + arguments + strings.Repeat(" ", 64) + test.ending)
			file, err := os.CreateTemp(t.TempDir(), "boot-file-")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.Write(original); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				if err := modifyCmdlineFile(file, 0, int64(len(original)), testbootCmdline); err != nil {
					t.Fatal(err)
				}
			}
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() != int64(len(original)) {
				t.Errorf("testboot wrote %d bytes beyond the retained FAT extent", info.Size()-int64(len(original)))
			}
			retained, err := io.ReadAll(io.NewSectionReader(file, 0, int64(len(original))))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(retained, []byte(test.prefix+arguments)) {
				t.Errorf("testboot changed boot metadata or the selected root: %q", retained)
			}
			if bytes.Count(retained, []byte("gokrazy.try_boot_inactive=1")) != 1 {
				t.Errorf("retained kernel arguments do not contain one complete testboot flag: %q", retained)
			}
		})
	}
}

func TestTestbootRejectsAnExhaustedFATExtent(t *testing.T) {
	original := []byte("root=PARTUUID=fixture quiet\n")
	const adjacent = "ADJACENT-FAT-FILE-MUST-SURVIVE"
	file, err := os.CreateTemp(t.TempDir(), "boot-boundary-")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	before := append(bytes.Clone(original), []byte(adjacent)...)
	if _, err := file.Write(before); err != nil {
		t.Fatal(err)
	}
	if err := modifyCmdlineFile(file, 0, int64(len(original)), testbootCmdline); err == nil {
		t.Error("testboot accepted a command line with no reserved space")
	}
	after, err := io.ReadAll(io.NewSectionReader(file, 0, int64(len(before))))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("refused testboot changed the boot file or adjacent data: %q", after)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(before)) {
		t.Fatalf("refused testboot grew the containing device to %d", info.Size())
	}
}
