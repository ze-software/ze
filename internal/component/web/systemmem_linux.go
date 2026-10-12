// Design: docs/architecture/web-components.md -- system memory for dashboard panel

//go:build linux

package web

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/host"
)

// totalSystemMemory answers the physical memory of the host in bytes, from the
// host component's memory detector.
func totalSystemMemory() (uint64, error) {
	d := &host.Detector{}
	m, err := d.DetectMemory()
	if err != nil {
		return 0, fmt.Errorf("detect memory: %w", err)
	}
	if m == nil {
		return 0, errors.New("detect memory: no memory information returned")
	}
	return m.TotalBytes, nil
}
