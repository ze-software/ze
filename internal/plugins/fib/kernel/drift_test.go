package fibkernel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type driftRouteBackend struct {
	routes  []installedRoute
	listErr error
	onList  func()
}

func (b *driftRouteBackend) addRoute(_, _ string) error      { return nil }
func (b *driftRouteBackend) delRoute(_ string) (bool, error) { return true, nil }
func (b *driftRouteBackend) replaceRoute(_, _ string) error  { return nil }
func (b *driftRouteBackend) close() error                    { return nil }
func (b *driftRouteBackend) listZeRoutes() ([]installedRoute, error) {
	if b.onList != nil {
		b.onList()
	}
	return b.routes, b.listErr
}

// VALIDATES: checkDrift returns StatusClean when main-table routes match, ignores non-main tables, and releases f.mu before calling listZeRoutes.
// PREVENTS: false drift from non-main routing tables or lock contention during netlink route dumps.
func TestFIBKernelDriftClean(t *testing.T) {
	var f *fibKernel
	lockReleased := false
	b := &driftRouteBackend{
		routes: []installedRoute{
			{prefix: "10.0.0.0/24", nextHop: "192.168.1.1", table: 0},
			{prefix: "10.1.0.0/24", nextHop: "192.168.1.2", table: 0},
			{prefix: "10.99.0.0/24", nextHop: "192.168.1.99", table: 100},
		},
		onList: func() {
			// Acquiring write lock inside listZeRoutes proves f.mu.RLock was released first.
			f.mu.Lock()
			lockReleased = true
			f.mu.Unlock()
		},
	}
	f = newFIBKernel(b)
	f.installed["10.0.0.0/24"] = "192.168.1.1"
	f.installed["10.1.0.0/24"] = "192.168.1.2"

	out := f.checkDrift()
	assert.True(t, lockReleased, "f.mu must be released before backend.listZeRoutes")
	assert.Equal(t, pluginName, out.Subsystem)
	assert.Equal(t, string(drift.StatusClean), out.Status)
	assert.Equal(t, 2, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	assert.False(t, out.Truncated)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkDrift reports KindMissing when an installed route is absent from kernel routes.
// PREVENTS: externally deleted Ze routes going undetected.
func TestFIBKernelDriftMissingRoute(t *testing.T) {
	b := &driftRouteBackend{
		routes: []installedRoute{
			{prefix: "10.0.0.0/24", nextHop: "192.168.1.1", table: 0},
		},
	}
	f := newFIBKernel(b)
	f.installed["10.0.0.0/24"] = "192.168.1.1"
	f.installed["10.2.0.0/24"] = "192.168.1.2"

	out := f.checkDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 2, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "10.2.0.0/24", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMissing), out.Findings[0].Kind)
	assert.Equal(t, "192.168.1.2", out.Findings[0].Expected)
	assert.Equal(t, driftStateAbsent, out.Findings[0].Actual)
}

// VALIDATES: checkDrift reports KindExtra when a Ze-owned main-table route exists in kernel but not in f.installed.
// PREVENTS: orphan Ze kernel routes forwarding stale traffic unnoticed.
func TestFIBKernelDriftExtraRoute(t *testing.T) {
	b := &driftRouteBackend{
		routes: []installedRoute{
			{prefix: "10.0.0.0/24", nextHop: "192.168.1.1", table: 0},
			{prefix: "10.8.0.0/24", nextHop: "192.168.1.8", table: 0},
		},
	}
	f := newFIBKernel(b)
	f.installed["10.0.0.0/24"] = "192.168.1.1"

	out := f.checkDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "10.8.0.0/24", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindExtra), out.Findings[0].Kind)
	assert.Equal(t, driftStateAbsent, out.Findings[0].Expected)
	assert.Equal(t, "192.168.1.8", out.Findings[0].Actual)
}

// VALIDATES: checkDrift reports KindMismatch when kernel route next-hop diverges from desired next-hop.
// PREVENTS: silent next-hop divergence between RIB intent and kernel FIB.
func TestFIBKernelDriftNextHopMismatch(t *testing.T) {
	b := &driftRouteBackend{
		routes: []installedRoute{
			{prefix: "10.0.0.0/24", nextHop: "192.168.9.9", table: 0},
		},
	}
	f := newFIBKernel(b)
	f.installed["10.0.0.0/24"] = "192.168.1.1"

	out := f.checkDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, 1, out.Checked)
	assert.Equal(t, 1, out.Drifted)
	require.Len(t, out.Findings, 1)
	assert.Equal(t, "10.0.0.0/24", out.Findings[0].Subject)
	assert.Equal(t, string(drift.KindMismatch), out.Findings[0].Kind)
	assert.Equal(t, "192.168.1.1", out.Findings[0].Expected)
	assert.Equal(t, "192.168.9.9", out.Findings[0].Actual)
}

// VALIDATES: checkDrift returns StatusError when backend.listZeRoutes returns an error.
// PREVENTS: netlink listing failures being hidden as clean FIB state.
func TestFIBKernelDriftBackendError(t *testing.T) {
	b := &driftRouteBackend{listErr: errors.New("netlink dump failed")}
	f := newFIBKernel(b)
	f.installed["10.0.0.0/24"] = "192.168.1.1"

	out := f.checkDrift()
	assert.Equal(t, string(drift.StatusError), out.Status)
	assert.Equal(t, "netlink dump failed", out.Error)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkDrift returns StatusUnavailable when f.backend is nil.
// PREVENTS: nil pointer dereference when backend is absent.
func TestFIBKernelDriftNilBackend(t *testing.T) {
	f := newFIBKernel(nil)

	out := f.checkDrift()
	assert.Equal(t, pluginName, out.Subsystem)
	assert.Equal(t, string(drift.StatusUnavailable), out.Status)
	assert.Equal(t, 0, out.Checked)
	assert.Equal(t, 0, out.Drifted)
	require.NotNil(t, out.Findings)
	assert.Empty(t, out.Findings)
}

// VALIDATES: checkDrift caps findings at drift.MaxFindingsPerSubsystem (64) and sets Truncated.
// PREVENTS: unbounded drift payloads when large routing tables diverge.
func TestFIBKernelDriftTruncation(t *testing.T) {
	const total = drift.MaxFindingsPerSubsystem + 6
	b := &driftRouteBackend{}
	f := newFIBKernel(b)
	for i := range total {
		var tb textbuf.Buffer
		pfx := tb.Str("10.0.").Int(int64(i)).Str(".0/24").String()
		f.installed[pfx] = "192.168.1.1"
	}

	out := f.checkDrift()
	assert.Equal(t, string(drift.StatusDrifted), out.Status)
	assert.Equal(t, total, out.Checked)
	assert.Equal(t, total, out.Drifted)
	assert.True(t, out.Truncated)
	assert.Len(t, out.Findings, drift.MaxFindingsPerSubsystem)

	handlerOut, err := f.handleDriftCheck(pluginName)
	require.NoError(t, err)
	assert.Equal(t, out, handlerOut)
}
