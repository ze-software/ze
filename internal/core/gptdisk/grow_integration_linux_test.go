// Design: docs/architecture/testing/qemu-integration.md -- the ext4 fixtures a guest test attaches
// Related: kernel_linux.go -- the partition resize under test
//
// The tests need `integration && linux`: they attach loop devices with
// partition scanning, mount ext4 and resize a partition in the kernel, which
// only root of the initial user namespace may do, so they run in the QEMU
// guest on Ze's runtime kernel (`./le test qemu all`).

//go:build integration && linux

package gptdisk

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/ext4fs"
	"github.com/ze-software/ze/internal/test/ext4fixture"
	"github.com/ze-software/ze/internal/test/gptfixture"
)

// liveGrownBytes is what the live disk grows to: 1 GiB.
const liveGrownBytes = 1 << 30

// writeLayoutFile writes layout into a new sparse file of sizeBytes.
func writeLayoutFile(t *testing.T, path string, layout gptfixture.Layout, sizeBytes uint64) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(int64(sizeBytes)); err != nil { //nolint:gosec // a test disk of a few GiB
		t.Fatal(err)
	}
	if err := layout.Write(file); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// requireKernelPartition fails unless the kernel holds partition 4 of disk at
// startLBA with sectors sectors.
func requireKernelPartition(t *testing.T, disk *ext4fixture.Disk, when string, startLBA, sectors uint64) {
	t.Helper()
	start, size, ok := disk.KernelPartition(t, PersistentPartition)
	if !ok {
		t.Fatalf("%s: the kernel holds no partition %d of %s", when, PersistentPartition, disk.Device)
	}
	if start != startLBA || size != sectors {
		t.Fatalf("%s: the kernel holds partition %d at sector %d with %d sectors, want sector %d with %d", when, PersistentPartition, start, size, startLBA, sectors)
	}
}

// TestGrowLivePartitionWhileMounted settles spec assumption A-6 and the
// kernel's reading of the grown table: on a disk whose partition 4 is
// mounted, the kernel refuses a full reread of the table (EBUSY), takes the
// partition's new size through BLKPG_RESIZE_PARTITION, and the mounted ext4
// then grows into it; once nothing is mounted, the kernel's own partition
// scan reads the grown table with the same bounds.
//
// Method: write the packer's layout with the format fixture as partition 4
// into a sparse file, attach it with partition scanning, mount partition 4
// and write a witness, enlarge the disk, write the grown table through the
// whole-disk node with Grow, ask for a reread (refused), resize partition 4
// in the kernel, resize the filesystem to the partition's whole blocks, then
// read the kernel's partition, the superblock and the witness; unmount,
// reread, and read them again after a remount. The write through the
// whole-disk node while partition 4 is mounted also shows the kernel allows
// it (CONFIG_BLK_DEV_WRITE_MOUNTED).
func TestGrowLivePartitionWhileMounted(t *testing.T) {
	layout := gptfixture.Appliance(uint64(ext4fixture.SizeBytes) / SectorBytes)
	perm := layout.Partitions[3]
	image := filepath.Join(t.TempDir(), "disk.img")
	writeLayoutFile(t, image, layout, layout.DiskSectors*SectorBytes)
	ext4fixture.CopyInto(t, ext4fixture.PermFormat, image, int64(perm.First*SectorBytes)) //nolint:gosec // a test disk of a few GiB
	disk := ext4fixture.AttachDisk(t, image)
	requireKernelPartition(t, disk, "at attach", perm.First, perm.Last-perm.First+1)

	dir := t.TempDir()
	node := disk.Partition(PersistentPartition)
	if err := unix.Mount(node, dir, "ext4", 0, ""); err != nil {
		t.Fatalf("mount %s: %v", node, err)
	}
	mounted := true
	t.Cleanup(func() {
		if mounted {
			if err := unix.Unmount(dir, 0); err != nil {
				t.Errorf("unmount %s: %v", dir, err)
			}
		}
	})
	witness := bytes.Repeat([]byte("persistent "), 4096)
	if err := os.WriteFile(filepath.Join(dir, "witness"), witness, 0o600); err != nil {
		t.Fatal(err)
	}

	disk.Grow(t, liveGrownBytes)
	diskBytes := ext4fixture.SysfsNumber(t, filepath.Join("/sys/class/block", disk.Name(), "size")) * SectorBytes
	sectorBytes := ext4fixture.SysfsNumber(t, filepath.Join("/sys/class/block", disk.Name(), "queue", "logical_block_size"))
	if diskBytes != liveGrownBytes || sectorBytes != SectorBytes {
		t.Fatalf("the kernel reads the grown disk as %d bytes in %d-byte sectors, want %d in %d", diskBytes, sectorBytes, liveGrownBytes, SectorBytes)
	}
	growth, err := Grow(disk.File(), Geometry{DiskBytes: diskBytes, SectorBytes: uint32(sectorBytes)}) //nolint:gosec // 512, checked above
	if err != nil {
		t.Fatalf("Grow through %s while partition 4 is mounted: %v", disk.Device, err)
	}
	if !growth.Written || growth.PartitionBytesAfter <= growth.PartitionBytesBefore {
		t.Fatalf("growth %+v: the partition did not grow", growth)
	}

	// A-6, first half: with partition 4 mounted, a full reread is refused.
	if err := unix.IoctlSetInt(int(disk.File().Fd()), unix.BLKRRPART, 0); !errors.Is(err, unix.EBUSY) {
		t.Fatalf("BLKRRPART with partition 4 mounted = %v, want EBUSY", err)
	}
	requireKernelPartition(t, disk, "after the refused reread", perm.First, perm.Last-perm.First+1)

	// A-6, second half: the kernel takes the new size of the partition.
	if err := ResizeInKernel(disk.File(), growth); err != nil {
		t.Fatalf("ResizeInKernel: %v", err)
	}
	grownSectors := growth.PartitionBytesAfter / SectorBytes
	requireKernelPartition(t, disk, "after BLKPG_RESIZE_PARTITION", perm.First, grownSectors)

	blocks := growth.PartitionBytesAfter / 4096
	if err := ext4fs.Resize(dir, blocks); err != nil {
		t.Fatalf("resize the mounted filesystem to the grown partition's %d blocks: %v", blocks, err)
	}
	requireFilesystem(t, node, dir, blocks, witness)

	if err := unix.Unmount(dir, 0); err != nil {
		t.Fatal(err)
	}
	mounted = false
	if err := unix.IoctlSetInt(int(disk.File().Fd()), unix.BLKRRPART, 0); err != nil {
		t.Fatalf("BLKRRPART with nothing mounted: %v", err)
	}
	requireKernelPartition(t, disk, "after the kernel's own scan of the grown table", perm.First, grownSectors)
	if err := unix.Mount(node, dir, "ext4", 0, ""); err != nil {
		t.Fatalf("mount %s after the reread: %v", node, err)
	}
	mounted = true
	requireFilesystem(t, node, dir, blocks, witness)
}

// requireFilesystem fails unless the filesystem on node has blocks blocks and
// the witness under dir reads back.
func requireFilesystem(t *testing.T, node, dir string, blocks uint64, witness []byte) {
	t.Helper()
	device, err := os.Open(filepath.Clean(node))
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close() //nolint:errcheck // read-only
	superblock, err := ext4fs.ReadSuperblock(device)
	if err != nil {
		t.Fatal(err)
	}
	if superblock.BlockCount != blocks {
		t.Fatalf("the filesystem on %s has %d blocks, want %d", node, superblock.BlockCount, blocks)
	}
	got, err := os.ReadFile(filepath.Join(dir, "witness"))
	if err != nil || !bytes.Equal(got, witness) {
		t.Fatalf("the witness file did not survive: %v", err)
	}
}

// TestKernelReadsATableAtEveryStopOfTheRewrite settles spec assumption A-15
// and AC-12 for the kernel's own reader: after the table rewrite is stopped
// at any sector it writes, the kernel's partition scan finds partition 4 at
// its old or its new bounds.
//
// The kernel reads the backup copy only through the primary's alternate
// pointer unless the `gpt` boot option is set (recalled from
// block/partitions/efi.c, find_valid_gpt; not read). If that holds, a stop
// inside the primary write leaves a disk the kernel reads with no partition,
// and this test fails at those stops while TestGrowLeavesValidTableAtEveryWrite
// (the firmware's reader) passes.
//
// Method: for every stop point, write the layout into a fresh sparse file of
// a larger disk, run Grow through a disk that stops after that many sectors,
// attach the file with partition scanning, and read the kernel's partition 4.
func TestKernelReadsATableAtEveryStopOfTheRewrite(t *testing.T) {
	layout := gptfixture.Appliance(2048)
	old := layout.Partitions[3]
	diskBytes := (layout.DiskSectors + 8192) * SectorBytes
	full := &stoppingDisk{inner: grownImage(layout, 8192), budget: -1}
	growth, err := Grow(full, geometry(diskBytes))
	if err != nil {
		t.Fatal(err)
	}
	oldSectors := old.Last - old.First + 1
	newSectors := growth.PartitionBytesAfter / SectorBytes
	for budget := range full.written + 1 {
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			image := filepath.Join(t.TempDir(), "disk.img")
			writeLayoutFile(t, image, layout, diskBytes)
			file, err := os.OpenFile(image, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Grow(&stoppingDisk{inner: file, budget: budget}, geometry(diskBytes))
			if budget < full.written && !errors.Is(err, errStopped) {
				t.Fatalf("Grow = %v, want the stop", err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			disk := ext4fixture.AttachDisk(t, image)
			start, size, ok := disk.KernelPartition(t, PersistentPartition)
			if !ok {
				t.Fatalf("stop after %d of %d sector writes: the kernel found no partition %d (A-15: it did not fall back to the backup)", budget, full.written, PersistentPartition)
			}
			if start != old.First || (size != oldSectors && size != newSectors) {
				t.Fatalf("stop after %d sector writes: the kernel holds partition %d at sector %d with %d sectors, want sector %d with %d or %d",
					budget, PersistentPartition, start, size, old.First, oldSectors, newSectors)
			}
		})
	}
}
