// Design: docs/research/l2tpv2-ze-integration.md -- non-Linux type stubs
// Related: kernel_other.go -- non-Linux kernelWorker stub
// Related: pppox_linux.go -- Linux-only definition of pppSessionFDs

//go:build !linux

package l2tp

// pppSessionFDs is referenced by kernelSetupSucceeded (kernel_event.go)
// and must exist on every build so that kernel_event.go compiles.
// The real struct lives in pppox_linux.go and carries the /dev/ppp fds
// opened via the Linux PPPoX path; on other platforms L2TP is
// userspace-only and no fds are produced, but the field must still be
// declared for the code path that references it. Only the fields
// that platform-independent code reads are declared: the /dev/ppp
// channel and unit fds exist on Linux alone.
type pppSessionFDs struct {
	pppoxFD int
	unitNum int
	bridged bool
}
