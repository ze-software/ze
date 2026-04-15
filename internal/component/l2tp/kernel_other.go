// Design: docs/research/l2tpv2-ze-integration.md -- non-Linux kernel stub
// Related: kernel_event.go -- event types (platform-independent definitions)

//go:build !linux

package l2tp

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

// Start is a no-op on non-Linux.
func (w *kernelWorker) Start() {}

// probeKernelModules is a no-op on non-Linux. L2TP kernel integration
// is Linux-only; other platforms run without data plane acceleration.
func probeKernelModules() error { return nil }

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
	var t kernelTeardownEvent
	_ = t.localTID
	_ = t.localSID
	var f kernelSetupFailed
	_ = f.localTID
	_ = f.localSID
	_ = f.err
}
