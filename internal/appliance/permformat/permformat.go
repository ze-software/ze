// Design: docs/architecture/appliance/on-device-installer.md -- the one definition of the persistent partition's format

// Package permformat declares the format of the appliance's persistent
// partition once: the arguments of the format step and of the quota step that
// follows it.
//
// The filesystem keeps its build-time properties for life, because a device
// grows it with the kernel's online resize and never formats it again. A
// format that inherited the build host's mke2fs defaults would therefore be a
// per-device difference nobody can see, so every property is named here.
//
// Every formatter of a persistent partition derives its arguments from this
// package and names no property of its own. Today the one caller is the ext4
// fixture set the QEMU guest tests attach (internal/test/ext4fixture). The
// appliance build (internal/appliance injectZeFS), the docker-host lab image
// (internal/le/setup/docker/host formatPerm) and the list of properties the
// build checks in the superblock join it with the build-format and own-system
// guest phases of plan/post-release/features/spec-appliance-perm-grows-at-install.md.
//
// The package holds declarations only and runs nothing. Safe for concurrent
// use.
package permformat

import (
	"slices"
	"strconv"
	"strings"
)

// The e2fsprogs programs the two steps run. Each caller resolves the path
// itself (appliance.ResolveE2FSTool on a build host).
const (
	// FormatTool makes the filesystem.
	FormatTool = "mkfs.ext4"
	// QuotaTool adds the quota feature and stores the mount option, after
	// the files the build injects are written.
	QuotaTool = "tune2fs"
)

// The properties of the format.
const (
	// BlockSize is the filesystem block size in bytes.
	BlockSize = 4096
	// InodeSize is the on-disk inode size in bytes. The project id lives in
	// the extended part of the inode, which a 128-byte inode lacks.
	InodeSize = 256
	// BytesPerInode fixes the inodes per group, which every group the
	// online resize adds carries too.
	BytesPerInode = 16384
	// JournalMiB is the journal size. The journal does not grow with the
	// filesystem, and image layer extraction is metadata heavy.
	JournalMiB = 64
	// ResizeBlocksMax is the block count the reserved group-descriptor
	// area is sized for: 2^31 blocks of 4096 bytes, 8 TiB. mke2fs caps the
	// reserved area at what one block of descriptor pointers holds, so this
	// asks for the largest area the block size allows.
	ResizeBlocksMax = 1 << 31
	// ProjectQuotaType is the only quota type the filesystem carries.
	ProjectQuotaType = "prjquota"
	// MountOption is the mount option stored in the superblock, which the
	// kernel applies at every mount: gokrazy mounts the persistent
	// partition with no option string.
	MountOption = "prjquota"
)

// features is the whole feature list of the format step. Metadata checksums
// are absent, as in the build before this definition existed.
var features = []string{
	"has_journal",
	"extent",
	"huge_file",
	"flex_bg",
	"64bit",
	"dir_nlink",
	"extra_isize",
	"resize_inode",
	"sparse_super",
	"large_file",
	"filetype",
	"dir_index",
	"ext_attr",
	"project",
}

// Features answers the feature list of the format step, in order. The caller
// owns the slice.
func Features() []string {
	return slices.Clone(features)
}

// FormatArgs answers the arguments of the format step, without the target
// and its size, which the caller appends.
//
// extended carries the extended options the caller needs beside the format's
// own (the appliance build will pass "offset=<bytes>"). They join the one -E
// argument, because mke2fs keeps only the last -E it is given.
//
// The feature list starts with "none", which clears the build host's
// mke2fs.conf defaults (mke2fs(8), "-O"), so the list is the whole set.
func FormatArgs(extended ...string) []string {
	options := append([]string{"resize=" + strconv.Itoa(ResizeBlocksMax)}, extended...)
	return []string{
		"-q", "-F",
		"-t", "ext4",
		"-b", strconv.Itoa(BlockSize),
		"-I", strconv.Itoa(InodeSize),
		"-i", strconv.Itoa(BytesPerInode),
		"-J", "size=" + strconv.Itoa(JournalMiB),
		"-E", strings.Join(options, ","),
		"-O", "none," + strings.Join(features, ","),
	}
}

// QuotaFeatureArgs answers the half of the quota step that adds the quota
// feature with a project quota inode only (tune2fs(8), "-Q"). tune2fs computes
// the usage record from the inodes already on the filesystem.
func QuotaFeatureArgs() []string {
	return []string{"-Q", ProjectQuotaType}
}

// MountOptionArgs answers the half of the quota step that stores the
// project-quota mount option in the superblock (tune2fs(8), "-E mount_opts").
func MountOptionArgs() []string {
	return []string{"-E", "mount_opts=" + MountOption}
}

// QuotaStepArgs answers the whole quota step, without the filesystem file,
// which the caller appends.
func QuotaStepArgs() []string {
	return append(QuotaFeatureArgs(), MountOptionArgs()...)
}
