// Design: docs/architecture/web-components.md -- system memory for dashboard panel

//go:build darwin

package web

import "golang.org/x/sys/unix"

func totalSystemMemory() uint64 {
	v, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return v
}
