// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: kernelcap_linux.go -- the capability and its probe
//
// The PPPoE access concentrator enrolls what it needs of the kernel from here.
// The enrolment is Linux-only because AF_PPPOX is a Linux socket family.

//go:build linux

package pppoe

import "github.com/ze-software/ze/internal/component/kernelcap"

func init() {
	kernelcap.MustRegister(pppoeKernelCapability())
}
