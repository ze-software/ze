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
		info.FDLimitSoftCurrent = limit.Cur
		info.FDLimitHardMax = limit.Max
		info.FDLimitRaisable = limit.Cur < limit.Max
	}
	return info, nil
}
