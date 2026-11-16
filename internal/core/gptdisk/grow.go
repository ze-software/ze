// Design: docs/architecture/appliance/on-device-installer.md -- extending the table of an installed disk
// Overview: gptdisk.go -- the table this file extends, and its validation

package gptdisk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
)

// blockSectors is the persistent partition's filesystem block, 4096 bytes, in
// sectors. The grown partition is a whole number of blocks, so the filesystem
// resized to it fills it.
const blockSectors = 4096 / SectorBytes

// ErrUnsupportedDisk is what Grow answers, wrapped with the reason, for a
// disk whose table it must not guess about. Nothing was written.
var ErrUnsupportedDisk = errors.New("the partition table of this disk cannot be extended")

// Disk is the device or image file whose table Grow extends.
type Disk interface {
	io.ReaderAt
	io.WriterAt
	// Sync flushes what was written to stable storage. Grow calls it after
	// each copy of the table, so the backup copy is on the disk before the
	// primary copy is changed.
	Sync() error
}

// Geometry is what the caller read from the device before it calls Grow.
type Geometry struct {
	// DiskBytes is the size of the whole disk.
	DiskBytes uint64
	// SectorBytes is the logical sector size of the disk.
	SectorBytes uint32
}

// Growth is what Grow did to the persistent partition.
type Growth struct {
	// Partition is the number of the partition Grow extended.
	Partition int
	// PartitionOffsetBytes is where the partition starts. Grow never moves it.
	PartitionOffsetBytes uint64
	// PartitionBytesBefore is the size of the partition as Grow found it.
	PartitionBytesBefore uint64
	// PartitionBytesAfter is its size when Grow returned: a whole number of
	// 4096-byte blocks when Grow extended it, and PartitionBytesBefore when
	// it did not.
	PartitionBytesAfter uint64
	// Written says Grow wrote the table. It is false when the table already
	// described the whole disk, and then no sector was written.
	Written bool
}

// Grow extends the table of disk so that the persistent partition, the
// fourth, ends at the disk's last usable sector, lowered to a whole number of
// 4096-byte blocks.
//
// The backup copy is written at the end of the disk and synced first, then
// the primary copy, then the backup copy the image carried is zeroed. Until
// the primary is changed, the old primary describes the old partition; from
// the moment the backup is synced, the backup at the disk's last sector
// describes the new one. A stop inside the primary write leaves a primary that
// fails its checksum and a valid backup at the last sector, which UEFI
// firmware and GPT tools fall back to.
//
// Everything else stays byte for byte: the MBR, the disk GUID, every
// partition GUID, type and name, the first three entries and the partition's
// first sector. A table that already describes the whole disk gets no write,
// so a second call writes nothing. The partition never shrinks.
//
// It refuses, before any write, a logical sector size that is not 512, a
// table Parse refuses (a failed checksum, a table larger than the disk), a
// table without partition 4, and a partition 4 that is not the last
// partition.
func Grow(disk Disk, geometry Geometry) (Growth, error) {
	if geometry.SectorBytes != SectorBytes {
		return Growth{}, fmt.Errorf("%w: its logical sector size is %d bytes, and the appliance's table is written in %d-byte sectors",
			ErrUnsupportedDisk, geometry.SectorBytes, SectorBytes)
	}
	table, err := Parse(disk, geometry.DiskBytes)
	if err != nil {
		return Growth{}, err
	}
	plan, err := planGrowth(table)
	if err != nil {
		return Growth{}, err
	}
	if !plan.write {
		return plan.growth, nil
	}
	if err := plan.apply(disk, table); err != nil {
		return Growth{}, err
	}
	plan.growth.Written = true
	return plan.growth, nil
}

// growthPlan is the grown table's numbers, computed before any write.
type growthPlan struct {
	growth     Growth
	write      bool
	partition  Entry
	lastLBA    uint64 // the partition's new last sector
	diskLast   uint64 // the disk's last sector: the new backup header
	lastUsable uint64 // the new last usable sector
}

func planGrowth(table Table) (growthPlan, error) {
	partition, ok := table.Partition(PersistentPartition)
	if !ok {
		return growthPlan{}, fmt.Errorf("%w: it has no partition %d", ErrUnsupportedDisk, PersistentPartition)
	}
	for _, entry := range table.Entries {
		if entry.Number != PersistentPartition && entry.LastLBA >= partition.FirstLBA {
			return growthPlan{}, fmt.Errorf("%w: partition %d lies after partition %d, so partition %d is not the last partition",
				ErrUnsupportedDisk, entry.Number, PersistentPartition, PersistentPartition)
		}
	}
	plan := growthPlan{partition: partition, diskLast: table.DiskSectors - 1}
	// Parse bounded the alternate header by the disk and the backup entries
	// by the old last usable sector, so this is at least the old one.
	plan.lastUsable = plan.diskLast - table.entrySectors() - 1
	plan.lastLBA = partition.LastLBA
	if whole := (plan.lastUsable - partition.FirstLBA + 1) / blockSectors * blockSectors; whole > 0 {
		plan.lastLBA = max(partition.LastLBA, partition.FirstLBA+whole-1)
	}
	plan.growth = Growth{
		Partition:            PersistentPartition,
		PartitionOffsetBytes: partition.OffsetBytes(),
		PartitionBytesBefore: partition.SizeBytes(),
		PartitionBytesAfter:  (plan.lastLBA - partition.FirstLBA + 1) * SectorBytes,
	}
	described := table.AlternateLBA == plan.diskLast && table.LastUsableLBA == plan.lastUsable
	plan.write = !described || plan.lastLBA != partition.LastLBA
	return plan, nil
}

// apply writes the backup copy, syncs, writes the primary copy, syncs, then
// zeroes the image's stale backup copy and syncs.
func (plan growthPlan) apply(disk Disk, table Table) error {
	entries := make([]byte, table.entrySectors()*SectorBytes)
	copy(entries, table.entries)
	at := (plan.partition.Number - 1) * entryBytes
	binary.LittleEndian.PutUint64(entries[at+40:at+48], plan.lastLBA)
	entriesCRC := crc32.ChecksumIEEE(entries[:len(table.entries)])

	primary := plan.header(table.header, headerLBA, plan.diskLast, table.entriesLBA, entriesCRC)
	backupEntries := plan.diskLast - table.entrySectors()
	backup := plan.header(table.header, plan.diskLast, headerLBA, backupEntries, entriesCRC)

	if err := writeSectors(disk, backupEntries, entries); err != nil {
		return fmt.Errorf("write the backup partition entries: %w", err)
	}
	if err := writeSectors(disk, plan.diskLast, backup); err != nil {
		return fmt.Errorf("write the backup partition table header: %w", err)
	}
	if err := disk.Sync(); err != nil {
		return fmt.Errorf("sync the backup partition table: %w", err)
	}
	if err := writeSectors(disk, table.entriesLBA, entries); err != nil {
		return fmt.Errorf("write the primary partition entries: %w", err)
	}
	if err := writeSectors(disk, headerLBA, primary); err != nil {
		return fmt.Errorf("write the primary partition table header: %w", err)
	}
	if err := disk.Sync(); err != nil {
		return fmt.Errorf("sync the primary partition table: %w", err)
	}
	return plan.clearStaleBackup(disk, table, backupEntries)
}

// clearStaleBackup zeroes the backup copy at the place the old primary
// pointed to, which now lies inside the grown partition, beyond its
// filesystem. A sector the new backup copy took is left alone: on a disk a
// few sectors larger than the table, the two copies overlap.
func (plan growthPlan) clearStaleBackup(disk Disk, table Table, backupEntries uint64) error {
	if table.AlternateLBA == plan.diskLast {
		return nil
	}
	first := table.AlternateLBA - table.entrySectors()
	last := min(table.AlternateLBA, backupEntries-1)
	if last < first {
		return nil
	}
	if err := writeSectors(disk, first, make([]byte, (last-first+1)*SectorBytes)); err != nil {
		return fmt.Errorf("clear the stale backup partition table: %w", err)
	}
	if err := disk.Sync(); err != nil {
		return fmt.Errorf("sync the cleared backup partition table: %w", err)
	}
	return nil
}

// header answers a header sector built from the primary as read, with the
// fields a copy at myLBA changes and both checksums.
func (plan growthPlan) header(original []byte, myLBA, alternateLBA, entriesLBA uint64, entriesCRC uint32) []byte {
	sector := make([]byte, SectorBytes)
	copy(sector, original)
	binary.LittleEndian.PutUint64(sector[24:32], myLBA)
	binary.LittleEndian.PutUint64(sector[32:40], alternateLBA)
	binary.LittleEndian.PutUint64(sector[48:56], plan.lastUsable)
	binary.LittleEndian.PutUint64(sector[72:80], entriesLBA)
	binary.LittleEndian.PutUint32(sector[88:92], entriesCRC)
	binary.LittleEndian.PutUint32(sector[16:20], headerChecksum(sector))
	return sector
}

func writeSectors(disk io.WriterAt, lba uint64, data []byte) error {
	_, err := disk.WriteAt(data, int64(lba*SectorBytes)) //nolint:gosec // lba is below the disk's sector count, which Parse read from a size in bytes
	return err
}
