// Design: docs/architecture/appliance/on-device-installer.md -- the shared GPT partition-table leaf the fixtures feed
//
// Package gptfixture writes GPT disks for tests: the layout the gokrazy packer
// gives an appliance image (vendor/github.com/gokrazy/tools/packer, writeGPT
// and writePartitionTable), at any size, so a test of a table reader or of the
// table growth runs on a few sectors instead of a 2 GiB image.
//
// It writes what it is given and validates nothing: a test that needs a broken
// table (a reversed entry, a partition past the disk) describes it and gets
// correct checksums over it, so the reader under test refuses the property the
// test names and not a checksum.
//
// It imports nothing from Ze, so the partition-table leaf's own tests import
// it without a cycle. Safe for concurrent use: no function holds state.
package gptfixture

import (
	"encoding/binary"
	"hash/crc32"
	"io"
)

// SectorBytes is the logical sector the packer writes the table in.
const SectorBytes = 512

// entryBytes is the size of one partition entry, as the packer writes it.
const entryBytes = 128

// The partition types the packer gives an amd64 appliance, in the
// mixed-endian order a GPT stores a GUID (packer.go, mustParseGUID).
var (
	// TypeEFISystem is C12A7328-F81F-11D2-BA4B-00A0C93EC93B, partition 1.
	TypeEFISystem = [16]byte{0x28, 0x73, 0x2a, 0xc1, 0x1f, 0xf8, 0xd2, 0x11, 0xba, 0x4b, 0x00, 0xa0, 0xc9, 0x3e, 0xc9, 0x3b}
	// TypeLinuxRootAMD64 is 4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709, partition 2.
	TypeLinuxRootAMD64 = [16]byte{0xe3, 0xbc, 0x68, 0x4f, 0xcd, 0xe8, 0xb1, 0x4d, 0x96, 0xe7, 0xfb, 0xca, 0xf9, 0x84, 0xb7, 0x09}
	// TypeLinuxFilesystem is 0FC63DAF-8483-4772-8E79-3D69D8477DE4,
	// partitions 3 and 4.
	TypeLinuxFilesystem = [16]byte{0xaf, 0x3d, 0xc6, 0x0f, 0x83, 0x84, 0x72, 0x47, 0x8e, 0x79, 0x3d, 0x69, 0xd8, 0x47, 0x7d, 0xe4}
)

// Partition is one entry of the array, written at its place in Layout.
type Partition struct {
	// Type is the partition type GUID. All zeros writes an unused entry.
	Type [16]byte
	// GUID is the partition's own GUID.
	GUID [16]byte
	// First and Last are the partition's first and last sector, inclusive.
	First uint64
	Last  uint64
}

// Layout is a whole disk: a hybrid MBR, the primary header and entries at
// sectors 1 and 2, and the backup entries and header in the last sectors.
type Layout struct {
	// DiskSectors is the size of the disk in sectors.
	DiskSectors uint64
	// EntryCount is the number of entries in the array. The packer writes
	// 128, which take 32 sectors.
	EntryCount uint32
	// FirstUsable is the first sector a partition may use. The packer
	// writes 34.
	FirstUsable uint64
	// DiskGUID is the disk's GUID.
	DiskGUID [16]byte
	// Partitions are written to entries 1, 2, ... in order.
	Partitions []Partition
}

// Appliance answers the packer's layout with a persistent partition of
// permSectors sectors: three small partitions of 2048 sectors each from
// sector 2048, then partition 4 ending one sector before the last usable
// sector, where the packer's permSize leaves it.
func Appliance(permSectors uint64) Layout {
	const step = 2048
	layout := Layout{EntryCount: 128, FirstUsable: 34, DiskGUID: guid(0)}
	types := [3][16]byte{TypeEFISystem, TypeLinuxRootAMD64, TypeLinuxFilesystem}
	for index, kind := range types {
		first := uint64(step * (index + 1))
		layout.Partitions = append(layout.Partitions, Partition{Type: kind, GUID: guid(index + 1), First: first, Last: first + step - 1})
	}
	first := uint64(step * 4)
	last := first + permSectors - 1
	layout.Partitions = append(layout.Partitions, Partition{Type: TypeLinuxFilesystem, GUID: guid(4), First: first, Last: last})
	// The last usable sector is last+1, and the backup entries and header
	// follow it: 32 sectors of entries and one of header.
	layout.DiskSectors = last + 2 + layout.entrySectors() + 1
	return layout
}

// Image answers the whole disk in memory. It is for small layouts only: the
// slice is DiskSectors sectors long.
func (layout Layout) Image() []byte {
	disk := make([]byte, layout.DiskSectors*SectorBytes)
	writer := sliceWriter(disk)
	if err := layout.Write(writer); err != nil {
		panic("BUG: gptfixture: a write into a slice of the disk's size failed: " + err.Error())
	}
	return disk
}

// Write writes the MBR, both headers and both entry arrays. The sectors in
// between are left as they are, so a sparse file of DiskSectors sectors stays
// sparse.
func (layout Layout) Write(disk io.WriterAt) error {
	entries := layout.entries()
	lastLBA := layout.DiskSectors - 1
	backupEntries := lastLBA - layout.entrySectors()
	if _, err := disk.WriteAt(layout.mbr(), 0); err != nil {
		return err
	}
	if _, err := disk.WriteAt(layout.header(1, lastLBA, 2, entries), SectorBytes); err != nil {
		return err
	}
	if _, err := disk.WriteAt(entries, 2*SectorBytes); err != nil {
		return err
	}
	if _, err := disk.WriteAt(entries, int64(backupEntries*SectorBytes)); err != nil { //nolint:gosec // a test disk is far below 2^63 bytes
		return err
	}
	_, err := disk.WriteAt(layout.header(lastLBA, 1, backupEntries, entries), int64(lastLBA*SectorBytes)) //nolint:gosec // a test disk is far below 2^63 bytes
	return err
}

// LastUsable is the last sector a partition may use: the one before the
// backup entries.
func (layout Layout) LastUsable() uint64 {
	return layout.DiskSectors - 1 - layout.entrySectors() - 1
}

func (layout Layout) entrySectors() uint64 {
	return (uint64(layout.EntryCount)*entryBytes + SectorBytes - 1) / SectorBytes
}

// mbr is the packer's hybrid MBR: record 1 is the FAT boot partition, record
// 2 the protective 0xEE record from sector 1 to the sector before partition 1.
func (layout Layout) mbr() []byte {
	sector := make([]byte, SectorBytes)
	firstPartition := layout.FirstUsable
	if len(layout.Partitions) > 0 {
		firstPartition = layout.Partitions[0].First
		boot := sector[446:462]
		boot[0] = 0x80
		boot[4] = 0x0c
		binary.LittleEndian.PutUint32(boot[8:], uint32(firstPartition))                              //nolint:gosec // a test disk
		binary.LittleEndian.PutUint32(boot[12:], uint32(layout.Partitions[0].Last-firstPartition+1)) //nolint:gosec // a test disk
	}
	protective := sector[462:478]
	protective[4] = 0xee
	binary.LittleEndian.PutUint32(protective[8:], 1)
	binary.LittleEndian.PutUint32(protective[12:], uint32(firstPartition-1)) //nolint:gosec // a test disk
	sector[510], sector[511] = 0x55, 0xaa
	return sector
}

func (layout Layout) entries() []byte {
	array := make([]byte, layout.entrySectors()*SectorBytes)
	for index, partition := range layout.Partitions {
		entry := array[index*entryBytes : (index+1)*entryBytes]
		copy(entry[0:16], partition.Type[:])
		copy(entry[16:32], partition.GUID[:])
		binary.LittleEndian.PutUint64(entry[32:40], partition.First)
		binary.LittleEndian.PutUint64(entry[40:48], partition.Last)
		copy(entry[56:], "\x4c\x00\x69\x00\x6e\x00\x75\x00\x78\x00") // "Linux" in UTF-16LE
	}
	return array
}

// header is the packer's header sector, with both checksums.
func (layout Layout) header(myLBA, alternateLBA, entriesLBA uint64, entries []byte) []byte {
	sector := make([]byte, SectorBytes)
	copy(sector, "EFI PART")
	binary.LittleEndian.PutUint32(sector[8:], 0x00010000)
	binary.LittleEndian.PutUint32(sector[12:], 92)
	binary.LittleEndian.PutUint64(sector[24:], myLBA)
	binary.LittleEndian.PutUint64(sector[32:], alternateLBA)
	binary.LittleEndian.PutUint64(sector[40:], layout.FirstUsable)
	binary.LittleEndian.PutUint64(sector[48:], layout.LastUsable())
	copy(sector[56:72], layout.DiskGUID[:])
	binary.LittleEndian.PutUint64(sector[72:], entriesLBA)
	binary.LittleEndian.PutUint32(sector[80:], layout.EntryCount)
	binary.LittleEndian.PutUint32(sector[84:], entryBytes)
	binary.LittleEndian.PutUint32(sector[88:], crc32.ChecksumIEEE(entries[:uint64(layout.EntryCount)*entryBytes]))
	binary.LittleEndian.PutUint32(sector[16:], crc32.ChecksumIEEE(sector[:92]))
	return sector
}

// guid answers a GUID that differs per partition, as the packer's
// GPTPARTUUID does.
func guid(number int) [16]byte {
	var value [16]byte
	copy(value[:], "\x60\xc2\x4c\xc1\xf3\xf9\x42\x7a\x81\x99")
	value[15] = byte(number)
	return value
}

// sliceWriter writes into a byte slice of fixed size.
type sliceWriter []byte

func (disk sliceWriter) WriteAt(data []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, io.ErrShortWrite
	}
	if offset+int64(len(data)) > int64(len(disk)) {
		return 0, io.ErrShortWrite
	}
	return copy(disk[offset:], data), nil
}
