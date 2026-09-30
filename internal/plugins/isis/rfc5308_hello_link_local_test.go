// Design: docs/architecture/isis/isis-12-ipv6.md -- the Hello TLV 232 address source.
//
// VALIDATES: RFC 5308 section 3 at the address source the engine hands the
// circuit (circuits.go interfaceIPv6LinkLocal -> circuit.Config.IPv6LinkLocal,
// which hello.go puts in TLV 232): the address is the link-local one assigned to
// the named interface, never a global address beside it and never another
// interface's.
// PREVENTS: a Hello TLV 232 carrying a global address, or the link-local
// address of a different interface.

package isis

import (
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// linkLocalBackend substitutes only the OS address source; the resolver's own
// link-local classification is real.
type linkLocalBackend struct{ iface.Backend }

func (*linkLocalBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	var addresses []iface.AddrInfo
	switch name {
	case "isis-ll-a":
		addresses = []iface.AddrInfo{
			{Address: "192.0.2.1", PrefixLength: 24, Family: "ipv4"},
			{Address: "2001:db8:a::1", PrefixLength: 64, Family: "ipv6"},
			{Address: "fe80::a:1", PrefixLength: 64, Family: "ipv6"},
		}
	case "isis-ll-b":
		addresses = []iface.AddrInfo{
			{Address: "fe80::b:2", PrefixLength: 64, Family: "ipv6"},
		}
	case "isis-ll-global":
		addresses = []iface.AddrInfo{
			{Address: "2001:db8:c::3", PrefixLength: 64, Family: "ipv6"},
		}
	default:
		return nil, fmt.Errorf("unknown test interface %s", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: name, State: "up", MTU: 1500, Addresses: addresses}, nil
}

func (*linkLocalBackend) Close() error { return nil }

var registerLinkLocalBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend("isis-link-local-test", func() (iface.Backend, error) {
		return &linkLocalBackend{}, nil
	})
})

// RFC requirement: RFC5308-3-1 positive -- the Hello TLV 232 address source
// (interfaceIPv6LinkLocal) returns the link-local address assigned to the
// interface sending the Hello: fe80::a:1 for isis-ll-a even though its global
// 2001:db8:a::1 is listed first, and fe80::b:2 for isis-ll-b.
// RFC requirement: RFC5308-3-1 negative -- an interface with only a global IPv6
// address yields no address at all (the global one is refused), and so does an
// interface whose IS-IS config does not enable ipv6-unicast.
func TestRFC5308HelloAddressIsTheInterfaceLinkLocal(t *testing.T) {
	useLinkLocalBackend(t)
	dual := []string{"ipv4-unicast", "ipv6-unicast"}

	for _, c := range []struct {
		name string
		want netip.Addr
	}{
		{"isis-ll-a", netip.MustParseAddr("fe80::a:1")},
		{"isis-ll-b", netip.MustParseAddr("fe80::b:2")},
	} {
		got := interfaceIPv6LinkLocal(InterfaceConfig{Name: c.name, AddressFamily: dual})
		if got != c.want {
			t.Fatalf("%s: Hello IPv6 address %v, want its link-local %v", c.name, got, c.want)
		}
	}

	if got := interfaceIPv6LinkLocal(InterfaceConfig{Name: "isis-ll-global", AddressFamily: dual}); got.IsValid() {
		t.Fatalf("interface with only a global IPv6 address: Hello IPv6 address %v, want none", got)
	}
	v4Only := InterfaceConfig{Name: "isis-ll-a", AddressFamily: []string{"ipv4-unicast"}}
	if got := interfaceIPv6LinkLocal(v4Only); got.IsValid() {
		t.Fatalf("ipv6-unicast not enabled: Hello IPv6 address %v, want none", got)
	}
}

// useLinkLocalBackend makes the test backend the active iface backend for the
// life of t and restores the previous one after.
func useLinkLocalBackend(t *testing.T) {
	t.Helper()
	if err := registerLinkLocalBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend("isis-link-local-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
}

// RFC requirement: RFC5308-3-2 positive -- the engine's own LSP (levelState ->
// origination, both levels) carries in TLV 232 exactly the non-link-local IPv6
// address assigned to its IPv6 circuits: 2001:db8:a::1.
// RFC requirement: RFC5308-3-2 negative -- neither link-local address the same
// circuits hold (fe80::a:1 on isis-ll-a, fe80::b:2, the only IPv6 address of
// isis-ll-b) appears in the LSP TLV 232.
func TestRFC5308LSPInterfaceAddressesAreNonLinkLocal(t *testing.T) {
	useLinkLocalBackend(t)
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2","interfaces":{"interface":{`+
		`"isis-ll-a":{"circuit-type":"point-to-point","level":"l1-l2","metric":"10","address-family":{"ipv4-unicast":{},"ipv6-unicast":{}}},`+
		`"isis-ll-b":{"circuit-type":"point-to-point","level":"l1-l2","metric":"10","address-family":{"ipv4-unicast":{},"ipv6-unicast":{}}}}}}}`)
	defer eng.shutdown()

	want := []netip.Addr{netip.MustParseAddr("2001:db8:a::1")}
	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		lsp := mustFrag0(t, eng, eng.cfg.SystemID, level)
		var got []netip.Addr
		for _, tl := range lsp.TLVs {
			if tl.Type != packet.TLVIPv6InterfaceAddress {
				continue
			}
			for off := 0; off+16 <= len(tl.Value); off += 16 {
				got = append(got, netip.AddrFrom16([16]byte(tl.Value[off:off+16])))
			}
		}
		for _, a := range got {
			if a.IsLinkLocalUnicast() {
				t.Fatalf("level %v LSP TLV 232 carries link-local %v: %v", level, a, got)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("level %v LSP TLV 232 = %v, want %v", level, got, want)
		}
	}
}
