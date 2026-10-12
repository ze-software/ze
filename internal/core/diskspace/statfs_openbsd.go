// Design: docs/architecture/doctor-and-health-checks.md -- usable disk accounting.
// Related: statfs.go -- the same answer from the other systems' field names

package diskspace

import "golang.org/x/sys/unix"

// blocksOf reads the block counts out of one statfs result. OpenBSD prefixes
// every Statfs_t field with F_.
func blocksOf(stat *unix.Statfs_t) Blocks {
	return Blocks{
		Total:     UsableBlocks(stat.F_blocks),
		Available: UsableBlocks(stat.F_bavail),
		Size:      UsableBlocks(stat.F_bsize),
	}
}
