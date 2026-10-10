// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: kernelcap_linux.go -- the capabilities and their probes
//
// The netlink interface backend enrolls the kernel pieces its interface kinds
// need from here, beside the backend registration in register.go.

//go:build linux

package ifacenetlink

import "github.com/ze-software/ze/internal/component/kernelcap"

func init() {
	for _, capability := range ifaceKernelCapabilities() {
		kernelcap.MustRegister(capability)
	}
}
