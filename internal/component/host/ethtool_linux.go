// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ifreqEthtool mirrors the kernel's `struct ifreq` used with SIOCETHTOOL.
// Layout on amd64 (matches sizeofIfreq = 40 bytes; kernel does
// copy_from_user(ifr, user, sizeof(struct ifreq))):
//
//	offset  0..15  name       IFNAMSIZ-byte NUL-padded interface name
//	offset 16..23  data       pointer to the ETHTOOL request struct
//	offset 24..39  _          padding so the total size matches the
//	                          kernel's ifr_ifru union (24 bytes on 64-bit)
//
// The `data` field being a uintptr gives the struct 8-byte alignment,
// guaranteeing `data` itself is aligned when Go places the value on
// the stack. Keeping the pointer in a uintptr field (rather than
// unsafe.Pointer) is fine because `ethtoolIoctl` receives the live
// unsafe.Pointer as an argument: the caller's stack frame holds the
// target object until Syscall returns, which pins it for GC purposes.
type ifreqEthtool struct {
	name [unix.IFNAMSIZ]byte
	data uintptr
	_    [16]byte
}

// ringparam is `struct ethtool_ringparam` from <linux/ethtool.h>.
// Cmd = ETHTOOL_GRINGPARAM (0x10) on request; kernel fills the rest.
type ringparam struct {
	cmd               uint32
	rxMaxPending      uint32
	rxMiniMaxPending  uint32
	rxJumboMaxPending uint32
	txMaxPending      uint32
	rxPending         uint32
	rxMiniPending     uint32
	rxJumboPending    uint32
	txPending         uint32
}

const (
	ethtoolGDrvinfo   = 0x00000003
	ethtoolGRingparam = 0x00000010
)

// enrichNICEthtool populates FirmwareVersion, RingRx, RingTx via the
// ETHTOOL ioctl. Best-effort: missing driver support or permission
// issues leave the fields at zero value. Never returns an error.
func enrichNICEthtool(nic *NICInfo) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return
	}
	defer func() { _ = unix.Close(fd) }()

	if fw, ok := ethtoolDrvinfoFW(fd, nic.Name); ok {
		nic.FirmwareVersion = fw
	}
	if rx, tx, ok := ethtoolRingparam(fd, nic.Name); ok {
		nic.RingRx = rx
		nic.RingTx = tx
	}
}

// ethtoolDrvinfoFW issues ETHTOOL_GDRVINFO and returns the trimmed
// firmware-version string. The second return is false when the ioctl
// fails (driver doesn't implement it, or permission denied).
func ethtoolDrvinfoFW(fd int, ifname string) (string, bool) {
	var drv unix.EthtoolDrvinfo
	drv.Cmd = ethtoolGDrvinfo
	if !ethtoolIoctl(fd, ifname, unsafe.Pointer(&drv)) { //nolint:gosec // ETHTOOL ioctl requires raw struct pointer
		return "", false
	}
	return trimCString(drv.Fw_version[:]), true
}

// ethtoolRingparam issues ETHTOOL_GRINGPARAM and returns the current
// rx_pending and tx_pending values (actual configured ring sizes).
func ethtoolRingparam(fd int, ifname string) (rx, tx int, ok bool) {
	var r ringparam
	r.cmd = ethtoolGRingparam
	if !ethtoolIoctl(fd, ifname, unsafe.Pointer(&r)) { //nolint:gosec // ETHTOOL ioctl requires raw struct pointer
		return 0, 0, false
	}
	return int(r.rxPending), int(r.txPending), true
}

// ethtoolIoctl is the lowest-level wrapper: build an ifreq with the
// ETHTOOL data pointer and fire SIOCETHTOOL via unix.Syscall. Returns
// false on any errno.
//
// The caller MUST keep `data` alive until this function returns.
// In practice every caller passes `unsafe.Pointer(&local)` where
// `local` is a stack variable in the immediate caller — that stack
// frame is live for the duration of this call, so `local` is pinned.
func ethtoolIoctl(fd int, ifname string, data unsafe.Pointer) bool {
	var ifr ifreqEthtool
	copy(ifr.name[:], ifname)
	ifr.data = uintptr(data)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.SIOCETHTOOL), uintptr(unsafe.Pointer(&ifr))) //nolint:gosec // SIOCETHTOOL requires raw ifreq pointer
	return errno == 0
}

// trimCString returns the Go string up to the first NUL byte in the
// C-style fixed-length buffer common in kernel ioctl responses.
func trimCString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return strings.TrimSpace(string(b))
}
