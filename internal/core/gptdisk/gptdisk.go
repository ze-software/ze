// Design: docs/architecture/appliance/on-device-installer.md -- the shared GPT partition-table leaf
// Detail: grow.go -- extending the table of an installed disk to the disk's size
// Detail: kernel_linux.go -- telling the kernel a partition's new size on a disk in use

// Package gptdisk reads, validates and extends the GPT partition table of an
// appliance disk or image.
//
// The table arithmetic is pure Go over a reader and a writer, so the build
// host, the installer initrd, the daemon and the test harness read a table
// the same way. One Linux-only call, ResizeInKernel, tells the kernel the new
// size of a partition on a disk whose other partitions are in use. A disk with
// nothing mounted is reread by its caller instead.
//
// A table is untrusted input: every offset, count and size is bounded before
// it is used, in 64-bit sector arithmetic.
//
// The layout is the GPT of the UEFI specification, chapter 5, as the gokrazy
// packer writes it (vendor/github.com/gokrazy/tools/packer/packer.go,
// writeGPT and writePartitionTable):
//
//	sector 0       MBR: boot code, four partition records, 0x55 0xAA
//	sector 1       primary header
//	sector 2..     primary entry array, 128 bytes per entry
//	...            partitions, from the first to the last usable sector
//	last-N..last-1 backup entry array
//	last           backup header
//
//	header field         offset  size
//	signature "EFI PART"      0     8
//	revision                  8     4
//	header size              12     4
//	header CRC32             16     4   computed with this field zero
//	reserved                 20     4
//	my LBA                   24     8
//	alternate LBA            32     8
//	first usable LBA         40     8
//	last usable LBA          48     8
//	disk GUID                56    16
//	entry array LBA          72     8
//	entry count              80     4
//	entry size               84     4
//	entry array CRC32        88     4
//
//	entry field          offset  size
//	type GUID                 0    16   all zeros: the entry is unused
//	partition GUID           16    16
//	first LBA                32     8
//	last LBA                 40     8   inclusive
//	attributes               48     8
//	name, UTF-16LE           56    72
//
// Safe for concurrent use: no function holds state.
package gptdisk

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
)

// SectorBytes is the logical sector the appliance's table is written in: the
// packer's writeGPT counts in 512-byte sectors. A disk of another logical
// sector size is outside what the image supports.
const SectorBytes = 512

// PersistentPartition is the number of the persistent partition in gokrazy's
// table: boot, root A, root B, persistent.
const PersistentPartition = 4

const (
	headerLBA      = 1
	headerBytesMin = 92
	entryBytes     = 128
	// entryCountMax bounds the entry array read from a header. The packer
	// writes 128 entries.
	entryCountMax = 128
	// mbrRecords is where the four MBR partition records start, 16 bytes
	// each, the type byte at 4 and the first sector at 8.
	mbrRecords      = 446
	protectiveType  = 0xee
	mbrRecordsCount = 4
)

var headerSignature = []byte("EFI PART")

// ErrInvalidTable is what Parse answers, wrapped with the reason, for bytes
// that do not hold a GPT the kernel reads.
var ErrInvalidTable = errors.New("not a valid GPT partition table")

// Entry is one used entry of the table.
type Entry struct {
	// Number is the entry's place in the array, from 1. The kernel names
	// the partition by it (sda4 is entry 4).
	Number int
	// Type is the partition type GUID, in the order the disk stores it.
	Type [16]byte
	// GUID is the partition's own GUID, in the order the disk stores it.
	GUID [16]byte
	// FirstLBA and LastLBA are the partition's first and last sector,
	// inclusive.
	FirstLBA uint64
	LastLBA  uint64
}

// OffsetBytes is where the partition starts on the disk.
func (entry Entry) OffsetBytes() uint64 { return entry.FirstLBA * SectorBytes }

// SizeBytes is the size of the partition.
func (entry Entry) SizeBytes() uint64 { return (entry.LastLBA - entry.FirstLBA + 1) * SectorBytes }

// Table is a primary table that Parse validated, with the bytes it was read
// from, so Grow changes only the fields it means to.
type Table struct {
	// DiskSectors is the size of the disk the table was read from.
	DiskSectors uint64
	// AlternateLBA is where the primary header says the backup header is.
	AlternateLBA uint64
	// FirstUsableLBA and LastUsableLBA bound every partition.
	FirstUsableLBA uint64
	LastUsableLBA  uint64
	// Entries are the used entries, in array order.
	Entries []Entry

	header     []byte // the header sector as read
	entries    []byte // the entry array as read, entryCount*entryBytes
	entriesLBA uint64
}

// Partition answers the used entry numbered number.
func (table Table) Partition(number int) (Entry, bool) {
	for _, entry := range table.Entries {
		if entry.Number == number {
			return entry, true
		}
	}
	return Entry{}, false
}

// entrySectors is the number of sectors the entry array takes.
func (table Table) entrySectors() uint64 {
	return (uint64(len(table.entries)) + SectorBytes - 1) / SectorBytes
}

// Parse reads and validates the primary table of a disk of diskBytes.
//
// It requires what the kernel requires to read a GPT at all (a protective
// MBR record, the header's own sector, both checksums), and what Grow relies
// on: the table fits the disk, the backup entries fit between the last usable
// sector and the backup header, every used entry lies inside the usable
// sectors and no two overlap. It never reads the backup: a caller that needs
// the copy the firmware falls back to reads the disk's last sector itself.
func Parse(disk io.ReaderAt, diskBytes uint64) (Table, error) {
	return parseAt(disk, diskBytes, headerLBA)
}

// parseAt validates the table whose header is at headerAt. Parse reads the
// primary; the tests read the backup with it, the way UEFI firmware falls
// back to it.
func parseAt(disk io.ReaderAt, diskBytes, headerAt uint64) (Table, error) {
	if diskBytes%SectorBytes != 0 {
		return Table{}, fmt.Errorf("%w: the disk is %d bytes, not a whole number of %d-byte sectors", ErrInvalidTable, diskBytes, SectorBytes)
	}
	table := Table{DiskSectors: diskBytes / SectorBytes}
	if table.DiskSectors <= headerAt {
		return Table{}, fmt.Errorf("%w: the disk holds %d sectors, too few for a GPT header", ErrInvalidTable, table.DiskSectors)
	}
	if err := readProtectiveMBR(disk); err != nil {
		return Table{}, err
	}
	header, err := readSector(disk, headerAt)
	if err != nil {
		return Table{}, fmt.Errorf("read the GPT header: %w", err)
	}
	if err := checkHeader(header, headerAt); err != nil {
		return Table{}, err
	}
	table.header = header
	table.AlternateLBA = binary.LittleEndian.Uint64(header[32:40])
	table.FirstUsableLBA = binary.LittleEndian.Uint64(header[40:48])
	table.LastUsableLBA = binary.LittleEndian.Uint64(header[48:56])
	table.entriesLBA = binary.LittleEndian.Uint64(header[72:80])
	if err := table.checkBounds(binary.LittleEndian.Uint32(header[80:84]), binary.LittleEndian.Uint32(header[84:88]), headerAt); err != nil {
		return Table{}, err
	}
	table.entries = make([]byte, uint64(binary.LittleEndian.Uint32(header[80:84]))*entryBytes)
	if _, err := disk.ReadAt(table.entries, int64(table.entriesLBA*SectorBytes)); err != nil { //nolint:gosec // bounded by checkBounds: below the disk's size
		return Table{}, fmt.Errorf("read the GPT entries: %w", err)
	}
	if crc32.ChecksumIEEE(table.entries) != binary.LittleEndian.Uint32(header[88:92]) {
		return Table{}, fmt.Errorf("%w: the entry array checksum does not match", ErrInvalidTable)
	}
	if err := table.readEntries(); err != nil {
		return Table{}, err
	}
	return table, nil
}

// readProtectiveMBR requires the MBR signature and a 0xEE record that starts
// at sector 1: without them the kernel does not read the GPT. The packer's
// hybrid MBR carries the 0xEE record beside the boot partition's record.
func readProtectiveMBR(disk io.ReaderAt) error {
	mbr, err := readSector(disk, 0)
	if err != nil {
		return fmt.Errorf("read the MBR: %w", err)
	}
	if mbr[510] != 0x55 {
		return fmt.Errorf("%w: sector 0 has no MBR signature (byte 510 is not 0x55)", ErrInvalidTable)
	}
	if mbr[511] != 0xaa {
		return fmt.Errorf("%w: sector 0 has no MBR signature (byte 511 is not 0xAA)", ErrInvalidTable)
	}
	for index := range mbrRecordsCount {
		record := mbr[mbrRecords+16*index : mbrRecords+16*(index+1)]
		if record[4] != protectiveType {
			continue
		}
		if binary.LittleEndian.Uint32(record[8:12]) == headerLBA {
			return nil
		}
	}
	return fmt.Errorf("%w: the MBR has no protective 0xEE record starting at sector 1", ErrInvalidTable)
}

// checkHeader validates the signature, the size, the checksum and the
// header's own sector number.
func checkHeader(header []byte, headerAt uint64) error {
	if !bytes.Equal(header[:8], headerSignature) {
		return fmt.Errorf("%w: sector %d has no GPT header signature", ErrInvalidTable, headerAt)
	}
	size := binary.LittleEndian.Uint32(header[12:16])
	if size < headerBytesMin {
		return fmt.Errorf("%w: the header is %d bytes, below the %d the format fixes", ErrInvalidTable, size, headerBytesMin)
	}
	if size > SectorBytes {
		return fmt.Errorf("%w: the header is %d bytes, more than its sector", ErrInvalidTable, size)
	}
	if headerChecksum(header) != binary.LittleEndian.Uint32(header[16:20]) {
		return fmt.Errorf("%w: the header checksum does not match", ErrInvalidTable)
	}
	if mine := binary.LittleEndian.Uint64(header[24:32]); mine != headerAt {
		return fmt.Errorf("%w: the header at sector %d says it is at sector %d", ErrInvalidTable, headerAt, mine)
	}
	return nil
}

// checkBounds validates the usable sectors, the alternate header and the
// entry array against the disk, before anything is read at those places.
func (table *Table) checkBounds(count, size uint32, headerAt uint64) error {
	if size != entryBytes {
		return fmt.Errorf("%w: its entries are %d bytes, and only %d-byte entries are read", ErrInvalidTable, size, entryBytes)
	}
	if count == 0 {
		return fmt.Errorf("%w: the header declares no entry", ErrInvalidTable)
	}
	if count > entryCountMax {
		return fmt.Errorf("%w: the header declares %d entries, above %d", ErrInvalidTable, count, entryCountMax)
	}
	entrySectors := (uint64(count)*entryBytes + SectorBytes - 1) / SectorBytes
	last := table.DiskSectors - 1
	if table.AlternateLBA > last {
		return fmt.Errorf("%w: the table describes a disk of at least %d sectors (its backup header is at sector %d), larger than this disk of %d sectors",
			ErrInvalidTable, table.AlternateLBA+1, table.AlternateLBA, table.DiskSectors)
	}
	if table.FirstUsableLBA > table.LastUsableLBA {
		return fmt.Errorf("%w: its first usable sector %d is after its last usable sector %d", ErrInvalidTable, table.FirstUsableLBA, table.LastUsableLBA)
	}
	if table.entriesLBA <= headerLBA {
		return fmt.Errorf("%w: its entry array at sector %d overlaps the MBR or the header", ErrInvalidTable, table.entriesLBA)
	}
	if headerAt == headerLBA {
		return table.checkPrimaryBounds(entrySectors)
	}
	return table.checkBackupBounds(entrySectors, headerAt)
}

// checkPrimaryBounds: the primary's entries sit before the first usable
// sector, and the backup's entries fit between the last usable sector and the
// alternate header.
func (table *Table) checkPrimaryBounds(entrySectors uint64) error {
	if table.entriesLBA+entrySectors > table.FirstUsableLBA {
		return fmt.Errorf("%w: its entry array runs into the first usable sector %d", ErrInvalidTable, table.FirstUsableLBA)
	}
	if table.AlternateLBA <= table.LastUsableLBA+entrySectors {
		return fmt.Errorf("%w: its backup header at sector %d leaves no room for the backup entries after the last usable sector %d",
			ErrInvalidTable, table.AlternateLBA, table.LastUsableLBA)
	}
	return nil
}

// checkBackupBounds: the backup's entries sit after the last usable sector
// and before the backup header.
func (table *Table) checkBackupBounds(entrySectors, headerAt uint64) error {
	if table.entriesLBA <= table.LastUsableLBA {
		return fmt.Errorf("%w: the backup entry array at sector %d is inside the usable sectors", ErrInvalidTable, table.entriesLBA)
	}
	if table.entriesLBA+entrySectors > headerAt {
		return fmt.Errorf("%w: the backup entry array runs into its header", ErrInvalidTable)
	}
	return nil
}

// readEntries decodes the used entries and refuses one outside the usable
// sectors, reversed, or overlapping another.
func (table *Table) readEntries() error {
	var unused [16]byte
	for index := range len(table.entries) / entryBytes {
		raw := table.entries[index*entryBytes : (index+1)*entryBytes]
		if bytes.Equal(raw[:16], unused[:]) {
			continue
		}
		entry := Entry{Number: index + 1, FirstLBA: binary.LittleEndian.Uint64(raw[32:40]), LastLBA: binary.LittleEndian.Uint64(raw[40:48])}
		copy(entry.Type[:], raw[0:16])
		copy(entry.GUID[:], raw[16:32])
		if entry.LastLBA < entry.FirstLBA {
			return fmt.Errorf("%w: partition %d ends at sector %d, before it starts at sector %d", ErrInvalidTable, entry.Number, entry.LastLBA, entry.FirstLBA)
		}
		if entry.FirstLBA < table.FirstUsableLBA {
			return fmt.Errorf("%w: partition %d starts at sector %d, inside the table's own sectors", ErrInvalidTable, entry.Number, entry.FirstLBA)
		}
		if entry.LastLBA > table.LastUsableLBA {
			return fmt.Errorf("%w: partition %d ends at sector %d, after the last usable sector %d", ErrInvalidTable, entry.Number, entry.LastLBA, table.LastUsableLBA)
		}
		for _, other := range table.Entries {
			if entry.FirstLBA <= other.LastLBA && other.FirstLBA <= entry.LastLBA {
				return fmt.Errorf("%w: partitions %d and %d overlap", ErrInvalidTable, other.Number, entry.Number)
			}
		}
		table.Entries = append(table.Entries, entry)
	}
	return nil
}

// headerChecksum is the CRC32 of the header's declared size, its own
// checksum field taken as zero. header is a whole sector checkHeader bounded.
func headerChecksum(header []byte) uint32 {
	size := binary.LittleEndian.Uint32(header[12:16])
	scratch := make([]byte, size)
	copy(scratch, header[:size])
	clear(scratch[16:20])
	return crc32.ChecksumIEEE(scratch)
}

func readSector(disk io.ReaderAt, lba uint64) ([]byte, error) {
	sector := make([]byte, SectorBytes)
	if _, err := disk.ReadAt(sector, int64(lba*SectorBytes)); err != nil { //nolint:gosec // every caller bounds lba by the disk's size
		return nil, err
	}
	return sector, nil
}
