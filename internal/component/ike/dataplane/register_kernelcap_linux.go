// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: kernelcap_linux.go -- the capabilities and their probes
//
// The XFRM backend enrols what it needs of the kernel from here, beside the
// backend registration in register.go. The enrolment is Linux-only because XFRM
// is: off Linux nothing registers, so a developer's `ze` on a laptop is not
// asked for a dataplane this build never programs.

//go:build linux

package dataplane

import "github.com/ze-software/ze/internal/component/kernelcap"

func init() {
	for _, capability := range xfrmKernelCapabilities() {
		kernelcap.MustRegister(capability)
	}
}
