// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias setup
//
// macOS and FreeBSD only bind 127.0.0.1 to the loopback interface by default.
// Multi-peer tests need additional addresses (127.0.0.2, etc.), so we add them
// as aliases on lo0 using the SIOCAIFADDR ioctl.
//
// Reference: https://cgit.freebsd.org/src/tree/sbin/ifconfig/af_inet.c -- canonical SIOCAIFADDR usage.

//go:build darwin || freebsd

package runner

import (
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// sockaddrIn is the BSD sockaddr_in layout (16 bytes).
type sockaddrIn struct {
	Len    uint8
	Family uint8
	Port   uint16
	Addr   [4]byte
	Zero   [8]byte
}

// inIfaliasreq is the struct passed to SIOCAIFADDR (64 bytes total).
type inIfaliasreq struct {
	Name      [unix.IFNAMSIZ]byte
	Addr      sockaddrIn
	Broadaddr sockaddrIn
	Mask      sockaddrIn
}

// ensureLoopbackAlias adds ip as an alias on lo0 if not already present.
// Requires root. If the alias already exists, this is a no-op.
// If the ioctl fails (e.g., no root), returns an error but does not panic --
// the caller should log a warning and let the test fail on bind with a clear error.
func ensureLoopbackAlias(ip net.IP) error {
	ip4 := ip.To4()
	if ip4 == nil {
		return fmt.Errorf("ensureLoopbackAlias: %v is not IPv4", ip)
	}
	if ip4[0] != 127 {
		return fmt.Errorf("ensureLoopbackAlias: %v is not in 127.0.0.0/8", ip)
	}

	// Check if the address is already usable.
	ln, err := net.Listen("tcp", net.JoinHostPort(ip.String(), "0")) //nolint:noctx // probe-only, no cancellation needed
	if err == nil {
		ln.Close() //nolint:errcheck // probe listener, result irrelevant
		return nil // Already available.
	}

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return fmt.Errorf("ensureLoopbackAlias: socket: %w", err)
	}
	defer unix.Close(fd) //nolint:errcheck // deferred close on best-effort ioctl fd

	var req inIfaliasreq
	copy(req.Name[:], "lo0")

	req.Addr = sockaddrIn{Len: 16, Family: unix.AF_INET}
	copy(req.Addr.Addr[:], ip4)

	req.Mask = sockaddrIn{Len: 16, Family: unix.AF_INET}
	req.Mask.Addr = [4]byte{255, 255, 255, 255}

	// SIOCAIFADDR is idempotent -- adding an existing alias is not an error.
	// unix.SIOCAIFADDR = 0x8040691a = _IOW('i', 26, 64) on darwin/freebsd.
	// SYS_IOCTL is deprecated (SA1019) but x/sys/unix has no exported ioctlPtr
	// for custom struct arguments -- only typed wrappers (Winsize, Termios, etc.).
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), //nolint:staticcheck,gosec // no generic ioctl-with-pointer in x/sys/unix; unsafe required for ioctl struct
		uintptr(unix.SIOCAIFADDR), uintptr(unsafe.Pointer(&req))); errno != 0 {
		return fmt.Errorf("ensureLoopbackAlias: ioctl SIOCAIFADDR %v on lo0: %w", ip, errno)
	}

	return nil
}
