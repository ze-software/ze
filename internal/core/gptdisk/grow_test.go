// Design: docs/architecture/appliance/on-device-installer.md -- extending the table of an installed disk

package gptdisk

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gokrazy/tools/packer"

	"github.com/ze-software/ze/internal/test/gptfixture"
)

// geometry is a 512-byte-sector disk of diskBytes.
func geometry(diskBytes uint64) Geometry {
	return Geometry{DiskBytes: diskBytes, SectorBytes: SectorBytes}
}

// requireUnchangedOutsideEntryFour fails unless the MBR, the header fields
// Grow keeps and every entry byte but entry 4's last sector equal before.
func requireUnchangedOutsideEntryFour(t *testing.T, before, after []byte) {
	t.Helper()
	if !bytes.Equal(before[:SectorBytes], after[:SectorBytes]) {
		t.Error("the MBR changed")
	}
	header := [][2]int{{0, 16}, {20, 24}, {40, 48}, {56, 88}, {92, SectorBytes}}
	for _, span := range header {
		if !bytes.Equal(before[SectorBytes+span[0]:SectorBytes+span[1]], after[SectorBytes+span[0]:SectorBytes+span[1]]) {
			t.Errorf("primary header bytes %d..%d changed", span[0], span[1])
		}
	}
	entries := 2 * SectorBytes
	four := entries + 3*entryBytes
	if !bytes.Equal(before[entries:four+40], after[entries:four+40]) {
		t.Error("entries 1 to 3, or entry 4's type, GUID or first sector, changed")
	}
	if !bytes.Equal(before[four+48:entries+128*entryBytes], after[four+48:entries+128*entryBytes]) {
		t.Error("entry 4's attributes or name, or a later entry, changed")
	}
}

// VALIDATES: on the table the gokrazy packer wrote for a 2 GiB image, on a
// disk of 3 GiB and seven sectors, Grow extends partition 4 to the last
// usable sector lowered to whole 4096-byte blocks, moves the backup to the
// disk's last sector, keeps both copies valid and equal, and changes no other
// byte of the table (AC-1's table).
// PREVENTS: a grown table the kernel or firmware reads differently from the
// image's, or a partition the resized filesystem cannot fill exactly.
func TestGrowExtendsFourthPartition(t *testing.T) {
	const imageBytes = 2 << 30
	const diskBytes = 3<<30 + 7*SectorBytes
	file, err := os.Create(filepath.Join(t.TempDir(), "disk.img"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // a test file
	pack := packer.Pack{UseGPT: true, UseGPTPartuuid: true, Partuuid: 0x2a, FirstPartitionOffsetSectors: 8192}
	if err := pack.Partition(file, imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(diskBytes); err != nil {
		t.Fatal(err)
	}
	before := make([]byte, 34*SectorBytes)
	if _, err := file.ReadAt(before, 0); err != nil {
		t.Fatal(err)
	}
	old, err := Parse(file, diskBytes)
	if err != nil {
		t.Fatal(err)
	}
	oldFour, _ := old.Partition(PersistentPartition)

	growth, err := Grow(file, geometry(diskBytes))
	if err != nil {
		t.Fatalf("Grow: %v", err)
	}
	const diskLast = diskBytes/SectorBytes - 1
	const lastUsable = diskLast - 33
	if !growth.Written || growth.PartitionBytesBefore != oldFour.SizeBytes() || growth.PartitionOffsetBytes != oldFour.OffsetBytes() {
		t.Fatalf("growth %+v, want written from %d bytes at %d", growth, oldFour.SizeBytes(), oldFour.OffsetBytes())
	}
	if growth.PartitionBytesAfter%4096 != 0 || growth.PartitionBytesAfter <= growth.PartitionBytesBefore {
		t.Fatalf("grown partition is %d bytes: not a whole number of 4096-byte blocks, or not larger", growth.PartitionBytesAfter)
	}
	primary, err := Parse(file, diskBytes)
	if err != nil {
		t.Fatalf("the grown primary does not parse: %v", err)
	}
	backup, err := parseAt(file, diskBytes, diskLast)
	if err != nil {
		t.Fatalf("the grown backup at the last sector does not parse: %v", err)
	}
	four, _ := primary.Partition(PersistentPartition)
	if primary.AlternateLBA != diskLast || primary.LastUsableLBA != lastUsable || backup.LastUsableLBA != lastUsable {
		t.Fatalf("grown headers: alternate %d last usable %d and %d, want %d and %d", primary.AlternateLBA, primary.LastUsableLBA, backup.LastUsableLBA, diskLast, lastUsable)
	}
	if four.SizeBytes() != growth.PartitionBytesAfter || four.LastLBA > lastUsable || lastUsable-four.LastLBA >= blockSectors {
		t.Fatalf("grown entry 4 %+v: want %d bytes ending within one block of sector %d", four, growth.PartitionBytesAfter, lastUsable)
	}
	if !equalEntries(primary.Entries, backup.Entries) {
		t.Fatalf("the two copies differ: %+v and %+v", primary.Entries, backup.Entries)
	}
	after := make([]byte, 34*SectorBytes)
	if _, err := file.ReadAt(after, 0); err != nil {
		t.Fatal(err)
	}
	requireUnchangedOutsideEntryFour(t, before, after)
}

func equalEntries(left, right []Entry) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// VALIDATES: a second Grow writes nothing, and a disk the size of the image
// gets no write at all (AC-3).
// PREVENTS: a step repeated at every boot or command that rewrites a table
// with nothing to extend.
func TestGrowIsIdempotent(t *testing.T) {
	layout := gptfixture.Appliance(1024)
	disk := grownImage(layout, 4096)
	if growth, err := Grow(disk, geometry(uint64(len(disk)))); err != nil || !growth.Written {
		t.Fatalf("first Grow = %+v, %v; want a write", growth, err)
	}
	grown := bytes.Clone(disk)
	counted := &stoppingDisk{inner: disk, budget: -1}
	growth, err := Grow(counted, geometry(uint64(len(disk))))
	if err != nil || growth.Written || growth.PartitionBytesAfter != growth.PartitionBytesBefore || counted.written != 0 {
		t.Fatalf("second Grow = %+v, %v, %d sectors written; want no write", growth, err, counted.written)
	}
	if !bytes.Equal(grown, disk) {
		t.Fatal("the second Grow changed the disk")
	}

	exact := &stoppingDisk{inner: memDisk(layout.Image()), budget: -1}
	growth, err = Grow(exact, geometry(layout.DiskSectors*SectorBytes))
	if err != nil || growth.Written || counted.written != 0 || exact.written != 0 {
		t.Fatalf("Grow on a disk the size of the image = %+v, %v, %d sectors written; want no write", growth, err, exact.written)
	}
	if growth.PartitionBytesAfter != (layout.Partitions[3].Last-layout.Partitions[3].First+1)*SectorBytes {
		t.Fatalf("Grow on a disk the size of the image answered %d bytes, not the partition's", growth.PartitionBytesAfter)
	}
}

// VALIDATES: the rewrite stopped after any sector it writes leaves a disk on
// which the primary, or else the backup at the last sector, is a valid table
// whose partition 4 starts where it did and holds at least the old partition,
// so the filesystem is whole (AC-12, for a reader that falls back to the
// backup as UEFI firmware does).
// PREVENTS: a write order with an instant at which no copy describes the
// filesystem.
func TestGrowLeavesValidTableAtEveryWrite(t *testing.T) {
	layout := gptfixture.Appliance(1024)
	old := layout.Partitions[3]
	full := &stoppingDisk{inner: grownImage(layout, 4096), budget: -1}
	growth, err := Grow(full, geometry(uint64(len(full.inner.(memDisk)))))
	if err != nil {
		t.Fatal(err)
	}
	newLast := old.First + growth.PartitionBytesAfter/SectorBytes - 1
	fallbacks := 0
	for budget := range full.written + 1 {
		disk := grownImage(layout, 4096)
		stopped := &stoppingDisk{inner: disk, budget: budget}
		_, err := Grow(stopped, geometry(uint64(len(disk))))
		if budget < full.written && !errors.Is(err, errStopped) {
			t.Fatalf("stop after %d sectors: Grow = %v, want the stop", budget, err)
		}
		table, copyName, err := firmwareView(disk, uint64(len(disk)))
		if err != nil {
			t.Fatalf("stop after %d of %d sectors: no valid table: %v", budget, full.written, err)
		}
		if copyName == "backup" {
			fallbacks++
		}
		four, ok := table.Partition(PersistentPartition)
		if !ok || four.FirstLBA != old.First || (four.LastLBA != old.Last && four.LastLBA != newLast) {
			t.Fatalf("stop after %d sectors: the %s copy holds partition 4 %+v, want sectors %d..%d or %d..%d",
				budget, copyName, four, old.First, old.Last, old.First, newLast)
		}
		for index := range 3 {
			entry, _ := table.Partition(index + 1)
			if entry.FirstLBA != layout.Partitions[index].First || entry.LastLBA != layout.Partitions[index].Last {
				t.Fatalf("stop after %d sectors: partition %d moved to %+v", budget, index+1, entry)
			}
		}
	}
	// The primary is rewritten after the backup, so some stop inside it
	// leaves only the backup valid: that is what the order exists for.
	if fallbacks == 0 {
		t.Fatal("no stop needed the backup: the cut did not reach inside the primary write")
	}
	t.Logf("%d sector writes; %d stops leave only the backup at the last sector valid", full.written, fallbacks)
}

// VALIDATES: the backup copy the image carried is zeroed once the new table
// is in place, and on a disk only eight sectors larger, where the two copies
// overlap, the clearing spares the new backup.
// PREVENTS: a stale backup header inside the grown partition that a GPT tool
// reads as the disk's table, or a clearing that destroys the new backup.
func TestGrowClearsStaleBackup(t *testing.T) {
	layout := gptfixture.Appliance(1024)
	oldLast := layout.DiskSectors - 1
	for _, extra := range []uint64{4096, 8} {
		disk := grownImage(layout, extra)
		if _, err := Grow(disk, geometry(uint64(len(disk)))); err != nil {
			t.Fatalf("grown by %d sectors: %v", extra, err)
		}
		diskLast := uint64(len(disk))/SectorBytes - 1
		if _, err := parseAt(disk, uint64(len(disk)), diskLast); err != nil {
			t.Fatalf("grown by %d sectors: the new backup does not parse: %v", extra, err)
		}
		newBackupStart := diskLast - 32
		for lba := oldLast - 32; lba <= oldLast && lba < newBackupStart; lba++ {
			sector := disk[lba*SectorBytes : (lba+1)*SectorBytes]
			if !bytes.Equal(sector, make([]byte, SectorBytes)) {
				t.Fatalf("grown by %d sectors: stale backup sector %d is not zero", extra, lba)
			}
		}
	}
}

// VALIDATES: a 4096-byte logical sector, a failed checksum, a table without
// partition 4, a partition 4 that is not the last, and a disk smaller than
// the table are each refused with the reason and no sector written.
// PREVENTS: a write to a disk this code would have to guess about.
func TestGrowRefusesUnsupportedDisk(t *testing.T) {
	layout := gptfixture.Appliance(1024)
	notLast := gptfixture.Appliance(1024)
	notLast.Partitions[3].Last -= 16
	notLast.Partitions = append(notLast.Partitions, gptfixture.Partition{
		Type: gptfixture.TypeLinuxFilesystem, First: notLast.Partitions[3].Last + 1, Last: notLast.Partitions[3].Last + 8,
	})
	noFour := gptfixture.Appliance(1024)
	noFour.Partitions = noFour.Partitions[:3]
	for _, test := range []struct {
		name     string
		disk     memDisk
		geometry func(memDisk) Geometry
		want     error
		reason   string
	}{
		{name: "4096-byte sectors", disk: grownImage(layout, 4096), want: ErrUnsupportedDisk, reason: "logical sector size is 4096 bytes",
			geometry: func(disk memDisk) Geometry { return Geometry{DiskBytes: uint64(len(disk)), SectorBytes: 4096} }},
		{name: "failed checksum", disk: func() memDisk {
			disk := grownImage(layout, 4096)
			disk[SectorBytes+48] ^= 1
			return disk
		}(), want: ErrInvalidTable, reason: "header checksum"},
		{name: "partition 4 not the last", disk: grownImage(notLast, 4096), want: ErrUnsupportedDisk, reason: "partition 5 lies after partition 4"},
		{name: "no partition 4", disk: grownImage(noFour, 4096), want: ErrUnsupportedDisk, reason: "no partition 4"},
		{name: "disk smaller than the table", disk: memDisk(layout.Image()), want: ErrInvalidTable, reason: "larger than this disk",
			geometry: func(disk memDisk) Geometry { return geometry(uint64(len(disk)) - SectorBytes) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			size := geometry(uint64(len(test.disk)))
			if test.geometry != nil {
				size = test.geometry(test.disk)
			}
			counted := &stoppingDisk{inner: test.disk, budget: -1}
			_, err := Grow(counted, size)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("Grow = %v, want %v with %q", err, test.want, test.reason)
			}
			if counted.written != 0 {
				t.Fatalf("Grow wrote %d sectors before it refused", counted.written)
			}
		})
	}
}

// VALIDATES: on a 3 TiB sparse disk, past 2^32 sectors, partition 4 grows
// to the disk's end with no wrapped sector number, and both copies parse.
// PREVENTS: the packer's 32-bit sector arithmetic (permSize) reaching the
// growth.
func TestGrowBeyondThirtyTwoBitSectors(t *testing.T) {
	const diskBytes = 3 << 40
	layout := gptfixture.Appliance(1024)
	file, err := os.Create(filepath.Join(t.TempDir(), "disk.img"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // a test file
	if err := file.Truncate(diskBytes); err != nil {
		t.Skipf("the test directory's filesystem holds no 3 TiB sparse file: %v", err)
	}
	if err := layout.Write(file); err != nil {
		t.Fatal(err)
	}
	// The fixture wrote its backup at the end of its own size, as an image
	// written raw to a larger disk carries it.
	growth, err := Grow(file, geometry(diskBytes))
	if err != nil {
		t.Fatalf("Grow: %v", err)
	}
	const diskLast = diskBytes/SectorBytes - 1
	table, err := Parse(file, diskBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseAt(file, diskBytes, diskLast); err != nil {
		t.Fatalf("the backup at sector %d does not parse: %v", uint64(diskLast), err)
	}
	four, _ := table.Partition(PersistentPartition)
	want := layout.Partitions[3].First + (diskLast-33-layout.Partitions[3].First+1)/blockSectors*blockSectors - 1
	if four.LastLBA != want || four.LastLBA <= 1<<32 || growth.PartitionBytesAfter != four.SizeBytes() {
		t.Fatalf("partition 4 ends at sector %d (%d bytes), want sector %d", four.LastLBA, growth.PartitionBytesAfter, want)
	}
}
