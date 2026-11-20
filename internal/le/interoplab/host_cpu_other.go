//go:build !linux && !darwin

// Design: docs/architecture/testing/interop.md -- the persistent image cache.
// Related: host_cpu.go -- the identity Linux and macOS answer.
package interoplab

import (
	"errors"
	"runtime"
)

// hostCPUIdentity answers that this system's CPU identity is not read, so a
// host-CPU-specific image is built rather than loaded from the cache.
func hostCPUIdentity() (string, error) {
	return "", errors.New("le reads no CPU identity on " + runtime.GOOS)
}
