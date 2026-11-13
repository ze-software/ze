// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification
// Related: parse.go -- parseVMStatAvailable, which states what the estimate counts

package hostload

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"golang.org/x/sys/unix"
)

// readMemoryTotal reads the machine's physical memory in bytes from the
// hw.memsize sysctl.
func readMemoryTotal() (uint64, error) {
	size, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	if size == 0 {
		return 0, errors.New("sysctl hw.memsize answered zero bytes")
	}
	return size, nil
}

// readMemoryAvailable reads the page statistics `vm_stat` prints and answers
// the free, speculative and inactive pages in bytes. macOS has no counterpart
// of Linux's MemAvailable, and the inactive count is printed by vm_stat alone.
func readMemoryAvailable() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "vm_stat").Output()
	if err != nil {
		return 0, fmt.Errorf("vm_stat: %w", err)
	}
	return parseVMStatAvailable(string(out))
}
