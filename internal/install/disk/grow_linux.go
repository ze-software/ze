// Design: docs/architecture/appliance/on-device-installer.md -- the installer's calls into the growth leaves
// Related: system.go -- growPermTable and growPermFS, the seams this file fills
// Related: run.go -- where an install runs the two growth steps

//go:build linux

package disk

import (
	"errors"
	"fmt"
)

// errPermTableGrowthNotWired is what sysGrowPermTable answers until the
// installer reads the disk's size and sector size: gptdisk.Grow needs both,
// and the installer-growth phase reads them.
var errPermTableGrowthNotWired = errors.New("the installer does not read the disk size for gptdisk.Grow yet")

// errPermFSGrowthNotWired is what sysGrowPermFS answers until the installer
// reads the partition's size: ext4fs.Resize needs the block count of the
// partition, which the installer-growth phase computes.
var errPermFSGrowthNotWired = errors.New("the installer does not read the partition size for ext4fs.Resize yet")

func init() {
	growPermTable = sysGrowPermTable
	growPermFS = sysGrowPermFS
}

// sysGrowPermTable extends the partition table of disk through the
// partition-table leaf.
//
// It is a wiring stub: it opens no device and answers
// errPermTableGrowthNotWired. The installer-growth phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md opens the
// disk, reads its size and sector size and calls gptdisk.Grow, which is
// written and tested.
func sysGrowPermTable(disk string) error {
	return fmt.Errorf("grow the partition table of %s: %w", disk, errPermTableGrowthNotWired)
}

// sysGrowPermFS grows the ext4 filesystem mounted on mountPoint to the size of
// partition, through the ext4 leaf.
//
// It is a wiring stub: it makes no kernel call and answers
// errPermFSGrowthNotWired. The installer-growth phase reads the partition's
// size and calls ext4fs.Resize.
func sysGrowPermFS(mountPoint, partition string) error {
	return fmt.Errorf("grow the filesystem of %s mounted on %s: %w", partition, mountPoint, errPermFSGrowthNotWired)
}
