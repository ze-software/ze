// Design: docs/architecture/appliance/on-device-installer.md -- the project-quota kernel calls
// Overview: quota.go -- the package, its states and its errors
// Related: quota_other.go -- the answers on a system that is not Linux

//go:build linux

package projquota

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/ext4fs"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The quota interface, read from the kernel's UAPI headers on the machine this
// was written on (linux/quota.h, linux/dqblk_xfs.h, linux/fs.h). Every call
// goes through quotactl_fd(2) on a descriptor of a path on the filesystem, so
// no block device has to be found: the kernel takes the filesystem from the
// descriptor.
const (
	// linux/quota.h: "#define SUBCMDSHIFT 8", and QCMD(cmd, type) is
	// "(((cmd) << SUBCMDSHIFT) | ((type) & SUBCMDMASK))".
	subcmdShift = 8
	// linux/quota.h: "#define PRJQUOTA 2". linux/dqblk_xfs.h gives
	// XQM_PRJQUOTA the same value for the XFS-style commands.
	prjQuota = 2
	// linux/quota.h: "#define Q_QUOTAON 0x800002".
	qQuotaOn = 0x800002
	// linux/quota.h: "#define Q_GETQUOTA 0x800007".
	qGetQuota = 0x800007
	// linux/quota.h: "#define Q_SETQUOTA 0x800008".
	qSetQuota = 0x800008
	// linux/dqblk_xfs.h: "#define Q_XGETQSTATV XQM_CMD(8)", with
	// "#define XQM_CMD(x) (('X'<<8)+(x))".
	qXGetQStatV = 'X'<<8 + 8
	// linux/quota.h: "#define QFMT_VFS_V1 4", the format of ext4's hidden
	// quota inodes. A quota-on of a filesystem whose quota lives in system
	// inodes ignores it; it is passed for the form of the call.
	qfmtVFSV1 = 4
	// linux/quota.h: QIF_BLIMITS is "(1 << QIF_BLIMITS_B)" with
	// QIF_BLIMITS_B = 0: the call sets the block limits.
	qifBLimits = 1
	// linux/quota.h: "#define QIF_DQBLKSIZE_BITS 10". A block limit in
	// struct if_dqblk counts 1024-byte quota blocks; the space in use counts
	// bytes.
	quotaBlockBytes = 1 << 10
	// linux/dqblk_xfs.h: "#define FS_QUOTA_PDQ_ACCT (1<<4)" and
	// "#define FS_QUOTA_PDQ_ENFD (1<<5)".
	fsQuotaPDQAcct = 1 << 4
	fsQuotaPDQEnfd = 1 << 5
	// linux/dqblk_xfs.h: "#define FS_QSTATV_VERSION1 1".
	fsQStatVVersion1 = 1
	// linux/fs.h: "#define FS_IOC_FSGETXATTR _IOR('X', 31, struct fsxattr)"
	// and "#define FS_IOC_FSSETXATTR _IOW('X', 32, struct fsxattr)", in the
	// generic encoding amd64 and arm64 use (linux/asm-generic/ioctl.h): the
	// direction in bits 30-31 (read 2, write 1), the size of struct fsxattr
	// (28) in bits 16-29, the type in bits 8-15, the number in bits 0-7.
	fsIocFSGetXAttr = 2<<30 | 28<<16 | 'X'<<8 | 31
	fsIocFSSetXAttr = 1<<30 | 28<<16 | 'X'<<8 | 32
	// linux/fs.h: "#define FS_XFLAG_PROJINHERIT 0x00000200".
	fsXFlagProjInherit = 0x200
)

// ifDqblk is struct if_dqblk of linux/quota.h: eight __u64 and one __u32,
// padded to 72 bytes.
type ifDqblk struct {
	BHardLimit uint64
	BSoftLimit uint64
	CurSpace   uint64
	IHardLimit uint64
	ISoftLimit uint64
	CurInodes  uint64
	BTime      uint64
	ITime      uint64
	Valid      uint32
	_          uint32
}

// fsQFileStatV is struct fs_qfilestatv of linux/dqblk_xfs.h.
type fsQFileStatV struct {
	Ino      uint64
	NBlks    uint64
	NExtents uint32
	_        uint32
}

// fsQuotaStatV is struct fs_quota_statv of linux/dqblk_xfs.h, 160 bytes.
type fsQuotaStatV struct {
	Version      int8
	_            uint8
	Flags        uint16
	InCoreDQs    uint32
	UQuota       fsQFileStatV
	GQuota       fsQFileStatV
	PQuota       fsQFileStatV
	BTimeLimit   int32
	ITimeLimit   int32
	RTBTimeLimit int32
	BWarnLimit   uint16
	IWarnLimit   uint16
	RTBWarnLimit uint16
	_            uint16
	_            uint32
	_            [7]uint64
}

// fsxattr is struct fsxattr of linux/fs.h, 28 bytes.
type fsxattr struct {
	XFlags     uint32
	ExtSize    uint32
	NExtents   uint32
	ProjID     uint32
	CowExtSize uint32
	_          [8]byte
}

// Probe answers whether the filesystem that holds path enforces project quota.
//
// It reads the quota state with Q_XGETQSTATV for the project type. Accounting
// with enforcement is StateEnforced, accounting alone is StateNotEnforced, no
// accounting is StateNoFeature. A query refused with ENOSYS is answered from
// the superblock (probeRefusedState). Any other refusal is StateUnknown with
// its reason, never an absence.
func Probe(path string) Status {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return Status{State: StateUnknown, Reason: "projquota: " + err.Error()}
	}
	defer file.Close() //nolint:errcheck // a read-only descriptor opened only for the call
	stat := fsQuotaStatV{Version: fsQStatVVersion1}
	if err := quotactlFd(file, qXGetQStatV, 0, unsafe.Pointer(&stat)); err != nil { //nolint:gosec // the kernel fills struct fs_quota_statv (160 bytes, laid out above) during the call and keeps no reference
		refusal := "projquota: read the quota state of the filesystem under " + path + ": " + err.Error()
		if errors.Is(err, unix.ENOSYS) {
			return probeRefusedState(file, refusal)
		}
		return Status{State: StateUnknown, Reason: refusal}
	}
	if stat.Flags&fsQuotaPDQEnfd != 0 {
		return Status{State: StateEnforced}
	}
	if stat.Flags&fsQuotaPDQAcct != 0 {
		return Status{State: StateNotEnforced}
	}
	return Status{State: StateNoFeature}
}

// probeRefusedState answers a Probe whose state query the kernel refused with
// ENOSYS, from the superblock of the filesystem file is on.
//
// On Ze's 7.3 runtime kernel the query answers ENOSYS for an ext4 filesystem
// made without the quota feature (TestProjectQuotaStateWithoutFeature, guest
// run of 2026-11-16): the filesystem offers no quota operations. ENOSYS also
// answers a kernel without quota support or without quotactl_fd, so the errno
// alone cannot say which. The superblock can: without the quota feature the
// filesystem accounts no project usage whatever the kernel, which is
// StateNoFeature. With the feature, the kernel is what refused, and the state
// stays unknown with the refusal. A filesystem that is not ext4, or whose
// device cannot be read, stays unknown too.
func probeRefusedState(file *os.File, refusal string) Status {
	device, err := mountedExt4Device(file)
	if err != nil {
		return Status{State: StateUnknown, Reason: refusal + "; " + err.Error()}
	}
	disk, err := os.Open(filepath.Clean(device))
	if err != nil {
		return Status{State: StateUnknown, Reason: refusal + "; " + err.Error()}
	}
	defer disk.Close() //nolint:errcheck // a read-only descriptor opened only for the superblock read
	superblock, err := ext4fs.ReadSuperblock(disk)
	if err != nil {
		return Status{State: StateUnknown, Reason: refusal + "; " + err.Error()}
	}
	if superblock.QuotaFeature {
		return Status{State: StateUnknown, Reason: refusal + "; the filesystem on " + device + " has the quota feature"}
	}
	return Status{State: StateNoFeature}
}

// mountinfoPath is the mount table of the calling process.
const mountinfoPath = "/proc/self/mountinfo"

// The fields of one /proc/self/mountinfo line, as proc(5) lays them out: the
// third is the device's major:minor, a field holding one hyphen ends the
// optional fields, and the filesystem type and the mount source follow it.
const (
	mountinfoDeviceField = 2
	mountinfoSeparator   = "-"
	mountinfoLineMax     = 64 << 10
)

// mountedExt4Device answers the device node of the ext4 filesystem file is
// on, found in the mount table by the device number of file.
func mountedExt4Device(file *os.File) (string, error) {
	var st unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &st); err != nil { //nolint:gosec // a descriptor number fits an int
		return "", fmt.Errorf("projquota: stat: %w", err)
	}
	var tb textbuf.Buffer
	want := tb.Uint32(unix.Major(st.Dev)).Byte(':').Uint32(unix.Minor(st.Dev)).String()
	table, err := os.Open(mountinfoPath)
	if err != nil {
		return "", fmt.Errorf("projquota: %w", err)
	}
	defer table.Close() //nolint:errcheck // read-only
	scanner := bufio.NewScanner(table)
	scanner.Buffer(make([]byte, 0, 4096), mountinfoLineMax)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= mountinfoDeviceField {
			continue
		}
		if fields[mountinfoDeviceField] != want {
			continue
		}
		separator := slices.Index(fields, mountinfoSeparator)
		if separator < 0 {
			return "", fmt.Errorf("projquota: a line of %s for device %s has no separator", mountinfoPath, want)
		}
		if separator+2 >= len(fields) {
			return "", fmt.Errorf("projquota: a line of %s for device %s ends before its source", mountinfoPath, want)
		}
		if fields[separator+1] != "ext4" {
			return "", fmt.Errorf("projquota: the filesystem of device %s is %s, not ext4", want, fields[separator+1])
		}
		return fields[separator+2], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("projquota: read %s: %w", mountinfoPath, err)
	}
	return "", fmt.Errorf("projquota: device %s is not in %s", want, mountinfoPath)
}

// Enable turns on enforcement of project quota on the mounted filesystem that
// holds path, which MUST already account project usage. It answers nil only
// when a Probe after it reads StateEnforced.
func Enable(path string) error {
	status := Probe(path)
	switch status.State {
	case StateEnforced:
		return nil
	case StateNotEnforced:
	case StateNoFeature:
		return ErrNoFeature
	case StateUnknown:
		return errors.New(status.Reason)
	case StateUnspecified:
		return errors.New("projquota: BUG: Probe answered no state")
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("projquota: %w", err)
	}
	defer file.Close() //nolint:errcheck // a read-only descriptor opened only for the call
	if err := quotactlFd(file, qQuotaOn, qfmtVFSV1, nil); err != nil {
		return fmt.Errorf("projquota: turn on project quota enforcement under %s: %w", path, err)
	}
	after := Probe(path)
	if after.State != StateEnforced {
		return fmt.Errorf("projquota: the quota-on call under %s succeeded and the state reads %s", path, after.State)
	}
	return nil
}

// AssignProject gives the empty folder dir the project id projectID, which new
// entries under it inherit.
func AssignProject(dir string, projectID uint32) error {
	if projectID == 0 {
		return ErrDefaultProject
	}
	folder, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return fmt.Errorf("projquota: %w", err)
	}
	defer folder.Close() //nolint:errcheck // a read-only descriptor opened only for the ioctls
	names, err := folder.Readdirnames(1)
	if len(names) > 0 {
		return fmt.Errorf("%w: %s holds %s", ErrFolderNotEmpty, dir, names[0])
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("projquota: read %s: %w", dir, err)
	}
	var attr fsxattr
	if err := ioctlPointer(folder, fsIocFSGetXAttr, unsafe.Pointer(&attr)); err != nil { //nolint:gosec // the kernel copies struct fsxattr (28 bytes, laid out above) during the call and keeps no reference
		return fmt.Errorf("projquota: read the attributes of %s: %w", dir, err)
	}
	attr.ProjID = projectID
	attr.XFlags |= fsXFlagProjInherit
	if err := ioctlPointer(folder, fsIocFSSetXAttr, unsafe.Pointer(&attr)); err != nil { //nolint:gosec // the kernel copies struct fsxattr (28 bytes, laid out above) during the call and keeps no reference
		return fmt.Errorf("projquota: give %s project %d: %w", dir, projectID, err)
	}
	return nil
}

// SetLimit sets limitBytes as the hard block limit of projectID on the
// filesystem that holds path. The limit is at least LimitBytesMin and a whole
// number of 1024-byte quota blocks.
func SetLimit(path string, projectID uint32, limitBytes uint64) error {
	if projectID == 0 {
		return ErrDefaultProject
	}
	if limitBytes < LimitBytesMin {
		return ErrLimitTooSmall
	}
	if limitBytes%quotaBlockBytes != 0 {
		return ErrLimitNotWhole
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("projquota: %w", err)
	}
	defer file.Close() //nolint:errcheck // a read-only descriptor opened only for the call
	quota := ifDqblk{BHardLimit: limitBytes / quotaBlockBytes, Valid: qifBLimits}
	if err := quotactlFd(file, qSetQuota, projectID, unsafe.Pointer(&quota)); err != nil { //nolint:gosec // the kernel copies struct if_dqblk (72 bytes, laid out above) during the call and keeps no reference
		return fmt.Errorf("projquota: set the limit of project %d under %s: %w", projectID, path, err)
	}
	return nil
}

// ReadUsage reads what projectID uses and may use on the filesystem that
// holds path.
func ReadUsage(path string, projectID uint32) (Usage, error) {
	if projectID == 0 {
		return Usage{}, ErrDefaultProject
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return Usage{}, fmt.Errorf("projquota: %w", err)
	}
	defer file.Close() //nolint:errcheck // a read-only descriptor opened only for the call
	var quota ifDqblk
	if err := quotactlFd(file, qGetQuota, projectID, unsafe.Pointer(&quota)); err != nil { //nolint:gosec // the kernel copies struct if_dqblk (72 bytes, laid out above) during the call and keeps no reference
		return Usage{}, fmt.Errorf("projquota: read the usage of project %d under %s: %w", projectID, path, err)
	}
	return Usage{UsedBytes: quota.CurSpace, LimitBytes: quota.BHardLimit * quotaBlockBytes}, nil
}

// quotactlFd calls quotactl_fd(2) with the project-type command cmd on the
// filesystem of file.
func quotactlFd(file *os.File, cmd, id uint32, addr unsafe.Pointer) error {
	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL_FD, file.Fd(), uintptr(cmd<<subcmdShift|prjQuota),
		uintptr(id), uintptr(addr), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// ioctlPointer calls ioctl(2) with a pointer argument on file.
func ioctlPointer(file *os.File, request uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, file.Fd(), request, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}
