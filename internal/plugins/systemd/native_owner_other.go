//go:build !linux

// Design: docs/architecture/cli/plugin-modes.md -- trusted service executables.

package systemd

import (
	"fmt"
	"io/fs"
)

func (realServiceOps) owner(info fs.FileInfo) (uint32, error) {
	return 0, fmt.Errorf("service ownership for %s requires Linux; use --no-service for foreground operation", info.Name())
}
