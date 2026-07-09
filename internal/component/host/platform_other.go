// Design: (none -- new platform detection for runtime environment)

//go:build !linux

package host

import (
	"runtime"
	"syscall"
)

// DetectPlatform on non-Linux platforms returns a minimal PlatformInfo
// with the OS type. Most capability probes are Linux-specific; FD
// limits are available on all Unix platforms.
func (d *Detector) DetectPlatform() (*PlatformInfo, error) {
	info := &PlatformInfo{}
	if runtime.GOOS == "darwin" {
		info.Type = PlatformDarwin
	}
	info.RebootAllowed = false
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err == nil {
		// syscall.Rlimit.Cur/.Max are uint64 on linux/darwin but int64 on
		// freebsd; the explicit conversions are no-ops on the former and make
		// this file vet/compile clean under GOOS=freebsd (AC-7).
		info.FDLimitSoftCurrent = uint64(limit.Cur)
		info.FDLimitHardMax = uint64(limit.Max)
		info.FDLimitRaisable = limit.Cur < limit.Max
	}
	return info, nil
}
