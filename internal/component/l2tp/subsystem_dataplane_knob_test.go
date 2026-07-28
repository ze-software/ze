// VALIDATES: ze.l2tp.disable-kernel-dataplane leaves the reactor with NO kernel
// worker, so a session establishes on the control plane alone and no privileged
// netlink call is attempted.
// PREVENTS: conflating this with ze.l2tp.skip-kernel-probe, which bypasses only
// the modprobe. That distinction is load-bearing:
// test/l2tp/session-stopccn-cascade.ci sets skip-kernel-probe AND requires the
// data plane (it carries option=needs-linux:caps=net-admin and says so at
// :202-206), so widening that knob would silently break it.

package l2tp

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

// useEnv applies t.Setenv values and rebuilds the env cache around them.
//
// env.Get reads a process-wide cache built ONCE from os.Environ (env.go:46-57),
// so a t.Setenv after any earlier Get is invisible. Resetting before AND after
// is what makes these tests order-independent: without the trailing reset, the
// next test in the package inherits this one's cached knob values.
func useEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// startWithListener starts a subsystem bound to an ephemeral loopback port and
// registers Stop. Port 0 lets the kernel choose, so concurrent tests never
// collide on a fixed port.
func startWithListener(t *testing.T) *Subsystem {
	t.Helper()
	sub := NewSubsystem(Parameters{
		Enabled:     true,
		ListenAddrs: []netip.AddrPort{netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0)},
	})
	ctx := context.Background()
	require.NoError(t, sub.Start(ctx, nil, nil))
	t.Cleanup(func() { _ = sub.Stop(ctx) })
	return sub
}

// withStubbedKernelWorker replaces the kernel-worker constructor for one test
// and reports how many times it was called.
func withStubbedKernelWorker(t *testing.T) *int {
	t.Helper()
	calls := 0
	saved := newSubsystemKernelWorkerFn
	newSubsystemKernelWorkerFn = func(chan<- kernelSetupFailed, chan<- kernelSetupSucceeded, *slog.Logger) *kernelWorker {
		calls++
		return nil
	}
	t.Cleanup(func() { newSubsystemKernelWorkerFn = saved })
	return &calls
}

// TestDisableKernelDataplaneSkipsTheWorker verifies the knob prevents the kernel
// worker from being constructed at all.
func TestDisableKernelDataplaneSkipsTheWorker(t *testing.T) {
	useEnv(t, map[string]string{
		"ze.l2tp.skip-kernel-probe":        "true",
		"ze.l2tp.disable-kernel-dataplane": "true",
	})
	calls := withStubbedKernelWorker(t)

	sub := startWithListener(t)

	assert.Zero(t, *calls, "the knob must skip construction, not just discard the result")
	require.NotEmpty(t, sub.reactors)
	for i, r := range sub.reactors {
		assert.Nil(t, r.kernelWorker, "reactor %d must have no kernel worker", i)
	}
}

// TestKernelDataplaneOnByDefault pins the default so the knob cannot silently
// become the normal path.
//
// PREVENTS: a production deployment losing its data plane because the default
// flipped. Asserting the CONSTRUCTOR ran keeps this deterministic on hosts where
// the real constructor would return nil anyway (non-Linux, genl resolve failure).
func TestKernelDataplaneOnByDefault(t *testing.T) {
	useEnv(t, map[string]string{"ze.l2tp.skip-kernel-probe": "true"})
	calls := withStubbedKernelWorker(t)

	startWithListener(t)

	assert.Equal(t, 1, *calls, "one listener means one kernel-worker construction")
}

// TestSkipKernelProbeAloneKeepsTheDataplane is the regression guard for the
// distinction this knob exists to make.
//
// PREVENTS: someone "simplifying" the two knobs into one.
// test/l2tp/session-stopccn-cascade.ci sets skip-kernel-probe and needs the data
// plane; if that knob ever skipped the worker, the test would fail with a
// symptom (no session) far from the cause (a knob widened elsewhere).
func TestSkipKernelProbeAloneKeepsTheDataplane(t *testing.T) {
	useEnv(t, map[string]string{
		"ze.l2tp.skip-kernel-probe":        "true",
		"ze.l2tp.disable-kernel-dataplane": "false",
	})
	calls := withStubbedKernelWorker(t)

	startWithListener(t)

	assert.Equal(t, 1, *calls,
		"skip-kernel-probe bypasses the modprobe only; the data plane must stay wired")
}
