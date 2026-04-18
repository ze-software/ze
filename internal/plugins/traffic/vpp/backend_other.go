// Design: plan/spec-fw-7-traffic-vpp.md -- VPP traffic backend stub for non-Linux

//go:build !linux

package trafficvpp

import (
	"fmt"
	"runtime"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

func newBackend() (traffic.Backend, error) {
	return nil, fmt.Errorf("trafficvpp: not supported on %s", runtime.GOOS)
}
