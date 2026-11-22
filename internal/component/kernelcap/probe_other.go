//go:build !linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the forced-answer override
//
// XFRM, AF_MPLS and seg6 LWTUNNEL are Linux dataplanes. Off Linux ze installs
// none of them, so no capability enrolls here (the owners' registration files
// are Linux-only) and these probes exist for the readers that ask outside the
// enrolment.
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
	return Result{State: StateUnknown, Reason: errNotLinux}
}

// MPLS reports cannot-determine off Linux.
func MPLS() Result {
	return Result{State: StateUnknown, Reason: errNotLinux}
}

// MPLSIPMTU reports cannot-determine off Linux.
func MPLSIPMTU() Result {
	return Result{State: StateUnknown, Reason: errNotLinux}
}

// SRv6Encap reports cannot-determine off Linux.
func SRv6Encap() Result {
	return Result{State: StateUnknown, Reason: errNotLinux}
}

// SRv6ForwardTTL reports cannot-determine off Linux.
func SRv6ForwardTTL() Result {
	return Result{State: StateUnknown, Reason: errNotLinux}
}
