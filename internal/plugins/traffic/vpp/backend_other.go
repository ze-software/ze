// Design: plan/learned/627-fw-7-traffic-vpp.md -- VPP traffic backend stub for non-Linux

//go:build !linux

package trafficvpp

import (
	"fmt"
	"runtime"

	"github.com/ze-software/ze/internal/component/traffic"
)

func newBackend() (traffic.Backend, error) {
	return nil, fmt.Errorf("trafficvpp: not supported on %s", runtime.GOOS)
}
