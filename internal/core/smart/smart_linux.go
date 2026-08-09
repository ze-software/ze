//go:build linux

// Design: docs/architecture/storage/smart-health.md -- SMART disk health ioctl library
// Related: smart.go — Info type, ParseNVMeBuf, NvmeNamespace

package smart

import (
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ATA ioctl constants from <linux/hdreg.h>.
const (
	hdioDriveCmd = 0x031f // HDIO_DRIVE_CMD

	ataSmartCmd       = 0xB0 // ATA SMART command
	smartReadData     = 0xD0 // SMART READ DATA subcommand
	smartReturnStatus = 0xDA // SMART RETURN STATUS subcommand
	smartEnableOps    = 0xD8 // SMART ENABLE OPERATIONS subcommand
	smartExecOffImm   = 0xD4 // SMART EXECUTE OFF-LINE IMMEDIATE subcommand
)

// NVMe admin ioctl constant.
const nvmeIoctlAdminCmd = 0xC0484E41 // NVME_IOCTL_ADMIN_CMD

// nvmePassthruCmd mirrors `struct nvme_passthru_cmd` from <linux/nvme_ioctl.h>.
type nvmePassthruCmd struct {
	opcode      uint8
	flags       uint8
	rsvd1       uint16
	nsid        uint32
	cdw2        uint32
	cdw3        uint32
	metadata    uint64
	addr        uint64
	metadataLen uint32
	dataLen     uint32
	cdw10       uint32
	cdw11       uint32
	cdw12       uint32
	cdw13       uint32
	cdw14       uint32
	cdw15       uint32
	timeoutMs   uint32
	result      uint32
}

// Detect reads SMART data directly from the block device via ioctls.
// Returns nil when root is non-empty (testdata mode) or when the device
// cannot be opened. Returns Info with Unavailable set on permission errors.
func Detect(deviceName, root string) *Info {
	if root != "" {
		return nil
	}

	if strings.HasPrefix(deviceName, "nvme") {
		return detectNVMe(NvmeNamespace(deviceName))
	}
	return detectATA(deviceName)
}

// detectATA reads SMART data from an ATA/SATA device using HDIO_DRIVE_CMD ioctls.
func detectATA(deviceName string) *Info {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		if isPermissionError(err) {
			return permissionDenied()
		}
		return nil
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	info := &Info{}

	// SMART RETURN STATUS: check drive health.
	var statusBuf [4]byte
	statusBuf[0] = ataSmartCmd
	statusBuf[1] = smartReturnStatus
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		hdioDriveCmd,
		uintptr(unsafe.Pointer(&statusBuf[0])), //nolint:gosec // HDIO_DRIVE_CMD requires raw buffer pointer
	)
	if errno != 0 {
		if isPermissionErrno(errno) {
			return permissionDenied()
		}
		return &Info{
			Unavailable:     true,
			UnavailableNote: "SMART not supported",
		}
	}
	// Healthy when LBA Mid=0x4F and LBA High=0xC2 (threshold not exceeded).
	info.Healthy = statusBuf[1] == 0x4F && statusBuf[2] == 0xC2

	// SMART READ DATA: fetch the 512-byte attribute table.
	var dataBuf [4 + 512]byte
	dataBuf[0] = ataSmartCmd
	dataBuf[1] = smartReadData
	dataBuf[2] = 1 // sector count
	dataBuf[3] = 1 // LBA low
	_, _, errno = unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		hdioDriveCmd,
		uintptr(unsafe.Pointer(&dataBuf[0])), //nolint:gosec // HDIO_DRIVE_CMD requires raw buffer pointer
	)
	if errno != 0 {
		return info
	}

	const (
		attrBase          = 4 + 2 // header(4) + table offset(2)
		attrSize          = 12
		attrCount         = 30
		attrIDReallocated = 5   // Reallocated Sectors Count
		attrIDPOH         = 9   // Power-On Hours
		attrIDTemp        = 194 // Temperature Celsius
	)

	for i := range attrCount {
		off := attrBase + i*attrSize
		if off+attrSize > len(dataBuf) {
			break
		}
		attrID := dataBuf[off]
		rawOff := off + 5
		switch attrID {
		case attrIDReallocated:
			info.ErrorCount = uint64(binary.LittleEndian.Uint32(dataBuf[rawOff : rawOff+4]))
		case attrIDPOH:
			info.PowerOnHours = uint64(binary.LittleEndian.Uint32(dataBuf[rawOff : rawOff+4]))
		case attrIDTemp:
			info.TempCelsius = int(dataBuf[rawOff])
		}
	}

	return info
}

// detectNVMe reads SMART data from an NVMe device using the admin passthrough ioctl.
func detectNVMe(deviceName string) *Info {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY, 0)
	if err != nil {
		if isPermissionError(err) {
			return permissionDenied()
		}
		return nil
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	var buf [512]byte
	cmd := nvmePassthruCmd{
		opcode:  0x02,                                     // Get Log Page
		nsid:    0xFFFFFFFF,                               // global
		addr:    uint64(uintptr(unsafe.Pointer(&buf[0]))), //nolint:gosec // NVMe passthru ioctl requires the raw buffer address
		dataLen: uint32(len(buf)),
		cdw10:   0x02 | (127 << 16), // log page 0x02 (SMART), numdl=127 (128 dwords = 512 bytes)
	}

	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		nvmeIoctlAdminCmd,
		uintptr(unsafe.Pointer(&cmd)), //nolint:gosec // NVMe admin passthrough requires raw struct pointer
	)
	if errno != 0 {
		if isPermissionErrno(errno) {
			return permissionDenied()
		}
		return &Info{
			Unavailable:     true,
			UnavailableNote: "NVMe SMART query failed",
		}
	}

	return ParseNVMeBuf(&buf)
}

// Enable sends the SMART ENABLE OPERATIONS command to the device.
func Enable(deviceName string) error {
	if strings.HasPrefix(deviceName, "nvme") {
		return enableNVMe(NvmeNamespace(deviceName))
	}
	return enableATA(deviceName)
}

func enableATA(deviceName string) error {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	var buf [4]byte
	buf[0] = ataSmartCmd
	buf[1] = smartEnableOps
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		hdioDriveCmd,
		uintptr(unsafe.Pointer(&buf[0])), //nolint:gosec // HDIO_DRIVE_CMD requires raw buffer pointer
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func enableNVMe(deviceName string) error {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	cmd := nvmePassthruCmd{
		opcode: 0x09,       // Set Features
		nsid:   0xFFFFFFFF, // global
		cdw10:  0x02,       // Feature ID: SMART / Health Information (02h)
		cdw11:  0x01,       // Enable
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		nvmeIoctlAdminCmd,
		uintptr(unsafe.Pointer(&cmd)), //nolint:gosec // NVMe admin passthrough requires raw struct pointer
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// StartSelfTest sends a SMART self-test command to the device.
func StartSelfTest(deviceName string, testType SelfTestType) error {
	if strings.HasPrefix(deviceName, "nvme") {
		return startSelfTestNVMe(NvmeNamespace(deviceName), testType)
	}
	return startSelfTestATA(deviceName, testType)
}

func startSelfTestATA(deviceName string, testType SelfTestType) error {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	var buf [4]byte
	buf[0] = ataSmartCmd
	buf[1] = smartExecOffImm
	buf[2] = 0
	buf[3] = byte(testType)
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		hdioDriveCmd,
		uintptr(unsafe.Pointer(&buf[0])), //nolint:gosec // HDIO_DRIVE_CMD requires raw buffer pointer
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func startSelfTestNVMe(deviceName string, testType SelfTestType) error {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	cmd := nvmePassthruCmd{
		opcode: 0x14,             // Device Self-test
		nsid:   0xFFFFFFFF,       // global
		cdw10:  uint32(testType), // 1=short, 2=extended
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		nvmeIoctlAdminCmd,
		uintptr(unsafe.Pointer(&cmd)), //nolint:gosec // NVMe admin passthrough requires raw struct pointer
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// IsSelfTestInProgress checks whether a self-test is currently running.
func IsSelfTestInProgress(deviceName string) bool {
	if strings.HasPrefix(deviceName, "nvme") {
		return isSelfTestInProgressNVMe(NvmeNamespace(deviceName))
	}
	return isSelfTestInProgressATA(deviceName)
}

func isSelfTestInProgressATA(deviceName string) bool {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	var dataBuf [4 + 512]byte
	dataBuf[0] = ataSmartCmd
	dataBuf[1] = smartReadData
	dataBuf[2] = 1
	dataBuf[3] = 1
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		hdioDriveCmd,
		uintptr(unsafe.Pointer(&dataBuf[0])), //nolint:gosec // HDIO_DRIVE_CMD requires raw buffer pointer
	)
	if errno != 0 {
		return false
	}

	// ATA8-ACS Table 69: self-test execution status byte at data offset 363.
	// Bits 7:4: 0x0F = in progress, 0x00 = idle/complete, 1-8 = completed with result.
	statusByte := dataBuf[4+363]
	status := (statusByte >> 4) & 0x0F
	return status == 0x0F
}

func isSelfTestInProgressNVMe(deviceName string) bool {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY, 0)
	if err != nil {
		return false
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	var buf [564]byte
	cmd := nvmePassthruCmd{
		opcode:  0x02,                                     // Get Log Page
		nsid:    0xFFFFFFFF,                               // global
		addr:    uint64(uintptr(unsafe.Pointer(&buf[0]))), //nolint:gosec // NVMe passthru ioctl requires the raw buffer address
		dataLen: uint32(len(buf)),
		cdw10:   0x06 | (uint32(len(buf)/4-1) << 16), // log page 0x06 (Device Self-test)
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		nvmeIoctlAdminCmd,
		uintptr(unsafe.Pointer(&cmd)), //nolint:gosec // NVMe admin passthrough requires raw struct pointer
	)
	if errno != 0 {
		return false
	}

	return (buf[0] >> 4) != 0
}

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, unix.EPERM) ||
		errors.Is(err, unix.EACCES)
}

func isPermissionErrno(errno unix.Errno) bool {
	return errno == unix.EPERM || errno == unix.EACCES
}

func permissionDenied() *Info {
	return &Info{
		Unavailable:     true,
		UnavailableNote: "insufficient privileges",
	}
}
