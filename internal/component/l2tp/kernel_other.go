// Design: docs/research/l2tpv2-ze-integration.md -- non-Linux kernel stub
// Related: kernel_event.go -- event types (platform-independent definitions)

//go:build !linux

package l2tp

import "log/slog"

// kernelWorker is nil on non-Linux platforms. The reactor checks for nil
// before enqueueing events, and the subsystem skips module probing and
// worker creation.
type kernelWorker struct{}

// Enqueue is a no-op on non-Linux. Satisfies the reactor's call site.
func (w *kernelWorker) Enqueue(_ any) {}

// TeardownAll is a no-op on non-Linux.
func (w *kernelWorker) TeardownAll() {}

// Stop is a no-op on non-Linux.
func (w *kernelWorker) Stop() {}

// SignalStop is a no-op on non-Linux.
func (w *kernelWorker) SignalStop() {}

// Start is a no-op on non-Linux.
func (w *kernelWorker) Start() {}

// probeKernelModules is a no-op on non-Linux. L2TP kernel integration
// is Linux-only; other platforms run without data plane acceleration.
func probeKernelModules() error { return nil }

// newSubsystemKernelWorker returns nil on non-Linux. The reactor checks
// for nil before using the worker, so the userspace control path still
// functions; sessions simply cannot program the kernel data plane.
func newSubsystemKernelWorker(_ chan<- kernelSetupFailed, _ chan<- kernelSetupSucceeded, _ *slog.Logger) *kernelWorker {
	return nil
}

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
	_ = ok.proxyInitialRecvLCPConfReq
	_ = ok.proxyLastSentLCPConfReq
	_ = ok.proxyLastRecvLCPConfReq
}
