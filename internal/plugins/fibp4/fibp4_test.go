package fibp4

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// mockBackend records P4 route operations for testing.
type mockBackend struct {
	mu       sync.Mutex
	added    map[string]string
	deleted  []string
	replaced map[string]string
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

// VALIDATES: fib-p4 installs forwarding entry on add event.
// PREVENTS: Routes not reaching the P4 switch.
func TestFIBP4Install(t *testing.T) {
	backend := newMockBackend()
	f := newFIBP4(backend)

	event := makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	})
	f.processEvent(event)

	assert.Equal(t, "192.168.1.1", backend.added["10.0.0.0/24"])
	assert.Equal(t, "192.168.1.1", f.installed["10.0.0.0/24"])
}

// VALIDATES: fib-p4 removes forwarding entry on withdraw.
// PREVENTS: Stale entries in P4 switch.
func TestFIBP4Remove(t *testing.T) {
	backend := newMockBackend()
	f := newFIBP4(backend)

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "withdraw", Prefix: "10.0.0.0/24"},
	}))

	assert.Contains(t, backend.deleted, "10.0.0.0/24")
	assert.Empty(t, f.installed)
}

// VALIDATES: fib-p4 replaces forwarding entry on update.
// PREVENTS: Stale next-hops in P4 switch.
func TestFIBP4Replace(t *testing.T) {
	backend := newMockBackend()
	f := newFIBP4(backend)

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "update", Prefix: "10.0.0.0/24", NextHop: "192.168.2.1", Protocol: "static"},
	}))

	assert.Equal(t, "192.168.2.1", backend.replaced["10.0.0.0/24"])
	assert.Equal(t, "192.168.2.1", f.installed["10.0.0.0/24"])
}

// VALIDATES: fib-p4 flushRoutes removes all entries.
// PREVENTS: Entries lingering after shutdown.
func TestFIBP4FlushOnStop(t *testing.T) {
	backend := newMockBackend()
	f := newFIBP4(backend)

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
		{Action: "add", Prefix: "172.16.0.0/16", NextHop: "192.168.1.2", Protocol: "static"},
	}))

	f.flushRoutes()

	assert.Len(t, backend.deleted, 2)
	assert.Empty(t, f.installed)
}

// VALIDATES: showInstalled returns correct JSON.
// PREVENTS: CLI showing wrong data.
func TestFIBP4ShowInstalled(t *testing.T) {
	backend := newMockBackend()
	f := newFIBP4(backend)

	f.processEvent(makeSysribEvent([]incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Protocol: "bgp"},
	}))

	data := f.showInstalled()
	require.Contains(t, data, "10.0.0.0/24")
	assert.Contains(t, data, "192.168.1.1")
}
