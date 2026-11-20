//go:build linux

// Design: docs/architecture/testing/interop.md -- the persistent image cache.
// Related: host_cpu.go -- the identity this reader's text is parsed into.
package interoplab

import (
	"fmt"
	"os"
)

// cpuinfoPath is the kernel's description of this machine's processors.
const cpuinfoPath = "/proc/cpuinfo"

// hostCPUIdentity answers this machine's CPU identity from /proc/cpuinfo.
func hostCPUIdentity() (string, error) {
	cpuinfo, err := os.ReadFile(cpuinfoPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", cpuinfoPath, err)
	}
	return cpuinfoIdentity(string(cpuinfo))
}
