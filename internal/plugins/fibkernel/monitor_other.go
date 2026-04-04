// Design: docs/architecture/core-design.md -- FIB noop route monitor
// Overview: fibkernel.go -- FIB kernel plugin
// Related: monitor.go -- external change handling
// Related: monitor_linux.go -- Linux netlink route monitor
//
// Noop monitor for non-Linux platforms.

//go:build !linux

package fibkernel

import "context"

// runMonitor is a noop on non-Linux platforms.
func (f *fibKernel) runMonitor(_ context.Context) {}
