// Design: docs/architecture/appliance/on-device-installer.md -- the installer's calls into the growth leaves
// Related: system.go -- growPermTable and growPermFS, the seams this file fills
// Related: run.go -- where an install runs the two growth steps

//go:build linux

package disk

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/ext4fs"
	"github.com/ze-software/ze/internal/core/gptdisk"
)

func init() {
	growPermTable = sysGrowPermTable
	growPermFS = sysGrowPermFS
}

// sysGrowPermTable extends the partition table of disk through the
// partition-table leaf.
//
// It is a wiring stub: it opens no device and answers the leaf's
// not-implemented error. The installer-growth phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md opens the
// disk, reads its size and sector size and calls gptdisk.Grow.
func sysGrowPermTable(disk string) error {
	return fmt.Errorf("grow the partition table of %s: %w", disk, gptdisk.ErrNotImplemented)
}

// sysGrowPermFS grows the ext4 filesystem mounted on mountPoint to the size of
// partition, through the ext4 leaf.
//
// It is a wiring stub: it makes no kernel call and answers the leaf's
// not-implemented error. The installer-growth phase reads the partition's size
// and calls ext4fs.Resize.
func sysGrowPermFS(mountPoint, partition string) error {
	return fmt.Errorf("grow the filesystem of %s mounted on %s: %w", partition, mountPoint, ext4fs.ErrNotImplemented)
}
