// Design: docs/architecture/core-design.md -- tc backend stub for non-Linux

//go:build !linux

package trafficnetlink

import (
	"fmt"
	"runtime"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

func newBackend() (traffic.Backend, error) {
	return nil, fmt.Errorf("trafficnetlink: not supported on %s", runtime.GOOS)
}
