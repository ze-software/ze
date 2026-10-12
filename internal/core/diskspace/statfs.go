// Design: docs/architecture/doctor-and-health-checks.md -- usable disk accounting.
// Related: statfs_openbsd.go -- the same answer from OpenBSD's field names

//go:build !openbsd

package diskspace

import "golang.org/x/sys/unix"

// blocksOf reads the block counts out of one statfs result.
func blocksOf(stat *unix.Statfs_t) Blocks {
	return Blocks{
		Total:     UsableBlocks(stat.Blocks),
		Available: UsableBlocks(stat.Bavail),
		Size:      UsableBlocks(stat.Bsize),
	}
}
