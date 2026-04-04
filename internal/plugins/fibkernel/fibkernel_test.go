package fibkernel

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
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

func makeSysribEvent(changes []incomingChange) ze.Event {
	batch := incomingBatch{Changes: changes}
	payload, _ := json.Marshal(batch)
	return ze.Event{
		Topic:    "sysrib/best-change",
		Payload:  payload,
		Metadata: map[string]string{"family": "ipv4/unicast"},
	}
}

// VALIDATES: AC-8 -- sysrib/best-change with action "add" for 10.0.0.0/24,
// fib-kernel installs route via backend.
// PREVENTS: Routes not being installed in OS.
func TestFIBKernelInstall(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	event := makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	})
	f.processEvent(event)

	assert.Equal(t, "192.168.1.1", backend.added["10.0.0.0/24"])
	assert.Equal(t, "192.168.1.1", f.installed["10.0.0.0/24"])
}

// VALIDATES: AC-9 -- sysrib/best-change with action "withdraw",
// fib-kernel removes route from OS.
// PREVENTS: Withdrawn routes remaining in kernel.
func TestFIBKernelRemove(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install first.
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	// Withdraw.
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "withdraw", Prefix: "10.0.0.0/24"},
	}))

	assert.Contains(t, backend.deleted, "10.0.0.0/24")
	assert.Empty(t, f.installed)
}

// VALIDATES: AC-10 -- sysrib/best-change with action "update",
// fib-kernel replaces route.
// PREVENTS: Route updates not being applied.
func TestFIBKernelReplace(t *testing.T) {
	backend := newMockBackend()
	f := newFIBKernel(backend)

	// Install.
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	// Update with new next-hop.
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "update", Prefix: "10.0.0.0/24", NextHop: "192.168.2.1", Protocol: "static"},
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
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
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
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
		{Action: "add", Prefix: "172.16.0.0/16", NextHop: "192.168.1.2", Protocol: "static"},
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

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	data := f.showInstalled()
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
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
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
	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	// Simulate external delete (shown as overwrite with empty next-hop).
	f.handleExternalChange("10.0.0.0/24", "", 0)

	// Should re-assert.
	assert.Equal(t, "192.168.1.1", backend.replaced["10.0.0.0/24"])
}
