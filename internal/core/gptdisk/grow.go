// Design: docs/architecture/appliance/on-device-installer.md -- extending the table of an installed disk
// Overview: gptdisk.go -- the package and its not-implemented answer

package gptdisk

import "io"

// Disk is the device or image file whose table Grow extends.
type Disk interface {
	io.ReaderAt
	io.WriterAt
	// Sync flushes what was written to stable storage. Grow calls it between
	// the backup copy and the primary copy of the table.
	Sync() error
}

// Geometry is what the caller read from the device before it calls Grow.
type Geometry struct {
	// DiskBytes is the size of the whole disk.
	DiskBytes uint64
	// SectorBytes is the logical sector size of the disk.
	SectorBytes uint32
}

// Growth is what Grow did to the fourth partition.
type Growth struct {
	// PartitionBytesBefore is the size of the fourth partition as Grow found it.
	PartitionBytesBefore uint64
	// PartitionBytesAfter is its size when Grow returned. It equals
	// PartitionBytesBefore when the table already reached the end of the disk
	// and nothing was written.
	PartitionBytesAfter uint64
}

// Grow extends the table on disk so that its fourth partition ends at the last
// usable sector of the disk.
//
// It is a wiring stub: it reads nothing, writes nothing and answers
// ErrNotImplemented.
func Grow(_ Disk, _ Geometry) (Growth, error) {
	return Growth{}, ErrNotImplemented
}
