//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: probe_linux.go -- MPLSIPMTU, which sizes a label space in the namespace
// Related: internal/plugins/iface/netlink/kernelcap_linux.go -- the xfrm interface probe
//
// Some capabilities have no read-only question: the kernel answers only a
// request that could change its state, or only once a per-namespace setting
// the caller's namespace does not hold. Those probes ask from a namespace of
// their own, so whatever the kernel does there disappears with it and the
// caller's namespace is never touched.

package kernelcap

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// InThrowawayNetworkNamespace runs probe inside a fresh network namespace
// created for this one call, and returns its answer. The probe MAY change
// anything in that namespace: a link, a route, a sysctl under /proc/sys/net.
// None of it reaches the caller's namespace. A probe that needs more, such as
// a mount namespace of its own, unshares it on the same locked thread.
//
// unshare moves only the calling OS thread, so the probe runs on a goroutine
// of its own that keeps its thread locked and returns; the Go runtime then
// destroys that thread, no other goroutine ever runs in the namespace, and it
// is freed with its last thread. Creating it needs CAP_SYS_ADMIN; without it
// the answer is unknown and says so.
//
// Safe for concurrent use: each call has its own thread and namespace.
func InThrowawayNetworkNamespace(probe func() Result) Result {
	verdict := make(chan Result, 1)
	go runInThrowawayNamespace(probe, verdict)
	return <-verdict
}

// runInThrowawayNamespace is the one-shot worker of
// InThrowawayNetworkNamespace. It MUST NOT unlock its OS thread: the thread is
// in the throwaway namespace, and an unlocked thread would go back to the
// scheduler and run other goroutines there.
func runInThrowawayNamespace(probe func() Result, verdict chan<- Result) {
	runtime.LockOSThread()

	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		verdict <- Result{
			State:  StateUnknown,
			Reason: fmt.Errorf("create a throwaway network namespace for the probe (needs CAP_SYS_ADMIN): %w", err),
		}
		return
	}

	verdict <- probe()
}
