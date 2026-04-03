package healthcheck

import (
	"context"
	"sync"
	"testing"
)

type mockIPManager struct {
	mu      sync.Mutex
	added   []string // "iface:cidr"
	removed []string
}

func (m *mockIPManager) AddAddress(ifaceName, cidr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.added = append(m.added, ifaceName+":"+cidr)
	return nil
}

func (m *mockIPManager) RemoveAddress(ifaceName, cidr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, ifaceName+":"+cidr)
	return nil
}

func TestIPSetupStartup(t *testing.T) {
	mock := &mockIPManager{}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32", "10.0.0.2/32"})
	ipt.addAll()

	if len(mock.added) != 2 {
		t.Fatalf("added = %d, want 2", len(mock.added))
	}
	if mock.added[0] != "lo:10.0.0.1/32" {
		t.Errorf("added[0] = %q, want lo:10.0.0.1/32", mock.added[0])
	}
	if mock.added[1] != "lo:10.0.0.2/32" {
		t.Errorf("added[1] = %q, want lo:10.0.0.2/32", mock.added[1])
	}
}

func TestIPRemoveAll(t *testing.T) {
	mock := &mockIPManager{}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32"})
	ipt.addAll()
	ipt.removeAll()

	if len(mock.removed) != 1 {
		t.Fatalf("removed = %d, want 1", len(mock.removed))
	}
	if mock.removed[0] != "lo:10.0.0.1/32" {
		t.Errorf("removed[0] = %q, want lo:10.0.0.1/32", mock.removed[0])
	}
}

func TestIPRemoveOnlyAdded(t *testing.T) {
	mock := &mockIPManager{}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32", "10.0.0.2/32"})
	ipt.addAll()
	ipt.removeAll()

	// removeAll should only remove IPs that were successfully added.
	if len(mock.removed) != 2 {
		t.Fatalf("removed = %d, want 2", len(mock.removed))
	}
}

func TestIPAddAllIdempotent(t *testing.T) {
	mock := &mockIPManager{}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32"})
	ipt.addAll()
	ipt.addAll() // second call should skip already-added

	if len(mock.added) != 1 {
		t.Fatalf("added = %d, want 1 (idempotent)", len(mock.added))
	}
}

func TestIPRemoveAllThenAdd(t *testing.T) {
	mock := &mockIPManager{}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32"})
	ipt.addAll()
	ipt.removeAll()
	ipt.addAll() // re-add after remove

	if len(mock.added) != 2 {
		t.Fatalf("added = %d, want 2 (initial + re-add)", len(mock.added))
	}
}

func TestIPDynamicRemoveOnDown(t *testing.T) {
	mock := &mockIPManager{}
	mgr := &probeManager{
		probes: make(map[string]*runningProbe),
		ipMgr:  mock,
		dispatchFn: func(ctx context.Context, _ string) (string, string, error) {
			return "done", "", nil
		},
	}
	cfg := ProbeConfig{IPDynamic: true}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32"})
	ipt.addAll()

	mgr.handleIPTransition(ipt, cfg, StateDown)
	if len(mock.removed) != 1 {
		t.Fatalf("removed = %d, want 1 (dynamic, DOWN)", len(mock.removed))
	}
}

func TestIPStaticKeepOnDown(t *testing.T) {
	mock := &mockIPManager{}
	mgr := &probeManager{
		probes: make(map[string]*runningProbe),
		ipMgr:  mock,
		dispatchFn: func(ctx context.Context, _ string) (string, string, error) {
			return "done", "", nil
		},
	}
	cfg := ProbeConfig{IPDynamic: false}
	ipt := newIPTracker(mock, "lo", []string{"10.0.0.1/32"})
	ipt.addAll()

	mgr.handleIPTransition(ipt, cfg, StateDown)
	if len(mock.removed) != 0 {
		t.Fatalf("removed = %d, want 0 (static, DOWN)", len(mock.removed))
	}
}
