// Design: docs/architecture/testing/qemu-integration.md -- the ext4 fixtures a guest test attaches
// Overview: fixture.go -- the fixture set and how the host makes it
// Related: fixture_linux.go -- a fixture attached alone, as a filesystem device

//go:build linux

package ext4fixture

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// Disk is a partitioned disk image attached to a loop device that scans its
// partition table, so the kernel makes a node per partition, the way it does
// for the appliance's boot disk. Its cleanup detaches the loop device. Not
// safe for concurrent use.
type Disk struct {
	// Backing is the image the loop device reads.
	Backing string
	// Device is the whole-disk loop node, /dev/loopN.
	Device string

	loop *os.File
}

// CopyInto copies fixture into the image file at offsetBytes, sparsely, as
// the content of a partition that starts there. The image keeps its size.
//
// It skips the test when DirEnv is unset, and fails it when the process is
// not root or the fixture is missing.
func CopyInto(t *testing.T, fixture Fixture, image string, offsetBytes int64) {
	t.Helper()
	source := guestFixture(t, fixture)
	in, err := os.Open(filepath.Clean(source))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close() //nolint:errcheck // read-only
	out, err := os.OpenFile(filepath.Clean(image), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := out.Seek(offsetBytes, io.SeekStart); err != nil {
		out.Close() //nolint:errcheck,gosec // the seek error is the one reported
		t.Fatal(err)
	}
	if _, err := copyChunks(in, out); err != nil {
		out.Close() //nolint:errcheck,gosec // the copy error is the one reported
		t.Fatalf("ext4fixture: copy %s into %s: %v", fixture, image, err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// AttachDisk attaches image to a free loop device with partition scanning,
// so the kernel reads its partition table at once.
//
// It skips the test when DirEnv is unset, and fails it when the process is
// not root.
func AttachDisk(t *testing.T, image string) *Disk {
	t.Helper()
	requireGuestRoot(t)
	device, loop, err := attachLoop(image, unix.LO_FLAGS_AUTOCLEAR|unix.LO_FLAGS_PARTSCAN)
	if err != nil {
		t.Fatalf("ext4fixture: attach %s with partition scanning: %v", image, err)
	}
	disk := &Disk{Backing: image, Device: device, loop: loop}
	t.Cleanup(func() {
		if err := disk.loop.Close(); err != nil {
			t.Errorf("ext4fixture: close %s: %v", disk.Device, err)
		}
	})
	return disk
}

// File is the open whole-disk device: the descriptor a table write, a reread
// and a partition resize go through.
func (disk *Disk) File() *os.File { return disk.loop }

// Name is the kernel's name of the whole disk, loopN.
func (disk *Disk) Name() string { return filepath.Base(disk.Device) }

// Partition answers the node of partition number, /dev/loopNpM.
func (disk *Disk) Partition(number int) string {
	return disk.Device + "p" + strconv.Itoa(number)
}

// Grow enlarges the image to sizeBytes and tells the loop device its new
// capacity: a larger disk under the same partition table.
func (disk *Disk) Grow(t *testing.T, sizeBytes int64) {
	t.Helper()
	growLoop(t, disk.loop, disk.Backing, disk.Device, sizeBytes)
}

// KernelPartition answers the start and the size, in 512-byte sectors, that
// the kernel holds for partition number of the disk, read from sysfs. ok is
// false when the kernel holds no such partition; an unreadable value fails
// the test.
func (disk *Disk) KernelPartition(t *testing.T, number int) (start, sectors uint64, ok bool) {
	t.Helper()
	dir := filepath.Join("/sys/class/block", disk.Name()+"p"+strconv.Itoa(number))
	if _, err := os.Stat(dir); err != nil {
		return 0, 0, false
	}
	return SysfsNumber(t, filepath.Join(dir, "start")), SysfsNumber(t, filepath.Join(dir, "size")), true
}

// SysfsNumber reads one decimal number from a sysfs file, and fails the test
// when it cannot.
func SysfsNumber(t *testing.T, path string) uint64 {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ext4fixture: read %s: %v", path, err)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		t.Fatalf("ext4fixture: %s holds %q, not a number: %v", path, raw, err)
	}
	return value
}
