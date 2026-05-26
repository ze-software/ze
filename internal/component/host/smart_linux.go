// Design: plan/learned/695-host-3-smart.md — SMART health via direct ioctl
// Overview: smart.go — SmartInfo type and JSON parsing
// Related: storage_linux.go — DetectStorage calls detectSMART

//go:build linux

package host

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
)

// NVMe admin ioctl constant.
const nvmeIoctlAdminCmd = 0xC0484E41 // NVME_IOCTL_ADMIN_CMD

// nvmePassthruCmd mirrors `struct nvme_passthru_cmd` from <linux/nvme_ioctl.h>.
// Layout must match exactly for the ioctl to work.
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

// detectSMART reads SMART data directly from the block device via ioctls.
// Returns nil when Root is set (testdata mode) or when the device cannot
// be opened. Returns SmartInfo with Unavailable set on permission errors.
func (d *Detector) detectSMART(deviceName string) *SmartInfo {
	if d.Root != "" {
		return nil
	}

	if strings.HasPrefix(deviceName, "nvme") {
		return detectSMARTNVMe(nvmeNamespace(deviceName))
	}
	return detectSMARTATA(deviceName)
}

// detectSMARTATA reads SMART data from an ATA/SATA device using
// HDIO_DRIVE_CMD ioctls.
func detectSMARTATA(deviceName string) *SmartInfo {
	fd, err := unix.Open("/dev/"+deviceName, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		if isPermissionError(err) {
			return permissionDenied()
		}
		return nil
	}
	defer unix.Close(fd) //nolint:errcheck // close error is non-actionable

	info := &SmartInfo{}

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
		// SMART not supported by this device.
		return &SmartInfo{
			Unavailable:     true,
			UnavailableNote: "SMART not supported",
		}
	}
	// HDIO_DRIVE_CMD returns: buf[0]=status, buf[1]=LBA Mid, buf[2]=LBA High.
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
		// Health check succeeded, attributes unavailable. Return partial data.
		return info
	}

	// Attribute table starts at data offset 2 (buf[4+2] = buf[6]).
	// 30 entries, 12 bytes each.
	const (
		attrBase   = 4 + 2 // header(4) + table offset(2)
		attrSize   = 12
		attrCount  = 30
		attrIDPOH  = 9   // Power-On Hours
		attrIDTemp = 194 // Temperature Celsius
	)

	for i := range attrCount {
		off := attrBase + i*attrSize
		if off+attrSize > len(dataBuf) {
			break
		}
		attrID := dataBuf[off]
		// Raw value starts at offset 5 within each attribute entry.
		rawOff := off + 5
		switch attrID {
		case attrIDPOH:
			info.PowerOnHours = uint64(binary.LittleEndian.Uint32(dataBuf[rawOff : rawOff+4]))
		case attrIDTemp:
			info.TempCelsius = int(dataBuf[rawOff])
		}
	}

	return info
}

// detectSMARTNVMe reads SMART data from an NVMe device using the NVMe
// admin passthrough ioctl.
func detectSMARTNVMe(deviceName string) *SmartInfo {
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
		opcode:  0x02,       // Get Log Page
		nsid:    0xFFFFFFFF, // global
		addr:    uint64(uintptr(unsafe.Pointer(&buf[0]))),
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
		return &SmartInfo{
			Unavailable:     true,
			UnavailableNote: "NVMe SMART query failed",
		}
	}

	return parseNVMeSMARTBuf(&buf)
}

func parseNVMeSMARTBuf(buf *[512]byte) *SmartInfo {
	info := &SmartInfo{}

	info.Healthy = buf[0] == 0

	if kelvin := int(binary.LittleEndian.Uint16(buf[1:3])); kelvin > 0 {
		info.TempCelsius = kelvin - 273
	}

	info.PowerOnHours = binary.LittleEndian.Uint64(buf[128:136])
	info.ErrorCount = binary.LittleEndian.Uint64(buf[176:184])

	return info
}

// isPermissionError checks whether an error from os.Open or unix.Open
// indicates insufficient privileges.
func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, unix.EPERM) ||
		errors.Is(err, unix.EACCES)
}

// isPermissionErrno checks whether a syscall errno indicates
// insufficient privileges.
func isPermissionErrno(errno unix.Errno) bool {
	return errno == unix.EPERM || errno == unix.EACCES
}

// permissionDenied returns a SmartInfo marking the device as unavailable
// due to insufficient privileges.
func permissionDenied() *SmartInfo {
	return &SmartInfo{
		Unavailable:     true,
		UnavailableNote: "insufficient privileges",
	}
}

// nvmeNamespace strips the partition suffix (e.g. "p1") from an NVMe
// device name so the admin ioctl targets the namespace, not a partition.
// "nvme0n1p1" -> "nvme0n1", "nvme0n1" -> "nvme0n1" (unchanged).
func nvmeNamespace(name string) string {
	idx := strings.LastIndex(name, "p")
	if idx < 0 {
		return name
	}
	suffix := name[idx+1:]
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return name
		}
	}
	if len(suffix) == 0 {
		return name
	}
	return name[:idx]
}
