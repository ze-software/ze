//go:build linux

// Design: docs/architecture/cli/plugin-modes.md -- trusted service executables.

package systemd

import (
	"fmt"
	"io/fs"
	"syscall"
)

func (realServiceOps) owner(info fs.FileInfo) (uint32, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("no ownership metadata for %s; inspect its filesystem before service installation", info.Name())
	}
	return stat.Uid, nil
}
