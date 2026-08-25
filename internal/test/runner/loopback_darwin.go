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

	"github.com/ze-software/ze/internal/core/textbuf"
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

// ensureLoopbackAlias makes ip usable on lo0, or says how to make it usable.
//
// IPv4: adds the alias through SIOCAIFADDR when the address is not already
// bindable. That needs root, so an unprivileged run gets EPERM back; the error
// then carries the command an operator can run.
//
// IPv6: probes only, and never adds. The IPv6 sibling ioctl (SIOCAIFADDR_IN6)
// returns EPERM to an unprivileged process just the same, and unlike IPv4 there
// is no second loopback address to fall back on -- see loopback.go for why the
// privilege lives in `./le setup` instead.
//
// The caller fails the test on any error: a missing address surfaces later as a
// bind failure or a whole-test timeout with nothing pointing at the cause.
func ensureLoopbackAlias(ip net.IP) error {
	ip4 := ip.To4()
	if ip4 == nil {
		if loopbackBindable(ip) {
			return nil
		}
		return loopbackMissing(ip)
	}
	if ip4[0] != 127 {
		return fmt.Errorf("ensureLoopbackAlias: %v is not in 127.0.0.0/8", ip)
	}

	// Check if the address is already usable.
	if loopbackBindable(ip) {
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
		return fmt.Errorf("ensureLoopbackAlias: ioctl SIOCAIFADDR %v on lo0: %w:"+
			" run `./le setup`, or add it by hand with `sudo ifconfig lo0 alias %v`", ip, errno, ip)
	}

	return nil
}

// loopbackIPv6AddCommand is the command an operator runs to put an IPv6 address
// on the loopback interface. `alias` is what BSD ifconfig calls a second address
// on an interface that already has one, and /128 keeps it a host address so no
// route toward the rest of its block is created.
func loopbackIPv6AddCommand(ip net.IP) string {
	var tb textbuf.Buffer
	return tb.Str("sudo ifconfig lo0 inet6 ").Str(ip.String()).Str("/128 alias").String()
}
