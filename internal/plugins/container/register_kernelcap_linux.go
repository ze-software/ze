// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: kernelcap_linux.go -- the capabilities and their probes
// Related: register.go -- the plugin registration, which publishes the codes the enrolments name
//
// The container plugin enrolls what it needs of the kernel from here. The
// enrolment is Linux-only because containers are.

//go:build linux

package container

import "github.com/ze-software/ze/internal/component/kernelcap"

//nolint:gochecknoinits // registration: the kernel capability registry names no feature, so the capability's owner enrolls it
func init() {
	for _, capability := range containerKernelCapabilities() {
		kernelcap.MustRegister(capability)
	}
}
