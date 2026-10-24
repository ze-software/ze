package l2tp

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/health"
)

// TestL2TPHealthReadsDataPlane checks that the l2tp health row reads whether the
// kernel data plane came up, not only whether the subsystem is registered.
//
// Method: start a subsystem whose kernel-worker constructor fails the way a host
// without the l2tp generic netlink family fails, then ask the registered check.
// A healthy answer there is the defect: sessions establish on the control plane
// and none is ever programmed into the kernel.
func TestL2TPHealthReadsDataPlane(t *testing.T) {
	useEnv(t, map[string]string{"ze.l2tp.skip-kernel-probe": "true"})
	saved := newSubsystemKernelWorkerFn
	newSubsystemKernelWorkerFn = func(chan<- kernelSetupFailed, chan<- kernelSetupSucceeded, *slog.Logger) (*kernelWorker, error) {
		return nil, errors.New("generic netlink family l2tp not found")
	}
	t.Cleanup(func() { newSubsystemKernelWorkerFn = saved })

	startWithListener(t)
	require.NotNil(t, LookupService(), "the started subsystem must be published")

	status, reason := checkHealth()
	assert.Equal(t, health.StatusDegraded, status, "a dead kernel data plane is not healthy")
	assert.Contains(t, reason, "kernel data plane", "the reason names what is down")
	assert.Contains(t, reason, "generic netlink family l2tp not found", "the reason carries the measured failure")
	assert.Contains(t, reason, "ze doctor", "the reason names the command that measures it")
}

// TestL2TPHealthHealthyOnlyWithDataPlane checks the other side of the row: a
// subsystem whose every listener has a kernel worker reads healthy, and the
// disable knob reads degraded under its own name.
func TestL2TPHealthHealthyOnlyWithDataPlane(t *testing.T) {
	status, reason := dataPlaneHealth(nil)
	assert.Equal(t, health.StatusHealthy, status)
	assert.Empty(t, reason)

	status, reason = dataPlaneHealth(errKernelDataPlaneDisabled)
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "ze.l2tp.disable-kernel-dataplane")
}
