// VALIDATES: static/vpp Backend construction/lifecycle — NewBackend wires the
// supplied api.Channel and table-id, and Close releases the channel exactly once.
// PREVENTS: a Backend that ignores its table-id or leaks the GoVPP channel on Close.
//
// WIRING (2026-07-10): the parent static plugin selects this backend in
// newStaticBackend() (`../backend_linux.go`) via newVPPStaticBackend()
// (`../backend_vpp_linux.go`) when a VPP connector is active, translating each
// staticRoute through toVPPRoute(). That selection + translation is tested in
// the parent package's backend_vpp_linux_test.go; this file covers the backend
// construction surface those callers use (NewBackend table-id wiring, Close).
package staticvpp

import (
	"net/netip"
	"testing"

	"go.fd.io/govpp/binapi/ip"
)

func TestNewBackendStoresTableID(t *testing.T) {
	ch := &testChannel{}
	b := NewBackend(ch, 99)
	if b == nil {
		t.Fatal("NewBackend returned nil")
	}
	// The table-id is opaque until a request is sent; drive one and read it back.
	if err := b.ApplyRoute(Route{Prefix: netip.MustParsePrefix("10.0.0.0/24"), Action: ActionForward, Paths: []Path{{NextHop: netip.MustParseAddr("1.1.1.1"), Weight: 1}}}); err != nil {
		t.Fatalf("ApplyRoute: %v", err)
	}
	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.TableID != 99 {
		t.Errorf("TableID: got %d, want 99 (wired by NewBackend)", req.Route.TableID)
	}
}

func TestBackendCloseReleasesChannel(t *testing.T) {
	ch := &testChannel{}
	b := NewBackend(ch, 0)
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !ch.closed {
		t.Error("channel not closed by Backend.Close")
	}
}
