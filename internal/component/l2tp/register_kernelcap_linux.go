// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: kernelcap_linux.go -- the capabilities and their probes
//
// The L2TP subsystem enrolls what it needs of the kernel from here. The
// enrolment is Linux-only because both pieces are Linux kernel interfaces.

//go:build linux

package l2tp

import "github.com/ze-software/ze/internal/component/kernelcap"

func init() {
	for _, capability := range l2tpKernelCapabilities() {
		kernelcap.MustRegister(capability)
	}
}
