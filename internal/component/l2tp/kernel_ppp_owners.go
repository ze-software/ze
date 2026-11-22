// Design: docs/research/l2tpv2-ze-integration.md -- worker custody of original PPP files
// Related: kernel_ppp_owners_linux.go -- native role preparation and cleanup

//go:build linux

package l2tp

import (
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/callsink"
)

// kernelPPPOwners is one worker-owned original pair. A bridge consumes both
// fresh channel owners; local termination owns channel and unit. Labels are
// diagnostic labels, never a substitute for these pointers. The worker mutex
// protects published records; setup owns its unpublished record exclusively.
// The worker MUST retain this record after uncertain Close. It MUST stop
// touching both files after takePPPConfig transfers the exact pair.
type kernelPPPOwners struct {
	channel   *pppkernel.CreatedChannel
	unit      *pppkernel.CreatedUnit
	bridge    kernelPPPBridge
	relayFile callsink.RelayFileLease
}

// kernelPPPBridge is private lifecycle injection, never a public native grant.
// Production supplies only OriginalChannelBridge. The owning worker MUST keep
// the handle and transport while ClosePending, and MUST retain terminal errors.
type kernelPPPBridge interface {
	Bridge() error
	Unbridge() error
	Close() error
	ClosePending() bool
}

// kernelPPPSetup is produced only after the real role's preflight succeeds.
// Execution may allocate resources; preparation performs no native allocation.
// A missing plan is a refusal, never permission to run an unprepared setup.
type kernelPPPSetup func() (pppSessionFDs, error)

// kernelPPPTransfer is the exact worker's non-consuming preparation. A later
// take rechecks both original pointers under the same worker mutex before
// releasing custody; the configuration's numeric fields grant no ownership.
type kernelPPPTransfer struct {
	owner     *pppSessionFDs
	resources *kernelPPPOwners
	config    pppkernel.SessionConfig
}
