// Design: docs/architecture/appliance/on-device-installer.md -- the ext4 leaf: superblock properties and online resize
// Detail: resize_linux.go -- the online resize of a mounted filesystem
// Detail: resize_other.go -- the refusal on a system that is not Linux

// Package ext4fs reads the properties of an ext4 superblock and grows a
// mounted ext4 filesystem.
//
// It is a core leaf with no registry and no config lifecycle. The superblock
// read is pure Go, so the build host checks an image with it; the resize is a
// kernel call in a Linux-only file.
//
// The superblock read answers the block size and the block count, which the
// resize needs, and the quota and project feature bits, which the
// project-quota leaf reads when the kernel refuses its state query. The other
// properties the persistent partition's format fixes (the remaining features,
// the inode size, the reserved area, the journal, the quota inode, the stored
// mount option) join Superblock with the build-format phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md.
//
// Safe for concurrent use: no function holds state.
package ext4fs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrUnsupported is what Resize answers on a system that is not Linux.
var ErrUnsupported = errors.New("ext4fs: an ext4 filesystem is resized on Linux only")

// ErrNotExt4 is what ReadSuperblock answers when the bytes hold no ext4
// superblock.
var ErrNotExt4 = errors.New("ext4fs: no ext4 superblock")

// ErrZeroBlockCount is what Resize answers for a block count of zero.
var ErrZeroBlockCount = errors.New("ext4fs: a filesystem cannot be resized to zero blocks")

// Superblock holds the properties of an ext4 filesystem read from its
// superblock.
type Superblock struct {
	// BlockSize is the filesystem block size in bytes.
	BlockSize uint32
	// BlockCount is the number of blocks of the filesystem.
	BlockCount uint64
	// QuotaFeature says the filesystem carries quota in hidden system inodes
	// (the read-only-compatible feature "quota").
	QuotaFeature bool
	// ProjectFeature says the filesystem stores a project id per inode (the
	// read-only-compatible feature "project").
	ProjectFeature bool
}

// The superblock layout this package reads. No kernel or e2fsprogs header
// carrying it is on the machine this was written on, so the offsets are the
// ext4 on-disk format as recalled: struct ext4_super_block in the kernel's
// fs/ext4/ext4.h. The guest test TestOnlineResizeGrowsMountedFilesystem reads
// a filesystem mkfs.ext4 made at a known size, so a wrong offset fails there,
// and TestProjectQuotaStateWithoutFeature reads both feature bits from a
// fixture made with the project feature and without the quota feature.
const (
	// superblockOffset is where the superblock starts, past the boot area.
	superblockOffset = 1024
	// superblockBytes is the superblock's size on disk.
	superblockBytes = 1024
	// s_blocks_count_lo, a little-endian 32-bit count.
	blocksCountLoOffset = 0x04
	// s_log_block_size: the block size is 1024 shifted left by it.
	logBlockSizeOffset = 0x18
	// s_magic, a little-endian 16-bit value.
	magicOffset = 0x38
	// s_feature_incompat, a little-endian 32-bit set.
	featureIncompatOffset = 0x60
	// s_feature_ro_compat, a little-endian 32-bit set.
	featureROCompatOffset = 0x64
	// roCompatQuota is EXT4_FEATURE_RO_COMPAT_QUOTA.
	roCompatQuota = 0x0100
	// roCompatProject is EXT4_FEATURE_RO_COMPAT_PROJECT.
	roCompatProject = 0x2000
	// s_blocks_count_hi, the high half of the block count with 64bit.
	blocksCountHiOffset = 0x150
	// ext4Magic is EXT4_SUPER_MAGIC (golang.org/x/sys/unix carries it too).
	ext4Magic = 0xEF53
	// incompat64Bit is EXT4_FEATURE_INCOMPAT_64BIT.
	incompat64Bit = 0x80
	// logBlockSizeMax bounds s_log_block_size: 64 KiB blocks, the largest
	// ext4 makes.
	logBlockSizeMax = 6
)

// ReadSuperblock reads the superblock of the ext4 filesystem that starts at
// the first byte of filesystem.
func ReadSuperblock(filesystem io.ReaderAt) (Superblock, error) {
	raw := make([]byte, superblockBytes)
	if _, err := filesystem.ReadAt(raw, superblockOffset); err != nil {
		return Superblock{}, fmt.Errorf("ext4fs: read the superblock: %w", err)
	}
	if binary.LittleEndian.Uint16(raw[magicOffset:]) != ext4Magic {
		return Superblock{}, ErrNotExt4
	}
	logBlockSize := binary.LittleEndian.Uint32(raw[logBlockSizeOffset:])
	if logBlockSize > logBlockSizeMax {
		return Superblock{}, fmt.Errorf("ext4fs: the superblock's block size exponent %d is above %d", logBlockSize, logBlockSizeMax)
	}
	count := uint64(binary.LittleEndian.Uint32(raw[blocksCountLoOffset:]))
	if binary.LittleEndian.Uint32(raw[featureIncompatOffset:])&incompat64Bit != 0 {
		count |= uint64(binary.LittleEndian.Uint32(raw[blocksCountHiOffset:])) << 32
	}
	roCompat := binary.LittleEndian.Uint32(raw[featureROCompatOffset:])
	return Superblock{
		BlockSize:      1024 << logBlockSize,
		BlockCount:     count,
		QuotaFeature:   roCompat&roCompatQuota != 0,
		ProjectFeature: roCompat&roCompatProject != 0,
	}, nil
}
