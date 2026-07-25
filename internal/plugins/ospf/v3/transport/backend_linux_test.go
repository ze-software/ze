//go:build linux

// VALIDATES: spec-ospfv3-3-ipv6-transport -- the Linux backend resolves the
// kernel device and its link-local source through the shared iface resolver
// before binding a socket, and reports ErrNoLinkLocal when no fe80:: source is
// ready (IPv6 DAD). PREVENTS binding to the wrong device or sending from a
// non-link-local source (which would fail the peer's checksum).

package transport

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

func withFakeResolver(t *testing.T, bind func(string) (iface.Binding, error), addrs func(string) ([]iface.AddrInfo, error)) {
	t.Helper()
	oldBind, oldAddr := resolveIfaceBinding, resolveIfaceAddresses
	t.Cleanup(func() { resolveIfaceBinding, resolveIfaceAddresses = oldBind, oldAddr })
	resolveIfaceBinding = bind
	resolveIfaceAddresses = addrs
}

func TestOSPFv3ResolveInterfaceUsesIfaceResolver(t *testing.T) {
	withFakeResolver(t,
		func(string) (iface.Binding, error) { return iface.Binding{OsName: "ens3", Ifindex: 7}, nil },
		func(string) ([]iface.AddrInfo, error) {
			return []iface.AddrInfo{
				{Address: "2001:db8::1", Family: "ipv6"},              // global -- skipped
				{Address: "fe80::1", Family: "ipv6", LinkLocal: true}, // link-local -- selected
			}, nil
		},
	)

	resolved, err := resolveOSPFv3Interface("ospf0")
	if err != nil {
		t.Fatalf("resolveOSPFv3Interface: %v", err)
	}
	if resolved.ifi.Index != 7 || resolved.ifi.Name != "ens3" {
		t.Fatalf("resolved ifi = %+v, want index 7 name ens3", resolved.ifi)
	}
	if resolved.linkLocal != netip.MustParseAddr("fe80::1") {
		t.Fatalf("resolved link-local = %v, want fe80::1", resolved.linkLocal)
	}
}

func TestOSPFv3ResolveInterfaceNoLinkLocalIsPending(t *testing.T) {
	withFakeResolver(t,
		func(string) (iface.Binding, error) { return iface.Binding{OsName: "ens3", Ifindex: 7}, nil },
		func(string) ([]iface.AddrInfo, error) {
			return []iface.AddrInfo{{Address: "2001:db8::1", Family: "ipv6"}}, nil // no link-local yet (DAD)
		},
	)
	if _, err := resolveOSPFv3Interface("ospf0"); !errors.Is(err, ErrNoLinkLocal) {
		t.Fatalf("resolveOSPFv3Interface err = %v, want ErrNoLinkLocal", err)
	}
}

func TestOSPFv3ResolveInterfaceSkipsTentativeLinkLocal(t *testing.T) {
	// A DAD-incomplete (IFA_F_TENTATIVE) link-local is not a usable source; the resolver must
	// skip it and pick a ready one instead of binding to the tentative address.
	withFakeResolver(t,
		func(string) (iface.Binding, error) { return iface.Binding{OsName: "ens3", Ifindex: 7}, nil },
		func(string) ([]iface.AddrInfo, error) {
			return []iface.AddrInfo{
				{Address: "fe80::dad", Family: "ipv6", LinkLocal: true, Tentative: true}, // DAD-incomplete -- skipped
				{Address: "fe80::1", Family: "ipv6", LinkLocal: true},                    // ready -- selected
			}, nil
		},
	)
	resolved, err := resolveOSPFv3Interface("ospf0")
	if err != nil {
		t.Fatalf("resolveOSPFv3Interface: %v", err)
	}
	if resolved.linkLocal != netip.MustParseAddr("fe80::1") {
		t.Fatalf("resolved link-local = %v, want fe80::1 (the tentative one must be skipped)", resolved.linkLocal)
	}
}

func TestOSPFv3ResolveInterfaceFallsBackToTentative(t *testing.T) {
	// When the only link-local is still tentative, bind to it anyway: in bridged-container
	// environments IPv6 DAD never completes and the address stays tentative yet is usable, so a
	// tentative source beats never forming an adjacency.
	withFakeResolver(t,
		func(string) (iface.Binding, error) { return iface.Binding{OsName: "ens3", Ifindex: 7}, nil },
		func(string) ([]iface.AddrInfo, error) {
			return []iface.AddrInfo{{Address: "fe80::dad", Family: "ipv6", LinkLocal: true, Tentative: true}}, nil
		},
	)
	resolved, err := resolveOSPFv3Interface("ospf0")
	if err != nil {
		t.Fatalf("resolveOSPFv3Interface fell through instead of using the tentative link-local: %v", err)
	}
	if resolved.linkLocal != netip.MustParseAddr("fe80::dad") {
		t.Fatalf("resolved link-local = %v, want fe80::dad (fallback to the only link-local)", resolved.linkLocal)
	}
}
