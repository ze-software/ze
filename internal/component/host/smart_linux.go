//go:build linux

// Design: docs/architecture/host/smart.md -- SMART health via direct ioctl
// Related: storage_linux.go — DetectStorage calls detectSMART per device

package host

import (
	"github.com/ze-software/ze/internal/core/smart"
)

// detectSMART delegates to the core smart library for ioctl-based detection.
func (d *Detector) detectSMART(deviceName string) *smart.Info {
	return smart.Detect(deviceName, d.Root)
}
