// Design: docs/research/l2tpv2-ze-integration.md -- non-Linux kernel stub
// Related: kernel_event.go -- event types (platform-independent definitions)

//go:build !linux

package l2tp

import (
	"context"
	"errors"
	"log/slog"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// kernelWorker is nil on non-Linux platforms. The reactor checks for nil
// before enqueueing events, and the subsystem skips module probing and
// worker creation.
type kernelWorker struct{}

// Enqueue is a no-op on non-Linux. Satisfies the reactor's call site.
func (w *kernelWorker) Enqueue(_ any) {}

// kernelPPPTransfer is empty on non-Linux, where no native pair is prepared.
type kernelPPPTransfer struct{}

// No native pair can be prepared or transferred on this platform.
func (w *kernelWorker) preparePPPTransfer(_ kernelSetupSucceeded, _ pppkernel.InterfaceOperations) (kernelPPPTransfer, bool, error) {
	return kernelPPPTransfer{}, false, nil
}
func (w *kernelWorker) takePPPConfig(_ kernelSetupSucceeded, _ kernelPPPTransfer) (pppkernel.SessionConfig, bool) {
	return pppkernel.SessionConfig{}, false
}

// setSocketHooks is a no-op on non-Linux: there is no kernel tunnel, so no
// connected socket ever displaces the listener for a peer's datagrams.
func (w *kernelWorker) setSocketHooks(_ func(tid uint16, fd int) error, _ func(tid uint16)) {}

// teardownAll is a no-op on non-Linux.
func (w *kernelWorker) teardownAll() {}

// No native session can be acquired by this platform's worker.
func (w *kernelWorker) originalCleanupStatus(_ *L2TPSession) error { return nil }
func (w *kernelWorker) cleanupOriginal(ctx context.Context, _ uint16, _ *L2TPSession) error {
	return ctx.Err()
}
func (w *kernelWorker) pendingCleanup() error { return nil }

// Stop is a no-op on non-Linux.
func (w *kernelWorker) Stop() {}

// signalStop is a no-op on non-Linux.
func (w *kernelWorker) signalStop() {}

// Start is a no-op on non-Linux.
func (w *kernelWorker) Start() {}

// probeKernelModules is a no-op on non-Linux. L2TP kernel integration
// is Linux-only; other platforms run without data plane acceleration.
func probeKernelModules() error { return nil }

// newSubsystemKernelWorker returns a nil worker and errNoKernelDataPlane on
// non-Linux. The reactor checks for nil before using the worker, so the
// userspace control path still functions; sessions simply cannot program the
// kernel data plane, and the l2tp health row says so (checkHealth).
func newSubsystemKernelWorker(_ chan<- kernelSetupFailed, _ chan<- kernelSetupSucceeded, _ *slog.Logger, _ kernelBackendPreparation) (*kernelWorker, error) {
	return nil, errNoKernelDataPlane
}

// errNoKernelDataPlane is the data-plane state on a platform with no L2TP
// kernel module.
var errNoKernelDataPlane = errors.New("this platform has no L2TP kernel data plane (Linux only)")

// Compile-time references so the linter does not flag event type fields
// as unused on non-Linux. All fields are consumed by kernel_linux.go.
var _ = func() {
	var s kernelSetupEvent
	_ = s.localTID
	_ = s.remoteTID
	_ = s.peerAddr
	_ = s.localSID
	_ = s.remoteSID
	_ = s.socketFD
	_ = s.lnsMode
	_ = s.sequencing
	_ = s.proxyInitialRecvLCPConfReq
	_ = s.proxyLastSentLCPConfReq
	_ = s.proxyLastRecvLCPConfReq
	var t kernelTeardownEvent
	_ = t.localTID
	_ = t.localSID
	var f kernelSetupFailed
	_ = f.localTID
	_ = f.localSID
	_ = f.err
	var ok kernelSetupSucceeded
	_ = ok.localTID
	_ = ok.localSID
	_ = ok.lnsMode
	_ = ok.sequencing
	_ = ok.fds
	_ = ok.fds.pppoxFD
	_ = ok.owner
	_ = ok.proxyInitialRecvLCPConfReq
	_ = ok.proxyLastSentLCPConfReq
	_ = ok.proxyLastRecvLCPConfReq
}
