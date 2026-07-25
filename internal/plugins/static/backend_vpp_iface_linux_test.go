// VALIDATES: the VPP static backend resolves an interface-only next-hop to a
// VPP sw_if_index through the shared iface.Resolve (AC-4), rejects an unknown
// interface without ever emitting index 0 (AC-5), and refuses to resolve when
// the active iface backend is not vpp so a kernel ifindex is never programmed
// as a VPP sw_if_index (AC-10 / R-7).
// PREVENTS: a static route silently programming the wrong VPP interface, or the
// interface-only next-hop form staying unusable on the VPP data plane.

//go:build linux && ze_vpp

package static

import (
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// fakeIfaceBackend is a minimal iface.Backend serving GetInterface from an
// in-memory name->index map so the VPP static translation is host-testable
// without a live VPP. The embedded Backend is nil; any other method panics if a
// test reaches it (none should) -- the resolve path only calls GetInterface.
type fakeIfaceBackend struct {
	iface.Backend
	mu     sync.Mutex
	byName map[string]int
}

func (f *fakeIfaceBackend) set(ifaces map[string]int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byName = ifaces
}

// Close overrides the embedded nil Backend so CloseBackend (called in test
// cleanup) does not dispatch to a nil interface and panic.
func (f *fakeIfaceBackend) Close() error { return nil }

func (f *fakeIfaceBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if idx, ok := f.byName[name]; ok {
		return &iface.InterfaceInfo{Name: name, OsName: name, Index: idx}, nil
	}
	return nil, fmt.Errorf("fake iface backend: interface %q not found", name)
}

var (
	fakeVPPBackend     = &fakeIfaceBackend{byName: map[string]int{}}
	fakeNetlinkBackend = &fakeIfaceBackend{byName: map[string]int{}}
	fakeIfaceRegOnce   sync.Once
)

// registerFakeIfaceBackends registers the "vpp" and "netlink" fake iface
// backends exactly once per test binary (RegisterBackend rejects duplicates and
// cannot unregister). Nothing in the static test binary registers these names,
// so the fakes are the sole registrants.
func registerFakeIfaceBackends(t *testing.T) {
	t.Helper()
	fakeIfaceRegOnce.Do(func() {
		if err := iface.RegisterBackend("vpp", func() (iface.Backend, error) { return fakeVPPBackend, nil }); err != nil {
			t.Fatalf("register fake vpp iface backend: %v", err)
		}
		if err := iface.RegisterBackend("netlink", func() (iface.Backend, error) { return fakeNetlinkBackend, nil }); err != nil {
			t.Fatalf("register fake netlink iface backend: %v", err)
		}
	})
}

// loadFakeVPPBackend loads the fake vpp iface backend reporting the given
// name->index map, and clears it on cleanup so a later no-backend test sees a
// clean process-global.
func loadFakeVPPBackend(t *testing.T, ifaces map[string]int) {
	t.Helper()
	registerFakeIfaceBackends(t)
	fakeVPPBackend.set(ifaces)
	if err := iface.LoadBackend("vpp"); err != nil {
		t.Fatalf("load fake vpp iface backend: %v", err)
	}
	if iface.ActiveBackendName() != "vpp" {
		t.Fatalf("active backend name = %q, want vpp (another vpp backend is registered)", iface.ActiveBackendName())
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })
}

// TestToVPPRouteInterfaceOnlyNextHopResolvesIndex pins AC-4: an interface-only
// next-hop under an active vpp iface backend resolves to a Path carrying the
// backend's sw_if_index, replacing the old outright rejection.
func TestToVPPRouteInterfaceOnlyNextHopResolvesIndex(t *testing.T) {
	const dev = "vppresolve0" // unique name: iface.Resolve caches per logical name
	loadFakeVPPBackend(t, map[string]int{dev: 42})

	out, err := toVPPRoute(staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Action:   actionForward,
		NextHops: []nextHop{{Interface: dev, Weight: 1}},
	})
	if err != nil {
		t.Fatalf("toVPPRoute: %v", err)
	}
	if len(out.Paths) != 1 {
		t.Fatalf("Paths: got %d, want 1", len(out.Paths))
	}
	if out.Paths[0].SwIfIndex != 42 {
		t.Errorf("SwIfIndex: got %d, want 42 (from the fake iface backend)", out.Paths[0].SwIfIndex)
	}
	if out.Paths[0].NextHop.IsValid() {
		t.Errorf("NextHop: got %v, want unset for an interface-only path", out.Paths[0].NextHop)
	}
}

// TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors pins AC-5: an unknown
// interface errors and never yields a zero-index path.
func TestToVPPRouteInterfaceOnlyUnknownInterfaceErrors(t *testing.T) {
	loadFakeVPPBackend(t, map[string]int{"vppknown0": 5})

	out, err := toVPPRoute(staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Action:   actionForward,
		NextHops: []nextHop{{Interface: "vppghost0"}},
	})
	if err == nil {
		t.Fatalf("toVPPRoute: want error for an unknown interface, got paths %+v", out.Paths)
	}
	if len(out.Paths) != 0 {
		t.Errorf("Paths: got %d, want 0 (no index-0 path on error)", len(out.Paths))
	}
}

// TestToVPPRouteInterfaceOnlyZeroIndexRejected pins the fail-closed guard: a
// backend reporting index 0 (VPP local0) for the name must be rejected, never
// emitted as a path.
func TestToVPPRouteInterfaceOnlyZeroIndexRejected(t *testing.T) {
	const dev = "vppzero0"
	loadFakeVPPBackend(t, map[string]int{dev: 0})

	if _, err := toVPPRoute(staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Action:   actionForward,
		NextHops: []nextHop{{Interface: dev}},
	}); err == nil {
		t.Fatal("toVPPRoute: want error for a zero sw_if_index, got nil")
	}
}

// TestToVPPRouteRefusesResolveWhenIfaceBackendNotVPP pins AC-10 / R-7: with a
// non-vpp iface backend active, toVPPRoute refuses to resolve rather than
// emitting a kernel ifindex as a VPP sw_if_index.
func TestToVPPRouteRefusesResolveWhenIfaceBackendNotVPP(t *testing.T) {
	registerFakeIfaceBackends(t)
	// Even if the fake netlink backend WOULD resolve the name, the gate must
	// fire first: give it an entry to prove resolution is refused, not merely
	// failing for absence.
	fakeNetlinkBackend.set(map[string]int{"vppnl0": 99})
	if err := iface.LoadBackend("netlink"); err != nil {
		t.Fatalf("load fake netlink iface backend: %v", err)
	}
	t.Cleanup(func() { _ = iface.CloseBackend() })

	out, err := toVPPRoute(staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Action:   actionForward,
		NextHops: []nextHop{{Interface: "vppnl0"}},
	})
	if err == nil {
		t.Fatalf("toVPPRoute: want refusal under a non-vpp iface backend, got paths %+v", out.Paths)
	}
	if len(out.Paths) != 0 {
		t.Errorf("Paths: got %d, want 0 (kernel ifindex must not be emitted)", len(out.Paths))
	}
}

// TestToVPPRouteMixedAddressAndInterfaceNextHops covers ECMP mixing an address
// next-hop and an interface-only next-hop: both paths are produced, the address
// one carrying NextHop and the interface one carrying SwIfIndex.
func TestToVPPRouteMixedAddressAndInterfaceNextHops(t *testing.T) {
	const dev = "vppmixed0"
	loadFakeVPPBackend(t, map[string]int{dev: 13})

	out, err := toVPPRoute(staticRoute{
		Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		Action: actionForward,
		NextHops: []nextHop{
			{Address: netip.MustParseAddr("192.168.1.1"), Weight: 1},
			{Interface: dev, Weight: 2},
		},
	})
	if err != nil {
		t.Fatalf("toVPPRoute: %v", err)
	}
	if len(out.Paths) != 2 {
		t.Fatalf("Paths: got %d, want 2", len(out.Paths))
	}
	if !out.Paths[0].NextHop.IsValid() || out.Paths[0].SwIfIndex != 0 {
		t.Errorf("path[0]: got NextHop=%v SwIfIndex=%d, want the address path", out.Paths[0].NextHop, out.Paths[0].SwIfIndex)
	}
	if out.Paths[1].NextHop.IsValid() || out.Paths[1].SwIfIndex != 13 {
		t.Errorf("path[1]: got NextHop=%v SwIfIndex=%d, want the interface path (index 13)", out.Paths[1].NextHop, out.Paths[1].SwIfIndex)
	}
}
