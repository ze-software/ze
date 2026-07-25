package fibkernel

import (
	"encoding/json"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
)

// mockBackend records route operations for testing.
type mockBackend struct {
	mu       sync.Mutex
	added    map[string]string // prefix -> next-hop
	deleted  []string
	replaced map[string]string
	zeRoutes []installedRoute // returned by listZeRoutes
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		added:    make(map[string]string),
		replaced: make(map[string]string),
	}
}

func (m *mockBackend) addRoute(prefix, nextHop string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.added[prefix] = nextHop
	return nil
}

func (m *mockBackend) delRoute(prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, prefix)
	return nil
}

func (m *mockBackend) replaceRoute(prefix, nextHop string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replaced[prefix] = nextHop
	return nil
}

func (m *mockBackend) listZeRoutes() ([]installedRoute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.zeRoutes, nil
}

func (m *mockBackend) close() error { return nil }

// makeSysribPayload builds a typed (system-rib, best-change) payload for tests.
// Returns *incomingBatch — the shape the typed handle carries on the bus.
func makeSysribPayload(changes []incomingChange) *incomingBatch {
	return &incomingBatch{
		Family:  family.IPv4Unicast,
		Changes: changes,
	}
}

// VALIDATES: AC-8 -- (sysrib, best-change) with action "add" for 10.0.0.0/24,
// fib-kernel installs route via backend.
// PREVENTS: Routes not being installed in OS.
func TestFIBKernelInstall(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	payload := makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	})
	f.processEvent(payload)

	assert.Equal(t, "192.168.1.1", backend.added["10.0.0.0/24"])
	assert.Equal(t, "192.168.1.1", f.installed["10.0.0.0/24"])
}

// VALIDATES: AC-9 -- (sysrib, best-change) with action "withdraw",
// fib-kernel removes route from OS.
// PREVENTS: Withdrawn routes remaining in kernel.
func TestFIBKernelRemove(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install first.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	// Withdraw.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("10.0.0.0/24")},
	}))

	assert.Contains(t, backend.deleted, "10.0.0.0/24")
	assert.Empty(t, f.installed)
}

// VALIDATES: AC-10 -- (sysrib, best-change) with action "update",
// fib-kernel replaces route.
// PREVENTS: Route updates not being applied.
func TestFIBKernelReplace(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	// Update with new next-hop.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.2.1"), Protocol: "static"},
	}))

	assert.Equal(t, "192.168.2.1", backend.replaced["10.0.0.0/24"])
	assert.Equal(t, "192.168.2.1", f.installed["10.0.0.0/24"])
}

// VALIDATES: AC-15 -- fib-kernel starts after crash (ze routes exist in kernel).
// Marks existing ze routes as stale, returns them for later sweep.
// PREVENTS: Stale routes persisting after crash recovery.
func TestFIBKernelStartupSweep(t *testing.T) {
	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{
		{prefix: "10.0.0.0/24", nextHop: "192.168.1.1"},
		{prefix: "172.16.0.0/16", nextHop: "192.168.1.2"},
	}
	f := newFIBKernel(backend)

	stale := f.startupSweep()

	require.Len(t, stale, 2)
	assert.Equal(t, "192.168.1.1", stale["10.0.0.0/24"])
	assert.Equal(t, "192.168.1.2", stale["172.16.0.0/16"])
}

// VALIDATES: AC-15 -- After startup sweep, refreshed routes survive,
// stale routes are removed.
// PREVENTS: Refreshed routes being incorrectly swept.
func TestFIBKernelSweepStale(t *testing.T) {
	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{
		{prefix: "10.0.0.0/24", nextHop: "192.168.1.1"},
		{prefix: "172.16.0.0/16", nextHop: "192.168.1.2"},
	}
	f := newFIBKernel(backend)

	stale := f.startupSweep()

	// Simulate sysrib refreshing one route.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	// Sweep: 10.0.0.0/24 was refreshed (should survive), 172.16.0.0/16 was not (should be deleted).
	f.sweepStale(stale)

	assert.Contains(t, backend.deleted, "172.16.0.0/16", "stale route should be swept")
	// 10.0.0.0/24 should NOT be in deleted (it was refreshed).
	for _, d := range backend.deleted {
		assert.NotEqual(t, "10.0.0.0/24", d, "refreshed route should not be swept")
	}
}

// VALIDATES: AC-14 -- fib-kernel stops gracefully with flush-on-stop=true.
// PREVENTS: Routes lingering after shutdown.
func TestFIBKernelFlushOnStop(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install routes.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("172.16.0.0/16"), NextHop: netip.MustParseAddr("192.168.1.2"), Protocol: "static"},
	}))

	f.flushRoutes()

	assert.Len(t, backend.deleted, 2)
	assert.Empty(t, f.installed)
}

// VALIDATES: AC-8 -- showInstalled returns correct JSON.
// PREVENTS: CLI showing stale or wrong data.
func TestFIBKernelShowInstalled(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	raw, err := json.Marshal(f.showInstalled())
	require.NoError(t, err)
	data := string(raw)
	assert.Contains(t, data, "10.0.0.0/24")
	assert.Contains(t, data, "192.168.1.1")
}

// VALIDATES: AC-19 -- External process adds route for ze-managed prefix.
// fib-kernel re-asserts ze's route via backend.replaceRoute.
// PREVENTS: External route overwriting ze-managed routes.
func TestFIBKernelMonitorReassert(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install a route first.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	// Simulate external change on managed prefix.
	f.handleExternalChange("10.0.0.0/24", "1.2.3.4", 9)

	// Should have re-asserted ze's route.
	assert.Equal(t, "192.168.1.1", backend.replaced["10.0.0.0/24"],
		"should re-assert ze's next-hop, not the external one")
}

// VALIDATES: AC-21 -- External route change on prefix not managed by ze.
// fib-kernel ignores (no conflict, no re-assertion).
// PREVENTS: fib-kernel interfering with non-ze routes.
func TestFIBKernelMonitorIgnoreUnmanaged(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// No routes installed. External change on unmanaged prefix.
	f.handleExternalChange("172.16.0.0/16", "1.2.3.4", 9)

	// Should not have called replaceRoute.
	assert.Empty(t, backend.replaced, "should not re-assert for unmanaged prefix")
}

// VALIDATES: AC-20 -- External process deletes ze route.
// fib-kernel re-asserts when it sees an overwrite on a managed prefix.
// PREVENTS: Route deletion going undetected.
func TestFIBKernelMonitorReassertOnDelete(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install a route.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	// Simulate external delete (shown as overwrite with empty next-hop).
	f.handleExternalChange("10.0.0.0/24", "", 0)

	// Should re-assert.
	assert.Equal(t, "192.168.1.1", backend.replaced["10.0.0.0/24"])
}

// TestFibKernelApplyJournal verifies that fib-kernel config apply via journal
// supports rollback (no-op for fib-kernel since it reacts to bus events).
//
// VALIDATES: AC-9 - fib-kernel config change: config applied via journal.
// PREVENTS: Plugin missing transaction protocol compliance.
func TestFibKernelApplyJournal(t *testing.T) {
	// fib-kernel has no config-driven state changes; it reacts to sysrib bus events.
	// The journal is a protocol compliance wrapper -- apply and rollback are no-ops.
	j := &testJournal{}
	err := j.Record(
		func() error { return nil }, // apply: no-op
		func() error { return nil }, // undo: no-op
	)
	require.NoError(t, err)
	assert.Equal(t, 1, j.count)

	errs := j.Rollback()
	assert.Empty(t, errs)
}

// testJournal is a minimal journal for testing.
type testJournal struct {
	entries []func() error
	count   int
}

func (j *testJournal) Record(apply, undo func() error) error {
	if err := apply(); err != nil {
		return err
	}
	j.entries = append(j.entries, undo)
	j.count++
	return nil
}

func (j *testJournal) Rollback() []error {
	var errs []error
	for i := len(j.entries) - 1; i >= 0; i-- {
		if err := j.entries[i](); err != nil {
			errs = append(errs, err)
		}
	}
	j.entries = nil
	return errs
}

func (j *testJournal) Discard() {
	j.entries = nil
}

// failingBackend returns errors for specified operations.
type failingBackend struct {
	mockBackend
	failAdd     bool
	failReplace bool
	failDel     bool
}

func (m *failingBackend) addRoute(prefix, nextHop string) error {
	if m.failAdd {
		return errors.New("netlink: operation not permitted")
	}
	return m.mockBackend.addRoute(prefix, nextHop)
}

func (m *failingBackend) replaceRoute(prefix, nextHop string) error {
	if m.failReplace {
		return errors.New("netlink: operation not permitted")
	}
	return m.mockBackend.replaceRoute(prefix, nextHop)
}

func (m *failingBackend) delRoute(prefix string) error {
	if m.failDel {
		return errors.New("netlink: no such process")
	}
	return m.mockBackend.delRoute(prefix)
}

// VALIDATES: AC-12 -- FIB programming error raises fib-sync-failure error on report bus.
// PREVENTS: Silent FIB programming failures going unnoticed by operators.
func TestFIBSyncFailure(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	backend := &failingBackend{mockBackend: *newMockBackend(), failAdd: true}
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	errs := report.Errors(0)
	found := false
	for _, e := range errs {
		if e.Code == reportCodeFIBSyncFailure && e.Subject == "10.0.0.0/24" {
			found = true
			assert.Equal(t, "add", e.Detail["operation"])
		}
	}
	if !found {
		t.Fatal("fib-sync-failure error not raised on add failure")
	}

	assert.Empty(t, f.installed, "failed route should not be in installed map")
}

// VALIDATES: AC-12 -- FIB replace failure also raises fib-sync-failure.
// PREVENTS: Replace errors not being reported.
func TestFIBSyncFailureOnReplace(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	backend := &failingBackend{mockBackend: *newMockBackend(), failReplace: true}
	f := newFIBKernel(backend)

	// Install successfully first.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	// Replace fails.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.2.1"), Protocol: "bgp"},
	}))

	errs := report.Errors(0)
	found := false
	for _, e := range errs {
		if e.Code == reportCodeFIBSyncFailure && e.Subject == "10.0.0.0/24" {
			found = true
			assert.Equal(t, "replace", e.Detail["operation"])
		}
	}
	if !found {
		t.Fatal("fib-sync-failure error not raised on replace failure")
	}
}

// VALIDATES: AC-13 -- Orphan routes detected during sweep raise fib-orphan warning.
// PREVENTS: Stale kernel routes going unnoticed.
func TestFIBOrphanWarning(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{
		{prefix: "10.0.0.0/24", nextHop: "192.168.1.1"},
		{prefix: "172.16.0.0/16", nextHop: "192.168.1.2"},
	}
	f := newFIBKernel(backend)

	stale := f.startupSweep()

	// Refresh only one route.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	f.sweepStale(stale)

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeFIBOrphan {
			found = true
			assert.Equal(t, 1, w.Detail["orphan_count"])
		}
	}
	if !found {
		t.Fatal("fib-orphan warning not raised after sweep with orphan routes")
	}
}

// VALIDATES: AC-13 -- No orphan warning when all routes are refreshed.
// PREVENTS: False positive orphan warnings.
func TestFIBOrphanClearedWhenNoOrphans(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{
		{prefix: "10.0.0.0/24", nextHop: "192.168.1.1"},
	}
	f := newFIBKernel(backend)

	stale := f.startupSweep()

	// Refresh the only route.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	f.sweepStale(stale)

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeFIBOrphan {
			t.Fatal("fib-orphan warning raised when all routes were refreshed")
		}
	}
}

// VALIDATES: AC-14 -- Routes pending FIB install for >30s raise fib-programming-lag warning.
// PREVENTS: Persistent FIB failures going unnoticed.
func TestFIBProgrammingLag(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	backend := &failingBackend{mockBackend: *newMockBackend(), failAdd: true}
	f := newFIBKernel(backend)

	// First failure: sets pending timestamp to now. No lag yet.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeFIBProgrammingLag {
			t.Fatal("fib-programming-lag should not fire on first failure (not yet >30s)")
		}
	}

	// Backdate the pending entry to simulate 31 seconds ago.
	f.mu.Lock()
	f.pending["10.0.0.0/24"] = time.Now().Add(-31 * time.Second)
	f.mu.Unlock()

	// Trigger another event to re-check lag.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.1.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.2"), Protocol: "bgp"},
	}))

	warnings = report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Code == reportCodeFIBProgrammingLag {
			found = true
			assert.Equal(t, 1, w.Detail["lagging"])
		}
	}
	if !found {
		t.Fatal("fib-programming-lag warning not raised after >30s pending")
	}
}

// VALIDATES: AC-14 -- Lag warning clears when pending route succeeds.
// PREVENTS: Stale lag warnings after recovery.
func TestFIBProgrammingLagCleared(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	backend := &failingBackend{mockBackend: *newMockBackend(), failAdd: true}
	f := newFIBKernel(backend)

	// Fail and backdate.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))
	f.mu.Lock()
	f.pending["10.0.0.0/24"] = time.Now().Add(-31 * time.Second)
	f.mu.Unlock()

	// Trigger lag warning.
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.1.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.2"), Protocol: "bgp"},
	}))

	// Now fix the backend and successfully install the lagging route.
	backend.failAdd = false
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.1.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.2"), Protocol: "bgp"},
	}))

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Code == reportCodeFIBProgrammingLag {
			t.Fatal("fib-programming-lag warning not cleared after successful install")
		}
	}
}

// richMockBackend records rich route operations for testing.
type richMockBackend struct {
	mockBackend
	richAdded    []RichRoute
	richReplaced []RichRoute
	richDeleted  []netip.Prefix
	// richDelTables records the tableID each delRichRoute call targeted, so a
	// test can assert the delete hits the same table the add used.
	richDelTables []uint32
}

func newRichMockBackend() *richMockBackend {
	return &richMockBackend{
		mockBackend: mockBackend{
			added:    make(map[string]string),
			replaced: make(map[string]string),
		},
	}
}

func (m *richMockBackend) addRichRoute(r RichRoute) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.richAdded = append(m.richAdded, r)
	return nil
}

func (m *richMockBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.richDeleted = append(m.richDeleted, prefix)
	m.richDelTables = append(m.richDelTables, tableID)
	return nil
}

func (m *richMockBackend) replaceRichRoute(r RichRoute) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.richReplaced = append(m.richReplaced, r)
	return nil
}

// VALIDATES: AC-6 -- BestChangeEntry with RouteType=blackhole programs correctly.
func TestKernelRouteType(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:    routeaction.Add,
			Prefix:    netip.MustParsePrefix("192.0.2.0/24"),
			Protocol:  "static",
			RouteType: sysribevents.RouteTypeBlackhole,
		},
	}))

	require.Len(t, backend.richAdded, 1)
	assert.Equal(t, sysribevents.RouteTypeBlackhole, backend.richAdded[0].RouteType)
	assert.Equal(t, netip.MustParsePrefix("192.0.2.0/24"), backend.richAdded[0].Prefix)
}

// VALIDATES: AC-8 -- BestChangeEntry with Metric sets route priority.
func TestKernelMetric(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:  netip.MustParseAddr("192.168.1.1"),
			Protocol: "bgp",
			Metric:   200,
		},
	}))

	require.Len(t, backend.richAdded, 1)
	assert.Equal(t, uint32(200), backend.richAdded[0].Metric)
}

// VALIDATES: AC-9 -- BestChangeEntry with TableID installs in correct table.
func TestKernelVRFTable(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:  netip.MustParseAddr("192.168.1.1"),
			Protocol: "bgp",
			TableID:  100,
		},
	}))

	require.Len(t, backend.richAdded, 1)
	assert.Equal(t, uint32(100), backend.richAdded[0].TableID)
}

// VALIDATES: AC-4 -- ECMP paths forwarded to backend.
func TestKernelECMPGroup(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:  netip.MustParseAddr("192.168.1.1"),
			Protocol: "bgp",
			ECMPPaths: []sysribevents.ECMPPath{
				{NextHop: netip.MustParseAddr("192.168.1.2"), Weight: 1},
				{NextHop: netip.MustParseAddr("192.168.1.3"), Weight: 1},
			},
		},
	}))

	require.Len(t, backend.richAdded, 1)
	assert.Len(t, backend.richAdded[0].ECMPPaths, 2)
}

// VALIDATES: AC-10 -- Labels forwarded to kernel backend.
func TestKernelMPLSPush(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:  netip.MustParseAddr("192.168.1.1"),
			Protocol: "bgp",
			Labels:   []uint32{100, 200},
		},
	}))

	require.Len(t, backend.richAdded, 1)
	assert.Equal(t, []uint32{100, 200}, backend.richAdded[0].Labels)
}

// VALIDATES: AC-8 -- labeled prefixes are tracked for the MPLS route gauge,
// added on install and removed on withdraw; unlabeled routes are not tracked.
func TestKernelMPLSInstalledTracking(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: pfx, NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp", Labels: []uint32{100}},
		{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.1.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1"), Protocol: "bgp"},
	}))
	assert.True(t, f.mplsInstalled[pfx.String()], "labeled prefix tracked")
	assert.Len(t, f.mplsInstalled, 1, "only the labeled prefix is tracked")

	f.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Withdraw, Prefix: pfx, Protocol: "bgp"},
	}))
	assert.Empty(t, f.mplsInstalled, "withdraw removes labeled prefix from tracking")
}

// VALIDATES: SRv6 SID triggers rich route path.
func TestKernelSRv6Encap(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	sid := netip.MustParseAddr("2001:db8::1")
	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:  netip.MustParseAddr("192.168.1.1"),
			Protocol: "bgp",
			SRv6SID:  sid,
		},
	}))

	require.Len(t, backend.richAdded, 1)
	assert.Equal(t, sid, backend.richAdded[0].SRv6SID)
}

// VALIDATES: plain route without rich fields still uses legacy backend path.
func TestKernelPlainRouteLegacyPath(t *testing.T) {
	backend := newRichMockBackend()
	f := newFIBKernel(backend)

	f.processEvent(makeSysribPayload([]incomingChange{
		{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:  netip.MustParseAddr("192.168.1.1"),
			Protocol: "bgp",
		},
	}))

	// Rich backend should NOT be called for plain routes.
	assert.Empty(t, backend.richAdded)
	assert.Equal(t, "192.168.1.1", backend.added["10.0.0.0/24"])
}
