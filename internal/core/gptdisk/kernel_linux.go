// Design: docs/architecture/appliance/on-device-installer.md -- telling the kernel a grown partition's size
// Overview: grow.go -- the growth whose result this file hands the kernel
// Related: kernel_other.go -- the refusal on a system that is not Linux

//go:build linux

package gptdisk

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// blkpgIoctl is BLKPG, `#define BLKPG _IO(0x12,105)` in
// /usr/include/linux/fs.h: no size and no direction, so (0x12<<8)|105. The
// vendored golang.org/x/sys/unix carries the BLKPG_RESIZE_PARTITION operation
// and the BlkpgIoctlArg and BlkpgPartition types (struct blkpg_ioctl_arg and
// struct blkpg_partition of /usr/include/linux/blkpg.h), and not this number.
const blkpgIoctl = 0x1269

// ResizeInKernel tells the kernel the size Grow gave the partition, on the
// whole-disk device disk, while the disk's partitions are in use: a full
// reread of the table is refused then. It changes no byte on the disk, only
// the kernel's partition, so the mounted filesystem can then be resized into
// it.
//
// It answers nil with no kernel call when Grow did not change the partition.
// The kernel keeps the partition's start: the call names the start Grow kept.
func ResizeInKernel(disk *os.File, growth Growth) error {
	if growth.PartitionBytesAfter == growth.PartitionBytesBefore {
		return nil
	}
	if growth.Partition <= 0 {
		return errors.New("BUG: gptdisk: ResizeInKernel was given a growth with no partition number")
	}
	partition := unix.BlkpgPartition{
		Start:  int64(growth.PartitionOffsetBytes), //nolint:gosec // an offset inside a disk whose size fits int64
		Length: int64(growth.PartitionBytesAfter),  //nolint:gosec // a size inside a disk whose size fits int64
		Pno:    int32(growth.Partition),            //nolint:gosec // a GPT entry number, at most 128
	}
	argument := unix.BlkpgIoctlArg{
		Op:      unix.BLKPG_RESIZE_PARTITION,
		Datalen: int32(unsafe.Sizeof(partition)),
		Data:    (*byte)(unsafe.Pointer(&partition)), //nolint:gosec // G103: the kernel copies struct blkpg_partition during the call and keeps no reference
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, disk.Fd(), blkpgIoctl, uintptr(unsafe.Pointer(&argument))) //nolint:gosec // G103: the kernel copies struct blkpg_ioctl_arg during the call
	runtime.KeepAlive(&partition)
	runtime.KeepAlive(&argument)
	if errno != 0 {
		return fmt.Errorf("tell the kernel that partition %d of %s is now %d bytes: %w", growth.Partition, disk.Name(), growth.PartitionBytesAfter, errno)
	}
	return nil
}
