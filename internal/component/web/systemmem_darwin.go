// Design: docs/architecture/web-components.md -- system memory for dashboard panel

//go:build darwin

package web

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// totalSystemMemory answers the physical memory of the host in bytes, from the
// hw.memsize sysctl.
func totalSystemMemory() (uint64, error) {
	v, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	return v, nil
}
