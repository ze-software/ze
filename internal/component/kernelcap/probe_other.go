//go:build !linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the test override
//
// XFRM and AF_MPLS are Linux dataplanes. Off Linux ze installs neither, so no
// capability enrols here (the owners' registration files are Linux-only) and
// these probes exist for the readers that ask outside the enrolment.
//
// The answer is cannot-determine rather than absent. Absent is a refusal, and
// refusing a developer's `ze` on a laptop for a kernel table this build never
// programs reports a fault that does not exist on this host.

package kernelcap

import (
	"errors"
	"runtime"
)

var errNotLinux = errors.New("kernel capabilities are a Linux question and this host runs " + runtime.GOOS)

// XFRM reports cannot-determine off Linux.
func XFRM() Result {
	if forced, ok := forcedXFRM(); ok {
		return forced
	}
	return Result{State: StateUnknown, Reason: errNotLinux}
}

// MPLS reports cannot-determine off Linux.
func MPLS() Result {
	return Result{State: StateUnknown, Reason: errNotLinux}
}
