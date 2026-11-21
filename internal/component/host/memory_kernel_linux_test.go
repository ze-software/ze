//go:build linux

package host

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// writeMeminfoFixture writes body as <root>/proc/meminfo and answers root.
func writeMeminfoFixture(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proc", "meminfo"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// VALIDATES: the kernel-memory trend rule reads MemTotal, MemAvailable and
// SUnreclaim through the host's one meminfo parser, converted from kB.
// Method: a fixture meminfo under t.TempDir with the three lines among others.
func TestDetectKernelMemoryReadsUnreclaimableSlab(t *testing.T) {
	root := writeMeminfoFixture(t, "MemTotal:        8053028 kB\n"+
		"MemFree:         1204568 kB\n"+
		"MemAvailable:    5612364 kB\n"+
		"Slab:             412000 kB\n"+
		"SReclaimable:     300000 kB\n"+
		"SUnreclaim:       112000 kB\n"+
		"HugePages_Total:       0\n")
	d := &Detector{Root: root}
	memory, err := d.DetectKernelMemory()
	if err != nil {
		t.Fatalf("DetectKernelMemory: %v", err)
	}
	if got, want := memory.TotalBytes, uint64(8053028*1024); got != want {
		t.Errorf("TotalBytes = %d, want %d", got, want)
	}
	if got, want := memory.AvailableBytes, uint64(5612364*1024); got != want {
		t.Errorf("AvailableBytes = %d, want %d", got, want)
	}
	if got, want := memory.UnreclaimableBytes, uint64(112000*1024); got != want {
		t.Errorf("UnreclaimableBytes = %d, want %d", got, want)
	}
}

// VALIDATES: a meminfo without the SUnreclaim line answers an error naming the
// file, never a zero counter the rule would read as no kernel memory.
// Method: the n100 fixture, which carries no SUnreclaim line.
func TestDetectKernelMemoryRefusesMissingLine(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	_, err := d.DetectKernelMemory()
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("DetectKernelMemory error = %v, want an *fs.PathError", err)
	}
	if want := filepath.Join("testdata", "n100-4x-igc", "proc", "meminfo"); pathErr.Path != want {
		t.Errorf("path = %q, want %q", pathErr.Path, want)
	}
	if !errors.Is(err, errMeminfoLineMissing) {
		t.Errorf("error = %v, want errMeminfoLineMissing", err)
	}
}

// VALIDATES: an absent meminfo answers an *fs.PathError naming the path.
// Method: a detector rooted at an empty t.TempDir.
func TestDetectKernelMemoryNamesUnreadablePath(t *testing.T) {
	root := t.TempDir()
	d := &Detector{Root: root}
	_, err := d.DetectKernelMemory()
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("DetectKernelMemory error = %v, want an *fs.PathError", err)
	}
	if want := filepath.Join(root, "proc", "meminfo"); pathErr.Path != want {
		t.Errorf("path = %q, want %q", pathErr.Path, want)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want fs.ErrNotExist", err)
	}
}
