// Design: docs/architecture/web-components.md -- system memory for dashboard panel

//go:build linux

package web

import "codeberg.org/thomas-mangin/ze/internal/component/host"

func totalSystemMemory() uint64 {
	d := &host.Detector{}
	m, err := d.DetectMemory()
	if err != nil || m == nil {
		return 0
	}
	return m.TotalBytes
}
