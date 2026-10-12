// Design: docs/architecture/web-components.md -- system memory for dashboard panel

//go:build openbsd

package web

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// totalSystemMemory answers the physical memory of the host in bytes, from the
// hw.physmem64 sysctl. OpenBSD keeps the 32-bit hw.physmem for compatibility,
// so only the 64-bit name answers for a host with more than 4 GiB.
func totalSystemMemory() (uint64, error) {
	v, err := unix.SysctlUint64("hw.physmem64")
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.physmem64: %w", err)
	}
	return v, nil
}
