// Design: docs/architecture/core-design.md -- sysctl Darwin backend
// Overview: backend.go -- backend interface

//go:build darwin

package sysctl

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// darwinForwardingKeys maps ze's kernel-native key names to Darwin MIB names.
// Only forwarding keys are supported on Darwin. Other keys are rejected with
// a platform-not-available error.
var darwinForwardingKeys = map[string]string{
	"net.inet.ip.forwarding":   "net.inet.ip.forwarding",
	"net.inet6.ip6.forwarding": "net.inet6.ip6.forwarding",
}

type darwinBackend struct{}

func newBackend() backend {
	return &darwinBackend{}
}

func (b *darwinBackend) read(key string) (string, error) {
	mib, ok := darwinForwardingKeys[key]
	if !ok {
		return "", fmt.Errorf("sysctl: key %q not available on darwin", key)
	}
	val, err := unix.SysctlUint32(mib)
	if err != nil {
		return "", fmt.Errorf("sysctl read %s: %w", key, err)
	}
	return strconv.FormatUint(uint64(val), 10), nil
}

func (b *darwinBackend) write(key, value string) error {
	mib, ok := darwinForwardingKeys[key]
	if !ok {
		return fmt.Errorf("sysctl: key %q not available on darwin", key)
	}
	v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil {
		return fmt.Errorf("sysctl: value %q for %s: %w", value, key, err)
	}
	return setSysctlInt32(mib, int32(v))
}

// setSysctlInt32 writes a 4-byte integer sysctl via the __sysctlbyname syscall.
// Darwin's SYS_SYSCTLBYNAME takes 6 args: name, namelen, oldp, oldlenp, newp, newlen.
//
//nolint:gosec // G103: unsafe.Pointer required for syscall arguments; buffer sizes are compile-time constants.
func setSysctlInt32(name string, value int32) error {
	nameBytes, err := unix.ByteSliceFromString(name)
	if err != nil {
		return err
	}
	nameLen := uintptr(len(nameBytes))
	_, _, errno := unix.Syscall6(
		unix.SYS_SYSCTLBYNAME, //nolint:staticcheck // SA1019: no libc wrapper for sysctl write in x/sys/unix
		uintptr(unsafe.Pointer(&nameBytes[0])),
		nameLen,
		0, 0, // no read (oldp, oldlenp)
		uintptr(unsafe.Pointer(&value)),
		uintptr(4),
	)
	if errno != 0 {
		return fmt.Errorf("sysctlbyname(%s) write: %w", name, errno)
	}
	return nil
}
